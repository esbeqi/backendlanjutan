package service

import (
	"strings"
	"unicode"

	"api-students/app/model"
)

func ValidateRegister(req model.RegisterRequest) map[string]string {
	errs := map[string]string{}

	username := strings.TrimSpace(req.Username)

	if username == "" {
		errs["username"] = "wajib diisi"
	} else if len(username) < 3 {
		errs["username"] = "minimal 3 karakter"
	} else if !isValidUsername(username) {
		errs["username"] = "hanya boleh huruf, angka, titik, dan garis bawah"
	}

	if !isValidEmail(req.Email) {
		errs["email"] = "format email tidak valid"
	}

	if msg := checkPasswordStrength(req.Password); msg != "" {
		errs["password"] = msg
	}

	return errs
}

func ValidateLogin(req model.LoginRequest) map[string]string {
	errs := map[string]string{}

	if strings.TrimSpace(req.Username) == "" {
		errs["username"] = "wajib diisi"
	}

	if req.Password == "" {
		errs["password"] = "wajib diisi"
	}

	return errs
}

func checkPasswordStrength(password string) string {
	if len(password) < 8 {
		return "minimal 8 karakter"
	}

	var hasLetter, hasDigit bool

	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasLetter || !hasDigit {
		return "harus memuat huruf dan angka"
	}

	weak := map[string]bool{
		"password1":   true,
		"12345678":    true,
		"qwerty123":   true,
		"admin123":    true,
		"password123": true,
	}

	if weak[strings.ToLower(password)] {
		return "password terlalu umum"
	}

	return ""
}

func isValidUsername(username string) bool {
	for _, r := range username {
		if !unicode.IsLetter(r) &&
			!unicode.IsDigit(r) &&
			r != '.' &&
			r != '_' {
			return false
		}
	}

	return true
}

func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)

	at := strings.LastIndex(email, "@")

	return at > 0 &&
		at < len(email)-1 &&
		strings.Contains(email[at+1:], ".")
}
