package repository

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"api-students/app/model"
)

const studentPortfoliosCollection = "student_portfolios"

const maxPortfolioProjects = 20

var (
	ErrPortfolioProjectLimit   = errors.New("batas project portofolio tercapai")
	ErrPortfolioProjectMissing = errors.New("project portofolio tidak ditemukan")
)

// StudentPortfolioRepository is the persistence boundary for student
// portfolios. MongoDB document details remain private to this package.
type StudentPortfolioRepository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, portfolio model.StudentPortfolio) (model.StudentPortfolio, error)
	FindByStudentID(ctx context.Context, studentID int) (model.StudentPortfolio, error)
	Update(ctx context.Context, portfolio model.StudentPortfolio) (model.StudentPortfolio, error)
	Upsert(ctx context.Context, portfolio model.StudentPortfolio) (model.StudentPortfolio, error)
	AddProject(ctx context.Context, studentID int, project model.PortfolioProject) (model.StudentPortfolio, error)
	RemoveProject(ctx context.Context, studentID int, projectID string) (model.StudentPortfolio, error)
	TopTechnologies(ctx context.Context) ([]model.TechnologyStat, error)
	Delete(ctx context.Context, studentID int) error
}

type studentPortfolioMongoRepository struct {
	collection *mongo.Collection
}

func NewStudentPortfolioRepository(db *mongo.Database) StudentPortfolioRepository {
	return &studentPortfolioMongoRepository{
		collection: db.Collection(studentPortfoliosCollection),
	}
}

type studentPortfolioDocument struct {
	StudentID    int                            `bson:"student_id"`
	Headline     string                         `bson:"headline"`
	Bio          string                         `bson:"bio"`
	Projects     []portfolioProjectDocument     `bson:"projects"`
	Certificates []portfolioCertificateDocument `bson:"certificates"`
}

type portfolioProjectDocument struct {
	ID           string   `bson:"id"`
	Title        string   `bson:"title"`
	Technologies []string `bson:"technologies"`
	Link         string   `bson:"link"`
	Year         int      `bson:"year"`
}

type portfolioCertificateDocument struct {
	Name   string        `bson:"name"`
	Issuer string        `bson:"issuer"`
	Date   bson.DateTime `bson:"date"`
}

func portfolioDocumentFromModel(portfolio model.StudentPortfolio) studentPortfolioDocument {
	doc := studentPortfolioDocument{
		StudentID:    portfolio.StudentID,
		Headline:     portfolio.Headline,
		Bio:          portfolio.Bio,
		Projects:     make([]portfolioProjectDocument, 0, len(portfolio.Projects)),
		Certificates: make([]portfolioCertificateDocument, 0, len(portfolio.Certificates)),
	}
	for _, project := range portfolio.Projects {
		technologies := make([]string, 0, len(project.Technologies))
		technologies = append(technologies, project.Technologies...)
		doc.Projects = append(doc.Projects, portfolioProjectDocument{
			ID:           project.ID,
			Title:        project.Title,
			Technologies: technologies,
			Link:         project.Link,
			Year:         project.Year,
		})
	}
	for _, certificate := range portfolio.Certificates {
		doc.Certificates = append(doc.Certificates, portfolioCertificateDocument{
			Name:   certificate.Name,
			Issuer: certificate.Issuer,
			Date:   bson.DateTime(certificate.Date.UnixMilli()),
		})
	}
	return doc
}

func (doc studentPortfolioDocument) toModel() model.StudentPortfolio {
	portfolio := model.StudentPortfolio{
		StudentID:    doc.StudentID,
		Headline:     doc.Headline,
		Bio:          doc.Bio,
		Projects:     make([]model.PortfolioProject, 0, len(doc.Projects)),
		Certificates: make([]model.PortfolioCertificate, 0, len(doc.Certificates)),
	}
	for _, project := range doc.Projects {
		technologies := make([]string, 0, len(project.Technologies))
		technologies = append(technologies, project.Technologies...)
		portfolio.Projects = append(portfolio.Projects, model.PortfolioProject{
			ID:           project.ID,
			Title:        project.Title,
			Technologies: technologies,
			Link:         project.Link,
			Year:         project.Year,
		})
	}
	for _, certificate := range doc.Certificates {
		portfolio.Certificates = append(portfolio.Certificates, model.PortfolioCertificate{
			Name:   certificate.Name,
			Issuer: certificate.Issuer,
			Date:   certificate.Date.Time(),
		})
	}
	return portfolio
}

func studentPortfolioIndexModel() mongo.IndexModel {
	return mongo.IndexModel{
		Keys: bson.D{{Key: "student_id", Value: 1}},
		Options: options.Index().
			SetName("student_id_unique").
			SetUnique(true),
	}
}

func (r *studentPortfolioMongoRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.collection.Indexes().CreateOne(ctx, studentPortfolioIndexModel()); err != nil {
		return fmt.Errorf("membuat index collection %s: %w", studentPortfoliosCollection, err)
	}
	return nil
}

func (r *studentPortfolioMongoRepository) Create(ctx context.Context, portfolio model.StudentPortfolio) (model.StudentPortfolio, error) {
	doc := portfolioDocumentFromModel(portfolio)
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return model.StudentPortfolio{}, ErrDuplicate
		}
		return model.StudentPortfolio{}, fmt.Errorf("menyimpan portofolio mahasiswa %d: %w", portfolio.StudentID, err)
	}
	return doc.toModel(), nil
}

func (r *studentPortfolioMongoRepository) FindByStudentID(ctx context.Context, studentID int) (model.StudentPortfolio, error) {
	var doc studentPortfolioDocument
	if err := r.collection.FindOne(ctx, bson.M{"student_id": studentID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.StudentPortfolio{}, ErrNotFound
		}
		return model.StudentPortfolio{}, fmt.Errorf("mengambil portofolio mahasiswa %d: %w", studentID, err)
	}
	return doc.toModel(), nil
}

func (r *studentPortfolioMongoRepository) Update(ctx context.Context, portfolio model.StudentPortfolio) (model.StudentPortfolio, error) {
	doc := portfolioDocumentFromModel(portfolio)
	var updated studentPortfolioDocument
	err := r.collection.FindOneAndUpdate(
		ctx,
		bson.M{"student_id": portfolio.StudentID},
		bson.M{"$set": bson.M{
			"headline":     doc.Headline,
			"bio":          doc.Bio,
			"projects":     doc.Projects,
			"certificates": doc.Certificates,
		}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.StudentPortfolio{}, ErrNotFound
		}
		return model.StudentPortfolio{}, fmt.Errorf("memperbarui portofolio mahasiswa %d: %w", portfolio.StudentID, err)
	}
	return updated.toModel(), nil
}

func (r *studentPortfolioMongoRepository) Upsert(ctx context.Context, portfolio model.StudentPortfolio) (model.StudentPortfolio, error) {
	for i := range portfolio.Projects {
		if portfolio.Projects[i].ID == "" {
			portfolio.Projects[i].ID = bson.NewObjectID().Hex()
		}
	}
	doc := portfolioDocumentFromModel(portfolio)
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"student_id": portfolio.StudentID},
		bson.M{
			"$set": bson.M{
				"headline":     doc.Headline,
				"bio":          doc.Bio,
				"projects":     doc.Projects,
				"certificates": doc.Certificates,
			},
			"$setOnInsert": bson.M{"student_id": portfolio.StudentID},
		},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return model.StudentPortfolio{}, ErrDuplicate
		}
		return model.StudentPortfolio{}, fmt.Errorf("menyimpan portofolio mahasiswa %d: %w", portfolio.StudentID, err)
	}
	return r.FindByStudentID(ctx, portfolio.StudentID)
}

func (r *studentPortfolioMongoRepository) AddProject(ctx context.Context, studentID int, project model.PortfolioProject) (model.StudentPortfolio, error) {
	if project.ID == "" {
		project.ID = bson.NewObjectID().Hex()
	}
	projectDoc := portfolioProjectDocument{
		ID: project.ID, Title: project.Title,
		Technologies: append([]string{}, project.Technologies...),
		Link:         project.Link, Year: project.Year,
	}
	var updated studentPortfolioDocument
	err := r.collection.FindOneAndUpdate(
		ctx,
		addProjectFilter(studentID),
		bson.M{"$push": bson.M{"projects": projectDoc}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if err == nil {
		return updated.toModel(), nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return model.StudentPortfolio{}, fmt.Errorf("menambahkan project portofolio mahasiswa %d: %w", studentID, err)
	}
	current, lookupErr := r.FindByStudentID(ctx, studentID)
	if lookupErr != nil {
		return model.StudentPortfolio{}, lookupErr
	}
	if len(current.Projects) >= maxPortfolioProjects {
		return model.StudentPortfolio{}, ErrPortfolioProjectLimit
	}
	return model.StudentPortfolio{}, fmt.Errorf("menambahkan project portofolio mahasiswa %d: %w", studentID, ErrNotFound)
}

func addProjectFilter(studentID int) bson.M {
	return bson.M{"student_id": studentID, "projects.19": bson.M{"$exists": false}}
}

func (r *studentPortfolioMongoRepository) RemoveProject(ctx context.Context, studentID int, projectID string) (model.StudentPortfolio, error) {
	var updated studentPortfolioDocument
	err := r.collection.FindOneAndUpdate(
		ctx,
		bson.M{"student_id": studentID, "projects.id": projectID},
		bson.M{"$pull": bson.M{"projects": bson.M{"id": projectID}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if err == nil {
		return updated.toModel(), nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return model.StudentPortfolio{}, fmt.Errorf("menghapus project %s dari portofolio mahasiswa %d: %w", projectID, studentID, err)
	}
	if _, lookupErr := r.FindByStudentID(ctx, studentID); lookupErr != nil {
		return model.StudentPortfolio{}, lookupErr
	}
	return model.StudentPortfolio{}, ErrPortfolioProjectMissing
}

func technologyAggregationPipeline() mongo.Pipeline {
	return mongo.Pipeline{
		{{Key: "$unwind", Value: "$projects"}},
		{{Key: "$unwind", Value: "$projects.technologies"}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$projects.technologies"},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}, {Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: 10}},
	}
}

func (r *studentPortfolioMongoRepository) TopTechnologies(ctx context.Context) ([]model.TechnologyStat, error) {
	cursor, err := r.collection.Aggregate(ctx, technologyAggregationPipeline())
	if err != nil {
		return nil, fmt.Errorf("mengagregasi teknologi portofolio: %w", err)
	}
	defer cursor.Close(ctx)

	type technologyResult struct {
		Technology string `bson:"_id"`
		Count      int64  `bson:"count"`
	}
	results := make([]model.TechnologyStat, 0)
	for cursor.Next(ctx) {
		var row technologyResult
		if err := cursor.Decode(&row); err != nil {
			return nil, fmt.Errorf("membaca hasil agregasi teknologi portofolio: %w", err)
		}
		results = append(results, model.TechnologyStat{Technology: row.Technology, Count: row.Count})
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("membaca cursor agregasi teknologi portofolio: %w", err)
	}
	return results, nil
}

func (r *studentPortfolioMongoRepository) Delete(ctx context.Context, studentID int) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"student_id": studentID})
	if err != nil {
		return fmt.Errorf("menghapus portofolio mahasiswa %d: %w", studentID, err)
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}
