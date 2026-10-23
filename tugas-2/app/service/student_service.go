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
	repo      repository.StudentRepository
	txManager *repository.TxManager
}

func NewStudentService(
	repo repository.StudentRepository,
	txManager *repository.TxManager,
) *StudentService {
	return &StudentService{
		repo:      repo,
		txManager: txManager,
	}
}

// GET /students
func (s *StudentService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	// Cursor pagination
	if c.Query("pagination") == "cursor" {
		q, err := helper.ParseCursorQuery(c)
		if err != nil {
			return fiber.NewError(
				fiber.StatusBadRequest,
				err.Error(),
			)
		}

		students, cursor, err := s.repo.FindAllCursor(ctx, q)
		if err != nil {
			return fiber.NewError(
				fiber.StatusInternalServerError,
				"gagal mengambil data mahasiswa",
			)
		}

		meta := model.CursorMeta{
			Limit:   q.Limit,
			HasMore: cursor != nil,
		}

		if cursor != nil {
			meta.NextCursor = helper.EncodeCursor(*cursor)
		}

		// Content Negotiation
		format, ok := helper.NegotiateFormat(c)
		if !ok {
			return fiber.NewError(
				fiber.StatusNotAcceptable,
				"format response tidak didukung",
			)
		}

		if format == "csv" {
			return helper.SuccessCSV(
				c,
				students,
			)
		}

		return helper.SuccessCursorList(
			c,
			"daftar mahasiswa berhasil diambil",
			students,
			&meta,
		)
	}

	// Offset pagination
	q := helper.ParseListQuery(c)

	students, total, err := s.repo.FindAll(ctx, q)
	if err != nil {
		return fiber.NewError(
			fiber.StatusInternalServerError,
			"gagal mengambil data mahasiswa",
		)
	}

	// Content Negotiation
	format, ok := helper.NegotiateFormat(c)
	if !ok {
		return fiber.NewError(
			fiber.StatusNotAcceptable,
			"format response tidak didukung",
		)
	}

	if format == "csv" {
		return helper.SuccessCSV(
			c,
			students,
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
		return fiber.NewError(
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	student, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "gagal mengambil data mahasiswa")
	}

	currentUser, ok := getAuthUser(c)
	if !ok {
		return fiber.NewError(
			fiber.StatusUnauthorized,
			"identitas user tidak ditemukan",
		)
	}

	perms, ok := getPermissionSet(c)
	if !ok {
		return fiber.NewError(
			fiber.StatusInternalServerError,
			"permission set tidak tersedia",
		)
	}

	if !CanAccessStudent(
		currentUser,
		student.OwnerID,
		perms,
		"student:read:any",
	) {
		return fiber.NewError(
			fiber.StatusForbidden,
			"tidak memiliki akses ke mahasiswa ini",
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
		return fiber.NewError(
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)

	if errs := ValidateCreate(req); len(errs) > 0 {
		return fiber.NewError(
			fiber.StatusUnprocessableEntity,
			"validasi gagal",
		)
	}

	userID, ok := c.Locals("user_id").(int)
	if !ok || userID <= 0 {
		return fiber.NewError(
			fiber.StatusUnauthorized,
			"identitas user tidak ditemukan",
		)
	}

	newStudent, err := s.repo.Create(
		ctx,
		model.Student{
			NIM:      req.NIM,
			Name:     req.Name,
			Grade:    req.Grade,
			IsActive: true,
			OwnerID:  userID,
		},
	)

	if err != nil {
		return translateError(err, "gagal menyimpan mahasiswa")
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
		return fiber.NewError(
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "gagal mengambil data mahasiswa")
	}

	currentUser, ok := getAuthUser(c)
	if !ok {
		return fiber.NewError(
			fiber.StatusUnauthorized,
			"identitas user tidak ditemukan",
		)
	}

	perms, ok := getPermissionSet(c)
	if !ok {
		return fiber.NewError(
			fiber.StatusInternalServerError,
			"permission set tidak tersedia",
		)
	}

	if !CanAccessStudent(
		currentUser,
		current.OwnerID,
		perms,
		"student:update:any",
	) {
		return fiber.NewError(
			fiber.StatusForbidden,
			"tidak memiliki akses untuk memperbarui mahasiswa ini",
		)
	}

	var req model.ReplaceStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)

	if errs := ValidateReplace(req); len(errs) > 0 {
		return fiber.NewError(
			fiber.StatusUnprocessableEntity,
			"validasi gagal",
		)
	}

	result, err := s.repo.Update(
		ctx,
		model.Student{
			ID:       id,
			NIM:      req.NIM,
			Name:     req.Name,
			Grade:    req.Grade,
			IsActive: req.IsActive,
			OwnerID:  current.OwnerID,
		},
	)

	if err != nil {
		return translateError(err, "gagal memperbarui mahasiswa")
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
		return fiber.NewError(
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "gagal mengambil data mahasiswa")
	}

	currentUser, ok := getAuthUser(c)
	if !ok {
		return fiber.NewError(
			fiber.StatusUnauthorized,
			"identitas user tidak ditemukan",
		)
	}

	perms, ok := getPermissionSet(c)
	if !ok {
		return fiber.NewError(
			fiber.StatusInternalServerError,
			"permission set tidak tersedia",
		)
	}

	if !CanAccessStudent(
		currentUser,
		current.OwnerID,
		perms,
		"student:update:any",
	) {
		return fiber.NewError(
			fiber.StatusForbidden,
			"tidak memiliki akses untuk memperbarui mahasiswa ini",
		)
	}

	var req model.PatchStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	if IsEmptyPatch(req) {
		return fiber.NewError(
			fiber.StatusBadRequest,
			"tidak ada field yang diubah",
		)
	}

	updated, errs := ApplyPatch(current, req)
	if len(errs) > 0 {
		return fiber.NewError(
			fiber.StatusUnprocessableEntity,
			"validasi gagal",
		)
	}

	result, err := s.repo.Update(ctx, updated)
	if err != nil {
		return translateError(err, "gagal memperbarui mahasiswa")
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
		return fiber.NewError(
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return translateError(err, "gagal menghapus mahasiswa")
	}

	return helper.NoContent(c)
}

// getAuthUser mengambil identitas user dari middleware.
func getAuthUser(c *fiber.Ctx) (model.AuthUser, bool) {
	userID, ok := c.Locals("user_id").(int)
	if !ok || userID <= 0 {
		return model.AuthUser{}, false
	}

	username, ok := c.Locals("username").(string)
	if !ok || strings.TrimSpace(username) == "" {
		return model.AuthUser{}, false
	}

	role, ok := c.Locals("role").(string)
	if !ok || strings.TrimSpace(role) == "" {
		return model.AuthUser{}, false
	}

	return model.AuthUser{
		UserID:   userID,
		Username: username,
		Role:     role,
	}, true
}

// getPermissionSet mengambil permission set dari middleware.
func getPermissionSet(c *fiber.Ctx) (*helper.PermissionSet, bool) {
	perms, ok := c.Locals("permission_set").(*helper.PermissionSet)

	return perms, ok && perms != nil
}

// translateError menerjemahkan error repository menjadi HTTP error.
func translateError(
	err error,
	generalMessage string,
) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return fiber.NewError(
			fiber.StatusNotFound,
			"mahasiswa tidak ditemukan",
		)

	case errors.Is(err, repository.ErrDuplicate):
		return fiber.NewError(
			fiber.StatusConflict,
			"NIM sudah digunakan",
		)

	default:
		return fiber.NewError(
			fiber.StatusInternalServerError,
			generalMessage,
		)
	}
}
