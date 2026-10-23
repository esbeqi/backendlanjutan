package model

import "time"

type Course struct {
	ID            int       `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Credits       int       `json:"credits"`
	Quota         int       `json:"quota"`
	EnrolledCount int       `json:"enrolled_count"`
	Available     int       `json:"available"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
}

type Enrollment struct {
	ID        int64     `json:"id"`
	CourseID  int       `json:"course_id"`
	UserID    int       `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateEnrollmentRequest struct {
	CourseID int `json:"course_id" validate:"required,gt=0"`
}

type SwapEnrollmentRequest struct {
	EnrollmentID int64 `json:"enrollment_id" validate:"required,gt=0"`
	NewCourseID  int   `json:"new_course_id" validate:"required,gt=0"`
}

type SwapRequest struct {
	FromCourseID int `json:"from_course_id" validate:"required,min=1"`
	ToCourseID   int `json:"to_course_id" validate:"required,min=1"`
}

type UpdateQuotaRequest struct {
	Quota   int `json:"quota" validate:"required,min=1,max=500"`
	Version int `json:"version" validate:"required,min=1"`
}
