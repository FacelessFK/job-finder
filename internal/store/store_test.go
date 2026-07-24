package store

import (
	"path/filepath"
	"testing"

	"github.com/aghaie/job-finder/internal/jsearch"
)

func TestFilterNewAndPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seen.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	jobs := []jsearch.Job{
		{ID: "a"}, {ID: "b"}, {ID: "a"}, // "a" دوبار در همین اجرا
	}
	fresh := s.FilterNew(jobs)
	if len(fresh) != 2 {
		t.Fatalf("expected 2 fresh jobs, got %d", len(fresh))
	}

	s.MarkSeen("a")
	s.MarkSeen("b")
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// بارگذاری دوباره: حالا a و b دیده‌شده‌اند
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	fresh2 := s2.FilterNew(jobs)
	if len(fresh2) != 0 {
		t.Fatalf("expected 0 fresh after reload, got %d", len(fresh2))
	}
}

func TestPruneKeepsMostRecent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seen.json")
	s, _ := Load(path)

	for _, id := range []string{"1", "2", "3", "4", "5"} {
		s.MarkSeen(id)
	}
	s.Prune(3) // فقط ۳ تای آخر بماند
	if got := len(s.ids); got != 3 {
		t.Fatalf("expected 3 ids after prune, got %d", got)
	}
	// جدیدترین‌ها باید بمانند: id 5 هنوز دیده‌شده است
	if len(s.FilterNew([]jsearch.Job{{ID: "5"}})) != 0 {
		t.Errorf("id 5 should still be seen")
	}
	// قدیمی‌ترین‌ها هرس شده‌اند: id 1 دیگر دیده‌شده نیست
	if len(s.FilterNew([]jsearch.Job{{ID: "1"}})) != 1 {
		t.Errorf("id 1 should have been pruned")
	}
}
