package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"latihan-fiber/app/repository"
	"latihan-fiber/app/service"
	"latihan-fiber/config"
	"latihan-fiber/database"
	"latihan-fiber/helper"
	"latihan-fiber/route"
)

const minSecretLength = 32

// main hanya berisi urutan perakitan.
func main() {

	// 1. Konfigurasi dan logger
	config.LoadEnv()

	logger := config.NewLogger()

	// Rahasia diperiksa SEBELUM server menyala.
	// Lebih baik gagal seketika daripada berjalan dengan token
	// yang mudah dipalsukan.
	jwtSecret := config.GetEnv("JWT_SECRET", "")

	if len(jwtSecret) < minSecretLength {
		logger.Error(
			"JWT_SECRET tidak diisi atau terlalu pendek",
			slog.Int("minimal_karakter", minSecretLength),
		)
		os.Exit(1)
	}

	// 2. Database
	pool, err := database.NewPool(context.Background())
	if err != nil {
		logger.Error(
			"gagal terhubung ke database",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}
	defer pool.Close()

	mongoClient, mongoDatabase, err := database.NewMongo(context.Background())
	if err != nil {
		logger.Error(
			"gagal terhubung ke MongoDB",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	// 3. JWT Manager
	jwtManager := helper.NewJWTManager(
		jwtSecret,
		config.GetEnv("JWT_ISSUER", "praktikum-backend"),
		time.Duration(
			config.GetEnvInt("JWT_ACCESS_TTL_MINUTES", 15),
		)*time.Minute,
	)

	// 4. Repository -> Service
	userRepository := repository.NewUserRepository(pool)
	tokenRepository := repository.NewTokenRepository(pool)
	roleRepository := repository.NewRoleRepository(pool)
	courseRepository := repository.NewCourseRepository(pool)
	enrollmentRepository := repository.NewEnrollmentRepository(pool)
	reviewRepository := repository.NewReviewRepository(mongoDatabase)
	indexCtx, cancelIndexCtx := context.WithTimeout(context.Background(), 10*time.Second)
	err = reviewRepository.EnsureIndexes(indexCtx)
	cancelIndexCtx()
	if err != nil {
		logger.Error(
			"gagal memastikan index MongoDB untuk course_reviews",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}
	txManager := repository.NewTxManager(pool)

	// Pemetaan role ke permission dibaca SEKALI saat aplikasi menyala.
	rawPermissions, err := roleRepository.LoadPermissions(context.Background())
	if err != nil {
		logger.Error(
			"gagal memuat permission",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	permissions := helper.NewPermissionSet(rawPermissions)

	logger.Info(
		"permission dimuat",
		slog.Any("roles", permissions.KnownRoles()),
	)

	userService := service.NewUserService(
		userRepository,
		permissions,
	)

	authService := service.NewAuthService(
		userRepository,
		tokenRepository,
		jwtManager,
		permissions,
		time.Duration(
			config.GetEnvInt("JWT_REFRESH_TTL_DAYS", 7),
		)*24*time.Hour,
	)
	courseService := service.NewCourseService(courseRepository, txManager)
	enrollmentService := service.NewEnrollmentService(pool, enrollmentRepository)

	// 5. App
	app := config.NewApp(
		logger,
		route.Dependencies{
			Pool:              pool,
			MongoClient:       mongoClient,
			MongoDatabase:     mongoDatabase,
			ReviewRepository:  reviewRepository,
			JWT:               jwtManager,
			Permissions:       permissions,
			UserService:       userService,
			AuthService:       authService,
			CourseService:     courseService,
			EnrollmentService: enrollmentService,
		},
	)

	port := config.GetEnv("APP_PORT", "3000")

	go func() {
		if err := app.Listen(":" + port); err != nil {
			logger.Error(
				"server berhenti",
				slog.String("error", err.Error()),
			)
			os.Exit(1)
		}
	}()

	logger.Info(
		"server berjalan",
		slog.String("port", port),
	)

	// 6. Graceful shutdown
	quit := make(chan os.Signal, 1)

	signal.Notify(
		quit,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	<-quit

	logger.Info("sinyal berhenti diterima, menutup server")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		logger.Error(
			"gagal menutup server dengan rapi",
			slog.String("error", err.Error()),
		)
	}

	disconnectCtx, disconnectCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer disconnectCancel()
	if err := mongoClient.Disconnect(disconnectCtx); err != nil {
		logger.Error(
			"gagal memutus koneksi MongoDB",
			slog.String("error", err.Error()),
		)
	}

	logger.Info("server berhenti dengan rapi")
}
