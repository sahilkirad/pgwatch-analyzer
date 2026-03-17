package model

type IntentKind string

const (
	IntentUnknown      IntentKind = "unknown"
	IntentSummary      IntentKind = "summary"
	IntentSlowQueries  IntentKind = "slow_queries"
	IntentLocks        IntentKind = "locks"
	IntentReplication  IntentKind = "replication"
	IntentConnections  IntentKind = "connections"
	IntentScans        IntentKind = "scans"
	IntentExplain      IntentKind = "explain"
	IntentHealthStatus IntentKind = "health_status"
)

type Intent struct {
	Kind       IntentKind `json:"kind"`
	Confidence float64    `json:"confidence"`
	Reason     string     `json:"reason,omitempty"`
}