package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"pgwatchai/internal/analysis"
	"pgwatchai/internal/model"
)

type IntentHandler func(ctx model.ExecutionContext, intent model.Intent) (string, error)

type Router struct {
	handlers           map[model.IntentKind]IntentHandler
	minConfidenceToRun float64
	engine             *analysis.Engine
}

func NewRouter() *Router {
	r := &Router{
		handlers:           make(map[model.IntentKind]IntentHandler),
		minConfidenceToRun: 0.35,
		engine:             analysis.NewDefaultEngine(),
	}

	r.handlers[model.IntentSlowQueries] = r.handleViaEngine
	r.handlers[model.IntentConnections] = r.handleViaEngine
	r.handlers[model.IntentLocks] = r.handleViaEngine
	r.handlers[model.IntentReplication] = r.handleViaEngine

	r.handlers[model.IntentSummary] = handleSummary
	r.handlers[model.IntentHealthStatus] = handleHealthStatus
	r.handlers[model.IntentScans] = handleScans
	r.handlers[model.IntentExplain] = handleExplain
	r.handlers[model.IntentUnknown] = handleUnknown

	return r
}
func buildReasoningHeader(intents []model.Intent) string {
	lines := make([]string, 0, len(intents)+1)
	lines = append(lines, "Intent reasoning:")
	for _, in := range intents {
		reason := in.Reason
		if strings.TrimSpace(reason) == "" {
			reason = "no reason provided"
		}
		lines = append(lines, fmt.Sprintf("- %s (confidence=%.2f): %s", in.Kind, in.Confidence, reason))
	}
	return strings.Join(lines, "\n")
}

func (r *Router) Route(execCtx model.ExecutionContext, intents []model.Intent) (string, error) {
	if len(intents) == 0 {
		return handleUnknown(execCtx, model.Intent{Kind: model.IntentUnknown})
	}

	sort.SliceStable(intents, func(i, j int) bool {
		return intents[i].Confidence > intents[j].Confidence
	})

	seen := make(map[model.IntentKind]bool)
	parts := make([]string, 0, len(intents))
	reasoning := buildReasoningHeader(intents)

	for _, in := range intents {
		if seen[in.Kind] {
			continue
		}
		seen[in.Kind] = true

		if in.Confidence < r.minConfidenceToRun && in.Kind != model.IntentUnknown {
			continue
		}

		h, ok := r.handlers[in.Kind]
		if !ok {
			h = handleUnknown
		}

		out, err := h(execCtx, in)
		if err != nil {
			return "", fmt.Errorf("intent %q failed: %w", in.Kind, err)
		}
		if strings.TrimSpace(out) != "" {
			parts = append(parts, out)
		}
	}

	if len(parts) == 0 {
		unknownMsg, _ := handleUnknown(execCtx, model.Intent{Kind: model.IntentUnknown, Confidence: 1.0})
		return reasoning + "\n\n" + unknownMsg, nil
	}
	return reasoning + "\n\n" + strings.Join(parts, "\n\n"), nil

}

func (r *Router) handleViaEngine(exec model.ExecutionContext, intent model.Intent) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	findings, err := r.engine.Run(ctx, exec, []model.Intent{intent})
	if err != nil {
		return "", err
	}
	if len(findings) == 0 {
		return "No findings generated for this category.", nil
	}

	lines := make([]string, 0, len(findings))
	for _, f := range findings {
		lines = append(lines, fmt.Sprintf("[%s] %s: %s", f.Severity, f.Title, f.Summary))
	}
	return strings.Join(lines, "\n"), nil
}

func handleSummary(_ model.ExecutionContext, _ model.Intent) (string, error) {
	return "Summary intent recognized. Add summary analyzer to aggregate multi-category findings.", nil
}
func handleHealthStatus(_ model.ExecutionContext, _ model.Intent) (string, error) {
	return "Health-status intent recognized. Add health analyzer for availability/error trends.", nil
}
func handleScans(_ model.ExecutionContext, _ model.Intent) (string, error) {
	return "Scans intent recognized. Add scans analyzer backed by sink scan metrics.", nil
}
func handleExplain(_ model.ExecutionContext, _ model.Intent) (string, error) {
	return "Explain intent recognized. Add responder module to narrate findings.", nil
}
func handleUnknown(_ model.ExecutionContext, _ model.Intent) (string, error) {
	return "Could not confidently map your prompt. Ask about slow queries, locks, replication, or connections.", nil
}
