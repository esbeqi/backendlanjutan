package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
)

const portfolioProjectLimit = 20

type StudentPortfolioService struct {
	students   repository.StudentRepository
	portfolios repository.StudentPortfolioRepository
}

func NewStudentPortfolioService(
	students repository.StudentRepository,
	portfolios repository.StudentPortfolioRepository,
) *StudentPortfolioService {
	return &StudentPortfolioService{students: students, portfolios: portfolios}
}

func (s *StudentPortfolioService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	student, err := s.loadStudent(ctx, c)
	if err != nil {
		return err
	}
	if err := authorizePortfolio(c, student.OwnerID, "student:read:any"); err != nil {
		return err
	}
	portfolio, err := s.portfolios.FindByStudentID(ctx, student.ID)
	if err != nil {
		return portfolioError(err, "gagal mengambil portofolio")
	}
	return helper.Success(c, fiber.StatusOK, "portofolio mahasiswa ditemukan", portfolio)
}

func (s *StudentPortfolioService) Replace(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	student, err := s.loadStudent(ctx, c)
	if err != nil {
		return err
	}
	if err := authorizePortfolio(c, student.OwnerID, "student:update:any"); err != nil {
		return err
	}
	var req model.ReplaceStudentPortfolioRequest
	if err := decodeStrictJSON(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "body harus berupa JSON valid dengan field yang dikenal")
	}
	if errs := ValidateReplacePortfolio(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}
	portfolio := model.StudentPortfolio{
		StudentID: student.ID, Headline: strings.TrimSpace(req.Headline),
		Bio: strings.TrimSpace(req.Bio), Projects: make([]model.PortfolioProject, 0, len(req.Projects)),
		Certificates: req.Certificates,
	}
	for _, project := range req.Projects {
		portfolio.Projects = append(portfolio.Projects, model.PortfolioProject{
			Title: strings.TrimSpace(project.Title), Technologies: normalizeTechnologies(project.Technologies),
			Link: strings.TrimSpace(project.Link), Year: project.Year,
		})
	}
	for i := range portfolio.Certificates {
		portfolio.Certificates[i].Name = strings.TrimSpace(portfolio.Certificates[i].Name)
		portfolio.Certificates[i].Issuer = strings.TrimSpace(portfolio.Certificates[i].Issuer)
	}
	result, err := s.portfolios.Upsert(ctx, portfolio)
	if err != nil {
		return portfolioError(err, "gagal menyimpan portofolio")
	}
	return helper.Success(c, fiber.StatusOK, "portofolio mahasiswa berhasil disimpan", result)
}

func (s *StudentPortfolioService) AddProject(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	student, err := s.loadStudent(ctx, c)
	if err != nil {
		return err
	}
	if err := authorizePortfolio(c, student.OwnerID, "student:update:any"); err != nil {
		return err
	}
	var req model.AddPortfolioProjectRequest
	if err := decodeStrictJSON(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "body harus berupa JSON valid dengan field yang dikenal")
	}
	if errs := ValidateAddPortfolioProject(req); len(errs) > 0 {
		return helper.FailValidation(c, errs)
	}
	result, err := s.portfolios.AddProject(ctx, student.ID, model.PortfolioProject{
		Title: strings.TrimSpace(req.Title), Technologies: normalizeTechnologies(req.Technologies),
		Link: strings.TrimSpace(req.Link), Year: req.Year,
	})
	if err != nil {
		return portfolioError(err, "gagal menambahkan project")
	}
	return helper.Created(c, "project berhasil ditambahkan", result, c.OriginalURL())
}

func (s *StudentPortfolioService) DeleteProject(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	student, err := s.loadStudent(ctx, c)
	if err != nil {
		return err
	}
	if err := authorizePortfolio(c, student.OwnerID, "student:update:any"); err != nil {
		return err
	}
	projectID := strings.TrimSpace(c.Params("projectId"))
	if projectID == "" || len(projectID) > 64 || strings.ContainsAny(projectID, "$.") {
		return fiber.NewError(fiber.StatusBadRequest, "projectId tidak valid")
	}
	result, err := s.portfolios.RemoveProject(ctx, student.ID, projectID)
	if err != nil {
		return portfolioError(err, "gagal menghapus project")
	}
	return helper.Success(c, fiber.StatusOK, "project berhasil dihapus", result)
}

func (s *StudentPortfolioService) TechnologyStats(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	stats, err := s.portfolios.TopTechnologies(ctx)
	if err != nil {
		return portfolioError(err, "gagal mengambil statistik teknologi")
	}
	return helper.Success(c, fiber.StatusOK, "statistik teknologi portofolio berhasil diambil", stats)
}

func (s *StudentPortfolioService) loadStudent(ctx context.Context, c *fiber.Ctx) (model.Student, error) {
	id, valid := helper.ParamID(c)
	if !valid {
		return model.Student{}, fiber.NewError(fiber.StatusBadRequest, "id harus berupa angka positif")
	}
	student, err := s.students.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Student{}, fiber.NewError(fiber.StatusNotFound, "mahasiswa tidak ditemukan")
		}
		return model.Student{}, fiber.NewError(fiber.StatusInternalServerError, "gagal mengambil data mahasiswa")
	}
	return student, nil
}

func authorizePortfolio(c *fiber.Ctx, ownerID int, anyPermission string) error {
	current, ok := getAuthUser(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "identitas user tidak ditemukan")
	}
	perms, ok := getPermissionSet(c)
	if !ok {
		return fiber.NewError(fiber.StatusInternalServerError, "permission set tidak tersedia")
	}
	if !CanAccessStudent(current, ownerID, perms, anyPermission) {
		return fiber.NewError(fiber.StatusForbidden, "tidak memiliki akses ke portofolio mahasiswa ini")
	}
	return nil
}

func portfolioError(err error, fallback string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, "portofolio mahasiswa tidak ditemukan")
	case errors.Is(err, repository.ErrPortfolioProjectMissing):
		return fiber.NewError(fiber.StatusNotFound, "project tidak ditemukan")
	case errors.Is(err, repository.ErrPortfolioProjectLimit):
		return fiber.NewError(fiber.StatusConflict, "maksimal 20 project dalam satu portofolio")
	case errors.Is(err, repository.ErrDuplicate):
		return fiber.NewError(fiber.StatusConflict, "portofolio mahasiswa sudah ada")
	default:
		return fiber.NewError(fiber.StatusInternalServerError, fallback)
	}
}

func decodeStrictJSON(body []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("hanya boleh satu nilai JSON")
		}
		return err
	}
	return nil
}

func ValidateReplacePortfolio(req model.ReplaceStudentPortfolioRequest) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(req.Headline) == "" {
		errs["headline"] = "wajib diisi"
	}
	if len(req.Projects) > portfolioProjectLimit {
		errs["projects"] = "maksimal 20 project"
	}
	for i, project := range req.Projects {
		validateProjectFields(errs, strconv.Itoa(i), project.Title, project.Technologies, project.Link, project.Year)
	}
	for i, certificate := range req.Certificates {
		prefix := fmt.Sprintf("certificates.%d.", i)
		if strings.TrimSpace(certificate.Name) == "" {
			errs[prefix+"name"] = "wajib diisi"
		}
		if strings.TrimSpace(certificate.Issuer) == "" {
			errs[prefix+"issuer"] = "wajib diisi"
		}
		if certificate.Date.IsZero() {
			errs[prefix+"date"] = "wajib diisi dengan tanggal valid"
		}
	}
	return errs
}

func ValidateAddPortfolioProject(req model.AddPortfolioProjectRequest) map[string]string {
	errs := map[string]string{}
	validateProjectFields(errs, "", req.Title, req.Technologies, req.Link, req.Year)
	return errs
}

func validateProjectFields(errs map[string]string, prefix, title string, technologies []string, link string, year int) {
	key := func(field string) string {
		if prefix == "" {
			return field
		}
		return "projects." + prefix + "." + field
	}
	if strings.TrimSpace(title) == "" {
		errs[key("title")] = "wajib diisi"
	}
	if len(technologies) == 0 {
		errs[key("technologies")] = "minimal satu teknologi wajib diisi"
	} else {
		for i, technology := range technologies {
			if strings.TrimSpace(technology) == "" {
				errs[fmt.Sprintf("%s.%d", key("technologies"), i)] = "wajib diisi"
			}
		}
	}
	parsed, err := url.ParseRequestURI(strings.TrimSpace(link))
	if err != nil || parsed.Scheme != "https" && parsed.Scheme != "http" || parsed.Host == "" {
		errs[key("link")] = "harus berupa URL http atau https yang valid"
	}
	if year < 1 {
		errs[key("year")] = "harus berupa tahun positif"
	}
}

func normalizeTechnologies(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strings.TrimSpace(value))
	}
	return result
}
