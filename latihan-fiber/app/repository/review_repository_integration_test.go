package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"latihan-fiber/app/model"
)

const (
	mongoTestURIEnv   = "MONGO_TEST_URI"
	mongoTestDatabase = "praktikum_backend_atomic_test"
)

func newReviewMongoIntegrationFixture(t *testing.T) (ReviewRepository, *mongo.Collection) {
	t.Helper()

	uri := os.Getenv(mongoTestURIEnv)
	if uri == "" {
		t.Skipf("integration test dilewati: set %s ke URI MongoDB khusus testing", mongoTestURIEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("gagal membuat client MongoDB: %v", err)
	}
	t.Cleanup(func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		if err := client.Disconnect(disconnectCtx); err != nil {
			t.Errorf("gagal memutus client MongoDB: %v", err)
		}
	})

	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("MongoDB test tidak dapat diakses: %v", err)
	}

	collection := client.Database(mongoTestDatabase).Collection("course_reviews")
	return NewReviewRepository(client.Database(mongoTestDatabase)), collection
}

func createIntegrationReview(t *testing.T, collection *mongo.Collection, initialReplies []replyDocument, helpfulCount int, helpfulBy []int) model.Review {
	t.Helper()

	doc := newReviewDocument(model.Review{
		CourseID: 987654321,
		UserID:   int(time.Now().UnixNano() % 1_000_000_000),
		Username: "mongo-atomic-integration-test",
		Rating:   5,
		Comment:  "review untuk integration test atomic update",
		Tags:     []string{"integration-test"},
	})
	if initialReplies != nil {
		doc.Replies = initialReplies
	}
	doc.HelpfulCount = helpfulCount
	if helpfulBy == nil {
		doc.HelpfulBy = make([]int, 0)
	} else {
		doc.HelpfulBy = helpfulBy
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := collection.InsertOne(ctx, doc); err != nil {
		t.Fatalf("gagal membuat review uji: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		result, err := collection.DeleteOne(cleanupCtx, bson.M{"_id": doc.ID})
		if err != nil {
			t.Errorf("gagal membersihkan review uji %s: %v", doc.ID.Hex(), err)
			return
		}
		if result.DeletedCount != 1 {
			t.Errorf("review uji %s tidak terhapus; deleted count = %d", doc.ID.Hex(), result.DeletedCount)
		}
	})

	return doc.toModel()
}

func runConcurrentReviewOperations(count int, operation func(index int) error) []error {
	ready := sync.WaitGroup{}
	ready.Add(count)
	start := make(chan struct{})
	results := make(chan error, count)
	var done sync.WaitGroup
	done.Add(count)

	for i := 0; i < count; i++ {
		go func(index int) {
			defer done.Done()
			ready.Done()
			<-start
			results <- operation(index)
		}(i)
	}

	ready.Wait()
	close(start)
	done.Wait()
	close(results)

	errs := make([]error, 0, count)
	for err := range results {
		errs = append(errs, err)
	}
	return errs
}

func TestReviewRepositoryAddReplyConcurrentLimit(t *testing.T) {
	repo, collection := newReviewMongoIntegrationFixture(t)
	initialReplies := make([]replyDocument, 48)
	for i := range initialReplies {
		initialReplies[i] = replyDocument{
			ID:        bson.NewObjectID(),
			UserID:    i + 1,
			Username:  fmt.Sprintf("seed-user-%d", i+1),
			Body:      fmt.Sprintf("reply awal %d", i+1),
			CreatedAt: time.Now().UTC(),
		}
	}
	review := createIntegrationReview(t, collection, initialReplies, 0, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	errs := runConcurrentReviewOperations(10, func(index int) error {
		_, err := repo.AddReply(ctx, review.ID, model.Reply{
			UserID:   1000 + index,
			Username: fmt.Sprintf("concurrent-user-%d", index),
			Body:     fmt.Sprintf("reply concurrent %d", index),
		})
		return err
	})

	successes, limitErrors := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrReplyLimit):
			limitErrors++
		default:
			t.Errorf("AddReply mengembalikan error tak terduga: %v", err)
		}
	}
	if successes != 2 || limitErrors != 8 {
		t.Fatalf("hasil AddReply: sukses=%d, batas=%d; ingin sukses=2 dan batas=8", successes, limitErrors)
	}

	stored, err := repo.FindByID(ctx, review.ID)
	if err != nil {
		t.Fatalf("gagal membaca kembali review uji: %v", err)
	}
	if got := len(stored.Replies); got != 50 {
		t.Fatalf("jumlah reply tersimpan = %d; ingin 50", got)
	}
}

func TestReviewRepositoryMarkHelpfulConcurrentOnce(t *testing.T) {
	repo, collection := newReviewMongoIntegrationFixture(t)
	review := createIntegrationReview(t, collection, nil, 0, []int{})
	const voterID = 246801357

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	errs := runConcurrentReviewOperations(20, func(int) error {
		_, err := repo.MarkHelpful(ctx, review.ID, voterID)
		return err
	})

	successes, duplicateErrors := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrAlreadyVoted):
			duplicateErrors++
		default:
			t.Errorf("MarkHelpful mengembalikan error tak terduga: %v", err)
		}
	}
	if successes != 1 || duplicateErrors != 19 {
		t.Fatalf("hasil MarkHelpful: sukses=%d, duplicate=%d; ingin sukses=1 dan duplicate=19", successes, duplicateErrors)
	}

	id, err := bson.ObjectIDFromHex(review.ID)
	if err != nil {
		t.Fatalf("ID review uji tidak valid: %v", err)
	}
	var stored reviewDocument
	if err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&stored); err != nil {
		t.Fatalf("gagal membaca dokumen review uji: %v", err)
	}
	if stored.HelpfulCount != 1 {
		t.Fatalf("helpful_count tersimpan = %d; ingin 1", stored.HelpfulCount)
	}
	voterCount := 0
	for _, userID := range stored.HelpfulBy {
		if userID == voterID {
			voterCount++
		}
	}
	if voterCount != 1 {
		t.Fatalf("kemunculan voter di helpful_by = %d; ingin 1", voterCount)
	}
}

func TestReviewRepositorySummaryAggregation(t *testing.T) {
	repo, collection := newReviewMongoIntegrationFixture(t)
	testID := bson.NewObjectID()
	courseID := int(testID[9])<<16 | int(testID[10])<<8 | int(testID[11])
	reviews := []model.Review{
		{CourseID: courseID, UserID: 710001, Username: "summary-1", Rating: 5, Comment: "review summary pertama", Tags: []string{"clear", "practical"}},
		{CourseID: courseID, UserID: 710002, Username: "summary-2", Rating: 4, Comment: "review summary kedua", Tags: []string{"clear", "examples"}},
		{CourseID: courseID, UserID: 710003, Username: "summary-3", Rating: 3, Comment: "review summary ketiga", Tags: []string{"practical"}},
	}

	for _, review := range reviews {
		doc := newReviewDocument(review)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := collection.InsertOne(ctx, doc)
		cancel()
		if err != nil {
			t.Fatalf("gagal membuat dokumen aggregation uji: %v", err)
		}
		id := doc.ID
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			result, err := collection.DeleteOne(cleanupCtx, bson.M{"_id": id})
			if err != nil {
				t.Errorf("gagal membersihkan dokumen aggregation uji %s: %v", id.Hex(), err)
				return
			}
			if result.DeletedCount != 1 {
				t.Errorf("dokumen aggregation uji %s tidak terhapus; deleted count = %d", id.Hex(), result.DeletedCount)
			}
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	got, err := repo.Summary(ctx, courseID)
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	want := model.ReviewSummary{
		CourseID:      courseID,
		Count:         3,
		AverageRating: 4,
		Distribution:  map[string]int{"3": 1, "4": 1, "5": 1},
		TopTags:       []model.TagCount{{Tag: "clear", Total: 2}, {Tag: "practical", Total: 2}, {Tag: "examples", Total: 1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Summary() = %#v, want %#v", got, want)
	}
}
