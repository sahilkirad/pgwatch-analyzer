package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"pgwatchai/internal/model"
)

type geminiIntentClassifier struct {
	apiKey string
	model  string
	client *http.Client
}

func NewLLMIntentClassifier() IntentClassifier {
	return &geminiIntentClassifier{
		apiKey: strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		model:  defaultIfEmpty(strings.TrimSpace(os.Getenv("GEMINI_MODEL")), "gemini-1.5-flash"),
		client: &http.Client{Timeout: 12 * time.Second},
	}
}

func (c *geminiIntentClassifier) Classify(ctx context.Context, prompt string) ([]model.Intent, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("empty prompt")
	}
	if c.apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY is not set")
	}

	reqBody := geminiGenerateRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{
						Text: buildClassificationPrompt(prompt),
					},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:      0,
			ResponseMimeType: "application/json",
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.model,
		c.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini returned %d: %s", resp.StatusCode, string(respBody))
	}

	var gr geminiGenerateResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return nil, fmt.Errorf("unmarshal gemini response: %w", err)
	}

	rawJSON := extractText(gr)
	if rawJSON == "" {
		return nil, errors.New("gemini response had no text")
	}
	rawJSON = stripCodeFence(rawJSON)

	var parsed intentPayload
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		return nil, fmt.Errorf("invalid classifier JSON: %w; raw=%s", err, rawJSON)
	}

	intents := make([]model.Intent, 0, len(parsed.Intents))
	for _, in := range parsed.Intents {
		kind := model.IntentKind(strings.TrimSpace(in.Kind))
		if !isSupportedIntent(kind) {
			continue
		}
		conf := in.Confidence
		if conf < 0 {
			conf = 0
		}
		if conf > 1 {
			conf = 1
		}
		reason := strings.TrimSpace(in.Reason)
		if reason == "" {
			reason = "classifier returned no reason"
		}
		intents = append(intents, model.Intent{
			Kind:       kind,
			Confidence: conf,
			Reason:     reason,
		})

	}

	if len(intents) == 0 {
		return nil, errors.New("no valid intents parsed from gemini output")
	}

	return intents, nil
}

func buildClassificationPrompt(userPrompt string) string {
	return `Classify the PostgreSQL monitoring question into one or more intents.
Allowed intents:
- summary
- slow_queries
- locks
- replication
- connections
- scans
- explain
- health_status
- unknown

Rules:
- Return only JSON (no markdown, no prose).
- Output schema:
{"intents":[{"kind":"slow_queries","confidence":0.0,"reason":"short reason"}]}
- confidence must be between 0 and 1.
- reason is mandatory and must be 15-30 words, make sure it is explainable and answers user's query accurately, user should get insights based on the query asked.
- Include unknown only when you cannot infer a valid intent.

User prompt:` + userPrompt
}

func isSupportedIntent(k model.IntentKind) bool {
	switch k {
	case model.IntentSummary,
		model.IntentSlowQueries,
		model.IntentLocks,
		model.IntentReplication,
		model.IntentConnections,
		model.IntentScans,
		model.IntentExplain,
		model.IntentHealthStatus,
		model.IntentUnknown:
		return true
	default:
		return false
	}
}

func extractText(r geminiGenerateResponse) string {
	for _, cand := range r.Candidates {
		for _, p := range cand.Content.Parts {
			if strings.TrimSpace(p.Text) != "" {
				return p.Text
			}
		}
	}
	return ""
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func defaultIfEmpty(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

type intentPayload struct {
	Intents []intentItem `json:"intents"`
}

type intentItem struct {
	Kind       string  `json:"kind"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type geminiGenerateRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}
