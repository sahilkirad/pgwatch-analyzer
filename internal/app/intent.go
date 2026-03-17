package app

import (
	"context"
	"pgwatchai/internal/model"
	"strings"
	"time"
)

type IntentClassifier interface {
	Classify(ctx context.Context, prompt string) ([]model.Intent, error)
}

type IntentDetector struct {
	classifier IntentClassifier
	timeout    time.Duration
}

func NewIntentDetector() *IntentDetector {
	return &IntentDetector{
		// Implemented in a later step (LLM-backed classifier).
		classifier: NewLLMIntentClassifier(),
		timeout:    8 * time.Second,
	}
}

func (d *IntentDetector) Detect(prompt string) []model.Intent {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return []model.Intent{{Kind: model.IntentUnknown, Confidence: 1.0, Reason: "empty prompt"}}
	}

	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()

	intents, err := d.classifier.Classify(ctx, prompt)
	if err != nil || len(intents) == 0 {
		return []model.Intent{{
			Kind:       model.IntentUnknown,
			Confidence: 1.0,
			Reason:     "llm classification failed or returned empty",
		}}
	}

	return normalizeIntents(intents)
}

func normalizeIntents(in []model.Intent) []model.Intent {
	seen := make(map[model.IntentKind]bool)
	out := make([]model.Intent, 0, len(in))

	for _, it := range in {
		if it.Kind == "" {
			continue
		}
		if it.Confidence < 0 {
			it.Confidence = 0
		}
		if it.Confidence > 1 {
			it.Confidence = 1
		}
		if seen[it.Kind] {
			continue
		}
		seen[it.Kind] = true
		out = append(out, it)
	}

	if len(out) == 0 {
		return []model.Intent{{
			Kind:       model.IntentUnknown,
			Confidence: 1.0,
			Reason:     "no valid intents after normalization",
		}}
	}
	return out
}
