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

// POST /supervisions/switch
func (s *SupervisorService) Switch(c *fiber.Ctx) error {
	var req model.SwitchSupervisorRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}
	validationErrors := make(map[string]string)
	if req.StudentID < 1 {
		validationErrors["student_id"] = "harus berupa angka positif"
	}
	if req.NewSupervisorID < 1 {
		validationErrors["new_supervisor_id"] = "harus berupa angka positif"
	}
	if len(validationErrors) > 0 {
		return helper.FailValidation(c, validationErrors)
	}

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	err := s.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.repo.WithTx(tx)
		if err := repo.LockStudent(ctx, req.StudentID); err != nil {
			return err
		}
		current, err := repo.FindSupervisionByStudent(ctx, req.StudentID)
		if err != nil {
			return err
		}
		if current.StudentID == 0 {
			return repository.ErrNotAssigned
		}
		if current.SupervisorID == req.NewSupervisorID {
			return repository.ErrSameSupervisor
		}
		if err := repo.LockSupervisors(ctx, current.SupervisorID, req.NewSupervisorID); err != nil {
			return err
		}
		if err := repo.DecrementSupervisedCount(ctx, current.SupervisorID); err != nil {
			return err
		}
		if _, err := repo.IncrementSupervisedCount(ctx, req.NewSupervisorID); err != nil {
			return err
		}
		return repo.SwitchSupervision(ctx, req.StudentID, current.SupervisorID, req.NewSupervisorID)
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrStudentNotFound):
			return fiber.NewError(fiber.StatusNotFound, "mahasiswa tidak ditemukan")
		case errors.Is(err, repository.ErrSupervisorNotFound):
			return fiber.NewError(fiber.StatusNotFound, "supervisor tidak ditemukan")
		case errors.Is(err, repository.ErrNotAssigned):
			return fiber.NewError(fiber.StatusConflict, "mahasiswa belum memiliki supervisor")
		case errors.Is(err, repository.ErrSameSupervisor):
			return fiber.NewError(fiber.StatusConflict, "supervisor tujuan sama dengan supervisor saat ini")
		case errors.Is(err, repository.ErrSupervisorFull):
			return fiber.NewError(fiber.StatusConflict, "kuota supervisor tujuan penuh")
		default:
			return fiber.NewError(fiber.StatusInternalServerError, "gagal mengganti supervisor")
		}
	}

	return helper.Success(c, fiber.StatusOK, "supervisor mahasiswa berhasil diganti", model.Supervision{
		StudentID: req.StudentID, SupervisorID: req.NewSupervisorID,
	})
}
