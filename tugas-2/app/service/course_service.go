package service

import (
	"errors"
	"fmt"
	"strconv"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
	"github.com/gofiber/fiber/v2"
)

type CourseService struct {
	courses       repository.CourseRepository
	outbox        repository.OutboxRepository
	notifications repository.NotificationRepository
	tx            *repository.TxManager
}

func NewCourseService(c repository.CourseRepository, o repository.OutboxRepository, n repository.NotificationRepository, tx *repository.TxManager) *CourseService {
	return &CourseService{c, o, n, tx}
}

func (s *CourseService) Patch(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return helper.Fail(c, 400, "id kelas tidak valid")
	}
	var req model.PatchCourseRequest
	if err := c.BodyParser(&req); err != nil || req.Quota < 1 || req.Version < 1 {
		return helper.Fail(c, 400, "kuota dan version wajib valid")
	}
	var updated model.Course
	err = s.tx.WithTx(c.UserContext(), func(tx repository.DBTX) error {
		courses, outbox := s.courses.WithTx(tx), s.outbox.WithTx(tx)
		if _, err := courses.LockByID(c.UserContext(), id); err != nil {
			return err
		}
		course, err := courses.UpdateQuota(c.UserContext(), id, req.Version, req.Quota)
		if err != nil {
			return err
		}
		recipients, err := courses.Recipients(c.UserContext(), id)
		if err != nil {
			return err
		}
		for i := range recipients {
			recipients[i].Subject = fmt.Sprintf("KRS: kuota %s %s berubah menjadi %d", course.Code, course.Name, course.Quota)
			recipients[i].Body = fmt.Sprintf("Kuota kelas %s (%s) berubah menjadi %d.", course.Code, course.Name, course.Quota)
		}
		_, err = outbox.Insert(c.UserContext(), model.EventCourseQuotaChanged, model.CourseEvent{CourseID: id, CourseCode: course.Code, CourseName: course.Name, Quota: course.Quota, Recipients: recipients})
		if err != nil {
			return err
		}
		updated = course
		return nil
	})
	if err != nil {
		return courseError(c, err)
	}
	return helper.Success(c, 200, "kuota kelas berhasil diperbarui", updated)
}

func (s *CourseService) Enroll(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return helper.Fail(c, 400, "id kelas tidak valid")
	}
	userID := c.Locals("user_id").(int)
	var course model.Course
	var recipient model.NotificationRecipient
	err = s.tx.WithTx(c.UserContext(), func(tx repository.DBTX) error {
		cr := s.courses.WithTx(tx)
		var e error
		course, recipient, e = cr.Enroll(c.UserContext(), id, userID)
		if e != nil {
			return e
		}
		recipient.Subject = fmt.Sprintf("KRS: berhasil terdaftar di %s %s", course.Code, course.Name)
		recipient.Body = fmt.Sprintf("Anda berhasil terdaftar di kelas %s (%s).", course.Code, course.Name)
		_, e = s.outbox.WithTx(tx).Insert(c.UserContext(), model.EventEnrollmentCreated, model.CourseEvent{CourseID: id, CourseCode: course.Code, CourseName: course.Name, Recipients: []model.NotificationRecipient{recipient}})
		return e
	})
	if err != nil {
		return courseError(c, err)
	}
	return helper.Success(c, 201, "berhasil mendaftar kelas", course)
}

func (s *CourseService) Drop(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return helper.Fail(c, 400, "id kelas tidak valid")
	}
	userID := c.Locals("user_id").(int)
	var course model.Course
	var recipient model.NotificationRecipient
	err = s.tx.WithTx(c.UserContext(), func(tx repository.DBTX) error {
		var e error
		course, recipient, e = s.courses.WithTx(tx).Drop(c.UserContext(), id, userID)
		if e != nil {
			return e
		}
		recipient.Subject = fmt.Sprintf("KRS: keluar dari %s %s", course.Code, course.Name)
		recipient.Body = fmt.Sprintf("Anda keluar dari kelas %s (%s).", course.Code, course.Name)
		_, e = s.outbox.WithTx(tx).Insert(c.UserContext(), model.EventEnrollmentDropped, model.CourseEvent{CourseID: id, CourseCode: course.Code, CourseName: course.Name, Recipients: []model.NotificationRecipient{recipient}})
		return e
	})
	if err != nil {
		return courseError(c, err)
	}
	return helper.Success(c, 200, "berhasil keluar dari kelas", course)
}
func (s *CourseService) Mine(c *fiber.Ctx) error {
	id := c.Locals("user_id").(int)
	items, err := s.notifications.FindByUser(c.UserContext(), id, 100)
	if err != nil {
		return helper.Fail(c, 500, "gagal mengambil notifikasi")
	}
	return helper.Success(c, 200, "daftar notifikasi", items)
}
func courseError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, repository.ErrCourseNotFound), errors.Is(err, repository.ErrNotEnrolled):
		return helper.Fail(c, 404, err.Error())
	case errors.Is(err, repository.ErrCourseVersion):
		return helper.Fail(c, 409, err.Error())
	case errors.Is(err, repository.ErrCourseFull), errors.Is(err, repository.ErrAlreadyEnrolled), errors.Is(err, repository.ErrCourseQuotaBelowEnrolled), errors.Is(err, repository.ErrStudentNotOwned):
		return helper.Fail(c, 409, err.Error())
	case errors.Is(err, repository.ErrRecipientUnavailable):
		return helper.Fail(c, 422, err.Error())
	default:
		return helper.Fail(c, 500, "gagal memproses kelas")
	}
}
