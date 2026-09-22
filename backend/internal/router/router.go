package router

import (
	"log/slog"
	"net/http"

	"github.com/blueship581/veterinary-lab-result-review/backend/internal/config"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/handler"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/middleware"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/repository"
	"github.com/blueship581/veterinary-lab-result-review/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client, logger *slog.Logger) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(middleware.RequestContext(logger))
	if len(cfg.TrustedProxies) > 0 {
		_ = engine.SetTrustedProxies(cfg.TrustedProxies)
	} else {
		_ = engine.SetTrustedProxies(nil)
	}

	securityRepository := repository.NewSecurityRepository(db)
	securityService := service.NewSecurityService(securityRepository, cfg)
	animalCaseRepository := repository.NewAnimalCaseRepository(db)
	specimenRepository := repository.NewSpecimenRepository(db)
	assayRunRepository := repository.NewAssayRunRepository(db)
	resultSignoffRepository := repository.NewResultSignoffRepository(db)
	animalCaseService := service.NewAnimalCaseService(animalCaseRepository, securityService)
	specimenService := service.NewSpecimenService(specimenRepository, securityService)
	assayRunService := service.NewAssayRunService(assayRunRepository, securityService)
	resultSignoffService := service.NewResultSignoffService(resultSignoffRepository, securityService)
	animalCaseHandler := handler.NewAnimalCaseHandler(animalCaseService)
	specimenHandler := handler.NewSpecimenHandler(specimenService)
	assayRunHandler := handler.NewAssayRunHandler(assayRunService)
	resultSignoffHandler := handler.NewResultSignoffHandler(resultSignoffService)
	systemHandler := handler.NewSystemHandler(securityService, animalCaseService, specimenService, assayRunService, resultSignoffService, db, redisClient)

	engine.GET("/healthz", systemHandler.Health)
	engine.POST("/api/auth/login", systemHandler.Login)

	limiter := middleware.NewLimiter(redisClient, cfg.RequestLimit)
	api := engine.Group("/api")
	api.Use(limiter.Middleware(), middleware.Authenticate(cfg))
	api.GET("/overview", systemHandler.Overview)
	api.GET("/audits", middleware.RequireMinimumRole("reviewer"), systemHandler.Audits)
	api.GET("/session", systemHandler.Session)
	api.GET("/runtime", systemHandler.Runtime)
	api.GET("/audit-summary", middleware.RequireMinimumRole("reviewer"), systemHandler.AuditSummary)
	api.GET("/audits/:entityType/:id", middleware.RequireMinimumRole("reviewer"), systemHandler.EntityHistory)
	animalCaseHandler.Register(api)
	specimenHandler.Register(api)
	assayRunHandler.Register(api)
	resultSignoffHandler.Register(api)

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "route_not_found"})
	})
	return engine
}
