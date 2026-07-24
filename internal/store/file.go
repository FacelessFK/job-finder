package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// FileStore پیاده‌سازی SeenStore روی یک فایل JSON. فقط وقتی تغییری باشد می‌نویسد.
type FileStore struct {
	path    string
	maxSeen int
	ids     map[string]struct{}
	order   []string
	dirty   bool
}

type fileFormat struct {
	Seen []string `json:"seen"`
}

// NewFileStore وضعیت را از فایل می‌خواند (اگر نبود، خالی).
func NewFileStore(path string, maxSeen int) (*FileStore, error) {
	s := &FileStore{path: path, maxSeen: maxSeen, ids: make(map[string]struct{})}
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
	for _, id := range f.Seen {
		if _, ok := s.ids[id]; !ok {
			s.ids[id] = struct{}{}
			s.order = append(s.order, id)
		}
	}
	return s, nil
}

func (s *FileStore) IsSeen(fp string) bool {
	_, ok := s.ids[fp]
	return ok
}

func (s *FileStore) MarkSeen(fp string) {
	if fp == "" {
		return
	}
	if _, ok := s.ids[fp]; ok {
		return
	}
	s.ids[fp] = struct{}{}
	s.order = append(s.order, fp)
	s.dirty = true
}

// Save فقط در صورت تغییر می‌نویسد و آیدی‌های قدیمی را هرس می‌کند.
func (s *FileStore) Save(ctx context.Context) error {
	if !s.dirty {
		return nil
	}
	s.prune()
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(fileFormat{Seen: s.order}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return err
	}
	s.dirty = false
	return nil
}

func (s *FileStore) prune() {
	if s.maxSeen <= 0 || len(s.order) <= s.maxSeen {
		return
	}
	drop := s.order[:len(s.order)-s.maxSeen]
	for _, id := range drop {
		delete(s.ids, id)
	}
	s.order = s.order[len(s.order)-s.maxSeen:]
}
