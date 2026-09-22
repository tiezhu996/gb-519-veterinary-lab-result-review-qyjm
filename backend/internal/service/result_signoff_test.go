package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blueship581/veterinary-lab-result-review/backend/internal/dto"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/model"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestResultSignoffPreservesVersionsAndRequiresIndependentReviewer(t *testing.T) {
	db := newSignoffTestDB(t)
	svc := NewResultSignoffService(repository.NewResultSignoffRepository(db), nil)
	ctx := context.Background()

	created, err := svc.Create(ctx, signoffInput("SIGNOFF-TEST-01"), "operator", "signoff-create-1")
	if err != nil {
		t.Fatalf("create signoff: %v", err)
	}
	if created.Version != 1 || created.PreparedBy != "operator" || len(created.Revisions) != 1 {
		t.Fatalf("unexpected initial signoff: %#v", created)
	}

	updatedInput := updateSignoffInput(created)
	updatedInput.Evidence = "PCR run sheet and control chart revision 2"
	updated, err := svc.Update(ctx, created.ID, updatedInput, "operator", "signoff-update-2")
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if updated.Version != 2 || len(updated.Revisions) != 2 || updated.Revisions[0].Evidence == updated.Revisions[1].Evidence {
		t.Fatalf("draft versions were not preserved: %#v", updated.Revisions)
	}

	peerReview, err := svc.Transition(ctx, updated.ID, dto.TransitionRequest{
		Status: "peer_review", ExpectedVersion: updated.Version, Reason: "result evidence complete",
	}, "operator", model.RoleOperator, "signoff-submit-3")
	if err != nil {
		t.Fatalf("submit peer review: %v", err)
	}
	if peerReview.Status != "peer_review" || peerReview.Version != 3 || len(peerReview.Revisions) != 3 {
		t.Fatalf("unexpected peer review version: %#v", peerReview)
	}

	decision := dto.TransitionRequest{Status: "signed", ExpectedVersion: peerReview.Version, Reason: "independent laboratory review passed"}
	if _, err := svc.Transition(ctx, peerReview.ID, decision, "operator", model.RoleOperator, "signoff-operator-denied"); !errors.Is(err, ErrReviewRequired) {
		t.Fatalf("operator signing must require reviewer role, got %v", err)
	}
	if _, err := svc.Transition(ctx, peerReview.ID, decision, "operator", model.RoleReviewer, "signoff-same-user-denied"); !errors.Is(err, ErrSeparationOfDuty) {
		t.Fatalf("same preparer and reviewer must be rejected, got %v", err)
	}

	signed, err := svc.Transition(ctx, peerReview.ID, decision, "reviewer", model.RoleReviewer, "signoff-sign-4")
	if err != nil {
		t.Fatalf("sign result: %v", err)
	}
	if signed.Status != "signed" || signed.Version != 4 || signed.ReviewedBy != "reviewer" || len(signed.Revisions) != 4 {
		t.Fatalf("unexpected signed result: %#v", signed)
	}
	for index, revision := range signed.Revisions {
		if revision.Evidence == "" || revision.Actor == "" || revision.RequestID == "" {
			t.Fatalf("revision %d lost attribution or evidence: %#v", index, revision)
		}
	}
	if signed.Revisions[0].RequestID != "signoff-create-1" || signed.Revisions[1].RequestID != "signoff-update-2" ||
		signed.Revisions[2].RequestID != "signoff-submit-3" || signed.Revisions[3].RequestID != "signoff-sign-4" {
		t.Fatalf("request ID chain is incomplete: %#v", signed.Revisions)
	}

	lateUpdate := updateSignoffInput(signed)
	if _, err := svc.Update(ctx, signed.ID, lateUpdate, "operator", "signoff-late-update"); !errors.Is(err, ErrLocked) {
		t.Fatalf("signed result must be immutable, got %v", err)
	}

	var auditCount int64
	if err := db.Model(&model.AuditLog{}).Where("entity_type = ? AND entity_id = ?", "ResultSignoff", signed.ID).Count(&auditCount).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if auditCount != 4 {
		t.Fatalf("expected 4 atomic audits, got %d", auditCount)
	}
}

func newSignoffTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AuditLog{}, &model.ResultSignoff{}, &model.ResultSignoffRevision{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func signoffInput(code string) dto.CreateResultSignoff {
	return dto.CreateResultSignoff{
		Code: code, Name: "PCR result signoff", Description: "controlled veterinary result",
		Facility: "Veterinary Lab 2", Owner: "Result desk", Category: "PCR", RiskLevel: "high",
		MetricValue: 99.8, MetricUnit: "percent", EffectiveAt: time.Now().UTC(),
		Evidence: "PCR run sheet and control chart revision 1", RelatedCode: "ASSAY-101",
	}
}

func updateSignoffInput(item model.ResultSignoff) dto.UpdateResultSignoff {
	return dto.UpdateResultSignoff{
		ExpectedVersion: item.Version, Name: item.Name, Description: item.Description,
		Facility: item.Facility, Owner: item.Owner, Category: item.Category, RiskLevel: item.RiskLevel,
		MetricValue: item.MetricValue, MetricUnit: item.MetricUnit, EffectiveAt: item.EffectiveAt,
		Evidence: item.Evidence, RelatedCode: item.RelatedCode,
	}
}
