package repository

import (
	"context"

	"github.com/blueship581/veterinary-lab-result-review/backend/internal/dto"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/model"
	"gorm.io/gorm"
)

// AssayRunRepository owns all persistence operations for 检测运行.
type AssayRunRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.AssayRun], error)
	Get(context.Context, uint) (model.AssayRun, error)
	Create(context.Context, *model.AssayRun) error
	Update(context.Context, uint, uint, *model.AssayRun) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type assayRunRepository struct {
	store *Store[model.AssayRun]
}

func NewAssayRunRepository(db *gorm.DB) AssayRunRepository {
	return &assayRunRepository{store: NewStore[model.AssayRun](db)}
}

func (r *assayRunRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.AssayRun], error) {
	return r.store.List(ctx, q)
}
func (r *assayRunRepository) Get(ctx context.Context, id uint) (model.AssayRun, error) {
	return r.store.Get(ctx, id)
}
func (r *assayRunRepository) Create(ctx context.Context, item *model.AssayRun) error {
	return r.store.Create(ctx, item)
}
func (r *assayRunRepository) Update(ctx context.Context, id, version uint, item *model.AssayRun) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *assayRunRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *assayRunRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
