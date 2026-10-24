package model

import "time"

// Review adalah representasi domain ulasan yang bebas dari detail MongoDB.
type Review struct {
	ID           string    `json:"id"`
	CourseID     int       `json:"course_id"`
	UserID       int       `json:"user_id"`
	Username     string    `json:"username"`
	Rating       int       `json:"rating"`
	Comment      string    `json:"comment"`
	Tags         []string  `json:"tags"`
	Replies      []Reply   `json:"replies"`
	HelpfulCount int       `json:"helpful_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Reply disimpan di dalam dokumen Review karena selalu dibaca bersama ulasannya.
type Reply struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateReviewRequest struct {
	Rating  int      `json:"rating" validate:"required,min=1,max=5"`
	Comment string   `json:"comment" validate:"required,min=10,max=1000"`
	Tags    []string `json:"tags" validate:"max=5,dive,min=2,max=30"`
}

// UpdateReviewRequest menggunakan pointer agar field yang tidak dikirim
// dapat dibedakan dari field yang diminta untuk dikosongkan.
type UpdateReviewRequest struct {
	Rating  *int      `json:"rating,omitempty" validate:"omitnil,min=1,max=5"`
	Comment *string   `json:"comment,omitempty" validate:"omitnil,min=10,max=1000"`
	Tags    *[]string `json:"tags,omitempty" validate:"omitnil,max=5,dive,min=2,max=30"`
}

type ReplyRequest struct {
	Body string `json:"body" validate:"required,min=2,max=500"`
}

// ReviewQuery menyimpan parameter untuk membaca ulasan satu kelas.
type ReviewQuery struct {
	CourseID int
	Tag      string
	Before   string
	Limit    int
}

type ReviewSummary struct {
	CourseID      int            `json:"course_id"`
	Count         int            `json:"count"`
	AverageRating float64        `json:"average_rating"`
	Distribution  map[string]int `json:"distribution"`
	TopTags       []TagCount     `json:"top_tags"`
}

type TagCount struct {
	Tag   string `json:"tag"`
	Total int    `json:"total"`
}
