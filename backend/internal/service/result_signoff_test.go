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
	input.RiskLevel = "medium"
	input.RelatedCode = "S-MEDIUM"
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
	if signed.SpecimenRiskLevel != "medium" {
		t.Fatalf("signed normal-risk signoff must snapshot specimen risk, got %q", signed.SpecimenRiskLevel)
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

func TestHighRiskSpecimenRequiresTwoDistinctReviewers(t *testing.T) {
	db := newSignoffTestDB(t)
	seedSpecimens(t, db)
	svc := NewResultSignoffService(repository.NewResultSignoffRepository(db), repository.NewSpecimenRepository(db), nil)
	ctx := context.Background()

	created := mustCreateSignoff(t, svc, "SIGNOFF-HIGH-01", "S-HIGH", "high", "operator")
	peerReview := mustSubmit(t, svc, created, "operator")

	first := dto.TransitionRequest{Status: "signed", ExpectedVersion: peerReview.Version, Reason: "first reviewer confirms high risk"}
	firstConfirmed, err := svc.Transition(ctx, peerReview.ID, first, "reviewer", model.RoleReviewer, "high-first-review")
	if err != nil {
		t.Fatalf("first reviewer confirmation: %v", err)
	}
	// High risk enters 待二级复核 instead of being signed directly.
	if firstConfirmed.Status != "secondary_review" {
		t.Fatalf("expected secondary_review, got %s", firstConfirmed.Status)
	}
	if firstConfirmed.FirstReviewedBy != "reviewer" || firstConfirmed.FirstReviewReason == "" || firstConfirmed.FirstReviewedAt == nil {
		t.Fatalf("first review conclusion/time not recorded: %#v", firstConfirmed)
	}
	if firstConfirmed.ReviewedBy != "" {
		t.Fatalf("final reviewer must stay empty while awaiting second review, got %q", firstConfirmed.ReviewedBy)
	}
	if firstConfirmed.SpecimenRiskLevel != "high" {
		t.Fatalf("expected high specimen risk snapshot, got %q", firstConfirmed.SpecimenRiskLevel)
	}
	if firstConfirmed.Version != peerReview.Version+1 || len(firstConfirmed.Revisions) != 3 {
		t.Fatalf("expected one new revision for first review: %#v", firstConfirmed.Revisions)
	}

	stale := dto.TransitionRequest{Status: "signed", ExpectedVersion: peerReview.Version, Reason: "stale version attempt"}
	if _, err := svc.Transition(ctx, peerReview.ID, stale, "admin", model.RoleAdmin, "high-stale"); !errors.Is(err, repository.ErrVersionConflict) {
		t.Fatalf("stale expected version must be rejected, got %v", err)
	}

	sameReviewer := dto.TransitionRequest{Status: "signed", ExpectedVersion: firstConfirmed.Version, Reason: "same reviewer must fail"}
	if _, err := svc.Transition(ctx, firstConfirmed.ID, sameReviewer, "reviewer", model.RoleReviewer, "high-same-reviewer"); !errors.Is(err, ErrDistinctReviewer) {
		t.Fatalf("same user repeating confirmation must be rejected, got %v", err)
	}

	operatorAttempt := dto.TransitionRequest{Status: "signed", ExpectedVersion: firstConfirmed.Version, Reason: "operator cannot finalize"}
	if _, err := svc.Transition(ctx, firstConfirmed.ID, operatorAttempt, "operator", model.RoleOperator, "high-operator"); !errors.Is(err, ErrReviewRequired) {
		t.Fatalf("non-reviewer second confirmation must be rejected, got %v", err)
	}

	// Failed attempts must not advance state or overwrite the first conclusion.
	reloaded, err := svc.Get(ctx, firstConfirmed.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != "secondary_review" || reloaded.FirstReviewedBy != "reviewer" || reloaded.ReviewedBy != "" {
		t.Fatalf("first confirmation was overwritten by failed attempts: %#v", reloaded)
	}
	if len(reloaded.Revisions) != 3 {
		t.Fatalf("failed attempts must not append revisions, got %d", len(reloaded.Revisions))
	}

	final := dto.TransitionRequest{Status: "signed", ExpectedVersion: firstConfirmed.Version, Reason: "second reviewer finalizes issuance"}
	signed, err := svc.Transition(ctx, firstConfirmed.ID, final, "admin", model.RoleAdmin, "high-second-review")
	if err != nil {
		t.Fatalf("second distinct reviewer sign: %v", err)
	}
	if signed.Status != "signed" || signed.ReviewedBy != "admin" || signed.FirstReviewedBy != "reviewer" {
		t.Fatalf("unexpected final signoff: %#v", signed)
	}
	if signed.Version != firstConfirmed.Version+1 || len(signed.Revisions) != 4 {
		t.Fatalf("expected final revision v4, got %#v", signed.Revisions)
	}

	if _, err := svc.Transition(ctx, signed.ID, dto.TransitionRequest{Status: "signed", ExpectedVersion: signed.Version}, "reviewer", model.RoleReviewer, "signed-closed"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("signed must be terminal, got %v", err)
	}
}

func TestCriticalSpecimenTakesSecondaryReviewPath(t *testing.T) {
	db := newSignoffTestDB(t)
	seedSpecimens(t, db)
	svc := NewResultSignoffService(repository.NewResultSignoffRepository(db), repository.NewSpecimenRepository(db), nil)
	ctx := context.Background()

	created := mustCreateSignoff(t, svc, "SIGNOFF-CRIT-01", "S-CRITICAL", "critical", "operator")
	peerReview := mustSubmit(t, svc, created, "operator")

	first, err := svc.Transition(ctx, peerReview.ID, dto.TransitionRequest{
		Status: "signed", ExpectedVersion: peerReview.Version, Reason: "critical first confirmation",
	}, "reviewer", model.RoleReviewer, "crit-first")
	if err != nil {
		t.Fatalf("critical first review: %v", err)
	}
	if first.Status != "secondary_review" || first.SpecimenRiskLevel != "critical" {
		t.Fatalf("critical signoff must await second review: %#v", first)
	}

	// A reviewer may reject during the second-level stage; rejection stays unchanged.
	rejected, err := svc.Transition(ctx, first.ID, dto.TransitionRequest{
		Status: "rejected", ExpectedVersion: first.Version, Reason: "second reviewer rejects",
	}, "admin", model.RoleAdmin, "crit-reject")
	if err != nil {
		t.Fatalf("reject from secondary review: %v", err)
	}
	if rejected.Status != "rejected" || rejected.ReviewedBy != "admin" {
		t.Fatalf("unexpected rejection: %#v", rejected)
	}
}

func TestMissingLinkedSpecimenRejectsFirstReview(t *testing.T) {
	db := newSignoffTestDB(t)
	seedSpecimens(t, db)
	svc := NewResultSignoffService(repository.NewResultSignoffRepository(db), repository.NewSpecimenRepository(db), nil)
	ctx := context.Background()

	created := mustCreateSignoff(t, svc, "SIGNOFF-MISSING-01", "S-DOES-NOT-EXIST", "high", "operator")
	peerReview := mustSubmit(t, svc, created, "operator")

	before := peerReview.Status
	_, err := svc.Transition(ctx, peerReview.ID, dto.TransitionRequest{
		Status: "signed", ExpectedVersion: peerReview.Version, Reason: "link broken",
	}, "reviewer", model.RoleReviewer, "missing-link")
	if !errors.Is(err, ErrLinkedSpecimen) {
		t.Fatalf("missing linked specimen must reject review, got %v", err)
	}
	reloaded, err := svc.Get(ctx, peerReview.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != before || reloaded.FirstReviewedBy != "" || reloaded.Version != peerReview.Version {
		t.Fatalf("failed review must not advance state, got %#v", reloaded)
	}
	if !reloaded.LinkedSpecimenMissing || reloaded.LinkedSpecimenRiskLevel != "" {
		t.Fatalf("read model must flag missing linked specimen: %#v", reloaded)
	}
}

func TestLinkedSpecimenRiskIsEnrichedOnRead(t *testing.T) {
	db := newSignoffTestDB(t)
	seedSpecimens(t, db)
	svc := NewResultSignoffService(repository.NewResultSignoffRepository(db), repository.NewSpecimenRepository(db), nil)
	ctx := context.Background()

	mustCreateSignoff(t, svc, "SIGNOFF-READ-01", "S-HIGH", "low", "operator")
	page, err := svc.List(ctx, dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(page.Items))
	}
	if page.Items[0].LinkedSpecimenRiskLevel != "high" || page.Items[0].LinkedSpecimenMissing {
		t.Fatalf("list must expose linked specimen risk: %#v", page.Items[0])
	}
}

func mustCreateSignoff(t *testing.T, svc ResultSignoffService, code, relatedCode, risk, actor string) model.ResultSignoff {
	t.Helper()
	input := signoffInput(code)
	input.RiskLevel = risk
	input.RelatedCode = relatedCode
	created, err := svc.Create(context.Background(), input, actor, code+"-create")
	if err != nil {
		t.Fatalf("create %s: %v", code, err)
	}
	return created
}

func mustSubmit(t *testing.T, svc ResultSignoffService, item model.ResultSignoff, actor string) model.ResultSignoff {
	t.Helper()
	peerReview, err := svc.Transition(context.Background(), item.ID, dto.TransitionRequest{
		Status: "peer_review", ExpectedVersion: item.Version, Reason: "evidence complete",
	}, actor, model.RoleOperator, item.Code+"-submit")
	if err != nil {
		t.Fatalf("submit %s: %v", item.Code, err)
	}
	return peerReview
}

func seedSpecimens(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	specimens := []model.Specimen{
		{BaseModel: model.BaseModel{Code: "S-LOW", Name: "低风险样本", Status: "released", Version: 1, CreatedAt: now, UpdatedAt: now}, RiskLevel: "low"},
		{BaseModel: model.BaseModel{Code: "S-MEDIUM", Name: "普通风险样本", Status: "testing", Version: 1, CreatedAt: now, UpdatedAt: now}, RiskLevel: "medium"},
		{BaseModel: model.BaseModel{Code: "S-HIGH", Name: "高风险样本", Status: "hold", Version: 1, CreatedAt: now, UpdatedAt: now}, RiskLevel: "high"},
		{BaseModel: model.BaseModel{Code: "S-CRITICAL", Name: "极高风险样本", Status: "testing", Version: 1, CreatedAt: now, UpdatedAt: now}, RiskLevel: "critical"},
	}
	if err := db.Create(&specimens).Error; err != nil {
		t.Fatalf("seed specimens: %v", err)
	}
}

func newSignoffTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AuditLog{}, &model.Specimen{}, &model.ResultSignoff{}, &model.ResultSignoffRevision{}); err != nil {
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
