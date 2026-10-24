package repository

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	mongooptions "go.mongodb.org/mongo-driver/v2/mongo/options"

	"latihan-fiber/app/model"
)

func TestReviewDocumentToModel(t *testing.T) {
	reviewID := bson.NewObjectID()
	replyID := bson.NewObjectID()
	createdAt := time.Date(2026, time.October, 19, 0, 33, 8, 334000000, time.UTC)
	replyAt := createdAt.Add(35 * time.Second)

	doc := reviewDocument{
		ID:           reviewID,
		CourseID:     1,
		UserID:       4,
		Username:     "mhs001",
		Rating:       5,
		Comment:      "Materi Go-nya jelas",
		Tags:         []string{"dosen-jelas", "seru"},
		Replies:      []replyDocument{{ID: replyID, UserID: 6, Username: "mhs003", Body: "Setuju", CreatedAt: replyAt}},
		HelpfulCount: 1,
		HelpfulBy:    []int{5},
		CreatedAt:    createdAt,
		UpdatedAt:    replyAt,
	}

	want := model.Review{
		ID:           reviewID.Hex(),
		CourseID:     1,
		UserID:       4,
		Username:     "mhs001",
		Rating:       5,
		Comment:      "Materi Go-nya jelas",
		Tags:         []string{"dosen-jelas", "seru"},
		Replies:      []model.Reply{{ID: replyID.Hex(), UserID: 6, Username: "mhs003", Body: "Setuju", CreatedAt: replyAt}},
		HelpfulCount: 1,
		CreatedAt:    createdAt,
		UpdatedAt:    replyAt,
	}

	encoded, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("bson.Marshal() error = %v", err)
	}
	var decoded reviewDocument
	if err := bson.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("bson.Unmarshal() error = %v", err)
	}

	if got := decoded.toModel(); !reflect.DeepEqual(got, want) {
		t.Fatalf("toModel() = %#v, want %#v", got, want)
	}
}

func TestReviewDocumentToModelNormalizesNilArrays(t *testing.T) {
	got := (reviewDocument{ID: bson.NewObjectID()}).toModel()
	if got.Tags == nil || got.Replies == nil {
		t.Fatalf("empty arrays must be non-nil: tags=%v replies=%v", got.Tags, got.Replies)
	}
}

func TestNewReviewDocumentSerializesHelpfulByAsEmptyArray(t *testing.T) {
	doc := newReviewDocument(model.Review{
		CourseID: 1,
		UserID:   2,
		Username: "mhs001",
		Rating:   5,
		Comment:  "Materi Go-nya jelas",
	})
	if doc.HelpfulBy == nil {
		t.Fatal("new review HelpfulBy must be a non-nil empty slice")
	}

	encoded, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("bson.Marshal() error = %v", err)
	}
	helpfulBy := bson.Raw(encoded).Lookup("helpful_by")
	if helpfulBy.Type != bson.TypeArray {
		t.Fatalf("helpful_by BSON type = %v, want array", helpfulBy.Type)
	}
	values, err := helpfulBy.Array().Values()
	if err != nil {
		t.Fatalf("helpful_by BSON array is invalid: %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("helpful_by array length = %d, want 0", len(values))
	}
}

func TestParseReviewID(t *testing.T) {
	want := bson.NewObjectID()
	got, err := parseReviewID(want.Hex())
	if err != nil {
		t.Fatalf("parseReviewID(valid) error = %v", err)
	}
	if got != want {
		t.Fatalf("parseReviewID(valid) = %v, want %v", got, want)
	}

	if _, err := parseReviewID("not-an-object-id"); !errors.Is(err, ErrInvalidReviewID) {
		t.Fatalf("parseReviewID(invalid) error = %v, want %v", err, ErrInvalidReviewID)
	}
}

func TestReviewIndexDefinitions(t *testing.T) {
	definitions := reviewIndexDefinitions()
	if len(definitions) != 3 {
		t.Fatalf("len(reviewIndexDefinitions()) = %d, want 3", len(definitions))
	}

	tests := []struct {
		name   string
		keys   bson.D
		unique bool
	}{
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

	for i, test := range tests {
		got := definitions[i]
		if got.name != test.name || got.unique != test.unique || !reflect.DeepEqual(got.keys, test.keys) {
			t.Errorf("definition[%d] = %#v, want name %q keys %#v unique %t", i, got, test.name, test.keys, test.unique)
		}
	}

	models := reviewIndexModels()
	if len(models) != len(tests) {
		t.Fatalf("len(reviewIndexModels()) = %d, want %d", len(models), len(tests))
	}
	for i, test := range tests {
		indexOptions := &mongooptions.IndexOptions{}
		for _, setter := range models[i].Options.List() {
			if err := setter(indexOptions); err != nil {
				t.Fatalf("apply index options[%d]: %v", i, err)
			}
		}
		if indexOptions.Name == nil || *indexOptions.Name != test.name {
			t.Errorf("index model[%d] name = %v, want %q", i, indexOptions.Name, test.name)
		}
		gotUnique := indexOptions.Unique != nil && *indexOptions.Unique
		if gotUnique != test.unique {
			t.Errorf("index model[%d] unique = %t, want %t", i, gotUnique, test.unique)
		}
	}
}

func TestReplyAppendFilterRequiresRoomBelowLimit(t *testing.T) {
	id := bson.NewObjectID()
	want := bson.M{
		"_id":        id,
		"replies.49": bson.M{"$exists": false},
	}
	if got := replyAppendFilter(id); !reflect.DeepEqual(got, want) {
		t.Fatalf("replyAppendFilter() = %#v, want %#v", got, want)
	}
}

func TestHelpfulVoteFilterAndUpdateArePaired(t *testing.T) {
	id := bson.NewObjectID()
	userID := 42

	wantFilter := bson.M{
		"_id":        id,
		"user_id":    bson.M{"$ne": userID},
		"helpful_by": bson.M{"$ne": userID},
	}
	if got := helpfulVoteFilter(id, userID); !reflect.DeepEqual(got, wantFilter) {
		t.Fatalf("helpfulVoteFilter() = %#v, want %#v", got, wantFilter)
	}

	wantUpdate := bson.M{
		"$addToSet": bson.M{"helpful_by": userID},
		"$inc":      bson.M{"helpful_count": 1},
	}
	if got := helpfulVoteUpdate(userID); !reflect.DeepEqual(got, wantUpdate) {
		t.Fatalf("helpfulVoteUpdate() = %#v, want %#v", got, wantUpdate)
	}
}

func TestReviewUpdateFieldsOnlyIncludeProvidedFields(t *testing.T) {
	rating := 4
	comment := "Diperbarui dengan rapi"
	updatedAt := time.Date(2026, time.October, 20, 1, 2, 3, 0, time.UTC)
	want := bson.M{
		"updated_at": updatedAt,
		"rating":     rating,
		"comment":    comment,
	}

	got := reviewUpdateFields(model.UpdateReviewRequest{
		Rating:  &rating,
		Comment: &comment,
	}, updatedAt)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reviewUpdateFields() = %#v, want %#v", got, want)
	}
}

func TestReviewSummaryDocumentToModel(t *testing.T) {
	doc := reviewSummaryDocument{
		Stats: []reviewSummaryStatsDocument{{Count: 3, Avg: 4.5}},
		Distribution: []reviewSummaryCountDocument{
			{ID: 4, Total: 1},
			{ID: 5, Total: 2},
		},
		TopTags: []reviewSummaryTagDocument{
			{Tag: "materi-jelas", Total: 2},
			{Tag: "praktis", Total: 1},
		},
	}
	want := model.ReviewSummary{
		CourseID:      17,
		Count:         3,
		AverageRating: 4.5,
		Distribution:  map[string]int{"1": 0, "2": 0, "3": 0, "4": 1, "5": 2},
		TopTags:       []model.TagCount{{Tag: "materi-jelas", Total: 2}, {Tag: "praktis", Total: 1}},
	}
	if got := doc.toModel(17); !reflect.DeepEqual(got, want) {
		t.Fatalf("toModel() = %#v, want %#v", got, want)
	}
}

func TestEmptyReviewSummaryDocumentToModel(t *testing.T) {
	got := (reviewSummaryDocument{}).toModel(17)
	want := model.ReviewSummary{
		CourseID:     17,
		Distribution: map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0},
		TopTags:      []model.TagCount{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("empty summary toModel() = %#v, want %#v", got, want)
	}
}

func TestReviewSummaryDocumentRoundsAverageRatingToTwoDecimals(t *testing.T) {
	doc := reviewSummaryDocument{Stats: []reviewSummaryStatsDocument{{Count: 8, Avg: 4.125}}}
	if got := doc.toModel(17).AverageRating; got != 4.13 {
		t.Fatalf("AverageRating = %.2f, want 4.13", got)
	}
}

func TestReviewSummaryPipeline(t *testing.T) {
	want := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "course_id", Value: 17}}}},
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

	if got := reviewSummaryPipeline(17); !reflect.DeepEqual(got, want) {
		t.Fatalf("reviewSummaryPipeline() = %#v, want %#v", got, want)
	}
}
