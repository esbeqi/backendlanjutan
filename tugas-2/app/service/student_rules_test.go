package service

import (
	"testing"

	"api-students/app/model"
)

func TestCountTotalPages(t *testing.T) {
	cases := []struct {
		total int
		limit int
		want  int
	}{
		{0, 10, 0},
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2},
		{137, 20, 7},
	}

	for _, tc := range cases {
		got := CountTotalPages(tc.total, tc.limit)

		if got != tc.want {
			t.Errorf(
				"total=%d limit=%d: harap %d, dapat %d",
				tc.total,
				tc.limit,
				tc.want,
				got,
			)
		}
	}
}

func TestApplyPatch(t *testing.T) {
	initial := model.Student{
		ID:       1,
		NIM:      "220001",
		Name:     "Budi",
		Grade:    80,
		IsActive: true,
	}

	newGrade := 90

	result, errs := ApplyPatch(initial, model.PatchStudentRequest{
		Grade: &newGrade,
	})

	if len(errs) != 0 {
		t.Fatalf("tidak seharusnya ada error: %v", errs)
	}

	if result.Grade != 90 {
		t.Error("grade seharusnya berubah menjadi 90")
	}

	if result.Name != "Budi" {
		t.Error("field yang tidak dikirim seharusnya tidak berubah")
	}

	if result.NIM != "220001" {
		t.Error("field yang tidak dikirim seharusnya tidak berubah")
	}
}

func TestValidateCreate(t *testing.T) {
	errs := ValidateCreate(model.CreateStudentRequest{})

	// Grade = 0 masih dianggap valid oleh business rules,
	// sehingga hanya NIM dan Name yang menghasilkan error.
	if len(errs) != 2 {
		t.Fatalf("harap 2 error, dapat %d", len(errs))
	}

	if _, ok := errs["nim"]; !ok {
		t.Error("error untuk field nim seharusnya ada")
	}

	if _, ok := errs["name"]; !ok {
		t.Error("error untuk field name seharusnya ada")
	}
}
