package model

import "time"

type ExecutionContext struct {
	Prompt       string        `json:"prompt"`
	Since        time.Duration `json:"since"`
	DBFilter     string        `json:"db_filter,omitempty"`
	OutputFormat string        `json:"output_format"` // text | json
	SinkDSN      string        `json:"-"`
}

func DefaultExecutionContext(prompt string, sinkDSN string) ExecutionContext {
	return ExecutionContext{
		Prompt:       prompt,
		Since:        1 * time.Hour,
		DBFilter:     "",
		OutputFormat: "text",
		SinkDSN:      sinkDSN,
	}
}
