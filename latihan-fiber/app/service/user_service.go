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

// ---------- GET /users ----------
//
// List menggunakan cursor pagination.
//
// Query parameter:
//   - limit
//   - cursor
//   - search
//   - is_active
func (s *UserService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	// Format dipilih sebelum query dijalankan.
	format, err := helper.Negotiate(
		c,
		helper.FormatJSON,
		helper.FormatCSV,
	)
	if err != nil {
		return err
	}

	// Default limit.
	limit := c.QueryInt("limit", 10)

	// Batasi limit agar client tidak meminta data terlalu banyak.
	if limit < 1 {
		return helper.BadRequest("limit harus berupa angka positif")
	}

	if limit > 100 {
		return helper.BadRequest("limit maksimal 100")
	}

	q := model.CursorQuery{
		Limit: limit,
		Search: strings.TrimSpace(
			c.Query("search"),
		),
	}

	// Filter status aktif bila diberikan.
	if value := c.Query("is_active"); value != "" {
		switch value {
		case "true":
			active := true
			q.IsActive = &active

		case "false":
			active := false
			q.IsActive = &active

		default:
			return helper.BadRequest(
				"is_active harus bernilai true atau false",
			)
		}
	}

	// Cursor bersifat opsional.
	//
	// Jika tidak ada cursor:
	//   ambil halaman pertama.
	//
	// Jika ada cursor:
	//   decode cursor lalu ambil data setelah posisi tersebut.
	if encoded := strings.TrimSpace(c.Query("cursor")); encoded != "" {
		cursor, err := helper.DecodeCursor(encoded)
		if err != nil {
			return helper.BadRequest("cursor tidak valid")
		}

		q.After = &cursor
	}

	rows, err := s.repo.FindAfterCursor(ctx, q)
	if err != nil {
		return helper.Internal(err)
	}

	// Repository mengambil limit+1.
	// Baris tambahan hanya digunakan untuk mengetahui apakah
	// masih terdapat halaman berikutnya.
	hasMore := len(rows) > q.Limit

	if hasMore {
		rows = rows[:q.Limit]
	}

	// Jika client meminta CSV, metadata cursor tidak diperlukan.
	if format == helper.FormatCSV {
		return helper.WriteUsersCSV(c, rows)
	}

	meta := &model.CursorMeta{
		Limit:   q.Limit,
		HasMore: hasMore,
	}

	// Cursor berikutnya dibuat dari baris terakhir yang benar-benar
	// dikirim kepada client.
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]

		meta.NextCursor = helper.EncodeCursor(
			last.CreatedAt,
			last.ID,
		)
	}

	return helper.SuccessCursor(
		c,
		"daftar user berhasil diambil",
		rows,
		meta,
	)
}

// ---------- GET /users/:id ----------
func (s *UserService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	// Pemeriksaan hak akses dilakukan SEBELUM data diambil.
	if !CanAccessUser(
		current,
		id,
		s.perms,
		"user:read:any",
	) {
		return helper.Forbidden(
			"tidak berhak mengakses data user lain",
		)
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "user")
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"user ditemukan",
		user,
	)
}

// ---------- POST /users ----------
func (s *UserService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.CreateUserRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest(
			"body harus berupa JSON yang valid",
		)
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	// Validasi deklaratif melalui tag pada struct request.
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	newUser, err := s.repo.Create(ctx, model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		IsActive: true,
	})

	if err != nil {
		return translateError(err, "user")
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
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	if !CanAccessUser(
		current,
		id,
		s.perms,
		"user:update:any",
	) {
		return helper.Forbidden(
			"tidak berhak mengubah data user lain",
		)
	}

	var req model.ReplaceUserRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest(
			"body harus berupa JSON yang valid",
		)
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	// Role tidak diubah melalui PUT.
	// Perubahan role memiliki endpoint khusus.
	existingUser, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "user")
	}

	result, err := s.repo.Update(ctx, model.User{
		ID:       id,
		Username: req.Username,
		Email:    req.Email,
		Role:     existingUser.Role,
		IsActive: req.IsActive,
	})

	if err != nil {
		return translateError(err, "user")
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
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	if !CanAccessUser(
		currentUser,
		id,
		s.perms,
		"user:update:any",
	) {
		return helper.Forbidden(
			"tidak berhak mengubah data user lain",
		)
	}

	var req model.PatchUserRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest(
			"body harus berupa JSON yang valid",
		)
	}

	// Aturan hubungan antar-field.
	if IsEmptyPatch(req) {
		return helper.BadRequest(
			"tidak ada field yang diubah",
		)
	}

	// Validasi field dilakukan melalui tag.
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "user")
	}

	// ApplyPatch pada versi project saat ini mengembalikan
	// user hasil perubahan dan map error.
	updated, errs := ApplyPatch(current, req)

	if len(errs) > 0 {
		return helper.Validation(errs)
	}

	result, err := s.repo.Update(ctx, updated)
	if err != nil {
		return translateError(err, "user")
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"user berhasil diperbarui sebagian",
		result,
	)
}

// ---------- PATCH /users/:id/role ----------
//
// Dijaga middleware dengan permission role:assign.
func (s *UserService) AssignRole(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.AssignRoleRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest(
			"body harus berupa JSON yang valid",
		)
	}

	req.Role = strings.TrimSpace(req.Role)

	if errs := ValidateAssignRole(
		current,
		id,
		req,
		s.perms,
	); len(errs) > 0 {
		return helper.Validation(errs)
	}

	result, err := s.repo.UpdateRole(
		ctx,
		id,
		req.Role,
	)

	if err != nil {
		return translateError(err, "user")
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
		return helper.Unauthorized("belum terautentikasi")
	}

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	// Tidak boleh menghapus akun sendiri.
	if current.UserID == id {
		return helper.Forbidden(
			"tidak boleh menghapus akun sendiri",
		)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return translateError(err, "user")
	}

	return helper.NoContent(c)
}

// ---------- translateError ----------

// translateError mengubah error repository menjadi AppError.
func translateError(err error, entity string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound(
			entity + " tidak ditemukan",
		)

	case errors.Is(err, repository.ErrDuplicate):
		return helper.Conflict(
			"username sudah dipakai",
		)

	default:
		return helper.Internal(err)
	}
}
