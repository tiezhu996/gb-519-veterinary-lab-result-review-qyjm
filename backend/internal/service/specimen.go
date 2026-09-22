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

type SpecimenService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.Specimen], error)
	Get(context.Context, uint) (model.Specimen, error)
	Create(context.Context, dto.CreateSpecimen, string, string) (model.Specimen, error)
	Update(context.Context, uint, dto.UpdateSpecimen, string, string) (model.Specimen, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.Specimen, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type specimenService struct {
	repository repository.SpecimenRepository
	security   SecurityService
}

func NewSpecimenService(repo repository.SpecimenRepository, security SecurityService) SpecimenService {
	return &specimenService{repository: repo, security: security}
}

func (s *specimenService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.Specimen], error) {
	return s.repository.List(ctx, query)
}

func (s *specimenService) Get(ctx context.Context, id uint) (model.Specimen, error) {
	return s.repository.Get(ctx, id)
}

func (s *specimenService) Create(ctx context.Context, input dto.CreateSpecimen, actor, requestID string) (model.Specimen, error) {
	if err := validateSpecimenBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.Specimen{}, err
	}
	item := model.Specimen{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.SpecimenInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.Specimen{}, fmt.Errorf("create 检验样本: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "Specimen", item.ID, "", item.Status, "created 检验样本")
	return item, nil
}

func (s *specimenService) Update(ctx context.Context, id uint, input dto.UpdateSpecimen, actor, requestID string) (model.Specimen, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.Specimen{}, err
	}
	if err := validateSpecimenBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.Specimen{}, err
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
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.Specimen{}, fmt.Errorf("update 检验样本: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "Specimen", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *specimenService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.Specimen, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.Specimen{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.SpecimenTransitions, current.Status, target) {
		return model.Specimen{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.Specimen{}, fmt.Errorf("transition 检验样本: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "Specimen", id, before, target, input.Reason); err != nil {
		return model.Specimen{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *specimenService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "Specimen", id, current.Status, "deleted", "soft deleted 检验样本")
}

func (s *specimenService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateSpecimenBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
