package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileStoreSeenAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seen.json")
	s, err := NewFileStore(path, 100)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if s.IsSeen("a") {
		t.Fatal("should not be seen yet")
	}
	s.MarkSeen("a")
	if !s.IsSeen("a") {
		t.Fatal("should be seen")
	}
	if err := s.Save(context.Background()); err != nil {
		t.Fatalf("save: %v", err)
	}

	s2, _ := NewFileStore(path, 100)
	if !s2.IsSeen("a") {
		t.Fatal("should persist across reload")
	}
}

func TestFileStorePrune(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seen.json")
	s, _ := NewFileStore(path, 3)
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		s.MarkSeen(id)
	}
	if err := s.Save(context.Background()); err != nil {
		t.Fatalf("save: %v", err)
	}
	s2, _ := NewFileStore(path, 3)
	if s2.IsSeen("1") {
		t.Error("oldest should be pruned")
	}
	if !s2.IsSeen("5") {
		t.Error("newest should remain")
	}
}
