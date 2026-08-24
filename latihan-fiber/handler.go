package main

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Penyimpanan sementara di memori
var users []User
var nextID = 1

// Mencari index user berdasarkan ID
func findUserIndex(id int) int {
	for i := range users {
		if users[i].ID == id {
			return i
		}
	}
	return -1
}

// Mengambil user berdasarkan ID
func findUser(id int) (*User, int) {
	index := findUserIndex(id)

	if index == -1 {
		return nil, -1
	}

	return &users[index], index
}

// Mengecek email sudah digunakan
func emailExists(email string, exceptID int) bool {
	for _, u := range users {
		if strings.EqualFold(u.Email, email) && u.ID != exceptID {
			return true
		}
	}
	return false
}

// Mengambil parameter ID
func paramID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))

	if err != nil || id < 1 {
		return 0, false
	}

	return id, true
}

// GET /users
func listUsers(c *fiber.Ctx) error {
	q := parseListQuery(c)

	result := []User{}

	for _, u := range users {

		if q.IsActive != nil && u.IsActive != *q.IsActive {
			continue
		}

		if q.Search != "" &&
			!strings.Contains(strings.ToLower(u.Username), strings.ToLower(q.Search)) {
			continue
		}

		result = append(result, u)
	}

	sort.SliceStable(result, func(i, j int) bool {

		var less bool

		switch q.Sort {

		case "username":
			less = result[i].Username < result[j].Username

		case "email":
			less = result[i].Email < result[j].Email

		case "created_at":
			less = result[i].CreatedAt.Before(result[j].CreatedAt)

		default:
			less = result[i].ID < result[j].ID
		}

		if q.Order == "desc" {
			return !less
		}

		return less
	})

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
		"daftar user berhasil diambil",
		result[start:end],
		&Meta{
			Page:       q.Page,
			Limit:      q.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	)
}

// GET /users/:id
func getUser(c *fiber.Ctx) error {

	id, valid := paramID(c)

	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	i := findUserIndex(id)

	if i == -1 {
		return fail(c, fiber.StatusNotFound, "user tidak ditemukan")
	}

	return ok(c, "user ditemukan", users[i])
}

// POST /users
func createUser(c *fiber.Ctx) error {
	var req CreateUserRequest

	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body request tidak valid")
	}

	if errs := validateUser(req.Username, req.Email, req.Password); errs != nil {
		return failValidation(c, errs)
	}

	if emailExists(req.Email, 0) {
		return fail(c, fiber.StatusConflict, "email sudah digunakan")
	}

	user := User{
		ID:        nextID,
		Username:  req.Username,
		Email:     req.Email,
		Password:  req.Password,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	nextID++
	users = append(users, user)

	location := "/api/v1/users/" + strconv.Itoa(user.ID)

	return created(
		c,
		"user berhasil ditambahkan",
		user,
		location,
	)
}

// PUT /users/:id
func replaceUser(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	user, _ := findUser(id)
	if user == nil {
		return fail(c, fiber.StatusNotFound, "user tidak ditemukan")
	}

	var req ReplaceUserRequest

	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body request tidak valid")
	}

	if errs := validateReplaceUser(req.Username, req.Email); errs != nil {
		return failValidation(c, errs)
	}

	if emailExists(req.Email, id) {
		return fail(c, fiber.StatusConflict, "email sudah digunakan")
	}

	user.Username = req.Username
	user.Email = req.Email
	user.IsActive = req.IsActive

	return ok(c, "user berhasil diperbarui", user)
}

// PATCH /users/:id
func patchUser(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	user, _ := findUser(id)
	if user == nil {
		return fail(c, fiber.StatusNotFound, "user tidak ditemukan")
	}

	var req PatchUserRequest

	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body request tidak valid")
	}

	if req.Username != nil {
		if strings.TrimSpace(*req.Username) == "" {
			return failValidation(c, map[string]string{
				"username": "Username tidak boleh kosong",
			})
		}
		user.Username = *req.Username
	}

	if req.Email != nil {
		if strings.TrimSpace(*req.Email) == "" {
			return failValidation(c, map[string]string{
				"email": "Email tidak boleh kosong",
			})
		}

		if emailExists(*req.Email, id) {
			return fail(c, fiber.StatusConflict, "email sudah digunakan")
		}

		user.Email = *req.Email
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	return ok(c, "user berhasil diperbarui", user)
}

// DELETE /users/:id
func deleteUser(c *fiber.Ctx) error {
	id, valid := paramID(c)

	if !valid {
		return fail(c, fiber.StatusBadRequest,
			"id harus berupa angka positif")
	}

	index := findUserIndex(id)

	if index == -1 {
		return fail(c, fiber.StatusNotFound,
			"user tidak ditemukan")
	}

	users = append(users[:index], users[index+1:]...)

	return noContent(c)
}

func validateUser(username, email, password string) map[string]string {
	errs := map[string]string{}

	if strings.TrimSpace(username) == "" {
		errs["username"] = "Username tidak boleh kosong"
	}

	if strings.TrimSpace(email) == "" {
		errs["email"] = "Email tidak boleh kosong"
	}

	if strings.TrimSpace(password) == "" {
		errs["password"] = "Password tidak boleh kosong"
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

func validateReplaceUser(username, email string) map[string]string {
	errs := map[string]string{}

	if strings.TrimSpace(username) == "" {
		errs["username"] = "Username tidak boleh kosong"
	}

	if strings.TrimSpace(email) == "" {
		errs["email"] = "Email tidak boleh kosong"
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}
