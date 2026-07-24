// Package providers رابط منابع جاب و رجیستری آن‌ها را تعریف می‌کند.
package providers

import (
	"context"
	"fmt"

	"github.com/aghaie/job-finder/internal/core"
)

// Provider یک منبع جاب.
type Provider interface {
	Name() string
	SearchJobs(ctx context.Context, f core.Filters) ([]core.Job, error)
}

// Factory یک Provider را از روی config می‌سازد.
type Factory func(cfg core.Config) (Provider, error)

var registry = map[string]Factory{}

// Register یک Provider را با نامش ثبت می‌کند (معمولاً در init هر پکیج).
func Register(name string, f Factory) {
	registry[name] = f
}

// Build فقط Providerهای نام‌برده‌شده را می‌سازد.
func Build(names []string, cfg core.Config) ([]Provider, error) {
	out := make([]Provider, 0, len(names))
	for _, n := range names {
		f, ok := registry[n]
		if !ok {
			return nil, fmt.Errorf("unknown provider %q", n)
		}
		p, err := f(cfg)
		if err != nil {
			return nil, fmt.Errorf("build provider %q: %w", n, err)
		}
		out = append(out, p)
	}
	return out, nil
}
