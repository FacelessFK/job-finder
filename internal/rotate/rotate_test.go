package rotate

import (
	"reflect"
	"testing"
	"time"
)

func at(hoursFromEpoch int) time.Time {
	return time.Unix(int64(hoursFromEpoch)*3600, 0).UTC()
}

func TestSliceAdvancesEverySlot(t *testing.T) {
	countries := []string{"US", "CA", "GB", "DE"}
	got := [][]string{}
	for h := 0; h < 16; h += 4 {
		got = append(got, Slice(countries, at(h), 4, 1))
	}
	want := [][]string{{"US"}, {"CA"}, {"GB"}, {"DE"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("slices = %v, want %v", got, want)
	}
}

func TestSliceIsStableWithinASlot(t *testing.T) {
	countries := []string{"US", "CA", "GB"}
	a := Slice(countries, at(0), 4, 1)
	b := Slice(countries, at(3), 4, 1) // هنوز همان اسلات
	if !reflect.DeepEqual(a, b) {
		t.Errorf("same slot gave %v then %v", a, b)
	}
	c := Slice(countries, at(4), 4, 1) // اسلات بعدی
	if reflect.DeepEqual(a, c) {
		t.Errorf("next slot should differ, both %v", a)
	}
}

func TestSliceWrapsAround(t *testing.T) {
	countries := []string{"US", "CA", "GB"}
	first := Slice(countries, at(0), 4, 1)
	afterFullCycle := Slice(countries, at(12), 4, 1)
	if !reflect.DeepEqual(first, afterFullCycle) {
		t.Errorf("cycle should repeat: %v vs %v", first, afterFullCycle)
	}
}

func TestSliceCoversEveryCountryOverOneCycle(t *testing.T) {
	countries := []string{"US", "CA", "GB", "DE", "NL"}
	seen := map[string]bool{}
	for slot := 0; slot < len(countries); slot++ {
		for _, c := range Slice(countries, at(slot*4), 4, 1) {
			seen[c] = true
		}
	}
	if len(seen) != len(countries) {
		t.Errorf("cycle covered %d of %d countries: %v", len(seen), len(countries), seen)
	}
}

func TestSliceSupportsMultiplePerRun(t *testing.T) {
	countries := []string{"US", "CA", "GB", "DE", "NL"}
	got := Slice(countries, at(0), 4, 2)
	if !reflect.DeepEqual(got, []string{"US", "CA"}) {
		t.Errorf("got %v, want [US CA]", got)
	}
	next := Slice(countries, at(4), 4, 2)
	if !reflect.DeepEqual(next, []string{"GB", "DE"}) {
		t.Errorf("got %v, want [GB DE]", next)
	}
	// اسلات سوم باید بپیچد و از ابتدا ادامه دهد
	third := Slice(countries, at(8), 4, 2)
	if !reflect.DeepEqual(third, []string{"NL", "US"}) {
		t.Errorf("got %v, want [NL US]", third)
	}
}

func TestSliceDisabledReturnsAll(t *testing.T) {
	countries := []string{"US", "CA", "GB"}
	if got := Slice(countries, at(0), 0, 1); !reflect.DeepEqual(got, countries) {
		t.Errorf("slotHours=0 should disable rotation, got %v", got)
	}
	if got := Slice(countries, at(0), 4, 0); !reflect.DeepEqual(got, countries) {
		t.Errorf("perSlot=0 should disable rotation, got %v", got)
	}
	if got := Slice(countries, at(0), 4, 99); !reflect.DeepEqual(got, countries) {
		t.Errorf("perSlot >= len should return all, got %v", got)
	}
}

func TestSliceEmptyInput(t *testing.T) {
	if got := Slice(nil, at(0), 4, 1); len(got) != 0 {
		t.Errorf("empty input should give empty output, got %v", got)
	}
}
