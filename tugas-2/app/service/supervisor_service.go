package service

import (
	"context"
	"errors"
	"strconv"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

type transactionRunner interface {
	WithTx(ctx context.Context, fn func(repository.DBTX) error) error
}

type SupervisorService struct {
	repo      repository.SupervisorRepository
	txManager transactionRunner
}

func NewSupervisorService(repo repository.SupervisorRepository, txManager transactionRunner) *SupervisorService {
	return &SupervisorService{repo: repo, txManager: txManager}
}

// POST /supervisors/:id/assign
func (s *SupervisorService) Assign(c *fiber.Ctx) error {
	supervisorID, valid := helper.ParamID(c)
	if !valid {
		return fiber.NewError(fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	var req model.AssignSupervisorRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}
	if req.StudentID < 1 {
		return helper.FailValidation(c, map[string]string{"student_id": "harus berupa angka positif"})
	}

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	err := s.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.repo.WithTx(tx)
		if err := repo.LockStudent(ctx, req.StudentID); err != nil {
			return err
		}
		existing, err := repo.FindSupervisionByStudent(ctx, req.StudentID)
		if err != nil {
			return err
		}
		if existing.StudentID != 0 {
			return repository.ErrAlreadyAssigned
		}
		if _, err := repo.IncrementSupervisedCount(ctx, supervisorID); err != nil {
			return err
		}
		return repo.CreateSupervision(ctx, model.Supervision{
			StudentID: req.StudentID, SupervisorID: supervisorID,
		})
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrStudentNotFound):
			return fiber.NewError(fiber.StatusNotFound, "mahasiswa tidak ditemukan")
		case errors.Is(err, repository.ErrSupervisorNotFound):
			return fiber.NewError(fiber.StatusNotFound, "supervisor tidak ditemukan")
		case errors.Is(err, repository.ErrAlreadyAssigned):
			return fiber.NewError(fiber.StatusConflict, "mahasiswa sudah memiliki supervisor")
		case errors.Is(err, repository.ErrSupervisorFull):
			return fiber.NewError(fiber.StatusConflict, "kuota supervisor penuh")
		default:
			return fiber.NewError(fiber.StatusInternalServerError, "gagal menetapkan supervisor")
		}
	}

	return helper.Created(c, "mahasiswa berhasil ditetapkan ke supervisor", model.Supervision{
		StudentID: req.StudentID, SupervisorID: supervisorID,
	}, "/api/v1/supervisors/"+strconv.Itoa(supervisorID)+"/assign")
}
