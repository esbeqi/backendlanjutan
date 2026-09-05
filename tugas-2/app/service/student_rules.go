package service

import (
	"strings"

	"api-students/app/model"
)

// Business Rules MURNI.
// Tidak mengimpor Fiber.
// Tidak mengetahui database.

// ValidateCreate memeriksa data POST.
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

// ValidateReplace memeriksa data PUT.
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

// ApplyPatch hanya mengubah field yang dikirim.
func ApplyPatch(
	current model.Student,
	req model.PatchStudentRequest,
) (model.Student, map[string]string) {

	errs := map[string]string{}

	if req.NIM != nil {
		if strings.TrimSpace(*req.NIM) == "" {
			errs["nim"] = "tidak boleh kosong"
		} else {
			current.NIM = *req.NIM
		}
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			errs["name"] = "tidak boleh kosong"
		} else {
			current.Name = *req.Name
		}
	}

	if req.Grade != nil {
		if *req.Grade < 0 || *req.Grade > 100 {
			errs["grade"] = "nilai harus antara 0 sampai 100"
		} else {
			current.Grade = *req.Grade
		}
	}

	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}

	return current, errs
}

// IsEmptyPatch mengecek apakah PATCH kosong.
func IsEmptyPatch(req model.PatchStudentRequest) bool {
	return req.NIM == nil &&
		req.Name == nil &&
		req.Grade == nil &&
		req.IsActive == nil
}

// CountTotalPages menghitung jumlah halaman.
func CountTotalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
