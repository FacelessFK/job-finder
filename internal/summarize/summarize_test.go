package summarize

import (
	"strings"
	"testing"
)

func TestSimpleSummarizerShortens(t *testing.T) {
	s := NewSimple(120)
	long := strings.Repeat("word ", 100) // خیلی طولانی
	out := s.Summarize(long)
	if len([]rune(out)) > 123 { // 120 + "..."
		t.Errorf("summary too long: %d runes", len([]rune(out)))
	}
	if !strings.HasSuffix(out, "...") {
		t.Errorf("expected ellipsis suffix, got %q", out)
	}
}

func TestSimpleSummarizerCollapsesWhitespace(t *testing.T) {
	s := NewSimple(200)
	out := s.Summarize("Hello\n\n   world\t\tfoo")
	if out != "Hello world foo" {
		t.Errorf("got %q", out)
	}
}

func TestSimpleSummarizerShortTextUnchanged(t *testing.T) {
	s := NewSimple(200)
	out := s.Summarize("Short one.")
	if out != "Short one." {
		t.Errorf("got %q", out)
	}
}
