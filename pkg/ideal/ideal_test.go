package ideal

import (
	"math"
	"strings"
	"testing"
)

func floatPtr(v float64) *float64 { return &v }

func TestDefaultProfileValid(t *testing.T) {
	p, err := ParseProfileJSON(DefaultProfileJSON)
	if err != nil {
		t.Fatalf("Failed to parse embedded DefaultProfileJSON: %v", err)
	}
	if p.Name != "default" {
		t.Errorf("Expected profile name 'default', got %q", p.Name)
	}
	if len(p.Attributes) != 12 {
		t.Fatalf("Expected 12 attributes in default profile, got %d", len(p.Attributes))
	}
}

func TestLookupAttribute_SaddleHb(t *testing.T) {
	meta, ok := LookupAttribute("saddle_hb")
	if !ok {
		t.Fatalf("LookupAttribute('saddle_hb') failed")
	}
	if meta.Label != "Saddle-HB" || meta.Unit != "mm" {
		t.Errorf("Unexpected meta: %+v", meta)
	}

	// Canonical name variants
	for _, variant := range []string{"saddle-hb", "saddlehb", "SADDLE_HB", "Saddle-HB"} {
		if _, ok := LookupAttribute(variant); !ok {
			t.Errorf("LookupAttribute(%q) failed", variant)
		}
	}
}

func TestEvaluate_OneSidedSeatToRc(t *testing.T) {
	// Seat-RC: ideal >= 235, acceptable >= 220, edge_score 70, falloff below 20, floor -100
	spec := AttributeSpec{
		ID:         "seat_to_rc",
		Weight:     4,
		Ideal:      &Range{Min: floatPtr(235)},
		Acceptable: &Range{Min: floatPtr(220)},
		EdgeScore:  floatPtr(70),
		Falloff:    &Falloff{Below: floatPtr(20)},
		Floor:      floatPtr(-100),
	}
	meta, ok := LookupAttribute("seat_to_rc")
	if !ok {
		t.Fatalf("LookupAttribute('seat_to_rc') failed")
	}

	// 235 -> Ideal (100)
	if res := Evaluate(spec, meta, 235, nil); res.Zone != ZoneIdeal || res.SubScore != 100 {
		t.Errorf("Seat-RC 235 expected ZoneIdeal / 100, got %v / %.1f", res.Zone, res.SubScore)
	}
	// 250 -> Ideal (100)
	if res := Evaluate(spec, meta, 250, nil); res.Zone != ZoneIdeal || res.SubScore != 100 {
		t.Errorf("Seat-RC 250 expected ZoneIdeal / 100, got %v / %.1f", res.Zone, res.SubScore)
	}
	// 220 -> Acceptable edge (70)
	if res := Evaluate(spec, meta, 220, nil); res.Zone != ZoneAcceptable || res.SubScore != 70 {
		t.Errorf("Seat-RC 220 expected ZoneAcceptable / 70, got %v / %.1f", res.Zone, res.SubScore)
	}
	// 200 -> 20 below acceptable (0)
	if res := Evaluate(spec, meta, 200, nil); res.Zone != ZoneOutside || math.Abs(res.SubScore-0) > 0.001 {
		t.Errorf("Seat-RC 200 expected ZoneOutside / 0, got %v / %.1f", res.Zone, res.SubScore)
	}
	// 150 -> Below floor (-100 clamped)
	if res := Evaluate(spec, meta, 150, nil); res.SubScore != -100 {
		t.Errorf("Seat-RC 150 expected clamped to -100, got %.1f", res.SubScore)
	}
}

func TestEvaluate_DesignDocTable(t *testing.T) {
	// AETT with acceptable 606–642, ideal 630–642, edge_score 60, falloff below 10 / above 5
	spec := AttributeSpec{
		ID:         "aett",
		Weight:     10,
		Ideal:      &Range{Min: floatPtr(630), Max: floatPtr(642)},
		Acceptable: &Range{Min: floatPtr(606), Max: floatPtr(642)},
		EdgeScore:  floatPtr(60),
		Falloff:    &Falloff{Below: floatPtr(10), Above: floatPtr(5)},
		Floor:      floatPtr(-200),
	}
	meta, ok := LookupAttribute("aett")
	if !ok {
		t.Fatalf("LookupAttribute('aett') failed")
	}

	testCases := []struct {
		val          float64
		expectedZone ZoneType
		expectedSub  float64
		expectedCell string
	}{
		{596, ZoneOutside, 0, "596 ✗ −34"},
		{601, ZoneOutside, 30, "601 ✗ −29"},
		{606, ZoneAcceptable, 60, "606 ~ −24"},
		{618, ZoneAcceptable, 80, "618 ~ −12"},
		{630, ZoneIdeal, 100, "630 ✓"},
		{635, ZoneIdeal, 100, "635 ✓"},
		{642, ZoneIdeal, 100, "642 ✓"},
		{645, ZoneOutside, 40, "645 ✗ +3"},
		{647, ZoneOutside, 0, "647 ✗ +5"},
	}

	for _, tc := range testCases {
		res := Evaluate(spec, meta, tc.val, nil)
		if res.Zone != tc.expectedZone {
			t.Errorf("val %.0f: expected zone %v, got %v", tc.val, tc.expectedZone, res.Zone)
		}
		if math.Abs(res.SubScore-tc.expectedSub) > 0.001 {
			t.Errorf("val %.0f: expected sub-score %.1f, got %.1f", tc.val, tc.expectedSub, res.SubScore)
		}
		if res.FormattedCell != tc.expectedCell {
			t.Errorf("val %.0f: expected cell %q, got %q", tc.val, tc.expectedCell, res.FormattedCell)
		}
	}
}

func TestEvaluate_OneSidedChainstay(t *testing.T) {
	// Chainstay: ideal >= 470, acceptable >= 445, edge_score 70, falloff below 25, floor -100
	spec := AttributeSpec{
		ID:         "chainstay",
		Weight:     4,
		Ideal:      &Range{Min: floatPtr(470)},
		Acceptable: &Range{Min: floatPtr(445)},
		EdgeScore:  floatPtr(70),
		Falloff:    &Falloff{Below: floatPtr(25)},
		Floor:      floatPtr(-100),
	}
	meta, ok := LookupAttribute("chainstay")
	if !ok {
		t.Fatalf("LookupAttribute('chainstay') failed")
	}

	// 470 -> Ideal (100)
	if res := Evaluate(spec, meta, 470, nil); res.Zone != ZoneIdeal || res.SubScore != 100 {
		t.Errorf("Chainstay 470 expected ZoneIdeal / 100, got %v / %.1f", res.Zone, res.SubScore)
	}
	// 480 -> Ideal (100)
	if res := Evaluate(spec, meta, 480, nil); res.Zone != ZoneIdeal || res.SubScore != 100 {
		t.Errorf("Chainstay 480 expected ZoneIdeal / 100, got %v / %.1f", res.Zone, res.SubScore)
	}
	// 445 -> Acceptable edge (70)
	if res := Evaluate(spec, meta, 445, nil); res.Zone != ZoneAcceptable || res.SubScore != 70 {
		t.Errorf("Chainstay 445 expected ZoneAcceptable / 70, got %v / %.1f", res.Zone, res.SubScore)
	}
	// 420 -> 25 below acceptable (0)
	if res := Evaluate(spec, meta, 420, nil); res.Zone != ZoneOutside || math.Abs(res.SubScore-0) > 0.001 {
		t.Errorf("Chainstay 420 expected ZoneOutside / 0, got %v / %.1f", res.Zone, res.SubScore)
	}
	// 350 -> Below floor (-100 clamped)
	if res := Evaluate(spec, meta, 350, nil); res.SubScore != -100 {
		t.Errorf("Chainstay 350 expected clamped to -100, got %.1f", res.SubScore)
	}
}

func TestEvaluate_DisplayOnly(t *testing.T) {
	spec := AttributeSpec{
		ID: "reach",
	}
	meta, ok := LookupAttribute("reach")
	if !ok {
		t.Fatalf("LookupAttribute('reach') failed")
	}

	res := Evaluate(spec, meta, 475.2, nil)
	if !res.IsDisplayOnly {
		t.Errorf("Expected IsDisplayOnly to be true")
	}
	if res.FormattedCell != "475" {
		t.Errorf("Expected formatted cell '475', got %q", res.FormattedCell)
	}
}

func TestValidation_StrictRejections(t *testing.T) {
	// 1. Unknown JSON field
	jsonUnknown := `{"name": "test", "extra_bad_field": 123, "attributes": [{"id": "reach"}]}`
	if _, err := ParseProfileJSON([]byte(jsonUnknown)); err == nil {
		t.Errorf("Expected error for unknown JSON field, got nil")
	}

	// 2. Unknown attribute ID
	jsonBadID := `{"name": "test", "attributes": [{"id": "nonexistent_attr", "weight": 5, "ideal": {"min": 10}}]}`
	if _, err := ParseProfileJSON([]byte(jsonBadID)); err == nil || !strings.Contains(err.Error(), "not a recognized attribute id") {
		t.Errorf("Expected unrecognized attribute error, got: %v", err)
	}

	// 3. Duplicate attribute ID
	jsonDup := `{"name": "test", "attributes": [{"id": "reach"}, {"id": "reach"}]}`
	if _, err := ParseProfileJSON([]byte(jsonDup)); err == nil || !strings.Contains(err.Error(), "duplicate attribute id") {
		t.Errorf("Expected duplicate attribute error, got: %v", err)
	}

	// 4. Inverted ideal min > max
	jsonInverted := `{"name": "test", "attributes": [{"id": "hta", "weight": 2, "ideal": {"min": 69, "max": 67}, "falloff": {"below": 1, "above": 1}}]}`
	if _, err := ParseProfileJSON([]byte(jsonInverted)); err == nil || !strings.Contains(err.Error(), "exceeds max") {
		t.Errorf("Expected inverted min/max error, got: %v", err)
	}

	// 5. Acceptable wider than ideal but missing edge_score
	jsonMissingEdge := `{"name": "test", "attributes": [{"id": "hta", "weight": 2, "ideal": {"min": 68, "max": 68}, "acceptable": {"min": 67, "max": 69}, "falloff": {"below": 1, "above": 1}}]}`
	if _, err := ParseProfileJSON([]byte(jsonMissingEdge)); err == nil || !strings.Contains(err.Error(), "edge_score is not defined") {
		t.Errorf("Expected missing edge_score error, got: %v", err)
	}

	// 6. Missing falloff
	jsonNoFalloff := `{"name": "test", "attributes": [{"id": "hta", "weight": 2, "ideal": {"min": 68, "max": 68}}]}`
	if _, err := ParseProfileJSON([]byte(jsonNoFalloff)); err == nil || !strings.Contains(err.Error(), "must define positive falloff") {
		t.Errorf("Expected missing falloff error, got: %v", err)
	}

	// 7. Non-negative floor
	jsonBadFloor := `{"name": "test", "attributes": [{"id": "hta", "weight": 2, "ideal": {"min": 68, "max": 68}, "falloff": {"below": 1, "above": 1}, "floor": 10}]}`
	if _, err := ParseProfileJSON([]byte(jsonBadFloor)); err == nil || !strings.Contains(err.Error(), "floor (10.0) must be negative") {
		t.Errorf("Expected negative floor error, got: %v", err)
	}
}

func TestLoadProfile_FallbackToEmbedded(t *testing.T) {
	p, err := LoadProfile("/nonexistent/directory", "default")
	if err != nil {
		t.Fatalf("Expected fallback to embedded default profile, got: %v", err)
	}
	if p.Name != "default" {
		t.Errorf("Expected profile name 'default', got %q", p.Name)
	}
}
