package ideal

import (
	"math"
	"strings"
	"testing"
)

func TestLookupAttribute_Trail(t *testing.T) {
	for _, variant := range []string{"trail", "trail_mm", "trail-mm", "TRAIL"} {
		meta, ok := LookupAttribute(variant)
		if !ok {
			t.Errorf("LookupAttribute(%q) failed", variant)
			continue
		}
		if meta.Label != "Trail" || meta.Unit != "mm" {
			t.Errorf("Unexpected meta for %q: %+v", variant, meta)
		}
	}
}

func TestLookupAttribute_SeatpostExtension(t *testing.T) {
	for _, variant := range []string{"seatpost_extension", "seatpost-extension", "seatpostextension", "seatpost_ext", "seatpost-ext", "seatpostext", "seatpost", "post_ext", "post-ext", "postext", "post_extension"} {
		meta, ok := LookupAttribute(variant)
		if !ok {
			t.Errorf("LookupAttribute(%q) failed", variant)
			continue
		}
		if meta.Label != "Seatpost Ext" || meta.Unit != "mm" {
			t.Errorf("Unexpected meta for %q: %+v", variant, meta)
		}
	}
}

func TestLookupAttribute_SaddleHb(t *testing.T) {
	meta, ok := LookupAttribute("saddle_hb")
	if !ok {
		t.Fatalf("LookupAttribute('saddle_hb') failed")
	}
	if meta.Label != "BRP–HB" || meta.Unit != "mm" {
		t.Errorf("Unexpected meta: %+v", meta)
	}

	// Canonical name variants
	for _, variant := range []string{"saddle-hb", "saddlehb", "SADDLE_HB", "Saddle-HB"} {
		if _, ok := LookupAttribute(variant); !ok {
			t.Errorf("LookupAttribute(%q) failed", variant)
		}
	}
}

func TestLookupAttribute_BrpSetbackAndDrop(t *testing.T) {
	// BRP Setback lookup and variants
	for _, variant := range []string{"brp_setback", "brp-setback", "brpsetback", "brp", "BRP", "BRP_SETBACK"} {
		meta, ok := LookupAttribute(variant)
		if !ok {
			t.Errorf("LookupAttribute(%q) failed", variant)
			continue
		}
		if meta.Label != "BRP Setback" || meta.Unit != "mm" {
			t.Errorf("Unexpected meta for %q: %+v", variant, meta)
		}
	}

	// Saddle-HB Drop / HB Rise lookup and variants
	for _, variant := range []string{"saddle_hb_drop", "saddle-hb-drop", "saddlehbdrop", "hb_drop", "drop", "SADDLE_HB_DROP", "hb_rise", "rise"} {
		meta, ok := LookupAttribute(variant)
		if !ok {
			t.Errorf("LookupAttribute(%q) failed", variant)
			continue
		}
		if meta.Label != "HB Rise" || meta.Unit != "mm" {
			t.Errorf("Unexpected meta for %q: %+v", variant, meta)
		}
	}

	// Best Stem lookup and variants
	for _, variant := range []string{"best_stem", "best-stem", "beststem", "stem"} {
		meta, ok := LookupAttribute(variant)
		if !ok {
			t.Errorf("LookupAttribute(%q) failed", variant)
			continue
		}
		if meta.Label != "Best Stem" {
			t.Errorf("Unexpected meta for %q: %+v", variant, meta)
		}
	}

	// Handlebar Clamp Fit / Oval Fits lookup and variants
	for _, variant := range []string{"handlebar_clamp_fit", "handlebar-clamp-fit", "cockpit_fit", "cockpit", "clamp_fit", "hb_fit_area", "oval_fits", "ovalfits"} {
		meta, ok := LookupAttribute(variant)
		if !ok {
			t.Errorf("LookupAttribute(%q) failed", variant)
			continue
		}
		if meta.Label != "Oval Fits" {
			t.Errorf("Unexpected meta for %q: %+v", variant, meta)
		}
	}
}

func TestIdmatchProfileValid(t *testing.T) {
	p, err := GetProfile("idmatch")
	if err != nil {
		t.Fatalf("Failed to get idmatch profile: %v", err)
	}
	if p.Name != "idmatch" {
		t.Errorf("Expected profile name 'idmatch', got %q", p.Name)
	}
	if p.SaddleHeightMm == nil || *p.SaddleHeightMm != 746.0 {
		t.Errorf("Expected SaddleHeightMm 746, got %v", p.SaddleHeightMm)
	}
	if err := ValidateProfile(p); err != nil {
		t.Fatalf("ValidateProfile failed on idmatch: %v", err)
	}
}

func TestEvaluate_OneSidedSeatToRc(t *testing.T) {
	// Seat-RC: ideal >= 235, acceptable >= 220, edge_score 70, falloff below 20, floor -100
	spec := AttributeSpec{
		ID:         "seat_to_rc",
		Weight:     4,
		Ideal:      AtLeast(235),
		Acceptable: AtLeast(220),
		EdgeScore:  new(float64(70)),
		Falloff:    FalloffBelow(20),
		Floor:      new(float64(-100)),
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
		Ideal:      Between(630, 642),
		Acceptable: Between(606, 642),
		EdgeScore:  new(float64(60)),
		Falloff:    FalloffBoth(10, 5),
		Floor:      new(float64(-200)),
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
		{596, ZoneOutside, 0, "596 ✗ 34 sht"},
		{601, ZoneOutside, 30, "601 ✗ 29 sht"},
		{606, ZoneAcceptable, 60, "606 ~ 24 sht"},
		{618, ZoneAcceptable, 80, "618 ~ 12 sht"},
		{630, ZoneIdeal, 100, "630 ✓"},
		{635, ZoneIdeal, 100, "635 ✓"},
		{642, ZoneIdeal, 100, "642 ✓"},
		{645, ZoneOutside, 40, "645 ✗ 3 lng"},
		{647, ZoneOutside, 0, "647 ✗ 5 lng"},
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
		Ideal:      AtLeast(470),
		Acceptable: AtLeast(445),
		EdgeScore:  new(float64(70)),
		Falloff:    FalloffBelow(25),
		Floor:      new(float64(-100)),
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
	spec := DisplayOnly("reach")
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
	// 1. Unknown attribute ID
	pBadID := &Profile{
		Name: "test",
		Attributes: []AttributeSpec{
			{ID: "nonexistent_attr", Weight: 5, Ideal: AtLeast(10)},
		},
	}
	if _, err := NormalizeAndValidate(pBadID); err == nil || !strings.Contains(err.Error(), "not a recognized attribute id") {
		t.Errorf("Expected unrecognized attribute error, got: %v", err)
	}

	// 2. Duplicate attribute ID
	pDup := &Profile{
		Name: "test",
		Attributes: []AttributeSpec{
			DisplayOnly("reach"),
			DisplayOnly("reach"),
		},
	}
	if _, err := NormalizeAndValidate(pDup); err == nil || !strings.Contains(err.Error(), "duplicate attribute id") {
		t.Errorf("Expected duplicate attribute error, got: %v", err)
	}

	// 3. Inverted ideal min > max
	pInverted := &Profile{
		Name: "test",
		Attributes: []AttributeSpec{
			{ID: "hta", Weight: 2, Ideal: Between(69, 67), Falloff: FalloffBoth(1, 1)},
		},
	}
	if _, err := NormalizeAndValidate(pInverted); err == nil || !strings.Contains(err.Error(), "exceeds max") {
		t.Errorf("Expected inverted min/max error, got: %v", err)
	}

	// 4. Acceptable wider than ideal but missing edge_score
	pMissingEdge := &Profile{
		Name: "test",
		Attributes: []AttributeSpec{
			{ID: "hta", Weight: 2, Ideal: Exact(68), Acceptable: Between(67, 69), Falloff: FalloffBoth(1, 1)},
		},
	}
	if _, err := NormalizeAndValidate(pMissingEdge); err == nil || !strings.Contains(err.Error(), "edge_score is not defined") {
		t.Errorf("Expected missing edge_score error, got: %v", err)
	}

	// 5. Missing falloff
	pNoFalloff := &Profile{
		Name: "test",
		Attributes: []AttributeSpec{
			{ID: "hta", Weight: 2, Ideal: Exact(68)},
		},
	}
	if _, err := NormalizeAndValidate(pNoFalloff); err == nil || !strings.Contains(err.Error(), "must define positive falloff") {
		t.Errorf("Expected missing falloff error, got: %v", err)
	}

	// 6. Non-negative floor
	pBadFloor := &Profile{
		Name: "test",
		Attributes: []AttributeSpec{
			{ID: "hta", Weight: 2, Ideal: Exact(68), Falloff: FalloffBoth(1, 1), Floor: new(float64(10))},
		},
	}
	if _, err := NormalizeAndValidate(pBadFloor); err == nil || !strings.Contains(err.Error(), "floor (10.0) must be negative") {
		t.Errorf("Expected negative floor error, got: %v", err)
	}
}

func TestGetProfile_Default(t *testing.T) {
	p, err := GetProfile("")
	if err != nil {
		t.Fatalf("Expected default profile, got: %v", err)
	}
	if p.Name != "idmatch" {
		t.Errorf("Expected profile name 'idmatch', got %q", p.Name)
	}

	pDef, err := GetProfile("default")
	if err != nil {
		t.Fatalf("Expected profile for 'default', got: %v", err)
	}
	if pDef.Name != "idmatch" {
		t.Errorf("Expected 'default' to resolve to 'idmatch', got %q", pDef.Name)
	}
}

func TestEvaluate_DirectionalWordsAndBoundaryRounding(t *testing.T) {
	p := IdmatchProfile()
	attrMap := make(map[string]AttributeSpec)
	for _, a := range p.Attributes {
		attrMap[a.ID] = a
	}

	cases := []struct {
		id       string
		val      float64
		expected string
	}{
		{"brp_setback", 204, "204 ✗ 15 fwd"},
		{"brp_setback", 221, "221 ~ 2 aft"},
		{"seatpost_extension", 221, "221 ~ 29 sht"},
		{"seatpost_extension", 335, "335 ~ 25 lng"},
		{"seat_to_rc", 221, "221"}, // DisplayOnly attribute
		{"sparevert", 60, "60"},    // DisplayOnly attribute
		{"sparevert", 60, "60"},    // DisplayOnly attribute
		{"bbdrop", 66, "66 ~ 4 hi"},
		{"bbdrop", 77, "77 ~ 7 lo"},
		{"hta", 67.7, "67.7"}, // DisplayOnly attribute
		{"frontwt", 35.0, "35.0 ~ 0.8% lo"},
		{"trail", 83, "83 ✗ 15 qck"},
		{"trail", 101, "101 ✓"},
		{"trail", 108, "108 ~ 2 slw"},
		{"trail", 115, "115 ✗ 9 slw"},
	}

	for _, tc := range cases {
		spec := attrMap[tc.id]
		meta, ok := LookupAttribute(tc.id)
		if !ok {
			t.Fatalf("LookupAttribute(%q) failed", tc.id)
		}
		ev := Evaluate(spec, meta, tc.val, nil)
		if ev.FormattedCell != tc.expected {
			t.Errorf("%s(%.2f): expected %q, got %q", tc.id, tc.val, tc.expected, ev.FormattedCell)
		}
	}
}
