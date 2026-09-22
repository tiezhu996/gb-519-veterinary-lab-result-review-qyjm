package model

import (
	"encoding/json"
	"time"
)

// ResultSignoff models 结果签发 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type ResultSignoff struct {
	BaseModel
	Facility     string    `json:"facility" gorm:"size:120;index"`
	Owner        string    `json:"owner" gorm:"size:120;index"`
	Category     string    `json:"category" gorm:"size:80;index"`
	RiskLevel    string    `json:"riskLevel" gorm:"size:32;index"`
	MetricValue  float64   `json:"metricValue"`
	MetricUnit   string    `json:"metricUnit" gorm:"size:24"`
	EffectiveAt  time.Time `json:"effectiveAt"`
	Evidence     string    `json:"evidence" gorm:"size:2000"`
	RelatedCode  string    `json:"relatedCode" gorm:"size:64;index"`
	PreparedBy   string    `json:"preparedBy" gorm:"size:80;index"`
	ReviewedBy   string    `json:"reviewedBy" gorm:"size:80;index"`
	ReviewReason string    `json:"reviewReason" gorm:"size:500"`
	// FirstReview fields snapshot the first confirmation for high/critical-risk
	// specimens. They are written exactly once when the record enters
	// second_review and must never be overwritten by later failed attempts.
	RelatedRiskLevel  string     `json:"relatedRiskLevel" gorm:"size:32;index"`
	FirstReviewBy     string     `json:"firstReviewBy" gorm:"size:80;index"`
	FirstReviewReason string     `json:"firstReviewReason" gorm:"size:500"`
	FirstReviewAt     *time.Time `json:"firstReviewAt"`
	// SpecimenRiskLevel is resolved at read time from the linked specimen so
	// the UI can tell whether peer_review requires two confirmations before the
	// first reviewer snapshot exists. It is not persisted.
	SpecimenRiskLevel string                  `json:"specimenRiskLevel" gorm:"-"`
	Revisions         []ResultSignoffRevision `json:"revisions,omitempty" gorm:"foreignKey:ResultSignoffID"`
}

// Risk levels mirror the oneof values shared by every risk-bearing aggregate.
const (
	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskHigh     = "high"
	RiskCritical = "critical"
)

// IsHighRisk reports whether a specimen risk level forces two-level review.
func IsHighRisk(level string) bool {
	return level == RiskHigh || level == RiskCritical
}

// PendingTodo reports who must act next, computed from the current state and
// the risk tier snapshot of the linked specimen. It is derived on read and is
// not persisted.
func (item ResultSignoff) PendingTodo() string {
	switch item.Status {
	case ResultSignoffInitialStatus:
		return "待制单人提交复核"
	case "peer_review":
		risk := item.RelatedRiskLevel
		if risk == "" {
			risk = item.SpecimenRiskLevel
		}
		if IsHighRisk(risk) {
			return "待首名复核员确认"
		}
		return "待复核员签发"
	case "second_review":
		return "待第二名不同复核员二级签发"
	case "signed":
		return "已签发"
	case "rejected":
		return "已退回"
	default:
		return ""
	}
}

// MarshalJSON renders the derived todo alongside persisted columns so the
// record page shows the next action without duplicating state-machine logic.
func (item ResultSignoff) MarshalJSON() ([]byte, error) {
	type alias ResultSignoff
	return json.Marshal(struct {
		alias
		PendingTodo string `json:"pendingTodo"`
	}{alias: alias(item), PendingTodo: item.PendingTodo()})
}

func (item *ResultSignoff) GetBase() *BaseModel { return &item.BaseModel }

func (item ResultSignoff) TableName() string { return "result_signoffs" }

var ResultSignoffInitialStatus = "draft"

// ResultSignoffRevision is append-only evidence for every signoff version.
type ResultSignoffRevision struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	ResultSignoffID uint      `json:"resultSignoffId" gorm:"not null;index;uniqueIndex:idx_signoff_revision_version,priority:1"`
	Version         uint      `json:"version" gorm:"not null;uniqueIndex:idx_signoff_revision_version,priority:2"`
	Status          string    `json:"status" gorm:"size:40;not null"`
	Evidence        string    `json:"evidence" gorm:"size:2000"`
	Actor           string    `json:"actor" gorm:"size:80;not null;index"`
	RequestID       string    `json:"requestId" gorm:"size:64;not null;index"`
	Action          string    `json:"action" gorm:"size:40;not null"`
	Reason          string    `json:"reason" gorm:"size:500"`
	CreatedAt       time.Time `json:"createdAt" gorm:"index"`
}
