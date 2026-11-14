package ideal

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"bikegeo/pkg/model"
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
	if p.SaddleHeightMm != model.Some(746.0) {
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
		EdgeScore:  model.Some(70.0),
		Falloff:    FalloffBelow(20),
		Floor:      model.Some(-100.0),
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
	// Far below: keeps losing points, approaching but never reaching the -100 floor.
	assertStrictlyDecreasingAboveFloor(t, spec, meta, []float64{199, 180, 150, 100, 0, -1000, -1e6}, -100)
}

func TestEvaluate_DesignDocTable(t *testing.T) {
	// AETT with acceptable 606–642, ideal 630–642, edge_score 60, falloff below 10 / above 5
	spec := AttributeSpec{
		ID:         "aett",
		Weight:     10,
		Ideal:      Between(630, 642),
		Acceptable: Between(606, 642),
		EdgeScore:  model.Some(60.0),
		Falloff:    FalloffBoth(10, 5),
		Floor:      model.Some(-200.0),
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
		{601, ZoneOutside, 26.1, "601 ✗ 29 sht"},
		{606, ZoneAcceptable, 60, "606 ~ 24 sht"},
		{618, ZoneAcceptable, 80, "618 ~ 12 sht"},
		{630, ZoneIdeal, 100, "630 ✓"},
		{635, ZoneIdeal, 100, "635 ✓"},
		{642, ZoneIdeal, 100, "642 ✓"},
		{645, ZoneOutside, 30.8, "645 ✗ 3 lng"},
		{647, ZoneOutside, 0, "647 ✗ 5 lng"},
	}

	for _, tc := range testCases {
		res := Evaluate(spec, meta, tc.val, nil)
		if res.Zone != tc.expectedZone {
			t.Errorf("val %.0f: expected zone %v, got %v", tc.val, tc.expectedZone, res.Zone)
		}
		if math.Abs(res.SubScore-tc.expectedSub) > 0.05 {
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
		EdgeScore:  model.Some(70.0),
		Falloff:    FalloffBelow(25),
		Floor:      model.Some(-100.0),
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
	// Far below: keeps losing points, approaching but never reaching the -100 floor.
	assertStrictlyDecreasingAboveFloor(t, spec, meta, []float64{419, 400, 350, 200, 0, -1000, -1e6}, -100)
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
			{ID: "hta", Weight: 2, Ideal: Exact(68), Falloff: FalloffBoth(1, 1), Floor: model.Some(10.0)},
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

// assertStrictlyDecreasingAboveFloor checks that each successive value (ordered
// from least to most extreme) scores strictly lower, and always above floor.
func assertStrictlyDecreasingAboveFloor(t *testing.T, spec AttributeSpec, meta AttributeMeta, vals []float64, floor float64) {
	t.Helper()
	prev := math.Inf(1)
	for _, v := range vals {
		got := Evaluate(spec, meta, v, nil).SubScore
		if !(got < prev) {
			t.Errorf("%s at %v: score %.4f is not below the previous %.4f", spec.ID, v, got, prev)
		}
		if !(got > floor) {
			t.Errorf("%s at %v: score %.4f reached floor %.1f", spec.ID, v, got, floor)
		}
		prev = got
	}
}

// TestIdmatchProfile_CurvesHaveNoFlatsOrJumps sweeps every scored attribute of the
// compiled profile far beyond its acceptable range on both sides.
func TestIdmatchProfile_CurvesHaveNoFlatsOrJumps(t *testing.T) {
	for _, spec := range IdmatchProfile().Attributes {
		if spec.Ideal == nil {
			continue
		}
		meta, ok := LookupAttribute(spec.ID)
		if !ok {
			t.Fatalf("unknown attribute %q", spec.ID)
		}
		floor := spec.Floor.OrZero()

		sweep := func(start, dir float64) {
			prev := Evaluate(spec, meta, start, nil).SubScore
			for i := 1; i <= 20000; i++ {
				v := start + dir*float64(i)*0.05
				got := Evaluate(spec, meta, v, nil).SubScore
				if !(got < prev) {
					t.Fatalf("%s: score not strictly decreasing at %v: %v then %v", spec.ID, v, prev, got)
				}
				if !(got > floor) {
					t.Fatalf("%s: score %v at %v reached floor %v", spec.ID, got, v, floor)
				}
				if prev-got > 5 {
					t.Fatalf("%s: jump of %v at %v", spec.ID, prev-got, v)
				}
				prev = got
			}
		}
		if minV, ok := spec.Ideal.Min.Get(); ok {
			sweep(minV, -1)
		}
		if maxV, ok := spec.Ideal.Max.Get(); ok {
			sweep(maxV, 1)
		}
	}
}

func TestValidateProfile_RejectsFlatCurve(t *testing.T) {
	p := &Profile{
		Name: "flat",
		Attributes: []AttributeSpec{
			{ID: "hta", Weight: 2, Ideal: Exact(68), Acceptable: Between(66, 70), EdgeScore: model.Some(100.0), Falloff: FalloffBoth(1, 1)},
		},
	}
	if _, err := NormalizeAndValidate(p); err == nil {
		t.Fatal("expected an edge_score of 100 on a non-empty acceptable band to be rejected as a flat curve")
	}
}

func parseFormattedCell(t *testing.T, cell string, unit string) (shownVal float64, marker string, hasDelta bool, shownDelta float64, word string) {
	t.Helper()
	fields := strings.Fields(cell)
	if len(fields) < 2 {
		t.Fatalf("unexpected cell format %q", cell)
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		t.Fatalf("parse cell value %q from %q: %v", fields[0], cell, err)
	}
	marker = fields[1]
	if len(fields) == 2 {
		return v, marker, false, 0, ""
	}
	if len(fields) != 4 {
		t.Fatalf("unexpected cell field count in %q", cell)
	}
	dStr := strings.TrimSuffix(fields[2], unit)
	d, err := strconv.ParseFloat(dStr, 64)
	if err != nil {
		t.Fatalf("parse cell delta %q from %q: %v", fields[2], cell, err)
	}
	return v, marker, true, d, fields[3]
}

func testSpecsForSweep() []AttributeSpec {
	var specs []AttributeSpec
	for _, spec := range IdmatchProfile().Attributes {
		if spec.Ideal == nil || spec.ID == "handlebar_clamp_fit" {
			continue
		}
		specs = append(specs, spec)
	}
	// Include Berthoud-translated non-integer bounds (679.9-689.9 / 669.9-699.9)
	// and an asymmetric non-integer BRP setback target.
	specs = append(specs,
		DisplayOnlyWithTarget("saddle_hb", Between(679.9, 689.9), Between(669.9, 699.9), model.Some(75.0), FalloffBoth(15, 15)),
		AttributeSpec{
			ID:         "brp_setback",
			Weight:     10,
			Ideal:      Exact(228.4),
			Acceptable: Between(223.4, 233.4),
			EdgeScore:  model.Some(75.0),
			Falloff:    FalloffBoth(20, 20),
			Floor:      model.Some(-200.0),
		},
	)
	return specs
}

func sweepGridForSpec(spec AttributeSpec, meta AttributeMeta) []float64 {
	lo := 0.0
	hi := 0.0
	if spec.Acceptable != nil && spec.Acceptable.Min.IsSet() {
		lo = spec.Acceptable.Min.OrZero() - 20
	} else if spec.Ideal.Min.IsSet() {
		lo = spec.Ideal.Min.OrZero() - 20
	}
	if spec.Acceptable != nil && spec.Acceptable.Max.IsSet() {
		hi = spec.Acceptable.Max.OrZero() + 20
	} else if spec.Ideal.Max.IsSet() {
		hi = spec.Ideal.Max.OrZero() + 20
	}
	step := 0.1
	if meta.Decimals > 0 {
		step = 0.05
		lo += 15 // keep range tight for 0.05 step
		hi -= 15
	}
	n := int(math.Round((hi - lo) / step))
	vals := make([]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		vals = append(vals, math.Round((lo+float64(i)*step)*1000)/1000)
	}
	return vals
}

func TestCellDeltaMatchesDisplayedValue(t *testing.T) {
	for _, spec := range testSpecsForSweep() {
		meta, ok := LookupAttribute(spec.ID)
		if !ok {
			t.Fatalf("unknown attribute %q", spec.ID)
		}
		for _, v := range sweepGridForSpec(spec, meta) {
			ev := Evaluate(spec, meta, v, nil)
			shownVal, marker, hasDelta, shownDelta, word := parseFormattedCell(t, ev.FormattedCell, meta.Unit)

			expectedDist := PrintedDistanceToIdeal(spec.Ideal, shownVal, meta.Decimals)
			expectedAbs := math.Abs(expectedDist)

			if expectedAbs == 0 {
				if hasDelta || marker != SymbolIdeal {
					t.Errorf("%s(%.3f) -> %q: expected ideal with no delta, got hasDelta=%v marker=%q",
						spec.ID, v, ev.FormattedCell, hasDelta, marker)
				}
				continue
			}
			if !hasDelta {
				t.Errorf("%s(%.3f) -> %q: expected delta %.2f, got no delta", spec.ID, v, ev.FormattedCell, expectedAbs)
				continue
			}
			if math.Abs(shownDelta-expectedAbs) > 1e-9 {
				t.Errorf("%s(%.3f) -> %q: |shown %.2f - bound| = %.2f, cell delta = %.2f",
					spec.ID, v, ev.FormattedCell, shownVal, expectedAbs, shownDelta)
			}
			wantWord := meta.AboveWord
			if expectedDist < 0 {
				wantWord = meta.BelowWord
			}
			if word != wantWord {
				t.Errorf("%s(%.3f) -> %q: word = %q, want %q", spec.ID, v, ev.FormattedCell, word, wantWord)
			}
		}
	}

	// Verify signed numeric delta path (when BelowWord/AboveWord are empty) also uses printed tie-to-even.
	noWordSpec := AttributeSpec{
		ID:         "reach",
		Weight:     1,
		Ideal:      Exact(450),
		Acceptable: Between(440, 460),
		EdgeScore:  model.Some(75.0),
		Falloff:    FalloffBoth(10, 10),
		Floor:      model.Some(-100.0),
	}
	noWordMeta := AttributeMeta{ID: "reach", Label: "Reach", Unit: "mm", Decimals: 0}
	if ev := Evaluate(noWordSpec, noWordMeta, 434.5, nil); ev.FormattedCell != "434 ✗ −16" {
		t.Errorf("434.5 tie = %q, want %q", ev.FormattedCell, "434 ✗ −16")
	}
	if ev := Evaluate(noWordSpec, noWordMeta, 435.5, nil); ev.FormattedCell != "436 ✗ −14" {
		t.Errorf("435.5 tie = %q, want %q", ev.FormattedCell, "436 ✗ −14")
	}
}

func TestZoneMatchesDisplayedText(t *testing.T) {
	for _, spec := range testSpecsForSweep() {
		meta, ok := LookupAttribute(spec.ID)
		if !ok {
			t.Fatalf("unknown attribute %q", spec.ID)
		}
		seen := map[float64]Evaluation{}
		for _, v := range sweepGridForSpec(spec, meta) {
			ev := Evaluate(spec, meta, v, nil)
			shownVal, _, _, _, _ := parseFormattedCell(t, ev.FormattedCell, meta.Unit)

			if prev, exists := seen[shownVal]; exists {
				if prev.Zone != ev.Zone || prev.FormattedCell != ev.FormattedCell {
					t.Errorf("%s: same shown value %v produced different outputs: %+v vs %+v",
						spec.ID, shownVal, prev, ev)
				}
			} else {
				seen[shownVal] = ev
			}

			// Verify Zone against printed Ideal and Acceptable bounds.
			wantZone := ZoneIdeal
			idealMin, hasIdealMin := spec.Ideal.Min.Get()
			idealMax, hasIdealMax := spec.Ideal.Max.Get()
			if hasIdealMin && shownVal < RoundPrinted(idealMin, meta.Decimals) ||
				hasIdealMax && shownVal > RoundPrinted(idealMax, meta.Decimals) {
				wantZone = ZoneOutside
				if spec.Acceptable != nil {
					accMin, hasAccMin := spec.Acceptable.Min.Get()
					accMax, hasAccMax := spec.Acceptable.Max.Get()
					inMin := !hasAccMin || shownVal >= RoundPrinted(accMin, meta.Decimals)
					inMax := !hasAccMax || shownVal <= RoundPrinted(accMax, meta.Decimals)
					if inMin && inMax {
						wantZone = ZoneAcceptable
					}
				}
			}
			if ev.Zone != wantZone {
				t.Errorf("%s(%.3f) shown=%v (%q): Zone=%q, want %q",
					spec.ID, v, shownVal, ev.FormattedCell, ev.Zone, wantZone)
			}
		}
	}

	// Specific Pilot regressions:
	p := IdmatchProfile()
	attrMap := map[string]AttributeSpec{}
	for _, a := range p.Attributes {
		attrMap[a.ID] = a
	}
	brpMeta, _ := LookupAttribute("brp_setback")
	if ev := Evaluate(attrMap["brp_setback"], brpMeta, 183.5, nil); ev.FormattedCell != "184 ✗ 35 fwd" {
		t.Errorf("183.5 -> %q, want %q", ev.FormattedCell, "184 ✗ 35 fwd")
	}
	if ev := Evaluate(attrMap["brp_setback"], brpMeta, 188.5, nil); ev.FormattedCell != "188 ✗ 31 fwd" {
		t.Errorf("188.5 -> %q, want %q", ev.FormattedCell, "188 ✗ 31 fwd")
	}
	if ev := Evaluate(attrMap["brp_setback"], brpMeta, 213.6, nil); ev.FormattedCell != "214 ~ 5 fwd" || ev.Zone != ZoneAcceptable {
		t.Errorf("213.6 -> %q (%s), want %q (acceptable)", ev.FormattedCell, ev.Zone, "214 ~ 5 fwd")
	}
	trailMeta, _ := LookupAttribute("trail")
	if ev := Evaluate(attrMap["trail"], trailMeta, 106.5, nil); ev.FormattedCell != "106 ✓" || ev.Zone != ZoneIdeal {
		t.Errorf("106.5 -> %q (%s), want %q (ideal)", ev.FormattedCell, ev.Zone, "106 ✓")
	}
}

func TestMarkerAgreesWithZone(t *testing.T) {
	markerToZone := map[string]ZoneType{
		SymbolIdeal:      ZoneIdeal,
		SymbolAcceptable: ZoneAcceptable,
		SymbolOutside:    ZoneOutside,
	}

	for _, spec := range testSpecsForSweep() {
		meta, _ := LookupAttribute(spec.ID)
		for _, v := range sweepGridForSpec(spec, meta) {
			ev := Evaluate(spec, meta, v, nil)
			_, marker, _, _, _ := parseFormattedCell(t, ev.FormattedCell, meta.Unit)
			if wantZone, ok := markerToZone[marker]; !ok || ev.Zone != wantZone {
				t.Fatalf("%s(%.3f): cell=%q marker=%q Zone=%q (want %q)",
					spec.ID, v, ev.FormattedCell, marker, ev.Zone, wantZone)
			}
		}
	}

	// Owner decision F-p4c-3: 106 ✓ with sub-score 97-99 is intended; Zone must match ✓.
	var trailSpec AttributeSpec
	for _, a := range IdmatchProfile().Attributes {
		if a.ID == "trail" {
			trailSpec = a
			break
		}
	}
	trailMeta, _ := LookupAttribute("trail")
	ev := Evaluate(trailSpec, trailMeta, 106.2, nil)
	if ev.FormattedCell != "106 ✓" || ev.Zone != ZoneIdeal {
		t.Errorf("trail(106.2) = %q (%s), want %q (ideal)", ev.FormattedCell, ev.Zone, "106 ✓")
	}
	if !(ev.SubScore > 97 && ev.SubScore < 99) {
		t.Errorf("trail(106.2) SubScore = %.2f, want in (97, 99)", ev.SubScore)
	}
}

func TestEvaluate_HandlebarClampFitZones(t *testing.T) {
	var spec AttributeSpec
	for _, a := range IdmatchProfile().Attributes {
		if a.ID == "handlebar_clamp_fit" {
			spec = a
			break
		}
	}
	meta, _ := LookupAttribute("handlebar_clamp_fit")

	cases := []struct {
		subScore float64
		count    int
		wantZone ZoneType
		wantCell string
	}{
		{100.0, 3, ZoneIdeal, "3 ✓"},
		{96.0, 27, ZoneAcceptable, "27 ~"},
		{91.0, 27, ZoneAcceptable, "27 ~"},
		{75.0, 1, ZoneAcceptable, "1 ~"},
		{74.2, 3, ZoneOutside, "3 ✗"},
		{65.0, 1, ZoneOutside, "1 ✗"},
		{48.0, 0, ZoneOutside, "0 ✗"},
	}

	for _, tc := range cases {
		row := model.CanonicalComparisonRow{
			Canonical: model.BikeFitAnalysis{
				CockpitFitResult: &model.CockpitFitResult{
					HasFit:        tc.count > 0,
					CountInside:   tc.count,
					FinalSubScore: tc.subScore,
				},
			},
		}
		ev := Evaluate(spec, meta, tc.subScore, &row)
		if ev.Zone != tc.wantZone || ev.FormattedCell != tc.wantCell {
			t.Errorf("subScore=%.1f count=%d: got %q (%s), want %q (%s)",
				tc.subScore, tc.count, ev.FormattedCell, ev.Zone, tc.wantCell, tc.wantZone)
		}
	}
}
