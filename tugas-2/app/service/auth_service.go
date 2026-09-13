package service

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
)

const refreshTokenTTL = 7 * 24 * time.Hour

type AuthService struct {
	userRepo  repository.UserRepository
	tokenRepo repository.TokenRepository
	jwt       *helper.JWTManager
}

func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	jwt *helper.JWTManager,
) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		jwt:       jwt,
	}
}

// POST /auth/register
func (s *AuthService) Register(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RegisterRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	if errs := ValidateRegister(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	// Pastikan username belum digunakan.
	if _, err := s.userRepo.FindByUsername(ctx, req.Username); err == nil {
		return helper.Fail(
			c,
			fiber.StatusConflict,
			"username sudah digunakan",
		)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			"gagal memeriksa username",
		)
	}

	passwordHash, err := helper.HashPassword(req.Password)
	if err != nil {
		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			"gagal memproses password",
		)
	}

	user := model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: passwordHash,

		// Role ditentukan server.
		Role: "user",

		IsActive: true,
	}

	created, err := s.userRepo.Create(ctx, user)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Fail(
				c,
				fiber.StatusConflict,
				"username atau email sudah digunakan",
			)
		}

		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			"gagal membuat akun",
		)
	}

	// Password tidak ikut dikirim karena json:"-".
	return helper.Created(
		c,
		"registrasi berhasil",
		created,
		"/api/v1/auth/me",
	)
}

// POST /auth/login
func (s *AuthService) Login(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.LoginRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	if errs := ValidateLogin(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}

	user, err := s.userRepo.FindByUsername(ctx, req.Username)

	// Pesan login sengaja dibuat sama untuk user tidak ditemukan
	// maupun password yang salah.
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.Fail(
				c,
				fiber.StatusUnauthorized,
				"username atau password salah",
			)
		}

		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			"gagal melakukan login",
		)
	}

	if !user.IsActive || !helper.VerifyPassword(user.Password, req.Password) {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"username atau password salah",
		)
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			"gagal membuat token",
		)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"login berhasil",
		tokens,
	)
}

// POST /auth/refresh
func (s *AuthService) Refresh(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RefreshRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	if req.RefreshToken == "" {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"refresh token tidak valid",
		)
	}

	tokenHash := helper.SHA256Hex(req.RefreshToken)

	oldToken, err := s.tokenRepo.FindActive(ctx, tokenHash)
	if err != nil {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"refresh token tidak valid",
		)
	}

	user, err := s.userRepo.FindByID(ctx, oldToken.UserID)
	if err != nil || !user.IsActive {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"refresh token tidak valid",
		)
	}

	// Rotation: token lama langsung dicabut.
	if err := s.tokenRepo.Revoke(ctx, tokenHash); err != nil {
		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			"gagal mencabut refresh token lama",
		)
	}

	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return helper.Fail(
			c,
			fiber.StatusInternalServerError,
			"gagal membuat token baru",
		)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"token berhasil diperbarui",
		tokens,
	)
}

// POST /auth/logout
func (s *AuthService) Logout(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RefreshRequest

	if err := c.BodyParser(&req); err != nil {
		return helper.Fail(
			c,
			fiber.StatusBadRequest,
			"body harus berupa JSON yang valid",
		)
	}

	if req.RefreshToken != "" {
		tokenHash := helper.SHA256Hex(req.RefreshToken)

		// Logout cukup dianggap berhasil walaupun token sudah tidak aktif.
		_ = s.tokenRepo.Revoke(ctx, tokenHash)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"logout berhasil",
		nil,
	)
}

// GET /auth/me
func (s *AuthService) Me(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	userID, ok := c.Locals("user_id").(int)
	if !ok {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"akses token tidak valid",
		)
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || !user.IsActive {
		return helper.Fail(
			c,
			fiber.StatusUnauthorized,
			"akun tidak valid",
		)
	}

	return helper.Success(
		c,
		fiber.StatusOK,
		"profil berhasil diambil",
		user,
	)
}

func (s *AuthService) issueTokenPair(
	ctx context.Context,
	user model.User,
) (model.TokenPair, error) {
	accessToken, err := s.jwt.GenerateAccessToken(user)
	if err != nil {
		return model.TokenPair{}, err
	}

	refreshToken, err := helper.RandomToken()
	if err != nil {
		return model.TokenPair{}, err
	}

	now := time.Now()

	token := model.RefreshToken{
		UserID:    user.ID,
		TokenHash: helper.SHA256Hex(refreshToken),
		ExpiresAt: now.Add(refreshTokenTTL),
		CreatedAt: now,
	}

	if err := s.tokenRepo.Save(ctx, token); err != nil {
		return model.TokenPair{}, err
	}

	return model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.jwt.AccessTTL.Seconds()),
	}, nil
}
