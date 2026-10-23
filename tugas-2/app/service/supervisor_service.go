package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

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

// PATCH /supervisors/:id
func (s *SupervisorService) Patch(c *fiber.Ctx) error {
	supervisorID, valid := helper.ParamID(c)
	if !valid || int64(supervisorID) > 1<<31-1 {
		return fiber.NewError(fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	var req model.UpdateSupervisorRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	validationErrors := make(map[string]string)
	if req.ExpectedVersion < 1 || int64(req.ExpectedVersion) > 1<<31-1 {
		validationErrors["version"] = "wajib berupa angka positif"
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		req.Name = &name
		if name == "" {
			validationErrors["name"] = "tidak boleh kosong"
		} else if utf8.RuneCountInString(name) > 100 {
			validationErrors["name"] = "maksimal 100 karakter"
		}
	}
	if req.Quota != nil {
		switch {
		case *req.Quota < 0:
			validationErrors["quota"] = "nilai minimal 0"
		case int64(*req.Quota) > 1<<31-1:
			validationErrors["quota"] = "nilai melebihi batas kolom database"
		}
	}
	if req.Name == nil && req.Quota == nil {
		validationErrors["body"] = "minimal satu field name atau quota harus diubah"
	}
	if len(validationErrors) > 0 {
		return helper.FailValidation(c, validationErrors)
	}

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var updated model.Supervisor
	err := s.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.repo.WithTx(tx)
		if err := repo.LockSupervisors(ctx, supervisorID); err != nil {
			return err
		}
		var err error
		updated, err = repo.UpdateSupervisor(ctx, supervisorID, req.ExpectedVersion, req.Name, req.Quota)
		return err
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrSupervisorNotFound):
			return fiber.NewError(fiber.StatusNotFound, "supervisor tidak ditemukan")
		case errors.Is(err, repository.ErrSupervisorVersion):
			return fiber.NewError(fiber.StatusConflict, "versi supervisor sudah berubah; muat ulang data lalu coba lagi")
		case errors.Is(err, repository.ErrQuotaBelowSupervisedCount):
			return fiber.NewError(fiber.StatusConflict, "kuota tidak boleh lebih kecil dari jumlah mahasiswa terbimbing")
		default:
			return fiber.NewError(fiber.StatusInternalServerError, "gagal memperbarui supervisor")
		}
	}

	return helper.Success(c, fiber.StatusOK, "supervisor berhasil diperbarui", updated)
}
