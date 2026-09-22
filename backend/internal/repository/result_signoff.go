package repository

import (
	"context"

	"github.com/blueship581/veterinary-lab-result-review/backend/internal/dto"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/model"
	"gorm.io/gorm"
)

// ResultSignoffRepository owns all persistence operations for 结果签发.
type ResultSignoffRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.ResultSignoff], error)
	Get(context.Context, uint) (model.ResultSignoff, error)
	CreateVersion(context.Context, *model.ResultSignoff, string, string) error
	UpdateVersion(context.Context, uint, uint, *model.ResultSignoff, string, string, string, string, string) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type resultSignoffRepository struct {
	store *Store[model.ResultSignoff]
	db    *gorm.DB
}

func NewResultSignoffRepository(db *gorm.DB) ResultSignoffRepository {
	return &resultSignoffRepository{store: NewStore[model.ResultSignoff](db), db: db}
}

func (r *resultSignoffRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.ResultSignoff], error) {
	page, err := r.store.List(ctx, q)
	if err != nil || len(page.Items) == 0 {
		return page, err
	}
	ids := make([]uint, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.ID)
	}
	var revisions []model.ResultSignoffRevision
	if err := r.db.WithContext(ctx).Where("result_signoff_id IN ?", ids).
		Order("result_signoff_id, version").Find(&revisions).Error; err != nil {
		return Page[model.ResultSignoff]{}, err
	}
	bySignoff := make(map[uint][]model.ResultSignoffRevision)
	for _, revision := range revisions {
		bySignoff[revision.ResultSignoffID] = append(bySignoff[revision.ResultSignoffID], revision)
	}
	for index := range page.Items {
		page.Items[index].Revisions = bySignoff[page.Items[index].ID]
	}
	return page, nil
}
func (r *resultSignoffRepository) Get(ctx context.Context, id uint) (model.ResultSignoff, error) {
	var item model.ResultSignoff
	err := r.db.WithContext(ctx).Preload("Revisions", func(db *gorm.DB) *gorm.DB {
		return db.Order("version")
	}).First(&item, id).Error
	return item, err
}
func (r *resultSignoffRepository) CreateVersion(ctx context.Context, item *model.ResultSignoff, actor, requestID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Revisions").Create(item).Error; err != nil {
			return err
		}
		revision := model.ResultSignoffRevision{
			ResultSignoffID: item.ID, Version: item.Version, Status: item.Status,
			Evidence: item.Evidence, Actor: actor, RequestID: requestID, Action: "create",
			Reason: "signoff drafted", CreatedAt: item.CreatedAt,
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		return appendAudit(tx, actor, requestID, "create", "ResultSignoff", item.ID, "", item.Status, "signoff version 1 drafted")
	})
}
func (r *resultSignoffRepository) UpdateVersion(ctx context.Context, id, expectedVersion uint, item *model.ResultSignoff, actor, requestID, action, before, reason string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item.Revisions = nil
		if err := optimisticUpdate(tx, id, expectedVersion, item); err != nil {
			return err
		}
		revision := model.ResultSignoffRevision{
			ResultSignoffID: id, Version: item.Version, Status: item.Status,
			Evidence: item.Evidence, Actor: actor, RequestID: requestID, Action: action,
			Reason: reason, CreatedAt: item.UpdatedAt,
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		return appendAudit(tx, actor, requestID, action, "ResultSignoff", id, before, item.Status, reason)
	})
}
func (r *resultSignoffRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *resultSignoffRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
