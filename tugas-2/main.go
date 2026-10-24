package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-students/app/repository"
	"api-students/app/service"
	"api-students/config"
	"api-students/database"
	"api-students/helper"
)

func main() {
	exitCode := 0
	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	// 1. Load konfigurasi & logger
	config.LoadEnv()
	logger := config.NewLogger()

	// 2. Database
	pool, err := database.NewPool(context.Background())
	if err != nil {
		logger.Error(
			"gagal terhubung ke database",
			slog.String("error", err.Error()),
		)
		exitCode = 1
		return
	}
	defer pool.Close()

	mongoClient, mongoDatabase, err := database.NewMongo(context.Background())
	if err != nil {
		logger.Error(
			"gagal terhubung ke MongoDB",
			slog.String("error", err.Error()),
		)
		exitCode = 1
		return
	}
	defer disconnectMongo(mongoClient, logger)

	portfolioRepository := repository.NewStudentPortfolioRepository(mongoDatabase)
	indexCtx, cancelIndexCtx := context.WithTimeout(context.Background(), 10*time.Second)
	err = portfolioRepository.EnsureIndexes(indexCtx)
	cancelIndexCtx()
	if err != nil {
		logger.Error(
			"gagal memastikan index MongoDB untuk student_portfolios",
			slog.String("error", err.Error()),
		)
		exitCode = 1
		return
	}

	// 3. Repository -> Service

	// Student
	studentRepository := repository.NewStudentRepository(pool)
	txManager := repository.NewTxManager(pool)
	studentService := service.NewStudentService(studentRepository, txManager)
	portfolioService := service.NewStudentPortfolioService(studentRepository, portfolioRepository)
	supervisorRepository := repository.NewSupervisorRepository(pool)
	supervisorService := service.NewSupervisorService(supervisorRepository, txManager)

	// Authentication
	userRepository := repository.NewUserRepository(pool)
	tokenRepository := repository.NewTokenRepository(pool)

	jwtManager := helper.NewJWTManager(
		config.GetEnv("JWT_SECRET", ""),
		config.GetEnv("JWT_ISSUER", "api-students"),
		15*time.Minute,
	)

	authService := service.NewAuthService(
		userRepository,
		tokenRepository,
		jwtManager,
	)

	// Role & Permission
	roleRepository := repository.NewRoleRepository(pool)

	rawPermissions, err := roleRepository.FindPermissions(context.Background())
	if err != nil {
		logger.Error(
			"gagal memuat permission",
			slog.String("error", err.Error()),
		)
		exitCode = 1
		return
	}

	perms := helper.NewPermissionSet(rawPermissions)

	// 4. Buat aplikasi
	app := config.NewApp(
		logger,
		pool,
		studentService,
		portfolioService,
		supervisorService,
		authService,
		jwtManager,
		perms,
	)

	port := config.GetEnv("APP_PORT", "3000")

	serverErr := make(chan error, 1)
	go func() {
		if err := app.Listen(":" + port); err != nil {
			serverErr <- err
		}
	}()

	logger.Info(
		"server berjalan",
		slog.String("port", port),
	)

	// 5. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(
		quit,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	serverFailed := false
	select {
	case <-quit:
		logger.Info("sinyal berhenti diterima, menutup server")
	case err := <-serverErr:
		logger.Error("server berhenti", slog.String("error", err.Error()))
		serverFailed = true
	}

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

	logger.Info("server berhenti dengan rapi")
	if serverFailed {
		exitCode = 1
	}
}

func disconnectMongo(client interface {
	Disconnect(context.Context) error
}, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Disconnect(ctx); err != nil {
		logger.Error(
			"gagal memutus koneksi MongoDB",
			slog.String("error", err.Error()),
		)
	}
}
