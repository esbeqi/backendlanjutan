package service

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"latihan-fiber/app/repository"
	"latihan-fiber/helper"
)

type EnrollmentService struct {
	db   repository.DBTX
	repo repository.EnrollmentRepository
}

func NewEnrollmentService(db *pgxpool.Pool, repo repository.EnrollmentRepository) *EnrollmentService {
	return &EnrollmentService{db: db, repo: repo}
}

func (s *EnrollmentService) List(c *fiber.Ctx) error {
	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	items, err := s.repo.ListByUser(ctx, s.db, user.UserID)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "daftar enrollment berhasil diambil", items)
}
