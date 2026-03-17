package model

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Finding struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Severity   Severity       `json:"severity"`
	Category   IntentKind     `json:"category"`
	Summary    string         `json:"summary"`
	Evidence   map[string]any `json:"evidence,omitempty"`
	DBName     string         `json:"db_name,omitempty"`
	Suggestion string         `json:"suggestion,omitempty"`
	Confidence float64        `json:"confidence,omitempty"`
}
