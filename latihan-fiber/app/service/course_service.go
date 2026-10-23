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

type CourseService struct {
	courses repository.CourseRepository
	tx      *repository.TxManager
}

func NewCourseService(courses repository.CourseRepository, tx *repository.TxManager) *CourseService {
	return &CourseService{courses: courses, tx: tx}
}

func (s *CourseService) List(c *fiber.Ctx) error {
	if _, ok := helper.CurrentUser(c); !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	page, err := queryPositiveInt(c, "page", 1)
	if err != nil {
		return helper.BadRequest("page harus berupa bilangan bulat positif")
	}
	limit, err := queryPositiveInt(c, "limit", 10)
	if err != nil || limit > 100 {
		return helper.BadRequest("limit harus antara 1 sampai 100")
	}
	if page-1 > int(^uint(0)>>1)/limit {
		return helper.BadRequest("page terlalu besar")
	}
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	result, err := s.courses.List(ctx, repository.CourseListQuery{
		Page: page, Limit: limit, Offset: (page - 1) * limit,
		Search: strings.TrimSpace(c.Query("search")),
	})
	if err != nil {
		return helper.Internal(err)
	}
	return helper.SuccessList(c, "daftar course berhasil diambil", result.Courses, &model.Meta{
		Page: result.Page, Limit: result.Limit, Total: result.Total, TotalPages: result.TotalPages,
	})
}

func (s *CourseService) Mine(c *fiber.Ctx) error {
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	courses, err := s.courses.FindByStudent(ctx, current.UserID)
	if err != nil {
		return helper.Internal(err)
	}
	total := 0
	for _, course := range courses {
		total += course.Credits
	}
	return helper.Success(c, fiber.StatusOK, "KRS berhasil diambil", fiber.Map{
		"courses": courses, "total_credits": total, "max_credits": MaxCreditsPerSemester,
	})
}

func (s *CourseService) Get(c *fiber.Ctx) error {
	if _, ok := helper.CurrentUser(c); !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	course, err := s.courses.FindByID(ctx, id)
	if err != nil {
		return translateCourseError(err)
	}
	return helper.Success(c, fiber.StatusOK, "course ditemukan", course)
}

func (s *CourseService) Enroll(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}
	var enrolled model.Course
	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.courses.WithTx(tx)
		if err := repo.LockStudent(ctx, current.UserID); err != nil {
			return err
		}
		course, err := repo.FindByID(ctx, courseID)
		if err != nil {
			return err
		}
		taken, err := repo.SumCredits(ctx, current.UserID)
		if err != nil {
			return err
		}
		if !WithinCreditLimit(taken, course.Credits) {
			return errCreditLimit
		}
		enrolled, err = repo.IncrementEnrolled(ctx, courseID)
		if err != nil {
			return err
		}
		return repo.InsertEnrollment(ctx, courseID, current.UserID)
	})
	if err != nil {
		return translateCourseError(err)
	}
	return helper.Created(c, "berhasil mendaftar kelas", enrolled, "/api/v1/courses/"+strconv.Itoa(courseID))
}

func (s *CourseService) Drop(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}
	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.courses.WithTx(tx)
		if err := repo.LockStudent(ctx, current.UserID); err != nil {
			return err
		}
		if err := repo.DeleteEnrollment(ctx, courseID, current.UserID); err != nil {
			return err
		}
		_, err := repo.DecrementEnrolled(ctx, courseID)
		return err
	})
	if err != nil {
		return translateCourseError(err)
	}
	return helper.NoContent(c)
}

func (s *CourseService) Swap(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	var req model.SwapRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if fields := helper.ValidateStruct(req); len(fields) > 0 {
		return helper.Validation(fields)
	}
	if req.FromCourseID == req.ToCourseID {
		return helper.UnprocessableValidation(map[string]string{"to_course_id": "tidak boleh sama dengan from_course_id"})
	}
	var joined model.Course
	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.courses.WithTx(tx)
		if err := repo.LockStudent(ctx, current.UserID); err != nil {
			return err
		}
		if err := repo.LockCourses(ctx, req.FromCourseID, req.ToCourseID); err != nil {
			return err
		}
		from, err := repo.FindByID(ctx, req.FromCourseID)
		if err != nil {
			return err
		}
		to, err := repo.FindByID(ctx, req.ToCourseID)
		if err != nil {
			return err
		}
		taken, err := repo.SumCredits(ctx, current.UserID)
		if err != nil {
			return err
		}
		if CreditsAfterSwap(taken, from.Credits, to.Credits) > MaxCreditsPerSemester {
			return errCreditLimit
		}
		if err := repo.DeleteEnrollment(ctx, req.FromCourseID, current.UserID); err != nil {
			return err
		}
		if _, err := repo.DecrementEnrolled(ctx, req.FromCourseID); err != nil {
			return err
		}
		joined, err = repo.IncrementEnrolled(ctx, req.ToCourseID)
		if err != nil {
			return err
		}
		return repo.InsertEnrollment(ctx, req.ToCourseID, current.UserID)
	})
	if err != nil {
		return translateCourseError(err)
	}
	return helper.Success(c, fiber.StatusOK, "berhasil pindah kelas", joined)
}

func (s *CourseService) UpdateQuota(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}
	var req model.UpdateQuotaRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if fields := helper.ValidateStruct(req); len(fields) > 0 {
		return helper.Validation(fields)
	}
	updated, err := s.courses.UpdateQuota(ctx, id, req.Quota, req.Version)
	if err != nil {
		return translateCourseError(err)
	}
	return helper.Success(c, fiber.StatusOK, "kuota berhasil diubah", updated)
}

func translateCourseError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound("kelas tidak ditemukan")
	case errors.Is(err, repository.ErrCourseFull):
		return helper.NewError(fiber.StatusConflict, "COURSE_FULL", "kuota kelas sudah penuh")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.NewError(fiber.StatusConflict, "ALREADY_ENROLLED", "Anda sudah terdaftar di kelas ini")
	case errors.Is(err, repository.ErrNotEnrolled):
		return helper.NewError(fiber.StatusConflict, "NOT_ENROLLED", "Anda tidak terdaftar di kelas tersebut")
	case errors.Is(err, errCreditLimit):
		return helper.NewError(fiber.StatusConflict, "CREDIT_LIMIT_EXCEEDED", "batas "+strconv.Itoa(MaxCreditsPerSemester)+" SKS terlampaui")
	case errors.Is(err, repository.ErrVersionConflict):
		return helper.NewError(fiber.StatusConflict, "VERSION_CONFLICT", "data kelas sudah diubah pihak lain, muat ulang lalu coba lagi")
	case errors.Is(err, repository.ErrQuotaBelowFilled):
		return helper.NewError(fiber.StatusConflict, "QUOTA_BELOW_ENROLLED", "kuota tidak boleh lebih kecil dari jumlah peserta saat ini")
	default:
		return helper.Internal(err)
	}
}

func queryPositiveInt(c *fiber.Ctx, key string, fallback int) (int, error) {
	raw := c.Query(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, errors.New("must be positive integer")
	}
	return value, nil
}
