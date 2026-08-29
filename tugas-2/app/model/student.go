package model

import "time"

type Student struct {
	ID        int       `json:"id"`
	NIM       string    `json:"nim"`
	Name      string    `json:"name"`
	Grade     int       `json:"grade"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// Digunakan saat POST /students
// Semua field wajib diisi.
type CreateStudentRequest struct {
	NIM   string `json:"nim"`
	Name  string `json:"name"`
	Grade int    `json:"grade"`
}

// Digunakan saat PUT /students/:id
// Mengganti seluruh data sehingga semua field wajib dikirim.
type ReplaceStudentRequest struct {
	NIM      string `json:"nim"`
	Name     string `json:"name"`
	Grade    int    `json:"grade"`
	IsActive bool   `json:"is_active"`
}

// Digunakan saat PATCH /students/:id
// Hanya field yang dikirim yang akan diubah.
type PatchStudentRequest struct {
	NIM      *string `json:"nim,omitempty"`
	Name     *string `json:"name,omitempty"`
	Grade    *int    `json:"grade,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// Format response yang digunakan di semua endpoint.
type WebResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Meta    *Meta  `json:"meta,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// Informasi pagination.
type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// Menampung query string dari endpoint GET /students.
type ListQuery struct {
	Page     int
	Limit    int
	Search   string
	Sort     string
	Order    string
	IsActive *bool
}

// Offset menghitung jumlah data yang dilewati.
func (q ListQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}
