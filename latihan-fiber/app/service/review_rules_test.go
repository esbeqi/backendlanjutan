package service

import (
	"reflect"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{
		" Tugas-Banyak ",
		"dosen-jelas",
		"tugas-banyak",
		" ",
		"SERU",
		"dosen-jelas",
	})
	want := []string{"tugas-banyak", "dosen-jelas", "seru"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeTags() = %#v, want %#v", got, want)
	}
}
