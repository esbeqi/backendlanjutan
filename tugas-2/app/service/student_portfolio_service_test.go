package service

import (
	"errors"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
	"api-students/app/repository"
)

func TestDecodeStrictJSONRejectsUnknownAndOperatorFields(t *testing.T) {
	for _, body := range []string{
		`{"headline":"Backend","bio":"","projects":[],"certificates":[],"student_id":7}`,
		`{"$set":{"headline":"changed"}}`,
		`{"title":"API","technologies":["Go"],"link":"https://example.test","year":2025,"id":"client-id"}`,
		`{"headline":"Backend"} {"headline":"second"}`,
	} {
		var request model.ReplaceStudentPortfolioRequest
		if err := decodeStrictJSON([]byte(body), &request); err == nil {
			t.Errorf("decodeStrictJSON accepted %s", body)
		}
	}
}

func TestDecodeStrictJSONAcceptsKnownFields(t *testing.T) {
	var request model.AddPortfolioProjectRequest
	err := decodeStrictJSON([]byte(`{"title":"API","technologies":["Go"],"link":"https://example.test","year":2025}`), &request)
	if err != nil {
		t.Fatalf("decode strict JSON: %v", err)
	}
	if request.Title != "API" || len(request.Technologies) != 1 {
		t.Fatalf("decoded request = %#v", request)
	}
}

func TestValidatePortfolioRequestsAndProjectLimit(t *testing.T) {
	validProject := model.ReplacePortfolioProjectRequest{
		Title: "Portfolio API", Technologies: []string{"Go", "MongoDB"}, Link: "https://example.test/project", Year: 2025,
	}
	valid := model.ReplaceStudentPortfolioRequest{
		Headline: "Backend Engineer", Projects: []model.ReplacePortfolioProjectRequest{validProject},
		Certificates: []model.PortfolioCertificate{{Name: "Go", Issuer: "Institute", Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}},
	}
	if errs := ValidateReplacePortfolio(valid); len(errs) != 0 {
		t.Fatalf("valid portfolio rejected: %v", errs)
	}
	tooMany := valid
	tooMany.Projects = make([]model.ReplacePortfolioProjectRequest, portfolioProjectLimit+1)
	if errs := ValidateReplacePortfolio(tooMany); errs["projects"] != "maksimal 20 project" {
		t.Fatalf("project limit error = %v", errs)
	}
	if errs := ValidateAddPortfolioProject(model.AddPortfolioProjectRequest{Title: "", Technologies: []string{"Go"}, Link: "javascript:alert(1)", Year: 0}); len(errs) < 3 {
		t.Fatalf("invalid project errors = %v; expected title, link and year errors", errs)
	}
}

func TestPortfolioErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{"missing portfolio", repository.ErrNotFound, fiber.StatusNotFound},
		{"missing project", repository.ErrPortfolioProjectMissing, fiber.StatusNotFound},
		{"project limit", repository.ErrPortfolioProjectLimit, fiber.StatusConflict},
		{"duplicate", repository.ErrDuplicate, fiber.StatusConflict},
		{"unexpected", errors.New("database unavailable"), fiber.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := portfolioError(tt.err, "failed")
			fiberErr, ok := err.(*fiber.Error)
			if !ok || fiberErr.Code != tt.code {
				t.Fatalf("mapped error = %#v; want status %d", err, tt.code)
			}
		})
	}
}
