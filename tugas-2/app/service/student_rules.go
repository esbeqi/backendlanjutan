package service

import (
	"strings"

	"api-students/app/model"
)

// Business Rules MURNI.
// Tidak mengimpor Fiber.
// Tidak mengetahui database.

// ValidateCreate memeriksa data POST /students.
//
// Semua field utama harus diisi dan nilai grade harus berada
// pada rentang 0 sampai 100.
func ValidateCreate(req model.CreateStudentRequest) map[string]string {
	errs := map[string]string{}

	if strings.TrimSpace(req.NIM) == "" {
		errs["nim"] = "wajib diisi"
	}

	if strings.TrimSpace(req.Name) == "" {
		errs["name"] = "wajib diisi"
	}

	if req.Grade < 0 || req.Grade > 100 {
		errs["grade"] = "nilai harus antara 0 sampai 100"
	}

	return errs
}

// ValidateReplace memeriksa data PUT /students/:id.
//
// Karena PUT mengganti seluruh data, NIM dan nama wajib diisi.
// Grade juga harus berada pada rentang 0 sampai 100.
func ValidateReplace(req model.ReplaceStudentRequest) map[string]string {
	errs := map[string]string{}

	if strings.TrimSpace(req.NIM) == "" {
		errs["nim"] = "wajib diisi pada PUT"
	}

	if strings.TrimSpace(req.Name) == "" {
		errs["name"] = "wajib diisi pada PUT"
	}

	if req.Grade < 0 || req.Grade > 100 {
		errs["grade"] = "nilai harus antara 0 sampai 100"
	}

	return errs
}

// ApplyPatch hanya menerapkan field yang dikirim.
//
// Pointer pada PatchStudentRequest membedakan antara:
// - field tidak dikirim  -> nil
// - field dikirim       -> memiliki nilai
//
// Validasi field dilakukan oleh validation layer sebelum
// perubahan diterapkan.
func ApplyPatch(
	current model.Student,
	req model.PatchStudentRequest,
) (model.Student, map[string]string) {

	errs := map[string]string{}

	if req.NIM != nil {
		current.NIM = *req.NIM
	}

	if req.Name != nil {
		current.Name = *req.Name
	}

	if req.Grade != nil {
		current.Grade = *req.Grade
	}

	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}

	return current, errs
}

// IsEmptyPatch mengecek apakah PATCH tidak mengirim
// satu field pun.
func IsEmptyPatch(req model.PatchStudentRequest) bool {
	return req.NIM == nil &&
		req.Name == nil &&
		req.Grade == nil &&
		req.IsActive == nil
}

// CountTotalPages menghitung jumlah halaman berdasarkan
// total data dan limit per halaman.
func CountTotalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}

	return (total + limit - 1) / limit
}
