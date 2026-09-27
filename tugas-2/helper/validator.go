package helper

import (
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._]+$`)
var nimPattern = regexp.MustCompile(`^[0-9]+$`)

func init() {
	_ = validate.RegisterValidation("username", validateUsername)
	_ = validate.RegisterValidation("strongpassword", validateStrongPassword)
	_ = validate.RegisterValidation("nim", validateNIM)
}

func validateUsername(fl validator.FieldLevel) bool {
	value := fl.Field().String()

	return usernamePattern.MatchString(value)
}

func validateStrongPassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	var hasLetter bool
	var hasDigit bool

	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasLetter || !hasDigit {
		return false
	}

	weak := map[string]bool{
		"password1":   true,
		"12345678":    true,
		"qwerty123":   true,
		"admin123":    true,
		"password123": true,
	}

	return !weak[strings.ToLower(password)]
}

func validateNIM(fl validator.FieldLevel) bool {
	value := fl.Field().String()

	return nimPattern.MatchString(value)
}

func Validate(v any) map[string]string {
	errs := map[string]string{}

	err := validate.Struct(v)
	if err == nil {
		return errs
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return errs
	}

	for _, fieldErr := range validationErrors {
		field := fieldName(fieldErr.StructField())
		errs[field] = messageFor(fieldErr)
	}

	return errs
}

func messageFor(err validator.FieldError) string {
	switch err.StructField() {
	case "Username":
		switch err.Tag() {
		case "required":
			return "wajib diisi"
		case "min":
			return "minimal 3 karakter"
		case "max":
			return "maksimal 30 karakter"
		case "username":
			return "hanya boleh huruf, angka, titik, dan garis bawah"
		}

	case "Email":
		switch err.Tag() {
		case "required":
			return "wajib diisi"
		case "email":
			return "format email tidak valid"
		case "max":
			return "maksimal 120 karakter"
		}

	case "Password":
		switch err.Tag() {
		case "required":
			return "wajib diisi"
		case "max":
			return "maksimal 72 karakter"
		case "strongpassword":
			return "harus memuat huruf dan angka"
		}

	case "NIM":
		switch err.Tag() {
		case "required":
			return "NIM wajib diisi"
		case "nim":
			return "NIM hanya boleh berisi angka"
		}

	case "Name":
		switch err.Tag() {
		case "required":
			return "nama wajib diisi"
		case "min":
			return "nama minimal 3 karakter"
		case "max":
			return "nama maksimal 100 karakter"
		}

	case "Grade":
		switch err.Tag() {
		case "min":
			return "nilai minimal 0"
		case "max":
			return "nilai maksimal 100"
		}
	}

	return "format tidak valid"
}

func fieldName(field string) string {
	switch field {
	case "NIM":
		return "nim"
	case "Name":
		return "name"
	case "Grade":
		return "grade"
	case "IsActive":
		return "is_active"
	case "Username":
		return "username"
	case "Email":
		return "email"
	case "Password":
		return "password"
	default:
		return field
	}
}

func ValidateWithJSONFields(v any) map[string]string {
	return Validate(v)
}

func isStruct(v any) bool {
	if v == nil {
		return false
	}

	return reflect.TypeOf(v).Kind() == reflect.Struct
}
