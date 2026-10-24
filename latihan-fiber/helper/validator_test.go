package helper

import (
	"reflect"
	"testing"
)

func TestValidateStructFormatsLengthMessagesByKind(t *testing.T) {
	type request struct {
		StringMin string         `json:"string_min" validate:"min=3"`
		StringMax string         `json:"string_max" validate:"max=2"`
		SliceMin  []string       `json:"slice_min" validate:"min=2"`
		SliceMax  []string       `json:"slice_max" validate:"max=1"`
		ArrayMin  [1]string      `json:"array_min" validate:"min=2"`
		ArrayMax  [2]string      `json:"array_max" validate:"max=1"`
		MapMin    map[string]int `json:"map_min" validate:"min=2"`
		MapMax    map[string]int `json:"map_max" validate:"max=1"`
		NumberMax int            `json:"number_max" validate:"max=1"`
	}
	input := request{
		StringMin: "a",
		StringMax: "long",
		SliceMin:  []string{"one"},
		SliceMax:  []string{"one", "two"},
		ArrayMin:  [1]string{"one"},
		ArrayMax:  [2]string{"one", "two"},
		MapMin:    map[string]int{"one": 1},
		MapMax:    map[string]int{"one": 1, "two": 2},
		NumberMax: 2,
	}

	want := map[string]string{
		"string_min": "minimal 3 karakter",
		"string_max": "maksimal 2 karakter",
		"slice_min":  "minimal 2 item",
		"slice_max":  "maksimal 1 item",
		"array_min":  "minimal 2 item",
		"array_max":  "maksimal 1 item",
		"map_min":    "minimal 2 item",
		"map_max":    "maksimal 1 item",
		"number_max": "nilai maksimal 1",
	}
	if got := ValidateStruct(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("ValidateStruct() = %#v, want %#v", got, want)
	}
}
