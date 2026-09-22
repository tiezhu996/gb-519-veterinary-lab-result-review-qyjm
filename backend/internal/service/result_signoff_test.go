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
	seedSpecimens(t, db)
	svc := NewResultSignoffService(repository.NewResultSignoffRepository(db), repository.NewSpecimenRepository(db), nil)
	ctx := context.Background()

	input := signoffInput("SIGNOFF-TEST-01")
	input.RelatedCode = "SPEC-MEDIUM"
	input.RiskLevel = "medium"
	created, err := svc.Create(ctx, input, "operator", "signoff-create-1")
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
	if signed.FirstReviewBy != "" || signed.PendingTodo() != "已签发" {
		t.Fatalf("ordinary risk must not carry first-review data: %#v", signed)
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

func TestHighRiskSpecimenRequiresTwoDifferentReviewers(t *testing.T) {
	db := newSignoffTestDB(t)
	seedSpecimens(t, db)
	svc := NewResultSignoffService(repository.NewResultSignoffRepository(db), repository.NewSpecimenRepository(db), nil)
	ctx := context.Background()

	input := signoffInput("SIGNOFF-TEST-HIGH")
	input.RelatedCode = "SPEC-HIGH"
	input.RiskLevel = "high"
	created, err := svc.Create(ctx, input, "operator", "high-create")
	if err != nil {
		t.Fatalf("create high-risk signoff: %v", err)
	}
	peerReview, err := svc.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "peer_review", ExpectedVersion: created.Version, Reason: "evidence complete",
	}, "operator", model.RoleOperator, "high-submit")
	if err != nil {
		t.Fatalf("submit high-risk signoff: %v", err)
	}

	confirm := dto.TransitionRequest{Status: "second_review", ExpectedVersion: peerReview.Version, Reason: "first reviewer confirmed"}

	// Operator role cannot confirm the first review.
	if _, err := svc.Transition(ctx, peerReview.ID, confirm, "operator", model.RoleOperator, "high-role-denied"); !errors.Is(err, ErrReviewRequired) {
		t.Fatalf("non-reviewer first confirmation must be rejected, got %v", err)
	}
	// The preparer cannot confirm even when holding a reviewer role.
	if _, err := svc.Transition(ctx, peerReview.ID, confirm, "operator", model.RoleReviewer, "high-self-denied"); !errors.Is(err, ErrSeparationOfDuty) {
		t.Fatalf("preparer first confirmation must be rejected, got %v", err)
	}

	secondReview, err := svc.Transition(ctx, peerReview.ID, confirm, "reviewer", model.RoleReviewer, "high-confirm-v3")
	if err != nil {
		t.Fatalf("first reviewer confirmation: %v", err)
	}
	if secondReview.Status != "second_review" || secondReview.Version != 3 {
		t.Fatalf("expected second_review v3, got status=%s version=%d", secondReview.Status, secondReview.Version)
	}
	if secondReview.FirstReviewBy != "reviewer" || secondReview.FirstReviewReason != "first reviewer confirmed" ||
		secondReview.FirstReviewAt == nil || secondReview.RelatedRiskLevel != "high" {
		t.Fatalf("first confirmation was not recorded: %#v", secondReview)
	}
	if secondReview.PendingTodo() != "待第二名不同复核员二级签发" {
		t.Fatalf("unexpected pending todo: %q", secondReview.PendingTodo())
	}

	sign := dto.TransitionRequest{Status: "signed", ExpectedVersion: secondReview.Version, Reason: "second reviewer signed"}

	// Repeating the first confirmation must not progress or overwrite it.
	duplicate := dto.TransitionRequest{Status: "second_review", ExpectedVersion: secondReview.Version, Reason: "duplicate confirm"}
	if _, err := svc.Transition(ctx, secondReview.ID, duplicate, "admin", model.RoleAdmin, "high-duplicate-denied"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("second_review must not accept another first confirmation, got %v", err)
	}
	// The same reviewer cannot perform the second-level signoff.
	if _, err := svc.Transition(ctx, secondReview.ID, sign, "reviewer", model.RoleReviewer, "high-same-reviewer-denied"); !errors.Is(err, ErrDuplicateReviewer) {
		t.Fatalf("same second reviewer must be rejected, got %v", err)
	}
	// A stale expectedVersion must be rejected without touching the record.
	stale := sign
	stale.ExpectedVersion = 1
	if _, err := svc.Transition(ctx, secondReview.ID, stale, "admin", model.RoleAdmin, "high-stale-denied"); !errors.Is(err, repository.ErrVersionConflict) {
		t.Fatalf("stale version must conflict, got %v", err)
	}
	// The first confirmation survives every failed attempt.
	reloaded, err := svc.Get(ctx, secondReview.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != "second_review" || reloaded.Version != 3 || reloaded.FirstReviewBy != "reviewer" ||
		reloaded.FirstReviewReason != "first reviewer confirmed" {
		t.Fatalf("failed attempts changed state or overwrote first confirmation: %#v", reloaded)
	}

	signed, err := svc.Transition(ctx, secondReview.ID, sign, "admin", model.RoleAdmin, "high-sign-v4")
	if err != nil {
		t.Fatalf("second reviewer signoff: %v", err)
	}
	if signed.Status != "signed" || signed.Version != 4 || signed.ReviewedBy != "admin" ||
		signed.FirstReviewBy != "reviewer" || len(signed.Revisions) != 4 {
		t.Fatalf("unexpected two-level signed result: %#v", signed)
	}
}

func TestSignoffReviewRejectsMissingSpecimenAndRiskMismatch(t *testing.T) {
	db := newSignoffTestDB(t)
	seedSpecimens(t, db)
	svc := NewResultSignoffService(repository.NewResultSignoffRepository(db), repository.NewSpecimenRepository(db), nil)
	ctx := context.Background()

	missing := signoffInput("SIGNOFF-TEST-MISSING")
	missing.RelatedCode = "SPEC-DOES-NOT-EXIST"
	created, err := svc.Create(ctx, missing, "operator", "missing-create")
	if err != nil {
		t.Fatalf("create signoff without existing specimen is allowed at draft stage: %v", err)
	}
	peerReview, err := svc.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "peer_review", ExpectedVersion: created.Version, Reason: "submit",
	}, "operator", model.RoleOperator, "missing-submit")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Transition(ctx, peerReview.ID, dto.TransitionRequest{
		Status: "signed", ExpectedVersion: peerReview.Version, Reason: "try sign without specimen",
	}, "reviewer", model.RoleReviewer, "missing-sign-denied"); !errors.Is(err, ErrRelatedSpecimenMissing) {
		t.Fatalf("missing related specimen must reject signoff, got %v", err)
	}

	// A high-risk specimen cannot bypass second review directly from peer_review.
	high := signoffInput("SIGNOFF-TEST-BYPASS")
	high.RelatedCode = "SPEC-HIGH"
	highDraft, err := svc.Create(ctx, high, "operator", "bypass-create")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	highPeer, err := svc.Transition(ctx, highDraft.ID, dto.TransitionRequest{
		Status: "peer_review", ExpectedVersion: highDraft.Version, Reason: "submit",
	}, "operator", model.RoleOperator, "bypass-submit")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Transition(ctx, highPeer.ID, dto.TransitionRequest{
		Status: "signed", ExpectedVersion: highPeer.Version, Reason: "skip second review",
	}, "reviewer", model.RoleReviewer, "bypass-denied"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("high-risk single signoff must be rejected, got %v", err)
	}

	// A medium-risk specimen cannot be pushed into second_review.
	medium := signoffInput("SIGNOFF-TEST-FORCE-SECOND")
	medium.RelatedCode = "SPEC-MEDIUM"
	mediumDraft, err := svc.Create(ctx, medium, "operator", "force-create")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	mediumPeer, err := svc.Transition(ctx, mediumDraft.ID, dto.TransitionRequest{
		Status: "peer_review", ExpectedVersion: mediumDraft.Version, Reason: "submit",
	}, "operator", model.RoleOperator, "force-submit")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Transition(ctx, mediumPeer.ID, dto.TransitionRequest{
		Status: "second_review", ExpectedVersion: mediumPeer.Version, Reason: "force second review",
	}, "reviewer", model.RoleReviewer, "force-denied"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("ordinary risk must not enter second review, got %v", err)
	}
}

func newSignoffTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AuditLog{}, &model.Specimen{}, &model.ResultSignoff{}, &model.ResultSignoffRevision{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func seedSpecimens(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	specimens := []model.Specimen{
		{BaseModel: model.BaseModel{Code: "SPEC-MEDIUM", Name: "普通风险样本", Status: "testing", Version: 1},
			RiskLevel: "medium", RelatedCode: "REL-M"},
		{BaseModel: model.BaseModel{Code: "SPEC-HIGH", Name: "高风险样本", Status: "testing", Version: 1},
			RiskLevel: "high", RelatedCode: "REL-H"},
		{BaseModel: model.BaseModel{Code: "SPEC-CRITICAL", Name: "极高风险样本", Status: "hold", Version: 1},
			RiskLevel: "critical", RelatedCode: "REL-C"},
	}
	for index := range specimens {
		specimens[index].CreatedAt = now
		specimens[index].UpdatedAt = now
	}
	if err := db.Create(&specimens).Error; err != nil {
		t.Fatalf("seed specimens: %v", err)
	}
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
