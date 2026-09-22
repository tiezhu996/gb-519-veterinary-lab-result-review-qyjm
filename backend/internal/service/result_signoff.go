package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/veterinary-lab-result-review/backend/internal/constants"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/dto"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/model"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/repository"
)

type ResultSignoffService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.ResultSignoff], error)
	Get(context.Context, uint) (model.ResultSignoff, error)
	Create(context.Context, dto.CreateResultSignoff, string, string) (model.ResultSignoff, error)
	Update(context.Context, uint, dto.UpdateResultSignoff, string, string) (model.ResultSignoff, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string, string) (model.ResultSignoff, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type resultSignoffService struct {
	repository repository.ResultSignoffRepository
	specimens  repository.SpecimenRepository
	security   SecurityService
}

func NewResultSignoffService(repo repository.ResultSignoffRepository, specimens repository.SpecimenRepository, security SecurityService) ResultSignoffService {
	return &resultSignoffService{repository: repo, specimens: specimens, security: security}
}

func (s *resultSignoffService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.ResultSignoff], error) {
	page, err := s.repository.List(ctx, query)
	if err != nil {
		return page, err
	}
	if err := s.enrichLinkedSpecimens(ctx, page.Items); err != nil {
		return repository.Page[model.ResultSignoff]{}, err
	}
	return page, nil
}

func (s *resultSignoffService) Get(ctx context.Context, id uint) (model.ResultSignoff, error) {
	item, err := s.repository.Get(ctx, id)
	if err != nil {
		return item, err
	}
	items := []model.ResultSignoff{item}
	if err := s.enrichLinkedSpecimens(ctx, items); err != nil {
		return model.ResultSignoff{}, err
	}
	return items[0], nil
}

// enrichLinkedSpecimens annotates each signoff with the current risk level of
// its linked specimen so the workbench can show 样本风险 and 原因 before and
// after the first confirmation. Missing links are flagged, not fabricated.
func (s *resultSignoffService) enrichLinkedSpecimens(ctx context.Context, items []model.ResultSignoff) error {
	if len(items) == 0 {
		return nil
	}
	codes := make([]string, 0, len(items))
	seen := make(map[string]bool)
	for _, item := range items {
		code := strings.TrimSpace(item.RelatedCode)
		if code != "" && !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}
	levels, err := s.specimens.RiskLevelsByCodes(ctx, codes)
	if err != nil {
		return err
	}
	for index := range items {
		code := strings.TrimSpace(items[index].RelatedCode)
		if code == "" {
			items[index].LinkedSpecimenMissing = true
			continue
		}
		level, ok := levels[code]
		if !ok {
			items[index].LinkedSpecimenMissing = true
			continue
		}
		items[index].LinkedSpecimenRiskLevel = level
	}
	return nil
}

func (s *resultSignoffService) Create(ctx context.Context, input dto.CreateResultSignoff, actor, requestID string) (model.ResultSignoff, error) {
	if err := validateResultSignoffBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ResultSignoff{}, err
	}
	item := model.ResultSignoff{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.ResultSignoffInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)), PreparedBy: actor,
	}
	if err := s.repository.CreateVersion(ctx, &item, actor, requestID); err != nil {
		return model.ResultSignoff{}, fmt.Errorf("create 结果签发: %w", err)
	}
	return s.Get(ctx, item.ID)
}

func (s *resultSignoffService) Update(ctx context.Context, id uint, input dto.UpdateResultSignoff, actor, requestID string) (model.ResultSignoff, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ResultSignoff{}, err
	}
	if current.Status != model.ResultSignoffInitialStatus {
		return model.ResultSignoff{}, ErrLocked
	}
	if actor != current.PreparedBy {
		return model.ResultSignoff{}, ErrPreparationOwner
	}
	if err := validateResultSignoffBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ResultSignoff{}, err
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = strings.TrimSpace(input.Facility)
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateVersion(ctx, id, input.ExpectedVersion, &current, actor, requestID, "update", current.Status, "draft signoff fields updated"); err != nil {
		return model.ResultSignoff{}, fmt.Errorf("update 结果签发: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *resultSignoffService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, role, requestID string) (model.ResultSignoff, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ResultSignoff{}, err
	}
	requested := strings.TrimSpace(input.Status)
	reason := strings.TrimSpace(input.Reason)

	if !isSignoffOperatorRole(role) {
		return model.ResultSignoff{}, ErrForbidden
	}
	// Reviewers always request "signed"; for high/critical linked specimens the
	// service downgrades that decision to secondary_review below. Both edges are
	// represented in the transition graph, so validate the requested edge first.
	if !constants.CanTransition(constants.ResultSignoffTransitions, current.Status, requested) {
		return model.ResultSignoff{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, requested)
	}

	switch {
	case requested == "peer_review":
		if actor != current.PreparedBy {
			return model.ResultSignoff{}, ErrPreparationOwner
		}
		current.Status = "peer_review"
		// Re-submission must not retain a previous reviewer's conclusion.
		current.ReviewedBy = ""
		current.ReviewReason = ""
		current.FirstReviewedBy = ""
		current.FirstReviewReason = ""
		current.FirstReviewedAt = nil
		current.SpecimenRiskLevel = ""

	case requested == "rejected":
		if err := s.applyRejection(&current, actor, role, reason); err != nil {
			return model.ResultSignoff{}, err
		}

	case requested == "signed" || requested == "secondary_review":
		if err := s.applyReviewDecision(ctx, &current, requested, actor, role, reason); err != nil {
			return model.ResultSignoff{}, err
		}

	default:
		return model.ResultSignoff{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, requested)
	}

	before := current.Status
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	action := "transition"
	persistedReason := current.ReviewReason
	if current.Status == "secondary_review" {
		action = "first_review"
		// The first reviewer's conclusion lives in FirstReviewReason; keep it on
		// the append-only revision as well.
		persistedReason = current.FirstReviewReason
	}
	if err := s.repository.UpdateVersion(ctx, id, input.ExpectedVersion, &current, actor, requestID, action, before, persistedReason); err != nil {
		return model.ResultSignoff{}, fmt.Errorf("transition 结果签发: %w", err)
	}
	return s.Get(ctx, id)
}

// applyRejection keeps the original reject path unchanged for both review
// stages: a reviewer/admin different from the preparer may reject at any time.
func (s *resultSignoffService) applyRejection(current *model.ResultSignoff, actor, role, reason string) error {
	if !isSignoffReviewerRole(role) {
		return ErrReviewRequired
	}
	if actor == current.PreparedBy {
		return ErrSeparationOfDuty
	}
	current.Status = "rejected"
	current.ReviewedBy = actor
	current.ReviewReason = reason
	return nil
}

// applyReviewDecision enforces risk-graded issuance derived from the linked
// specimen. All failures return before mutating persisted state, and a failed
// second confirmation never overwrites the first reviewer's conclusion.
func (s *resultSignoffService) applyReviewDecision(ctx context.Context, current *model.ResultSignoff, requested, actor, role, reason string) error {
	if !isSignoffReviewerRole(role) {
		return ErrReviewRequired
	}
	if actor == current.PreparedBy {
		return ErrSeparationOfDuty
	}

	if current.Status == "secondary_review" {
		// Second-level review: a different reviewer completes issuance.
		if current.FirstReviewedBy == "" || current.FirstReviewedAt == nil {
			return fmt.Errorf("%w: missing first review", ErrInvalidTransition)
		}
		if actor == current.FirstReviewedBy {
			return ErrDistinctReviewer
		}
		current.Status = "signed"
		current.ReviewedBy = actor
		current.ReviewReason = reason
		return nil
	}

	if current.Status != "peer_review" {
		return fmt.Errorf("%w: %s -> review decision", ErrInvalidTransition, current.Status)
	}

	// First-level review: grading comes from the linked specimen, not from the
	// signoff's own risk label. A missing link blocks any decision.
	specimen, err := s.specimens.GetByCode(ctx, strings.TrimSpace(current.RelatedCode))
	if err != nil {
		return ErrLinkedSpecimen
	}
	highRisk := constants.HighSignoffSpecimenRisks[specimen.RiskLevel]
	if !highRisk && requested == "secondary_review" {
		// Normal-risk specimens stay on the single-approval path; callers cannot
		// manufacture a two-reviewer stage for them.
		return fmt.Errorf("%w: normal-risk specimen does not require secondary review", ErrInvalidTransition)
	}
	if highRisk {
		now := time.Now().UTC()
		current.Status = "secondary_review"
		current.SpecimenRiskLevel = specimen.RiskLevel
		current.FirstReviewedBy = actor
		current.FirstReviewReason = reason
		current.FirstReviewedAt = &now
		// Final approval fields stay empty until the second reviewer signs.
		current.ReviewedBy = ""
		current.ReviewReason = ""
		return nil
	}

	current.Status = "signed"
	current.SpecimenRiskLevel = specimen.RiskLevel
	current.ReviewedBy = actor
	current.ReviewReason = reason
	return nil
}

func (s *resultSignoffService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != model.ResultSignoffInitialStatus {
		return ErrLocked
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "ResultSignoff", id, current.Status, "deleted", "soft deleted 结果签发")
}

func isSignoffOperatorRole(role string) bool {
	return role == model.RoleOperator || role == model.RoleReviewer || role == model.RoleAdmin
}

func isSignoffReviewerRole(role string) bool {
	return role == model.RoleReviewer || role == model.RoleAdmin
}

func (s *resultSignoffService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateResultSignoffBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
