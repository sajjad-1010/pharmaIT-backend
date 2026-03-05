package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pharmalink/server/internal/app"
	"pharmalink/server/internal/auth"
	"pharmalink/server/internal/cache"
	"pharmalink/server/internal/config"
	"pharmalink/server/internal/db"
	"pharmalink/server/internal/db/model"
	_ "pharmalink/server/internal/docs"
	"pharmalink/server/internal/http/middleware"
	"pharmalink/server/internal/logger"
	"pharmalink/server/internal/modules/catalog"
	"pharmalink/server/internal/modules/discounts"
	"pharmalink/server/internal/modules/inventory"
	"pharmalink/server/internal/modules/manufacturer"
	"pharmalink/server/internal/modules/offers"
	"pharmalink/server/internal/modules/orders"
	"pharmalink/server/internal/modules/outbox"
	"pharmalink/server/internal/modules/payments"
	"pharmalink/server/internal/modules/rare"
	"pharmalink/server/internal/modules/sse"
	"pharmalink/server/internal/modules/users"
	"pharmalink/server/internal/rbac"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	swagger "github.com/gofiber/swagger"
	"github.com/swaggo/swag"
)

// @title PharmaLink API
// @version 1.0
// @description Backend API for PharmaLink B2B marketplace
// @BasePath /api/v1
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New(cfg.AppEnv)

	dbConn, err := db.NewPostgres(cfg.Postgres, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect postgres")
	}
	sqlDB, _ := dbConn.DB()
	defer sqlDB.Close()

	if cfg.AppEnv == "development" {
		if err := app.EnsureDevSeedUsers(context.Background(), dbConn, log); err != nil {
			log.Fatal().Err(err).Msg("failed to seed development users")
		}
	}

	redisClient := cache.NewRedis(cfg.Redis)
	if err := cache.Ping(context.Background(), redisClient); err != nil {
		log.Fatal().Err(err).Msg("failed to connect redis")
	}
	defer redisClient.Close()

	authSvc := auth.NewService(cfg.JWT)
	outboxSvc := outbox.NewService(dbConn, cfg.OutboxChannel)
	sseBroker := sse.NewBroker()

	userSvc := users.NewService(dbConn, authSvc)
	userHandler := users.NewHandler(userSvc)

	catalogSvc := catalog.NewService(dbConn, redisClient)
	catalogHandler := catalog.NewHandler(catalogSvc)

	offersSvc := offers.NewService(dbConn, redisClient, outboxSvc)
	offersHandler := offers.NewHandler(offersSvc)

	inventorySvc := inventory.NewService(dbConn, outboxSvc)
	inventoryHandler := inventory.NewHandler(inventorySvc)

	ordersSvc := orders.NewService(dbConn, inventorySvc, outboxSvc)
	ordersHandler := orders.NewHandler(ordersSvc)

	rareSvc := rare.NewService(dbConn, outboxSvc)
	rareHandler := rare.NewHandler(rareSvc)

	manufacturerSvc := manufacturer.NewService(dbConn, outboxSvc)
	manufacturerHandler := manufacturer.NewHandler(manufacturerSvc)

	discountSvc := discounts.NewService(dbConn, outboxSvc)
	discountHandler := discounts.NewHandler(discountSvc)

	paymentSvc := payments.NewService(dbConn, cfg.Payment, outboxSvc)
	paymentHandler := payments.NewHandler(paymentSvc)

	sseHandler := sse.NewHandler(sse.NewStreamHandler(sseBroker))

	go sse.StartRedisBridge(context.Background(), redisClient, "sse_offers", sseBroker, log)

	app := fiber.New(fiber.Config{
		AppName:      cfg.AppName,
		ErrorHandler: middleware.ErrorHandler(),
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	})

	app.Use(middleware.RequestID())
	app.Use(middleware.Recover(log))
	app.Use(middleware.Logging(log))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Authorization,Content-Type,X-Request-ID,X-Signature",
		ExposeHeaders:    "X-Request-ID",
		AllowCredentials: false,
		MaxAge:           300,
	}))
	app.Use(func(c *fiber.Ctx) error {
		err := c.Next()
		_ = middleware.RateLimitedLog(log)(c)
		return err
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"time":   time.Now().UTC(),
		})
	})

	v1 := app.Group("/api/v1")
	v1.Get("/docs/swagger.json", func(c *fiber.Ctx) error {
		doc, err := swag.ReadDoc()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "SWAGGER_NOT_GENERATED",
					"message": "swagger spec is not generated yet; run `make swagger`",
				},
			})
		}
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.SendString(doc)
	})
	v1.Get("/docs", func(c *fiber.Ctx) error {
		return c.Redirect("/api/v1/docs/index.html", fiber.StatusTemporaryRedirect)
	})
	v1.Get("/docs/*", swagger.New(swagger.Config{
		URL: "/api/v1/docs/swagger.json",
	}))

	authMW := middleware.JWTAuth(authSvc)
	adminOnly := rbac.Allow(model.UserRoleAdmin)
	wholesalerOnly := rbac.Allow(model.UserRoleWholesaler)

	userHandler.RegisterRoutes(v1, authMW, adminOnly)
	catalogHandler.RegisterRoutes(v1, authMW, adminOnly)
	offersHandler.RegisterRoutes(v1, authMW, wholesalerOnly)
	inventoryHandler.RegisterRoutes(v1, authMW, wholesalerOnly)
	ordersHandler.RegisterRoutes(v1, authMW)
	rareHandler.RegisterRoutes(v1, authMW)
	manufacturerHandler.RegisterRoutes(v1, authMW)
	discountHandler.RegisterRoutes(v1, authMW, wholesalerOnly)
	paymentHandler.RegisterRoutes(v1, authMW)
	sseHandler.RegisterRoutes(v1)

	go func() {
		log.Info().Str("addr", cfg.HTTPAddr).Msg("api server started")
		if err := app.Listen(cfg.HTTPAddr); err != nil {
			log.Fatal().Err(err).Msg("api server crashed")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = app.ShutdownWithContext(ctx)
}
