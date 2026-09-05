package service

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
)

type StudentService struct {
	repo repository.StudentRepository
}

func NewStudentService(repo repository.StudentRepository) *StudentService {
	return &StudentService{
		repo: repo,
	}
}

// GET /students
func (s *StudentService) List(c *fiber.Ctx) error {

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	q := helper.ParseListQuery(c)

	students, total, err := s.repo.FindAll(ctx, q)
	if err != nil {
		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			"gagal mengambil data mahasiswa",
		)
	}

	return helper.SuccessList(
		c,
		"daftar mahasiswa berhasil diambil",
		students,
		&model.Meta{
			Page:       q.Page,
			Limit:      q.Limit,
			Total:      total,
			TotalPages: CountTotalPages(total, q.Limit),
		},
	)
}

// GET /students/:id
func (s *StudentService) Get(c *fiber.Ctx) error {

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	student, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(
			c,
			err,
			"gagal mengambil data mahasiswa",
		)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"mahasiswa ditemukan",
		student,
	)
}

// POST /students
func (s *StudentService) Create(c *fiber.Ctx) error {

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.CreateStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)

	if errs := ValidateCreate(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	newStudent, err := s.repo.Create(ctx, model.Student{
		NIM:      req.NIM,
		Name:     req.Name,
		Grade:    req.Grade,
		IsActive: true,
	})

	if err != nil {
		return translateError(
			c,
			err,
			"gagal menyimpan mahasiswa",
		)
	}

	return helper.Created(
		c,
		"mahasiswa berhasil ditambahkan",
		newStudent,
		"/api/v1/students/"+strconv.Itoa(newStudent.ID),
	)
}

// PUT /students/:id
func (s *StudentService) Replace(c *fiber.Ctx) error {

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	var req model.ReplaceStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)

	if errs := ValidateReplace(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	result, err := s.repo.Update(ctx, model.Student{
		ID:       id,
		NIM:      req.NIM,
		Name:     req.Name,
		Grade:    req.Grade,
		IsActive: req.IsActive,
	})

	if err != nil {
		return translateError(
			c,
			err,
			"gagal memperbarui mahasiswa",
		)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"mahasiswa berhasil diperbarui",
		result,
	)
}

// PATCH /students/:id
func (s *StudentService) Patch(c *fiber.Ctx) error {

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	var req model.PatchStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	if IsEmptyPatch(req) {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"tidak ada field yang diubah",
		)
	}

	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(
			c,
			err,
			"gagal mengambil data mahasiswa",
		)
	}

	updated, errs := ApplyPatch(current, req)
	if len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	result, err := s.repo.Update(ctx, updated)
	if err != nil {
		return translateError(
			c,
			err,
			"gagal memperbarui mahasiswa",
		)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"mahasiswa berhasil diperbarui",
		result,
	)
}

// DELETE /students/:id
func (s *StudentService) Delete(c *fiber.Ctx) error {

	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return translateError(
			c,
			err,
			"gagal menghapus mahasiswa",
		)
	}

	return helper.NoContent(c)
}

func translateError(c *fiber.Ctx, err error, generalMessage string) error {
	switch {

	case errors.Is(err, repository.ErrNotFound):
		return helper.Fail(
			c,
			fiber.StatusNotFound,
			"mahasiswa tidak ditemukan",
		)

	case errors.Is(err, repository.ErrDuplicate):
		return helper.Fail(
			c,
			fiber.StatusConflict,
			"NIM sudah digunakan",
		)

	default:
		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			generalMessage,
		)
	}
}
