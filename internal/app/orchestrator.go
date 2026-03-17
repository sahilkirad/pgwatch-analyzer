package app

import (
	"fmt"
	"strings"

	"pgwatchai/internal/model"
)

type Orchestrator struct {
	intentDetector *IntentDetector
	router         *Router
}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		intentDetector: NewIntentDetector(),
		router:         NewRouter(),
	}
}

func (o *Orchestrator) Handle(prompt, sinkDSN string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf("empty prompt")
	}
	if strings.TrimSpace(sinkDSN) == "" {
		return "", fmt.Errorf("empty sink dsn")
	}

	execCtx := model.DefaultExecutionContext(prompt, sinkDSN)
	intents := o.intentDetector.Detect(prompt)

	response, err := o.router.Route(execCtx, intents)
	if err != nil {
		return "", err
	}
	return response, nil
}
