package model

import "time"

type Student struct {
	ID        int       `json:"id"`
	NIM       string    `json:"nim"`
	Name      string    `json:"name"`
	Grade     int       `json:"grade"`
	IsActive  bool      `json:"is_active"`
	OwnerID   int       `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Digunakan saat POST /students.
type CreateStudentRequest struct {
	NIM   string `json:"nim" validate:"required,nim"`
	Name  string `json:"name" validate:"required,min=3,max=100"`
	Grade int    `json:"grade" validate:"min=0,max=100"`
}

// Digunakan saat PUT /students/:id.
// PUT mengganti seluruh data sehingga seluruh field wajib dikirim.
type ReplaceStudentRequest struct {
	NIM      string `json:"nim" validate:"required,nim"`
	Name     string `json:"name" validate:"required,min=3,max=100"`
	Grade    int    `json:"grade" validate:"min=0,max=100"`
	IsActive bool   `json:"is_active"`
}

// Digunakan saat PATCH /students/:id.
// Pointer membedakan field yang tidak dikirim dari field yang dikirim.
type PatchStudentRequest struct {
	NIM      *string `json:"nim,omitempty" validate:"omitempty,nim"`
	Name     *string `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
	Grade    *int    `json:"grade,omitempty" validate:"omitempty,min=0,max=100"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// Format response yang digunakan di endpoint.
type WebResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Meta    *Meta  `json:"meta,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// Informasi pagination berbasis halaman.
type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// Menampung query string untuk pagination berbasis offset.
type ListQuery struct {
	Page     int
	Limit    int
	Search   string
	Sort     string
	Order    string
	IsActive *bool
	Cursor   string
}

// Menampung cursor hasil decode.
// Cursor menggunakan pasangan (created_at, id)
// sebagai pengurut yang unik.
type Cursor struct {
	CreatedAt time.Time
	ID        int
}

// Query khusus cursor pagination.
type CursorQuery struct {
	Limit           int
	Search          string
	IsActive        *bool
	Cursor          string
	CursorCreatedAt *time.Time
	CursorID        *int
}

// Metadata response cursor pagination.
type CursorMeta struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// Offset menghitung jumlah data yang dilewati.
func (q ListQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}
