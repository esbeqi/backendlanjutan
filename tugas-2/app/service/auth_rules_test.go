package service

import (
	"testing"

	"api-students/app/model"
)

func TestValidateRegisterPasswordValid(t *testing.T) {
	req := model.RegisterRequest{
		Username: "sari",
		Email:    "sari@example.com",
		Password: "rahasia123",
	}

	errs := ValidateRegister(req)

	if _, ok := errs["password"]; ok {
		t.Fatalf("password valid tetapi menghasilkan error: %v", errs["password"])
	}
}

func TestValidateRegisterPasswordTooShort(t *testing.T) {
	req := model.RegisterRequest{
		Username: "sari",
		Email:    "sari@example.com",
		Password: "abc123",
	}

	errs := ValidateRegister(req)

	if errs["password"] != "minimal 8 karakter" {
		t.Fatalf("hasil tidak sesuai: %q", errs["password"])
	}
}

func TestValidateRegisterPasswordTooCommon(t *testing.T) {
	req := model.RegisterRequest{
		Username: "sari",
		Email:    "sari@example.com",
		Password: "password1",
	}

	errs := ValidateRegister(req)

	if errs["password"] != "password terlalu umum" {
		t.Fatalf("hasil tidak sesuai: %q", errs["password"])
	}
}
