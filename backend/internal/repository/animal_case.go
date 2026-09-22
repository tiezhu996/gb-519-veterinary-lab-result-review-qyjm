package repository

import (
	"context"

	"github.com/blueship581/veterinary-lab-result-review/backend/internal/dto"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/model"
	"gorm.io/gorm"
)

// AnimalCaseRepository owns all persistence operations for 动物样本来源.
type AnimalCaseRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.AnimalCase], error)
	Get(context.Context, uint) (model.AnimalCase, error)
	Create(context.Context, *model.AnimalCase) error
	Update(context.Context, uint, uint, *model.AnimalCase) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type animalCaseRepository struct {
	store *Store[model.AnimalCase]
}

func NewAnimalCaseRepository(db *gorm.DB) AnimalCaseRepository {
	return &animalCaseRepository{store: NewStore[model.AnimalCase](db)}
}

func (r *animalCaseRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.AnimalCase], error) {
	return r.store.List(ctx, q)
}
func (r *animalCaseRepository) Get(ctx context.Context, id uint) (model.AnimalCase, error) {
	return r.store.Get(ctx, id)
}
func (r *animalCaseRepository) Create(ctx context.Context, item *model.AnimalCase) error {
	return r.store.Create(ctx, item)
}
func (r *animalCaseRepository) Update(ctx context.Context, id, version uint, item *model.AnimalCase) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *animalCaseRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *animalCaseRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
