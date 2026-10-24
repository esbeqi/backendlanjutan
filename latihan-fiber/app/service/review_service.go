package service

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
	"latihan-fiber/helper"
)

// ReviewService coordinates course enrollment in PostgreSQL and reviews in
// MongoDB. Cross-database business rules belong here because neither database
// can enforce a foreign key or transaction across the other.
type ReviewService struct {
	reviews repository.ReviewRepository
	courses repository.CourseRepository
	perms   *helper.PermissionSet
}

func NewReviewService(
	reviews repository.ReviewRepository,
	courses repository.CourseRepository,
	perms *helper.PermissionSet,
) *ReviewService {
	return &ReviewService{reviews: reviews, courses: courses, perms: perms}
}

func (s *ReviewService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id kelas harus berupa angka positif")
	}

	var request model.CreateReviewRequest
	if err := c.BodyParser(&request); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	request.Comment = strings.TrimSpace(request.Comment)
	request.Tags = NormalizeTags(request.Tags)
	if fields := helper.ValidateStruct(request); len(fields) > 0 {
		return helper.UnprocessableValidation(fields)
	}

	if _, err := s.courses.FindByID(ctx, courseID); err != nil {
		return translateReviewError(err)
	}
	enrolled, err := s.courses.IsEnrolled(ctx, courseID, current.UserID)
	if err != nil {
		return helper.Internal(err)
	}
	if !enrolled {
		return helper.NewError(fiber.StatusForbidden, "NOT_ENROLLED", "hanya peserta kelas yang boleh menulis ulasan")
	}

	review, err := s.reviews.Create(ctx, model.Review{
		CourseID: courseID,
		UserID:   current.UserID,
		Username: current.Username,
		Rating:   request.Rating,
		Comment:  request.Comment,
		Tags:     request.Tags,
	})
	if err != nil {
		return translateReviewError(err)
	}
	return helper.Created(c, "ulasan berhasil disimpan", review, "/api/v1/reviews/"+review.ID)
}

func (s *ReviewService) ListByCourse(c *fiber.Ctx) error {
	if _, ok := helper.CurrentUser(c); !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id kelas harus berupa angka positif")
	}
	limit := c.QueryInt("limit", 10)
	if limit < 1 || limit > 50 {
		limit = 10
	}
	query := model.ReviewQuery{
		CourseID: courseID,
		Tag:      strings.ToLower(strings.TrimSpace(c.Query("tag"))),
		Before:   c.Query("before"),
		Limit:    limit + 1,
	}
	reviews, err := s.reviews.FindByCourse(ctx, query)
	if err != nil {
		return translateReviewError(err)
	}

	hasMore := len(reviews) > limit
	if hasMore {
		reviews = reviews[:limit]
	}
	nextBefore := ""
	if hasMore {
		nextBefore = reviews[len(reviews)-1].ID
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "daftar ulasan berhasil diambil",
		"data":    reviews,
		"meta": fiber.Map{
			"limit":       limit,
			"has_more":    hasMore,
			"next_before": nextBefore,
		},
	})
}

func (s *ReviewService) Summary(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id kelas harus berupa angka positif")
	}
	if _, err := s.courses.FindByID(ctx, courseID); err != nil {
		return translateReviewError(err)
	}
	summary, err := s.reviews.Summary(ctx, courseID)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "ringkasan ulasan berhasil diambil", summary)
}

func (s *ReviewService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	review, err := s.reviews.FindByID(ctx, c.Params("id"))
	if err != nil {
		return translateReviewError(err)
	}
	return helper.Success(c, fiber.StatusOK, "ulasan ditemukan", review)
}

func (s *ReviewService) Update(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	var request model.UpdateReviewRequest
	if err := c.BodyParser(&request); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if request.Rating == nil && request.Comment == nil && request.Tags == nil {
		return helper.BadRequest("tidak ada field yang diubah")
	}
	if request.Comment != nil {
		comment := strings.TrimSpace(*request.Comment)
		request.Comment = &comment
	}
	if request.Tags != nil {
		tags := NormalizeTags(*request.Tags)
		request.Tags = &tags
	}
	if fields := helper.ValidateStruct(request); len(fields) > 0 {
		return helper.UnprocessableValidation(fields)
	}

	updated, err := s.reviews.UpdateOwn(ctx, c.Params("id"), current.UserID, request)
	if err != nil {
		return translateReviewError(err)
	}
	return helper.Success(c, fiber.StatusOK, "ulasan berhasil diubah", updated)
}

func (s *ReviewService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	anyOwner := s.perms.Can(current.Role, "review:moderate")
	if err := s.reviews.Delete(ctx, c.Params("id"), current.UserID, anyOwner); err != nil {
		return translateReviewError(err)
	}
	return helper.NoContent(c)
}

func (s *ReviewService) Reply(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	var request model.ReplyRequest
	if err := c.BodyParser(&request); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	request.Body = strings.TrimSpace(request.Body)
	if fields := helper.ValidateStruct(request); len(fields) > 0 {
		return helper.UnprocessableValidation(fields)
	}
	updated, err := s.reviews.AddReply(ctx, c.Params("id"), model.Reply{
		UserID:   current.UserID,
		Username: current.Username,
		Body:     request.Body,
	})
	if err != nil {
		return translateReviewError(err)
	}
	return helper.Success(c, fiber.StatusCreated, "balasan berhasil ditambahkan", updated)
}

func (s *ReviewService) MarkHelpful(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	updated, err := s.reviews.MarkHelpful(ctx, c.Params("id"), current.UserID)
	if err != nil {
		return translateReviewError(err)
	}
	return helper.Success(c, fiber.StatusOK, "terima kasih atas penilaiannya", updated)
}

func translateReviewError(err error) error {
	switch {
	case errors.Is(err, repository.ErrInvalidReviewID):
		return helper.BadRequest("id ulasan tidak valid")
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound("data tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.NewError(fiber.StatusConflict, "ALREADY_REVIEWED", "Anda sudah menulis ulasan untuk kelas ini, ubah ulasan yang ada")
	case errors.Is(err, repository.ErrNotOwner):
		return helper.Forbidden("hanya pemilik ulasan yang boleh melakukan ini")
	case errors.Is(err, repository.ErrOwnReview):
		return helper.NewError(fiber.StatusConflict, "OWN_REVIEW", "tidak dapat menilai ulasan sendiri")
	case errors.Is(err, repository.ErrAlreadyVoted):
		return helper.NewError(fiber.StatusConflict, "ALREADY_VOTED", "Anda sudah menilai ulasan ini")
	case errors.Is(err, repository.ErrReplyLimit):
		return helper.NewError(fiber.StatusConflict, "REPLY_LIMIT", "jumlah balasan sudah mencapai batas")
	default:
		return helper.Internal(err)
	}
}
