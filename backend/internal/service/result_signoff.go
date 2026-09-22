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
	security   SecurityService
}

func NewResultSignoffService(repo repository.ResultSignoffRepository, security SecurityService) ResultSignoffService {
	return &resultSignoffService{repository: repo, security: security}
}

func (s *resultSignoffService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.ResultSignoff], error) {
	return s.repository.List(ctx, query)
}

func (s *resultSignoffService) Get(ctx context.Context, id uint) (model.ResultSignoff, error) {
	return s.repository.Get(ctx, id)
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
	return s.repository.Get(ctx, item.ID)
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
	return s.repository.Get(ctx, id)
}

func (s *resultSignoffService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, role, requestID string) (model.ResultSignoff, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ResultSignoff{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.ResultSignoffTransitions, current.Status, target) {
		return model.ResultSignoff{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	if !isSignoffOperatorRole(role) {
		return model.ResultSignoff{}, ErrForbidden
	}
	if target == "peer_review" {
		if actor != current.PreparedBy {
			return model.ResultSignoff{}, ErrPreparationOwner
		}
		current.ReviewedBy = ""
		current.ReviewReason = ""
	}
	if target == "signed" || target == "rejected" {
		if !isSignoffReviewerRole(role) {
			return model.ResultSignoff{}, ErrReviewRequired
		}
		if actor == current.PreparedBy {
			return model.ResultSignoff{}, ErrSeparationOfDuty
		}
		current.ReviewedBy = actor
		current.ReviewReason = strings.TrimSpace(input.Reason)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateVersion(ctx, id, input.ExpectedVersion, &current, actor, requestID, "transition", before, strings.TrimSpace(input.Reason)); err != nil {
		return model.ResultSignoff{}, fmt.Errorf("transition 结果签发: %w", err)
	}
	return s.repository.Get(ctx, id)
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
