package providers

import (
	"context"
	"testing"

	"github.com/aghaie/job-finder/internal/core"
)

type fakeProvider struct{ name string }

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) SearchJobs(context.Context, core.Filters) ([]core.Job, error) {
	return nil, nil
}

func TestRegisterAndBuild(t *testing.T) {
	Register("fake", func(core.Config) (Provider, error) { return fakeProvider{"fake"}, nil })
	ps, err := Build([]string{"fake"}, core.Config{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(ps) != 1 || ps[0].Name() != "fake" {
		t.Fatalf("unexpected: %+v", ps)
	}
}

func TestBuildUnknown(t *testing.T) {
	if _, err := Build([]string{"nope"}, core.Config{}); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
