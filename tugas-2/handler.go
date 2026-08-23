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
