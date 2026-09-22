package model

import "time"

// ResultSignoff models 结果签发 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type ResultSignoff struct {
	BaseModel
	Facility     string                  `json:"facility" gorm:"size:120;index"`
	Owner        string                  `json:"owner" gorm:"size:120;index"`
	Category     string                  `json:"category" gorm:"size:80;index"`
	RiskLevel    string                  `json:"riskLevel" gorm:"size:32;index"`
	MetricValue  float64                 `json:"metricValue"`
	MetricUnit   string                  `json:"metricUnit" gorm:"size:24"`
	EffectiveAt  time.Time               `json:"effectiveAt"`
	Evidence     string                  `json:"evidence" gorm:"size:2000"`
	RelatedCode  string                  `json:"relatedCode" gorm:"size:64;index"`
	PreparedBy   string                  `json:"preparedBy" gorm:"size:80;index"`
	ReviewedBy   string                  `json:"reviewedBy" gorm:"size:80;index"`
	ReviewReason string                  `json:"reviewReason" gorm:"size:500"`
	Revisions    []ResultSignoffRevision `json:"revisions,omitempty" gorm:"foreignKey:ResultSignoffID"`
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
