package main

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
	"api-students/app/repository"
)

type StudentHandler struct {
	repo repository.StudentRepository
}

func NewStudentHandler(repo repository.StudentRepository) *StudentHandler {
	return &StudentHandler{
		repo: repo,
	}
}

func reqCtx(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithCancel(c.UserContext())
}

func paramID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, false
	}

	return id, true
}

func terjemahkanError(c *fiber.Ctx, err error, pesan string) error {

	switch {

	case errors.Is(err, repository.ErrNotFound):
		return fail(c, fiber.StatusNotFound,
			"mahasiswa tidak ditemukan")

	case errors.Is(err, repository.ErrDuplicate):
		return fail(c, fiber.StatusConflict,
			"NIM sudah digunakan")

	default:
		return fail(c,
			fiber.StatusInternalServerError,
			pesan)
	}
}

// GET /students
func (h *StudentHandler) List(c *fiber.Ctx) error {

	ctx, cancel := reqCtx(c)
	defer cancel()

	q := parseListQuery(c)

	students, total, err := h.repo.FindAll(ctx, q)
	if err != nil {
		return fail(
			c,
			fiber.StatusInternalServerError,
			"gagal mengambil data mahasiswa",
		)
	}

	totalPages := 0
	if q.Limit > 0 {
		totalPages = (total + q.Limit - 1) / q.Limit
	}

	return okList(
		c,
		"daftar mahasiswa berhasil diambil",
		students,
		&model.Meta{
			Page:       q.Page,
			Limit:      q.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	)
}

// GET /students/:id
func (h *StudentHandler) Get(c *fiber.Ctx) error {

	ctx, cancel := reqCtx(c)
	defer cancel()

	id, valid := paramID(c)
	if !valid {
		return fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	student, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return terjemahkanError(
			c,
			err,
			"gagal mengambil data mahasiswa",
		)
	}

	return ok(c, "mahasiswa ditemukan", student)
}

// POST /students
func (h *StudentHandler) Create(c *fiber.Ctx) error {

	ctx, cancel := reqCtx(c)
	defer cancel()

	var req model.CreateStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)

	errs := map[string]string{}

	if req.NIM == "" {
		errs["nim"] = "wajib diisi"
	}

	if req.Name == "" {
		errs["name"] = "wajib diisi"
	}

	if req.Grade < 0 || req.Grade > 100 {
		errs["grade"] = "grade harus berada pada rentang 0-100"
	}

	if len(errs) > 0 {
		return failValidation(c, errs)
	}

	baru, err := h.repo.Create(ctx, model.Student{
		NIM:      req.NIM,
		Name:     req.Name,
		Grade:    req.Grade,
		IsActive: true,
	})

	if err != nil {
		return terjemahkanError(
			c,
			err,
			"gagal menyimpan mahasiswa",
		)
	}

	return created(
		c,
		"mahasiswa berhasil ditambahkan",
		baru,
		"/api/v1/students/"+strconv.Itoa(baru.ID),
	)
}

// PUT /students/:id
func (h *StudentHandler) Replace(c *fiber.Ctx) error {

	ctx, cancel := reqCtx(c)
	defer cancel()

	id, valid := paramID(c)
	if !valid {
		return fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	var req model.ReplaceStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)

	errs := map[string]string{}

	if req.NIM == "" {
		errs["nim"] = "wajib diisi"
	}

	if req.Name == "" {
		errs["name"] = "wajib diisi"
	}

	if req.Grade < 0 || req.Grade > 100 {
		errs["grade"] = "grade harus berada pada rentang 0-100"
	}

	if len(errs) > 0 {
		return failValidation(c, errs)
	}

	hasil, err := h.repo.Update(ctx, model.Student{
		ID:       id,
		NIM:      req.NIM,
		Name:     req.Name,
		Grade:    req.Grade,
		IsActive: req.IsActive,
	})

	if err != nil {
		return terjemahkanError(
			c,
			err,
			"gagal memperbarui mahasiswa",
		)
	}

	return ok(
		c,
		"mahasiswa berhasil diperbarui",
		hasil,
	)
}

// PATCH /students/:id
func (h *StudentHandler) Patch(c *fiber.Ctx) error {

	ctx, cancel := reqCtx(c)
	defer cancel()

	id, valid := paramID(c)
	if !valid {
		return fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	var req model.PatchStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	if req.NIM == nil &&
		req.Name == nil &&
		req.Grade == nil &&
		req.IsActive == nil {
		return fail(
			c,
			fiber.StatusBadRequest,
			"tidak ada field yang diubah",
		)
	}

	student, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return terjemahkanError(
			c,
			err,
			"gagal mengambil data mahasiswa",
		)
	}

	if req.NIM != nil {
		if strings.TrimSpace(*req.NIM) == "" {
			return failValidation(c, map[string]string{
				"nim": "NIM tidak boleh kosong",
			})
		}
		student.NIM = strings.TrimSpace(*req.NIM)
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return failValidation(c, map[string]string{
				"name": "Nama tidak boleh kosong",
			})
		}
		student.Name = strings.TrimSpace(*req.Name)
	}

	if req.Grade != nil {
		if *req.Grade < 0 || *req.Grade > 100 {
			return failValidation(c, map[string]string{
				"grade": "Grade harus berada pada rentang 0-100",
			})
		}
		student.Grade = *req.Grade
	}

	if req.IsActive != nil {
		student.IsActive = *req.IsActive
	}

	hasil, err := h.repo.Update(ctx, student)
	if err != nil {
		return terjemahkanError(
			c,
			err,
			"gagal memperbarui mahasiswa",
		)
	}

	return ok(
		c,
		"mahasiswa berhasil diperbarui",
		hasil,
	)
}

// DELETE /students/:id
func (h *StudentHandler) Delete(c *fiber.Ctx) error {

	ctx, cancel := reqCtx(c)
	defer cancel()

	id, valid := paramID(c)
	if !valid {
		return fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	if err := h.repo.Delete(ctx, id); err != nil {
		return terjemahkanError(
			c,
			err,
			"gagal menghapus mahasiswa",
		)
	}

	return noContent(c)
}
