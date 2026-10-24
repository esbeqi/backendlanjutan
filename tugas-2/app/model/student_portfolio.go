package model

import "time"

// StudentPortfolio stores the portfolio owned by one PostgreSQL student row.
// It intentionally uses only Go types so the domain model stays independent
// of the MongoDB driver.
type StudentPortfolio struct {
	StudentID    int                    `json:"student_id"`
	Headline     string                 `json:"headline"`
	Bio          string                 `json:"bio"`
	Projects     []PortfolioProject     `json:"projects"`
	Certificates []PortfolioCertificate `json:"certificates"`
}

type PortfolioProject struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Technologies []string `json:"technologies"`
	Link         string   `json:"link"`
	Year         int      `json:"year"`
}

type PortfolioCertificate struct {
	Name   string    `json:"name"`
	Issuer string    `json:"issuer"`
	Date   time.Time `json:"date"`
}

// ReplaceStudentPortfolioRequest is the public payload for replacing a
// portfolio. StudentID is intentionally excluded and always comes from URL.
type ReplaceStudentPortfolioRequest struct {
	Headline     string                           `json:"headline"`
	Bio          string                           `json:"bio"`
	Projects     []ReplacePortfolioProjectRequest `json:"projects"`
	Certificates []PortfolioCertificate           `json:"certificates"`
}

type ReplacePortfolioProjectRequest struct {
	Title        string   `json:"title"`
	Technologies []string `json:"technologies"`
	Link         string   `json:"link"`
	Year         int      `json:"year"`
}

// AddPortfolioProjectRequest excludes ID so clients cannot choose MongoDB
// document identifiers for embedded projects.
type AddPortfolioProjectRequest struct {
	Title        string   `json:"title"`
	Technologies []string `json:"technologies"`
	Link         string   `json:"link"`
	Year         int      `json:"year"`
}

type TechnologyStat struct {
	Technology string `json:"technology"`
	Count      int64  `json:"count"`
}
