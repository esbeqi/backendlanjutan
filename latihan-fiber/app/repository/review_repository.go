package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"latihan-fiber/app/model"
)

var (
	ErrInvalidReviewID = errors.New("format id ulasan tidak valid")
	ErrNotOwner        = errors.New("bukan pemilik data")
	ErrOwnReview       = errors.New("tidak boleh menilai ulasan sendiri")
	ErrAlreadyVoted    = errors.New("sudah pernah menilai ulasan ini")
	ErrReplyLimit      = errors.New("jumlah balasan sudah mencapai batas")
)

const MaxRepliesPerReview = 50

// ReviewRepository adalah kontrak operasi baca/tulis dasar ulasan MongoDB.
type ReviewRepository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, review model.Review) (model.Review, error)
	FindByID(ctx context.Context, id string) (model.Review, error)
	FindByCourse(ctx context.Context, query model.ReviewQuery) ([]model.Review, error)
	UpdateOwn(ctx context.Context, id string, userID int, request model.UpdateReviewRequest) (model.Review, error)
	Delete(ctx context.Context, id string, userID int, anyOwner bool) error
	AddReply(ctx context.Context, id string, reply model.Reply) (model.Review, error)
	MarkHelpful(ctx context.Context, id string, userID int) (model.Review, error)
	Summary(ctx context.Context, courseID int) (model.ReviewSummary, error)
}

func reviewUpdateFields(request model.UpdateReviewRequest, updatedAt time.Time) bson.M {
	fields := bson.M{"updated_at": updatedAt}
	if request.Rating != nil {
		fields["rating"] = *request.Rating
	}
	if request.Comment != nil {
		fields["comment"] = *request.Comment
	}
	if request.Tags != nil {
		fields["tags"] = *request.Tags
	}
	return fields
}

func replyAppendFilter(id bson.ObjectID) bson.M {
	limitField := "replies." + strconv.Itoa(MaxRepliesPerReview-1)
	return bson.M{"_id": id, limitField: bson.M{"$exists": false}}
}

func helpfulVoteFilter(id bson.ObjectID, userID int) bson.M {
	return bson.M{
		"_id":        id,
		"user_id":    bson.M{"$ne": userID},
		"helpful_by": bson.M{"$ne": userID},
	}
}

func helpfulVoteUpdate(userID int) bson.M {
	return bson.M{
		"$addToSet": bson.M{"helpful_by": userID},
		"$inc":      bson.M{"helpful_count": 1},
	}
}

type reviewIndexDefinition struct {
	name   string
	keys   bson.D
	unique bool
}

func reviewIndexDefinitions() []reviewIndexDefinition {
	return []reviewIndexDefinition{
		{
			name:   "course_user_unique",
			keys:   bson.D{{Key: "course_id", Value: 1}, {Key: "user_id", Value: 1}},
			unique: true,
		},
		{
			name: "course_newest",
			keys: bson.D{{Key: "course_id", Value: 1}, {Key: "_id", Value: -1}},
		},
		{
			name: "course_tags",
			keys: bson.D{{Key: "course_id", Value: 1}, {Key: "tags", Value: 1}},
		},
	}
}

func reviewIndexModels() []mongo.IndexModel {
	definitions := reviewIndexDefinitions()
	models := make([]mongo.IndexModel, 0, len(definitions))
	for _, definition := range definitions {
		indexOptions := options.Index().SetName(definition.name)
		if definition.unique {
			indexOptions.SetUnique(true)
		}
		models = append(models, mongo.IndexModel{
			Keys:    definition.keys,
			Options: indexOptions,
		})
	}
	return models
}

// reviewDocument adalah representasi BSON yang hanya digunakan repository.
type reviewDocument struct {
	ID           bson.ObjectID   `bson:"_id,omitempty"`
	CourseID     int             `bson:"course_id"`
	UserID       int             `bson:"user_id"`
	Username     string          `bson:"username"`
	Rating       int             `bson:"rating"`
	Comment      string          `bson:"comment"`
	Tags         []string        `bson:"tags"`
	Replies      []replyDocument `bson:"replies"`
	HelpfulCount int             `bson:"helpful_count"`
	HelpfulBy    []int           `bson:"helpful_by"`
	CreatedAt    time.Time       `bson:"created_at"`
	UpdatedAt    time.Time       `bson:"updated_at"`
}

type replyDocument struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    int           `bson:"user_id"`
	Username  string        `bson:"username"`
	Body      string        `bson:"body"`
	CreatedAt time.Time     `bson:"created_at"`
}

type reviewSummaryStatsDocument struct {
	Count int     `bson:"count"`
	Avg   float64 `bson:"avg"`
}

type reviewSummaryCountDocument struct {
	ID    int `bson:"_id"`
	Total int `bson:"total"`
}

type reviewSummaryTagDocument struct {
	Tag   string `bson:"_id"`
	Total int    `bson:"total"`
}

type reviewSummaryDocument struct {
	Stats        []reviewSummaryStatsDocument `bson:"stats"`
	Distribution []reviewSummaryCountDocument `bson:"distribution"`
	TopTags      []reviewSummaryTagDocument   `bson:"top_tags"`
}

func reviewSummaryPipeline(courseID int) mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "course_id", Value: courseID}}}},
		bson.D{{Key: "$facet", Value: bson.D{
			{Key: "stats", Value: mongo.Pipeline{
				bson.D{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: nil},
					{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
					{Key: "avg", Value: bson.D{{Key: "$avg", Value: "$rating"}}},
				}}},
			}},
			{Key: "distribution", Value: mongo.Pipeline{
				bson.D{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: "$rating"},
					{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
				}}},
				bson.D{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
			}},
			{Key: "top_tags", Value: mongo.Pipeline{
				bson.D{{Key: "$unwind", Value: "$tags"}},
				bson.D{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: "$tags"},
					{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
				}}},
				bson.D{{Key: "$sort", Value: bson.D{{Key: "total", Value: -1}, {Key: "_id", Value: 1}}}},
				bson.D{{Key: "$limit", Value: 5}},
			}},
		}}},
	}
}

func (d reviewSummaryDocument) toModel(courseID int) model.ReviewSummary {
	summary := model.ReviewSummary{
		CourseID:     courseID,
		Distribution: make(map[string]int, len(d.Distribution)),
		TopTags:      make([]model.TagCount, 0, len(d.TopTags)),
	}
	if len(d.Stats) > 0 {
		summary.Count = d.Stats[0].Count
		summary.AverageRating = d.Stats[0].Avg
	}
	for _, item := range d.Distribution {
		summary.Distribution[strconv.Itoa(item.ID)] = item.Total
	}
	for _, item := range d.TopTags {
		summary.TopTags = append(summary.TopTags, model.TagCount{Tag: item.Tag, Total: item.Total})
	}
	return summary
}

func (d reviewDocument) toModel() model.Review {
	tags := d.Tags
	if tags == nil {
		tags = []string{}
	}

	replies := make([]model.Reply, 0, len(d.Replies))
	for _, reply := range d.Replies {
		replies = append(replies, model.Reply{
			ID:        reply.ID.Hex(),
			UserID:    reply.UserID,
			Username:  reply.Username,
			Body:      reply.Body,
			CreatedAt: reply.CreatedAt,
		})
	}

	return model.Review{
		ID:           d.ID.Hex(),
		CourseID:     d.CourseID,
		UserID:       d.UserID,
		Username:     d.Username,
		Rating:       d.Rating,
		Comment:      d.Comment,
		Tags:         tags,
		Replies:      replies,
		HelpfulCount: d.HelpfulCount,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

type reviewMongoRepository struct {
	collection *mongo.Collection
}

func NewReviewRepository(db *mongo.Database) ReviewRepository {
	return &reviewMongoRepository{collection: db.Collection("course_reviews")}
}

// EnsureIndexes memastikan index course_reviews tersedia. Pemanggilan ulang
// dengan nama dan spesifikasi yang sama aman: MongoDB mempertahankan index itu.
func (r *reviewMongoRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.collection.Indexes().CreateMany(ctx, reviewIndexModels()); err != nil {
		return fmt.Errorf("membuat index collection course_reviews: %w", err)
	}
	return nil
}

func parseReviewID(id string) (bson.ObjectID, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return bson.ObjectID{}, ErrInvalidReviewID
	}
	return objectID, nil
}

// nowReview dibulatkan ke milidetik karena BSON Date tidak menyimpan nanodetik.
func nowReview() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

func newReviewDocument(review model.Review) reviewDocument {
	doc := reviewDocument{
		ID:           bson.NewObjectID(),
		CourseID:     review.CourseID,
		UserID:       review.UserID,
		Username:     review.Username,
		Rating:       review.Rating,
		Comment:      review.Comment,
		Tags:         review.Tags,
		Replies:      make([]replyDocument, 0),
		HelpfulCount: 0,
		HelpfulBy:    make([]int, 0),
		CreatedAt:    nowReview(),
	}
	doc.UpdatedAt = doc.CreatedAt
	if doc.Tags == nil {
		doc.Tags = make([]string, 0)
	}
	return doc
}

func (r *reviewMongoRepository) Create(ctx context.Context, review model.Review) (model.Review, error) {
	doc := newReviewDocument(review)
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		return model.Review{}, fmt.Errorf("menyimpan ulasan: %w", err)
	}
	return doc.toModel(), nil
}

func (r *reviewMongoRepository) FindByID(ctx context.Context, id string) (model.Review, error) {
	objectID, err := parseReviewID(id)
	if err != nil {
		return model.Review{}, err
	}

	var doc reviewDocument
	if err := r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.Review{}, ErrNotFound
		}
		return model.Review{}, fmt.Errorf("mengambil ulasan: %w", err)
	}
	return doc.toModel(), nil
}

func (r *reviewMongoRepository) FindByCourse(ctx context.Context, query model.ReviewQuery) ([]model.Review, error) {
	filter := bson.M{"course_id": query.CourseID}
	if query.Tag != "" {
		filter["tags"] = query.Tag
	}
	if query.Before != "" {
		before, err := parseReviewID(query.Before)
		if err != nil {
			return nil, err
		}
		filter["_id"] = bson.M{"$lt": before}
	}

	findOptions := options.Find().SetSort(bson.D{{Key: "_id", Value: -1}})
	if query.Limit > 0 {
		findOptions.SetLimit(int64(query.Limit))
	}

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("mengambil daftar ulasan: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []reviewDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("membaca daftar ulasan: %w", err)
	}

	reviews := make([]model.Review, 0, len(docs))
	for _, doc := range docs {
		reviews = append(reviews, doc.toModel())
	}
	return reviews, nil
}

// UpdateOwn hanya mengubah field yang diberikan dan memeriksa kepemilikan
// sebagai bagian dari filter MongoDB yang sama dengan operasi $set.
func (r *reviewMongoRepository) UpdateOwn(
	ctx context.Context,
	id string,
	userID int,
	request model.UpdateReviewRequest,
) (model.Review, error) {
	objectID, err := parseReviewID(id)
	if err != nil {
		return model.Review{}, err
	}

	var updated reviewDocument
	err = r.collection.FindOneAndUpdate(
		ctx,
		bson.M{"_id": objectID, "user_id": userID},
		bson.M{"$set": reviewUpdateFields(request, nowReview())},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if err == nil {
		return updated.toModel(), nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return model.Review{}, fmt.Errorf("mengubah ulasan: %w", err)
	}
	return model.Review{}, r.whyNoMatch(ctx, objectID)
}

// Delete menghapus review milik user; anyOwner hanya boleh diberikan setelah
// lapisan pemanggil memastikan hak moderasi yang sesuai.
func (r *reviewMongoRepository) Delete(ctx context.Context, id string, userID int, anyOwner bool) error {
	objectID, err := parseReviewID(id)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": objectID}
	if !anyOwner {
		filter["user_id"] = userID
	}
	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("menghapus ulasan: %w", err)
	}
	if result.DeletedCount == 0 {
		return r.whyNoMatch(ctx, objectID)
	}
	return nil
}

// AddReply menambahkan balasan hanya jika slot ke-50 belum terisi. Filter dan
// $push dijalankan dalam satu operasi sehingga request serentak tidak melewati
// batas maksimum.
func (r *reviewMongoRepository) AddReply(ctx context.Context, id string, reply model.Reply) (model.Review, error) {
	objectID, err := parseReviewID(id)
	if err != nil {
		return model.Review{}, err
	}

	replyDoc := replyDocument{
		ID:        bson.NewObjectID(),
		UserID:    reply.UserID,
		Username:  reply.Username,
		Body:      reply.Body,
		CreatedAt: nowReview(),
	}
	var updated reviewDocument
	err = r.collection.FindOneAndUpdate(
		ctx,
		replyAppendFilter(objectID),
		bson.M{"$push": bson.M{"replies": replyDoc}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if err == nil {
		return updated.toModel(), nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return model.Review{}, fmt.Errorf("menambah balasan: %w", err)
	}

	count, err := r.collection.CountDocuments(ctx, bson.M{"_id": objectID})
	if err != nil {
		return model.Review{}, fmt.Errorf("memeriksa ulasan: %w", err)
	}
	if count == 0 {
		return model.Review{}, ErrNotFound
	}
	return model.Review{}, ErrReplyLimit
}

// MarkHelpful mencatat pemilih dan menaikkan counter secara atomik. Filter
// memastikan review bukan milik pemilih dan pemilih belum tercatat.
func (r *reviewMongoRepository) MarkHelpful(ctx context.Context, id string, userID int) (model.Review, error) {
	objectID, err := parseReviewID(id)
	if err != nil {
		return model.Review{}, err
	}

	var updated reviewDocument
	err = r.collection.FindOneAndUpdate(
		ctx,
		helpfulVoteFilter(objectID, userID),
		helpfulVoteUpdate(userID),
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if err == nil {
		return updated.toModel(), nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return model.Review{}, fmt.Errorf("menilai ulasan: %w", err)
	}

	var current reviewDocument
	if err := r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&current); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.Review{}, ErrNotFound
		}
		return model.Review{}, fmt.Errorf("memeriksa ulasan: %w", err)
	}
	if current.UserID == userID {
		return model.Review{}, ErrOwnReview
	}
	return model.Review{}, ErrAlreadyVoted
}

// Summary merangkum jumlah dan rata-rata rating, distribusi bintang, serta
// lima tag terpopuler untuk satu kursus dalam satu aggregation pipeline.
func (r *reviewMongoRepository) Summary(ctx context.Context, courseID int) (model.ReviewSummary, error) {
	cursor, err := r.collection.Aggregate(ctx, reviewSummaryPipeline(courseID))
	if err != nil {
		return model.ReviewSummary{}, fmt.Errorf("menjalankan aggregation ringkasan ulasan kursus %d: %w", courseID, err)
	}
	defer cursor.Close(ctx)

	var results []reviewSummaryDocument
	if err := cursor.All(ctx, &results); err != nil {
		return model.ReviewSummary{}, fmt.Errorf("membaca hasil aggregation ringkasan kursus %d: %w", courseID, err)
	}
	if len(results) == 0 {
		return reviewSummaryDocument{}.toModel(courseID), nil
	}
	return results[0].toModel(courseID), nil
}

func (r *reviewMongoRepository) whyNoMatch(ctx context.Context, objectID bson.ObjectID) error {
	count, err := r.collection.CountDocuments(ctx, bson.M{"_id": objectID})
	if err != nil {
		return fmt.Errorf("memeriksa ulasan: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return ErrNotOwner
}
