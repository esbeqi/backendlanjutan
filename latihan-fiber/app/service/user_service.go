package service

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
	"latihan-fiber/helper"
)

// UserService memegang dua tanggung jawab sekaligus pada struktur baku
// mata kuliah ini: menerima *fiber.Ctx (peran controller) dan menjalankan
// business rules (peran use case).
type UserService struct {
	repo  repository.UserRepository
	perms *helper.PermissionSet
}

// NewUserService menerima INTERFACE, bukan struct konkret.
func NewUserService(
	repo repository.UserRepository,
	perms *helper.PermissionSet,
) *UserService {
	return &UserService{
		repo:  repo,
		perms: perms,
	}
}

func (s *UserService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	q := helper.ParseListQuery(c)

	users, total, err := s.repo.FindAll(ctx, q)
	if err != nil {
		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			"gagal mengambil data user",
		)
	}

	return helper.SuccessList(
		c,
		"daftar user berhasil diambil",
		users,
		&model.Meta{
			Page:       q.Page,
			Limit:      q.Limit,
			Total:      total,
			TotalPages: CountTotalPages(total, q.Limit),
		},
	)
}

// ---------- GET /users/:id ----------
func (s *UserService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"belum terautentikasi",
		)
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	// Pemeriksaan hak akses dilakukan SEBELUM data diambil.
	// Bila dibalik, perbedaan waktu tanggap antara 403 dan 404
	// dapat membantu memetakan ID yang ada di dalam sistem.
	if !CanAccessUser(
		current,
		id,
		s.perms,
		"user:read:any",
	) {
		return helper.Fail(
			c,
			fiber.StatusForbidden,
			"tidak berhak mengakses data user lain",
		)
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(
			c,
			err,
			"gagal mengambil data user",
		)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"user ditemukan",
		user,
	)
}

func (s *UserService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.CreateUserRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	// Business rules dipanggil, bukan ditulis ulang di sini.
	if errs := ValidateCreate(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	newUser, err := s.repo.Create(ctx, model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		IsActive: true,
	})

	if err != nil {
		return translateError(
			c,
			err,
			"gagal menyimpan user",
		)
	}

	return helper.Created(
		c,
		"user berhasil dibuat",
		newUser,
		"/api/v1/users/"+strconv.Itoa(newUser.ID),
	)
}

// ---------- PUT /users/:id ----------
func (s *UserService) Replace(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"belum terautentikasi",
		)
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	// Ownership atau permission user:update:any diperiksa
	// sebelum data diproses lebih lanjut.
	if !CanAccessUser(
		current,
		id,
		s.perms,
		"user:update:any",
	) {
		return helper.Fail(
			c,
			fiber.StatusForbidden,
			"tidak berhak mengubah data user lain",
		)
	}

	var req model.ReplaceUserRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	if errs := ValidateReplace(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	// Role tidak diubah melalui PUT.
	// Perubahan role memiliki endpoint khusus PATCH /:id/role
	// dan permission khusus role:assign.
	// Karena repository.Update juga menyimpan role, kita perlu
	// mempertahankan role yang sudah dimiliki user.
	existingUser, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(
			c,
			err,
			"gagal mengambil data user",
		)
	}

	result, err := s.repo.Update(ctx, model.User{
		ID:       id,
		Username: strings.TrimSpace(req.Username),
		Email:    strings.TrimSpace(req.Email),
		Role:     existingUser.Role,
		IsActive: req.IsActive,
	})

	if err != nil {
		return translateError(
			c,
			err,
			"gagal memperbarui user",
		)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"user berhasil diganti seluruhnya",
		result,
	)
}

// ---------- PATCH /users/:id ----------
func (s *UserService) Patch(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	currentUser, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"belum terautentikasi",
		)
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	// Ownership atau permission user:update:any diperiksa
	// sebelum data diambil.
	if !CanAccessUser(
		currentUser,
		id,
		s.perms,
		"user:update:any",
	) {
		return helper.Fail(
			c,
			fiber.StatusForbidden,
			"tidak berhak mengubah data user lain",
		)
	}

	var req model.PatchUserRequest

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
			"gagal mengambil data user",
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
			"gagal memperbarui user",
		)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"user berhasil diperbarui sebagian",
		result,
	)
}

// ---------- PATCH /users/:id/role ----------
// Dijaga middleware dengan permission role:assign.
func (s *UserService) AssignRole(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"belum terautentikasi",
		)
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	var req model.AssignRoleRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	if errs := ValidateAssignRole(
		current,
		id,
		req,
		s.perms,
	); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	result, err := s.repo.UpdateRole(
		ctx,
		id,
		strings.TrimSpace(req.Role),
	)
	if err != nil {
		return translateError(
			c,
			err,
			"gagal mengubah role user",
		)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"role user berhasil diubah",
		result,
	)
}

// ---------- DELETE /users/:id ----------
func (s *UserService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"belum terautentikasi",
		)
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"id harus berupa angka positif",
		)
	}

	// Punya permission user:delete tidak berarti boleh
	// menghapus akun sendiri.
	if current.UserID == id {
		return helper.Fail(
			c,
			fiber.StatusForbidden,
			"tidak boleh menghapus akun sendiri",
		)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return translateError(
			c,
			err,
			"gagal menghapus user",
		)
	}

	return helper.NoContent(c)
}

// translateError memetakan error milik repository menjadi status HTTP.
func translateError(
	c *fiber.Ctx,
	err error,
	generalMessage string,
) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.Fail(
			c,
			fiber.StatusNotFound,
			"user tidak ditemukan",
		)

	case errors.Is(err, repository.ErrDuplicate):
		return helper.Fail(
			c,
			fiber.StatusConflict,
			"username sudah dipakai",
		)

	default:
		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			generalMessage,
		)
	}
}
