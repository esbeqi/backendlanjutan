package repository

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"api-students/app/model"
)

func TestStudentPortfolioDocumentRoundTrip(t *testing.T) {
	certificateDate := time.Date(2025, time.March, 10, 12, 30, 0, 0, time.UTC)
	want := model.StudentPortfolio{
		StudentID: 42,
		Headline:  "Backend developer",
		Bio:       "Membangun layanan yang dapat diandalkan.",
		Projects: []model.PortfolioProject{{
			ID:           "project-1",
			Title:        "API Mahasiswa",
			Technologies: []string{"Go", "PostgreSQL"},
			Link:         "https://example.test/project",
			Year:         2025,
		}},
		Certificates: []model.PortfolioCertificate{{
			Name:   "Backend Fundamentals",
			Issuer: "Example Institute",
			Date:   certificateDate,
		}},
	}

	encoded, err := bson.Marshal(portfolioDocumentFromModel(want))
	if err != nil {
		t.Fatalf("marshal portfolio document: %v", err)
	}
	var decoded studentPortfolioDocument
	if err := bson.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal portfolio document: %v", err)
	}
	got := decoded.toModel()
	if len(got.Certificates) != len(want.Certificates) {
		t.Fatalf("certificate count = %d, want %d", len(got.Certificates), len(want.Certificates))
	}
	gotCertificateDate := got.Certificates[0].Date
	got.Certificates[0].Date = want.Certificates[0].Date
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
	if !gotCertificateDate.Equal(want.Certificates[0].Date) {
		t.Fatalf("certificate date = %s, want instant %s", gotCertificateDate, want.Certificates[0].Date)
	}
}

func TestStudentPortfolioEmptySlicesSerializeAsBSONArrays(t *testing.T) {
	encoded, err := bson.Marshal(portfolioDocumentFromModel(model.StudentPortfolio{StudentID: 42}))
	if err != nil {
		t.Fatalf("marshal empty portfolio document: %v", err)
	}
	var raw bson.Raw
	if err := bson.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("decode raw BSON: %v", err)
	}
	for _, field := range []string{"projects", "certificates"} {
		value := raw.Lookup(field)
		if value.Type != bson.TypeArray {
			t.Errorf("field %q BSON type = %v, want array", field, value.Type)
		}
	}
}

func TestStudentPortfolioIndexDefinition(t *testing.T) {
	index := studentPortfolioIndexModel()
	wantKeys := bson.D{{Key: "student_id", Value: 1}}
	if !reflect.DeepEqual(index.Keys, wantKeys) {
		t.Fatalf("index keys = %#v, want %#v", index.Keys, wantKeys)
	}
	if index.Options == nil {
		t.Fatal("index options are nil")
	}
	var parsed options.IndexOptions
	for _, apply := range index.Options.List() {
		if err := apply(&parsed); err != nil {
			t.Fatalf("apply index option: %v", err)
		}
	}
	if parsed.Name == nil || *parsed.Name != "student_id_unique" {
		t.Fatalf("index name = %v, want student_id_unique", parsed.Name)
	}
	if parsed.Unique == nil || !*parsed.Unique {
		t.Fatalf("index unique = %v, want true", parsed.Unique)
	}
}
