package catalog_test

import (
	"math"
	"testing"

	"bikegeo/internal/reboot/catalog"
	"bikegeo/internal/reboot/pose"
)

func TestStems(t *testing.T) {
	// Q58/Q64 anchor rows:
	//   40 mm ±6° = 59.0, 40 mm 0° = 74.0
	//   50 mm ±6° = 69.0, 50 mm 0° = 84.0
	//   70 mm 0°/±6°/±7° = 100, 80 mm ±6°/±7° = 100
	//   120 mm −20°/±30° = 0
	const (
		abundant = catalog.LengthAbundant
		commonL  = catalog.LengthCommon
		limited  = catalog.LengthLimited
		sparse   = catalog.LengthSparse

		dom   = catalog.AngleDominant
		com   = catalog.AngleCommon
		niche = catalog.AngleNiche
	)

	want := []struct {
		L     float64
		A     pose.Deg
		LA    catalog.LengthAvailability
		AA    catalog.AngleAvailability
		Score float64
	}{
		// 40 (abundant)
		{40, -6, abundant, com, 59.0},
		{40, 0, abundant, dom, 74.0},
		{40, 6, abundant, com, 59.0},
		// 50 (abundant)
		{50, -20, abundant, niche, 45.0},
		{50, -17, abundant, niche, 45.0},
		{50, -6, abundant, com, 69.0},
		{50, -5, abundant, niche, 46.5},
		{50, 0, abundant, dom, 84.0},
		{50, 5, abundant, niche, 46.5},
		{50, 6, abundant, com, 69.0},
		// 60 (abundant)
		{60, -30, abundant, niche, 55.0},
		{60, -20, abundant, niche, 55.0},
		{60, -17, abundant, niche, 55.0},
		{60, -12, abundant, niche, 55.0},
		{60, -7, abundant, com, 79.0},
		{60, -6, abundant, dom, 86.5},
		{60, 0, abundant, dom, 95.5},
		{60, 6, abundant, dom, 86.5},
		{60, 7, abundant, com, 79.0},
		{60, 12, abundant, niche, 55.0},
		{60, 17, abundant, niche, 55.0},
		{60, 30, abundant, niche, 55.0},
		// 70 (abundant)
		{70, -30, abundant, niche, 55.0},
		{70, -20, abundant, niche, 55.0},
		{70, -17, abundant, com, 79.0},
		{70, -15, abundant, niche, 58.0},
		{70, -12, abundant, niche, 62.5},
		{70, -10, abundant, com, 89.5},
		{70, -7, abundant, dom, 100.0},
		{70, -6, abundant, dom, 100.0},
		{70, 0, abundant, dom, 100.0},
		{70, 6, abundant, dom, 100.0},
		{70, 7, abundant, dom, 100.0},
		{70, 10, abundant, com, 89.5},
		{70, 12, abundant, niche, 62.5},
		{70, 15, abundant, niche, 58.0},
		{70, 17, abundant, com, 79.0},
		{70, 30, abundant, niche, 55.0},
		// 80 (abundant)
		{80, -30, abundant, niche, 55.0},
		{80, -20, abundant, niche, 55.0},
		{80, -17, abundant, com, 79.0},
		{80, -12, abundant, niche, 62.5},
		{80, -10, abundant, com, 89.5},
		{80, -7, abundant, dom, 100.0},
		{80, -6, abundant, dom, 100.0},
		{80, 0, abundant, com, 94.0},
		{80, 6, abundant, dom, 100.0},
		{80, 7, abundant, dom, 100.0},
		{80, 10, abundant, com, 89.5},
		{80, 12, abundant, niche, 62.5},
		{80, 17, abundant, com, 79.0},
		{80, 30, abundant, niche, 55.0},
		// 90 (common)
		{90, -30, commonL, niche, 38.0},
		{90, -20, commonL, niche, 38.0},
		{90, -17, commonL, com, 62.0},
		{90, -15, commonL, niche, 41.0},
		{90, -12, commonL, niche, 45.5},
		{90, -10, commonL, com, 72.5},
		{90, -7, commonL, dom, 83.0},
		{90, -6, commonL, dom, 83.0},
		{90, 0, commonL, com, 77.0},
		{90, 6, commonL, dom, 83.0},
		{90, 7, commonL, dom, 83.0},
		{90, 10, commonL, com, 72.5},
		{90, 15, commonL, niche, 41.0},
		{90, 17, commonL, com, 62.0},
		{90, 30, commonL, niche, 38.0},
		// 100 (common)
		{100, -30, commonL, niche, 28.0},
		{100, -20, commonL, niche, 28.0},
		{100, -17, commonL, com, 52.0},
		{100, -12, commonL, niche, 35.5},
		{100, -10, commonL, com, 62.5},
		{100, -7, commonL, dom, 73.0},
		{100, -6, commonL, dom, 73.0},
		{100, 0, commonL, com, 67.0},
		{100, 6, commonL, dom, 73.0},
		{100, 7, commonL, dom, 73.0},
		{100, 10, commonL, com, 62.5},
		{100, 17, commonL, com, 52.0},
		{100, 30, commonL, niche, 28.0},
		// 110 (limited)
		{110, -30, limited, niche, 12.25},
		{110, -20, limited, niche, 12.25},
		{110, -17, limited, com, 36.25},
		{110, -12, limited, niche, 19.75},
		{110, -10, limited, com, 46.75},
		{110, -7, limited, dom, 57.25},
		{110, -6, limited, dom, 57.25},
		{110, 0, limited, com, 51.25},
		{110, 6, limited, dom, 57.25},
		{110, 7, limited, dom, 57.25},
		{110, 10, limited, com, 46.75},
		{110, 17, limited, com, 36.25},
		{110, 30, limited, niche, 12.25},
		// 120 (sparse)
		{120, -30, sparse, niche, 0.0},
		{120, -20, sparse, niche, 0.0},
		{120, -17, sparse, com, 24.0},
		{120, -12, sparse, niche, 7.5},
		{120, -10, sparse, com, 34.5},
		{120, -7, sparse, dom, 45.0},
		{120, -6, sparse, dom, 45.0},
		{120, 0, sparse, niche, 15.0},
		{120, 6, sparse, dom, 45.0},
		{120, 7, sparse, dom, 45.0},
		{120, 10, sparse, com, 34.5},
		{120, 12, sparse, niche, 7.5},
		{120, 17, sparse, com, 24.0},
		{120, 30, sparse, niche, 0.0},
	}

	got := catalog.Stems()
	if len(got) != 107 {
		t.Fatalf("len(Stems()) = %d, want 107", len(got))
	}
	if len(got) != len(want) {
		t.Fatalf("len(Stems()) = %d, want %d", len(got), len(want))
	}

	wantCounts := map[float64]int{
		40: 3, 50: 7, 60: 12, 70: 16, 80: 14, 90: 15, 100: 13, 110: 13, 120: 14,
	}
	gotCounts := make(map[float64]int)
	type key struct {
		L float64
		A pose.Deg
	}
	seen := make(map[key]bool)
	var sum float64

	for i, s := range got {
		gotCounts[s.LengthMm]++
		k := key{s.LengthMm, s.AngleDeg}
		if seen[k] {
			t.Fatalf("duplicate stem (%v mm, %v°)", s.LengthMm, s.AngleDeg)
		}
		seen[k] = true
		if i > 0 {
			prev := got[i-1]
			if prev.LengthMm > s.LengthMm || (prev.LengthMm == s.LengthMm && prev.AngleDeg >= s.AngleDeg) {
				t.Fatalf("stems not strictly sorted at %d: %+v then %+v", i, prev, s)
			}
		}
		if !(s.Score >= 0 && s.Score <= 100) {
			t.Errorf("stem (%v mm, %v°) score = %v, want in [0, 100]", s.LengthMm, s.AngleDeg, s.Score)
		}
		sum += s.Score

		w := want[i]
		if s.LengthMm != w.L || s.AngleDeg != w.A || s.LengthAvailability != w.LA || s.AngleAvailability != w.AA {
			t.Errorf("row %d: got (%v, %v, %s, %s), want (%v, %v, %s, %s)",
				i, s.LengthMm, s.AngleDeg, s.LengthAvailability, s.AngleAvailability,
				w.L, w.A, w.LA, w.AA)
		}
		if math.Abs(s.Score-w.Score) > 1e-9 {
			t.Errorf("row %d (%v mm, %v°): Score = %.12g, want %.12g", i, s.LengthMm, s.AngleDeg, s.Score, w.Score)
		}
	}

	for L, wc := range wantCounts {
		if gotCounts[L] != wc {
			t.Errorf("count for %.0f mm = %d, want %d", L, gotCounts[L], wc)
		}
	}
	if math.Abs(sum-6229.25) > 1e-9 {
		t.Errorf("sum of scores = %.12g, want 6229.25", sum)
	}
}

func TestStemsDedupKeepsHigherTier(t *testing.T) {
	stems := catalog.Stems()

	find := func(length float64, angle pose.Deg) (catalog.Stem, bool) {
		for _, s := range stems {
			if s.LengthMm == length && s.AngleDeg == angle {
				return s, true
			}
		}
		return catalog.Stem{}, false
	}

	// 70 mm -17°: yaml lists common "+/-17" and niche -17 -> AngleCommon.
	if s, ok := find(70, -17); !ok || s.AngleAvailability != catalog.AngleCommon {
		t.Errorf("70 mm -17° = %+v (ok=%v), want AngleCommon", s, ok)
	}

	// 60 mm -17° appears once, as AngleNiche.
	count60m17 := 0
	for _, s := range stems {
		if s.LengthMm == 60 && s.AngleDeg == -17 {
			count60m17++
			if s.AngleAvailability != catalog.AngleNiche {
				t.Errorf("60 mm -17° tier = %s, want AngleNiche", s.AngleAvailability)
			}
		}
	}
	if count60m17 != 1 {
		t.Errorf("60 mm -17° count = %d, want 1", count60m17)
	}

	// 120 mm 0° is AngleNiche.
	if s, ok := find(120, 0); !ok || s.AngleAvailability != catalog.AngleNiche {
		t.Errorf("120 mm 0° = %+v (ok=%v), want AngleNiche", s, ok)
	}
}

func TestStemOptions(t *testing.T) {
	stems := catalog.Stems()
	options := catalog.StemOptions()
	if len(options) != 107 || len(options) != len(stems) {
		t.Fatalf("StemOptions = %d, Stems = %d, want 107", len(options), len(stems))
	}
	for i, s := range options {
		if s.LengthMm != stems[i].LengthMm || s.AngleDeg != stems[i].AngleDeg || s.Score != stems[i].Score {
			t.Errorf("stem option %d = %+v, want %+v", i, s, stems[i])
		}
		if err := s.Validate(); err != nil {
			t.Errorf("stem option %d: %v", i, err)
		}
	}
}

func TestStemByDimensions(t *testing.T) {
	for _, tc := range []struct {
		length, angle, score float64
	}{
		{60, -6, 86.5}, {70, -6, 100.0}, {100, -10, 62.5}, {120, -30, 0.0},
	} {
		s, ok := catalog.StemByDimensions(tc.length, pose.Deg(tc.angle))
		if !ok || s.Score != tc.score {
			t.Errorf("StemByDimensions(%v, %v) = (%+v, %v), want score %v", tc.length, tc.angle, s, ok, tc.score)
		}
	}
	s, ok := catalog.StemByDimensions(70+1e-12, -6-1e-12)
	if !ok || s.LengthMm != 70 || s.AngleDeg != -6 {
		t.Errorf("tolerance match = (%+v, %v), want 70/-6", s, ok)
	}
	for _, tc := range []struct{ length, angle float64 }{
		{65, -6}, {70, -8}, {134.2, -6}, {math.NaN(), -6}, {70, math.Inf(1)},
	} {
		if _, ok := catalog.StemByDimensions(tc.length, pose.Deg(tc.angle)); ok {
			t.Errorf("StemByDimensions(%v, %v) ok = true, want false", tc.length, tc.angle)
		}
	}
}
