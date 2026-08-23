package main

import (
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Penyimpanan sementara di memori
var students []Student
var nextID = 1

// Mencari index student berdasarkan ID
func findStudentIndex(id int) int {
	for i := range students {
		if students[i].ID == id {
			return i
		}
	}

	return -1
}

// Mengambil student berdasarkan ID
func findStudent(id int) (*Student, int) {
	index := findStudentIndex(id)

	if index == -1 {
		return nil, -1
	}

	return &students[index], index
}

// Mengecek apakah NIM sudah digunakan
func nimExists(nim string, exceptID int) bool {
	for _, s := range students {
		if s.NIM == nim && s.ID != exceptID {
			return true
		}
	}
	return false
}

// Mengecek apakah keyword terdapat pada nama mahasiswa
func matchSearch(s Student, keyword string) bool {
	keyword = strings.ToLower(keyword)

	return strings.Contains(strings.ToLower(s.Name), keyword)
}

// Mengambil parameter ID dari URL
func paramID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))

	if err != nil || id < 1 {
		return 0, false
	}

	return id, true
}

// GET /students
func listStudents(c *fiber.Ctx) error {
	q := parseListQuery(c)

	// Filter data
	result := []Student{}

	for _, s := range students {

		if q.IsActive != nil && s.IsActive != *q.IsActive {
			continue
		}

		if q.Search != "" && !matchSearch(s, q.Search) {
			continue
		}

		result = append(result, s)
	}

	// Sorting
	sort.SliceStable(result, func(i, j int) bool {

		var less bool

		switch q.Sort {

		case "name":
			less = result[i].Name < result[j].Name

		case "nim":
			less = result[i].NIM < result[j].NIM

		case "grade":
			less = result[i].Grade < result[j].Grade

		default:
			less = result[i].ID < result[j].ID

		}

		if q.Order == "desc" {
			return !less
		}

		return less
	})

	// Pagination
	total := len(result)

	totalPages := (total + q.Limit - 1) / q.Limit

	start := (q.Page - 1) * q.Limit

	if start > total {
		start = total
	}

	end := start + q.Limit

	if end > total {
		end = total
	}

	return okList(
		c,
		"daftar mahasiswa berhasil diambil",
		result[start:end],
		&Meta{
			Page:       q.Page,
			Limit:      q.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	)
}

// GET /students/:id
func getStudent(c *fiber.Ctx) error {

	id, valid := paramID(c)

	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	i := findStudentIndex(id)

	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	return ok(c, "mahasiswa ditemukan", students[i])
}

// POST /students
func createStudent(c *fiber.Ctx) error {
	var req CreateStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body request tidak valid")
	}

	if errs := validateStudent(req.NIM, req.Name, req.Grade); errs != nil {
		return failValidation(c, errs)
	}

	if nimExists(req.NIM, 0) {
		return fail(c, fiber.StatusConflict, "NIM sudah digunakan")
	}

	student := Student{
		ID:       nextID,
		NIM:      req.NIM,
		Name:     req.Name,
		Grade:    req.Grade,
		IsActive: true,
	}

	nextID++
	students = append(students, student)

	location := "/api/v1/students/" + strconv.Itoa(student.ID)

	return created(
		c,
		"mahasiswa berhasil ditambahkan",
		student,
		location,
	)
}

// PUT /students/:id
func replaceStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	student, _ := findStudent(id)
	if student == nil {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	var req ReplaceStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body request tidak valid")
	}

	if errs := validateStudent(req.NIM, req.Name, req.Grade); errs != nil {
		return failValidation(c, errs)
	}

	if nimExists(req.NIM, id) {
		return fail(c, fiber.StatusConflict, "NIM sudah digunakan")
	}

	student.NIM = req.NIM
	student.Name = req.Name
	student.Grade = req.Grade
	student.IsActive = req.IsActive

	return ok(c, "mahasiswa berhasil diperbarui", student)
}

// PATCH /students/:id
func patchStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	student, _ := findStudent(id)
	if student == nil {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	var req PatchStudentRequest

	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body request tidak valid")
	}

	if req.NIM != nil {
		if strings.TrimSpace(*req.NIM) == "" {
			return failValidation(c, map[string]string{
				"nim": "NIM tidak boleh kosong",
			})
		}

		if nimExists(*req.NIM, id) {
			return fail(c, fiber.StatusConflict, "NIM sudah digunakan")
		}

		student.NIM = *req.NIM
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return failValidation(c, map[string]string{
				"name": "Nama tidak boleh kosong",
			})
		}

		student.Name = *req.Name
	}

	if req.Grade != nil {
		if *req.Grade < 0 || *req.Grade > 100 {
			return failValidation(c, map[string]string{
				"grade": "Grade harus berada pada rentang 0-100",
			})
		}

		student.Grade = *req.Grade
	}

	if req.IsActive != nil {
		student.IsActive = *req.IsActive
	}

	return ok(c, "mahasiswa berhasil diperbarui", student)
}

func validateStudent(nim, name string, grade int) map[string]string {
	errs := map[string]string{}

	if strings.TrimSpace(nim) == "" {
		errs["nim"] = "NIM tidak boleh kosong"
	}

	if strings.TrimSpace(name) == "" {
		errs["name"] = "Nama tidak boleh kosong"
	}

	if grade < 0 || grade > 100 {
		errs["grade"] = "Grade harus berada pada rentang 0-100"
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// DELETE /students/:id
func deleteStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	index := findStudentIndex(id)
	if index == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	students = append(students[:index], students[index+1:]...)

	return c.SendStatus(fiber.StatusNoContent)
}
