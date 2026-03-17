package model

type RecommendationPriority string

const (
	PriorityLow    RecommendationPriority = "low"
	PriorityMedium RecommendationPriority = "medium"
	PriorityHigh   RecommendationPriority = "high"
)

type Recommendation struct {
	ID         string                 `json:"id"`
	FindingID  string                 `json:"finding_id"`
	Title      string                 `json:"title"`
	Priority   RecommendationPriority `json:"priority"`
	Action     string                 `json:"action"`
	Reason     string                 `json:"reason"`
	Risk       string                 `json:"risk,omitempty"`
	Impact     string                 `json:"impact,omitempty"`
	References []string               `json:"references,omitempty"`
}
