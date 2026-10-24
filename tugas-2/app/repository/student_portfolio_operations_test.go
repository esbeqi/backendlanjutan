package repository

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestAddProjectFilterEnforcesAtomicLimit(t *testing.T) {
	filter := addProjectFilter(17)
	if filter["student_id"] != 17 {
		t.Fatalf("filter student_id = %v, want 17", filter["student_id"])
	}
	position, ok := filter["projects.19"].(bson.M)
	if !ok || position["$exists"] != false {
		t.Fatalf("limit filter = %#v; want projects.19 $exists:false", filter)
	}
}

func TestTechnologyAggregationPipelineUsesTwoUnwindsAndTopTen(t *testing.T) {
	pipeline := technologyAggregationPipeline()
	if len(pipeline) != 5 {
		t.Fatalf("pipeline has %d stages, want 5", len(pipeline))
	}
	wantUnwinds := []any{"$projects", "$projects.technologies"}
	for i, want := range wantUnwinds {
		stage := pipeline[i]
		if len(stage) != 1 || stage[0].Key != "$unwind" || stage[0].Value != want {
			t.Fatalf("stage %d = %#v, want unwind %s", i, stage, want)
		}
	}
	group := pipeline[2]
	if len(group) != 1 || group[0].Key != "$group" {
		t.Fatalf("group stage = %#v", group)
	}
	groupFields, ok := group[0].Value.(bson.D)
	if !ok || len(groupFields) != 2 || groupFields[0].Key != "_id" || groupFields[1].Key != "count" {
		t.Fatalf("group fields = %#v, want technology id and count", group[0].Value)
	}
	sort := pipeline[3]
	if len(sort) != 1 || sort[0].Key != "$sort" {
		t.Fatalf("sort stage = %#v", sort)
	}
	last := pipeline[4]
	if len(last) != 1 || last[0].Key != "$limit" || last[0].Value != 10 {
		t.Fatalf("last stage = %#v, want limit 10", last)
	}
}
