package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/blueship581/veterinary-lab-result-review/backend/internal/config"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, cfg config.Config, log *slog.Logger) (*gorm.DB, *redis.Client, error) {
	var dialector gorm.Dialector
	switch cfg.DatabaseDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DatabaseDSN)
	case "mysql":
		dialector = mysql.Open(cfg.DatabaseDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DatabaseDSN)
	default:
		return nil, nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
	logLevel := logger.Warn
	if cfg.Environment == "development" {
		logLevel = logger.Info
	}
	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		db, err = gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil && sqlDB.PingContext(ctx) == nil {
				break
			}
			if dbErr != nil {
				err = dbErr
			} else {
				err = sqlDB.PingContext(ctx)
			}
		}
		log.Warn("database not ready", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connect database: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, nil, err
	}
	if err := Seed(ctx, db); err != nil {
		return nil, nil, err
	}
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, nil, fmt.Errorf("connect redis: %w", err)
		}
	}
	return db, redisClient, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{}, &model.AuditLog{},
		&model.AnimalCase{},
		&model.Specimen{},
		&model.AssayRun{},
		&model.ResultSignoff{},
		&model.ResultSignoffRevision{},
	)
}

func Seed(ctx context.Context, db *gorm.DB) error {
	var users int64
	if err := db.WithContext(ctx).Model(&model.User{}).Count(&users).Error; err != nil {
		return err
	}
	if users == 0 {
		password, err := bcrypt.GenerateFromPassword([]byte("Admin123!"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		seedUsers := []model.User{
			{Username: "admin", DisplayName: "系统管理员", PasswordHash: string(password), Role: model.RoleAdmin, Active: true},
			{Username: "reviewer", DisplayName: "质量复核员", PasswordHash: string(password), Role: model.RoleReviewer, Active: true},
			{Username: "operator", DisplayName: "现场操作员", PasswordHash: string(password), Role: model.RoleOperator, Active: true},
			{Username: "viewer", DisplayName: "只读审计员", PasswordHash: string(password), Role: model.RoleViewer, Active: true},
		}
		if err := db.WithContext(ctx).Create(&seedUsers).Error; err != nil {
			return err
		}
	}

	if err := seedAnimalCase(ctx, db); err != nil {
		return err
	}

	if err := seedSpecimen(ctx, db); err != nil {
		return err
	}

	if err := seedAssayRun(ctx, db); err != nil {
		return err
	}

	if err := seedResultSignoff(ctx, db); err != nil {
		return err
	}

	return nil
}

func seedAnimalCase(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.AnimalCase{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.AnimalCase{

		{BaseModel: model.BaseModel{Code: "AC-001", Name: "动物样本来源示例一", Status: "registered", Version: 1,
			Description: "用于启动验证和主要流程演示的动物样本来源记录"}, Facility: "兽医检验样本结果复核区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-01"},

		{BaseModel: model.BaseModel{Code: "AC-002", Name: "动物样本来源示例二", Status: "sampling", Version: 1,
			Description: "用于启动验证和主要流程演示的动物样本来源记录"}, Facility: "兽医检验样本结果复核区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-02"},

		{BaseModel: model.BaseModel{Code: "AC-003", Name: "动物样本来源示例三", Status: "testing", Version: 1,
			Description: "用于启动验证和主要流程演示的动物样本来源记录"}, Facility: "兽医检验样本结果复核区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedSpecimen(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.Specimen{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.Specimen{

		{BaseModel: model.BaseModel{Code: "S-001", Name: "检验样本示例一", Status: "received", Version: 1,
			Description: "用于启动验证和主要流程演示的检验样本记录"}, Facility: "兽医检验样本结果复核区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-01"},

		{BaseModel: model.BaseModel{Code: "S-002", Name: "检验样本示例二", Status: "testing", Version: 1,
			Description: "用于启动验证和主要流程演示的检验样本记录"}, Facility: "兽医检验样本结果复核区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-02"},

		{BaseModel: model.BaseModel{Code: "S-003", Name: "检验样本示例三", Status: "hold", Version: 1,
			Description: "用于启动验证和主要流程演示的检验样本记录"}, Facility: "兽医检验样本结果复核区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedAssayRun(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.AssayRun{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.AssayRun{

		{BaseModel: model.BaseModel{Code: "AR-001", Name: "检测运行示例一", Status: "planned", Version: 1,
			Description: "用于启动验证和主要流程演示的检测运行记录"}, Facility: "兽医检验样本结果复核区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-01"},

		{BaseModel: model.BaseModel{Code: "AR-002", Name: "检测运行示例二", Status: "running", Version: 1,
			Description: "用于启动验证和主要流程演示的检测运行记录"}, Facility: "兽医检验样本结果复核区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-02"},

		{BaseModel: model.BaseModel{Code: "AR-003", Name: "检测运行示例三", Status: "validated", Version: 1,
			Description: "用于启动验证和主要流程演示的检测运行记录"}, Facility: "兽医检验样本结果复核区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedResultSignoff(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.ResultSignoff{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.ResultSignoff{

		{BaseModel: model.BaseModel{Code: "RS-001", Name: "结果签发示例一", Status: "draft", Version: 1,
			Description: "用于启动验证和主要流程演示的结果签发记录"}, Facility: "兽医检验样本结果复核区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-01", PreparedBy: "operator"},

		{BaseModel: model.BaseModel{Code: "RS-002", Name: "结果签发示例二", Status: "peer_review", Version: 1,
			Description: "用于启动验证和主要流程演示的结果签发记录"}, Facility: "兽医检验样本结果复核区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-02", PreparedBy: "operator"},

		{BaseModel: model.BaseModel{Code: "RS-003", Name: "结果签发示例三", Status: "signed", Version: 1,
			Description: "用于启动验证和主要流程演示的结果签发记录"}, Facility: "兽医检验样本结果复核区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-519-03", PreparedBy: "operator", ReviewedBy: "reviewer", ReviewReason: "演示数据双人复核通过"},
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&items).Error; err != nil {
			return err
		}
		revisions := make([]model.ResultSignoffRevision, 0, len(items))
		for _, item := range items {
			revisions = append(revisions, model.ResultSignoffRevision{
				ResultSignoffID: item.ID, Version: item.Version, Status: item.Status,
				Evidence: item.Evidence, Actor: "system-seed", RequestID: "seed-gb-519",
				Action: "seed", Reason: "initial demonstration signoff", CreatedAt: now,
			})
		}
		return tx.Create(&revisions).Error
	})
}
