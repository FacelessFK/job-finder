// Package store وضعیت جاب‌های دیده‌شده را برای جلوگیری از ارسال تکراری نگه می‌دارد.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aghaie/job-finder/internal/jsearch"
)

// Store مجموعه آیدی جاب‌های دیده‌شده را با حفظ ترتیب افزوده‌شدن نگه می‌دارد.
type Store struct {
	path  string
	ids   map[string]struct{}
	order []string // ترتیب افزوده‌شدن، برای هرس
}

type fileFormat struct {
	SeenIDs []string `json:"seen_ids"`
}

// Load وضعیت را از فایل می‌خواند. اگر فایل نباشد، وضعیت خالی برمی‌گرداند.
func Load(path string) (*Store, error) {
	s := &Store{
		path:  path,
		ids:   make(map[string]struct{}),
		order: nil,
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	for _, id := range f.SeenIDs {
		if _, ok := s.ids[id]; !ok {
			s.ids[id] = struct{}{}
			s.order = append(s.order, id)
		}
	}
	return s, nil
}

// FilterNew جاب‌هایی را برمی‌گرداند که نه در وضعیت ذخیره‌شده هستند و نه در همین
// فراخوانی تکراری‌اند. وضعیت را تغییر نمی‌دهد.
func (s *Store) FilterNew(jobs []jsearch.Job) []jsearch.Job {
	var fresh []jsearch.Job
	inBatch := make(map[string]struct{})
	for _, j := range jobs {
		if j.ID == "" {
			continue
		}
		if _, seen := s.ids[j.ID]; seen {
			continue
		}
		if _, dup := inBatch[j.ID]; dup {
			continue
		}
		inBatch[j.ID] = struct{}{}
		fresh = append(fresh, j)
	}
	return fresh
}

// MarkSeen یک آیدی را دیده‌شده علامت می‌زند.
func (s *Store) MarkSeen(id string) {
	if id == "" {
		return
	}
	if _, ok := s.ids[id]; ok {
		return
	}
	s.ids[id] = struct{}{}
	s.order = append(s.order, id)
}

// Prune فقط max آیدی آخر (جدیدترین‌ها) را نگه می‌دارد تا فایل بی‌نهایت بزرگ نشود.
func (s *Store) Prune(max int) {
	if max <= 0 || len(s.order) <= max {
		return
	}
	drop := s.order[:len(s.order)-max]
	for _, id := range drop {
		delete(s.ids, id)
	}
	s.order = s.order[len(s.order)-max:]
}

// Save وضعیت را روی دیسک می‌نویسد.
func (s *Store) Save() error {
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f := fileFormat{SeenIDs: s.order}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
