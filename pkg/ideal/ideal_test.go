package ideal

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"bikegeo/pkg/model"
)

func TestLookupAttribute_AllRegistered(t *testing.T) {
	all := AllAttributes()
	if len(all) == 0 {
		t.Fatal("AllAttributes() returned empty slice")
	}
	for _, meta := range all {
		got, ok := LookupAttribute(meta.ID)
		if !ok {
			t.Errorf("LookupAttribute(%q) failed", meta.ID)
			continue
		}
		if got.ID != meta.ID || got.Label != meta.Label || got.Unit != meta.Unit {
			t.Errorf("LookupAttribute(%q) = %+v, want %+v", meta.ID, got, meta)
		}
	}
	if _, ok := LookupAttribute(MetricID("nonexistent_metric")); ok {
		t.Error("LookupAttribute(nonexistent_metric) = true, want false")
	}
}

func TestLookupAttribute_Trail(t *testing.T) {
	meta, ok := LookupAttribute(MetricTrail)
	if !ok {
		t.Fatalf("LookupAttribute(%q) failed", MetricTrail)
	}
	if meta.Label != "Trail" || meta.Unit != "mm" {
		t.Errorf("Unexpected meta: %+v", meta)
	}
}

func TestLookupAttribute_SeatpostExtension(t *testing.T) {
	meta, ok := LookupAttribute(MetricSeatpostExtension)
	if !ok {
		t.Fatalf("LookupAttribute(%q) failed", MetricSeatpostExtension)
	}
	if meta.Label != "Seatpost Ext" || meta.Unit != "mm" {
		t.Errorf("Unexpected meta: %+v", meta)
	}
}

func TestLookupAttribute_SaddleHb(t *testing.T) {
	meta, ok := LookupAttribute(MetricSaddleHb)
	if !ok {
		t.Fatalf("LookupAttribute(%q) failed", MetricSaddleHb)
	}
	if meta.Label != "BRP–HandGrip" || meta.Unit != "mm" {
		t.Errorf("Unexpected meta: %+v", meta)
	}
}

func TestLookupAttribute_BrpSetbackAndDrop(t *testing.T) {
	meta, ok := LookupAttribute(MetricBrpSetback)
	if !ok || meta.Label != "BRP Setback" || meta.Unit != "mm" {
		t.Errorf("LookupAttribute(%q) = %+v, %v", MetricBrpSetback, meta, ok)
	}
	meta, ok = LookupAttribute(MetricSaddleHbDrop)
	if !ok || meta.Label != "HandGrip Rise" || meta.Unit != "mm" {
		t.Errorf("LookupAttribute(%q) = %+v, %v", MetricSaddleHbDrop, meta, ok)
	}
	meta, ok = LookupAttribute(MetricBestStem)
	if !ok || meta.Label != "Best Stem" {
		t.Errorf("LookupAttribute(%q) = %+v, %v", MetricBestStem, meta, ok)
	}
	meta, ok = LookupAttribute(MetricHandlebarClampFit)
	if !ok || meta.Label != "Oval Fits" {
		t.Errorf("LookupAttribute(%q) = %+v, %v", MetricHandlebarClampFit, meta, ok)
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
	if p.TargetRef != model.TargetHandGrip {
		t.Errorf("Expected idmatch TargetRef HandGrip (%q), got %q", model.TargetHandGrip, p.TargetRef)
	}
	if err := ValidateProfile(p); err != nil {
		t.Fatalf("ValidateProfile failed on idmatch: %v", err)
	}
}

func TestEvaluate_OneSidedSeatToRc(t *testing.T) {
	// Seat-RC: free >= 235, acceptable >= 220 (display only), 0.5 points per mm below.
	spec := AttributeSpec{
		ID:         "seat_to_rc",
		Cost:       CostBoth(0.5, 0),
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

	for _, tc := range []struct {
		val      float64
		wantZone ZoneType
		wantLost float64
	}{
		{235, ZoneIdeal, 0},
		{250, ZoneIdeal, 0},
		{1e6, ZoneIdeal, 0}, // unbounded above: never priced
		{220, ZoneAcceptable, 7.5},
		{200, ZoneOutside, 17.5},
	} {
		if res := Evaluate(spec, meta, tc.val, nil); res.Zone != tc.wantZone || math.Abs(res.PointsLost-tc.wantLost) > 1e-9 {
			t.Errorf("Seat-RC %.0f expected %v / -%.1f, got %v / -%.4f", tc.val, tc.wantZone, tc.wantLost, res.Zone, res.PointsLost)
		}
	}
	// Far below: keeps losing points linearly, with no floor.
	assertLinearLoss(t, spec, meta, 235, -1, []float64{199, 180, 150, 100, 0, -1000, -1e6})
}

func TestEvaluate_DesignDocTable(t *testing.T) {
	// AETT free 630–642, acceptable 606–642 (display only), 2 points/mm below, 3 above.
	spec := AttributeSpec{
		ID:         "aett",
		Cost:       CostBoth(2, 3),
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
		expectedLost float64
		expectedCell string
	}{
		{596, ZoneOutside, 68, "596 ✗ 34 sht"},
		{601, ZoneOutside, 58, "601 ✗ 29 sht"},
		{606, ZoneAcceptable, 48, "606 ~ 24 sht"},
		{618, ZoneAcceptable, 24, "618 ~ 12 sht"},
		{630, ZoneIdeal, 0, "630 ✓"},
		{635, ZoneIdeal, 0, "635 ✓"},
		{642, ZoneIdeal, 0, "642 ✓"},
		{645, ZoneOutside, 9, "645 ✗ 3 lng"},
		{647, ZoneOutside, 15, "647 ✗ 5 lng"},
	}

	for _, tc := range testCases {
		res := Evaluate(spec, meta, tc.val, nil)
		if res.Zone != tc.expectedZone {
			t.Errorf("val %.0f: expected zone %v, got %v", tc.val, tc.expectedZone, res.Zone)
		}
		if math.Abs(res.PointsLost-tc.expectedLost) > 1e-9 {
			t.Errorf("val %.0f: expected points lost %.1f, got %.4f", tc.val, tc.expectedLost, res.PointsLost)
		}
		if res.FormattedCell != tc.expectedCell {
			t.Errorf("val %.0f: expected cell %q, got %q", tc.val, tc.expectedCell, res.FormattedCell)
		}
	}
}

func TestEvaluate_OneSidedChainstay(t *testing.T) {
	// Chainstay: free >= 470, acceptable >= 445 (display only), 0.4 points per mm below.
	spec := AttributeSpec{
		ID:         "chainstay",
		Cost:       CostBoth(0.4, 0),
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

	for _, tc := range []struct {
		val      float64
		wantZone ZoneType
		wantLost float64
	}{
		{470, ZoneIdeal, 0},
		{480, ZoneIdeal, 0},
		{445, ZoneAcceptable, 10},
		{420, ZoneOutside, 20},
	} {
		if res := Evaluate(spec, meta, tc.val, nil); res.Zone != tc.wantZone || math.Abs(res.PointsLost-tc.wantLost) > 1e-9 {
			t.Errorf("Chainstay %.0f expected %v / -%.1f, got %v / -%.4f", tc.val, tc.wantZone, tc.wantLost, res.Zone, res.PointsLost)
		}
	}
	// Far below: keeps losing points linearly, with no floor.
	assertLinearLoss(t, spec, meta, 470, -1, []float64{419, 400, 350, 200, 0, -1000, -1e6})
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
			{ID: "nonexistent_attr", Cost: CostEach(1), Ideal: AtLeast(10)},
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
			{ID: "hta", Cost: CostEach(1), Ideal: Between(69, 67), Falloff: FalloffBoth(1, 1)},
		},
	}
	if _, err := NormalizeAndValidate(pInverted); err == nil || !strings.Contains(err.Error(), "exceeds max") {
		t.Errorf("Expected inverted min/max error, got: %v", err)
	}

	// 4. Acceptable wider than ideal but missing edge_score
	pMissingEdge := &Profile{
		Name: "test",
		Attributes: []AttributeSpec{
			{ID: "hta", Cost: CostEach(1), Ideal: Exact(68), Acceptable: Between(67, 69), Falloff: FalloffBoth(1, 1)},
		},
	}
	if _, err := NormalizeAndValidate(pMissingEdge); err == nil || !strings.Contains(err.Error(), "edge_score is not defined") {
		t.Errorf("Expected missing edge_score error, got: %v", err)
	}

	// 5. A scored band needs only a free range and a cost: no falloff is required.
	pNoFalloff := &Profile{
		Name: "test",
		Attributes: []AttributeSpec{
			{ID: "hta", Cost: CostEach(1), Ideal: Exact(68)},
		},
	}
	if _, err := NormalizeAndValidate(pNoFalloff); err != nil {
		t.Errorf("Expected a band without falloff to validate, got: %v", err)
	}

	// 5b. Hardware bands: limit_points only on attributes with hardware limits, never with cost.
	for _, tc := range []struct {
		attr AttributeSpec
		want string
	}{
		{AttributeSpec{ID: "hta", LimitPoints: 8, Ideal: Exact(68)}, "has no hardware limit"},
		{AttributeSpec{ID: "spacers", LimitPoints: 8, Cost: CostEach(1), Ideal: Between(0, 20)}, "not both"},
		{AttributeSpec{ID: "spacers", LimitPoints: -1, Ideal: Between(0, 20)}, "limit_points must be non-negative"},
	} {
		p := &Profile{Name: "test", Attributes: []AttributeSpec{tc.attr}}
		if _, err := NormalizeAndValidate(p); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: expected error containing %q, got: %v", tc.attr, tc.want, err)
		}
	}

	// 6. Non-negative floor
	pBadFloor := &Profile{
		Name: "test",
		Attributes: []AttributeSpec{
			{ID: "hta", Cost: CostEach(1), Ideal: Exact(68), Falloff: FalloffBoth(1, 1), Floor: model.Some(10.0)},
		},
	}
	if _, err := NormalizeAndValidate(pBadFloor); err == nil || !strings.Contains(err.Error(), "floor (10.0) must be negative") {
		t.Errorf("Expected negative floor error, got: %v", err)
	}

	// 7. Invalid non-empty TargetRef
	pBadTargetRef := &Profile{
		Name:      "test-bad-ref",
		TargetRef: "hb",
		Attributes: []AttributeSpec{
			{ID: "hta", Cost: CostEach(1), Ideal: Exact(68), Falloff: FalloffBoth(1, 1)},
		},
	}
	if _, err := NormalizeAndValidate(pBadTargetRef); err == nil || !strings.Contains(err.Error(), "target_ref") {
		t.Errorf("Expected invalid target_ref error, got: %v", err)
	}

	// 8. Missing TargetRef on cockpit-targeted profile
	pMissingTargetRef := &Profile{
		Name: "test-missing-ref",
		Attributes: []AttributeSpec{
			{ID: "saddle_hb", Cost: CostEach(1), Ideal: Exact(700), Falloff: FalloffBoth(20, 20)},
		},
	}
	if _, err := NormalizeAndValidate(pMissingTargetRef); err == nil || !strings.Contains(err.Error(), "target_ref") {
		t.Errorf("Expected missing cockpit target_ref error, got: %v", err)
	}

	// 9. TargetReferenceBarClamp without valid ReferenceBar
	pMissingRefBar := &Profile{
		Name:      "test-missing-ref-bar",
		TargetRef: model.TargetReferenceBarClamp,
		Attributes: []AttributeSpec{
			{ID: "saddle_hb", Cost: CostEach(1), Ideal: Exact(630), Falloff: FalloffBoth(20, 20)},
		},
	}
	if _, err := NormalizeAndValidate(pMissingRefBar); err == nil || !strings.Contains(err.Error(), "reference_bar") {
		t.Errorf("Expected missing reference_bar error, got: %v", err)
	}

	// 10. Valid TargetReferenceBarClamp with positive-width ReferenceBar
	pValidRefBar := &Profile{
		Name:         "test-valid-ref-bar",
		TargetRef:    model.TargetReferenceBarClamp,
		ReferenceBar: model.Some(model.HandlebarSpec{ID: "flat", StockWidthMm: 760, CurrentWidthMm: 760}),
		Attributes: []AttributeSpec{
			{ID: "saddle_hb", Cost: CostEach(1), Ideal: Exact(630), Falloff: FalloffBoth(20, 20)},
		},
	}
	if _, err := NormalizeAndValidate(pValidRefBar); err != nil {
		t.Errorf("Expected valid TargetReferenceBarClamp profile to pass, got: %v", err)
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
	attrMap := make(map[MetricID]AttributeSpec)
	for _, a := range p.Attributes {
		attrMap[a.ID] = a
	}

	cases := []struct {
		id       MetricID
		val      float64
		expected string
	}{
		{"brp_setback", 204, "204 ✗ 15 fwd"},
		{"brp_setback", 221, "221 ~ 2 aft"},
		{"seatpost_extension", 221, "221 ~ 29 sht"},
		{"seatpost_extension", 335, "335 ~ 25 lng"},
		{"seat_to_rc", 221, "221"},  // DisplayOnly attribute
		{"sparevert", 60, "60"},     // DisplayOnly attribute
		{"sparevert", 60, "60"},     // DisplayOnly attribute
		{"bbdrop", 64, "64 ~ 1 hi"}, // free range is 65–75
		{"bbdrop", 66, "66 ✓"},
		{"bbdrop", 77, "77 ~ 2 lo"},
		{"hta", 67.7, "67.7"}, // DisplayOnly attribute
		{"rc_wb_ratio", 38.9, "38.9 ✓"},
		{"rc_wb_ratio", 36.5, "36.5 ✗ 0.5% lo"},
		{"rc_wb_ratio", 39.6, "39.6 ✗ 0.6% hi"},
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

// assertLinearLoss checks that each successive value (ordered from least to most
// extreme, all on the dir side of bound) loses strictly more points, exactly
// Cost × distance past bound, with no floor.
func assertLinearLoss(t *testing.T, spec AttributeSpec, meta AttributeMeta, bound, dir float64, vals []float64) {
	t.Helper()
	perUnit := spec.Cost.Above
	if dir < 0 {
		perUnit = spec.Cost.Below
	}
	prev := 0.0
	for _, v := range vals {
		got := Evaluate(spec, meta, v, nil).PointsLost
		if !(got > prev) {
			t.Errorf("%s at %v: points lost %.4f is not above the previous %.4f", spec.ID, v, got, prev)
		}
		want := perUnit * math.Abs(v-bound)
		if math.Abs(got-want) > 1e-9*math.Max(1, want) {
			t.Errorf("%s at %v: points lost %.6f, want linear %.6f", spec.ID, v, got, want)
		}
		prev = got
	}
}

// TestIdmatchProfile_BandsAreFreeInsideAndLinearOutside sweeps every attribute with a
// free range far beyond it on both sides: scored bands lose nothing inside and exactly
// Cost per unit outside (no flats, jumps, tails or floors); display-only ones never lose.
func TestIdmatchProfile_BandsAreFreeInsideAndLinearOutside(t *testing.T) {
	for _, spec := range IdmatchProfile().Attributes {
		if spec.Ideal == nil || spec.ID == MetricHandlebarClampFit {
			continue // Oval Fits is priced on R; see TestOvalPointsLost.
		}
		if spec.LimitPoints > 0 {
			continue // Hardware bands need a row's limits; see TestLimitPointsLost_LinearToHardwareLimit.
		}
		meta, ok := LookupAttribute(spec.ID)
		if !ok {
			t.Fatalf("unknown attribute %q", spec.ID)
		}
		minV, hasMin := spec.Ideal.Min.Get()
		maxV, hasMax := spec.Ideal.Max.Get()
		if hasMin && hasMax {
			for _, v := range []float64{minV, (minV + maxV) / 2, maxV} {
				if got := Evaluate(spec, meta, v, nil).PointsLost; got != 0 {
					t.Errorf("%s: %v inside the free range lost %v", spec.ID, v, got)
				}
			}
		}
		if !spec.IsScored() {
			for _, v := range []float64{minV - 100, maxV + 100} {
				if got := Evaluate(spec, meta, v, nil).PointsLost; got != 0 {
					t.Errorf("%s: display-only attribute lost %v at %v", spec.ID, got, v)
				}
			}
			continue
		}
		var vals []float64
		for i := 1; i <= 2000; i++ {
			vals = append(vals, float64(i)*0.5)
		}
		if hasMin {
			below := make([]float64, len(vals))
			for i, d := range vals {
				below[i] = minV - d
			}
			assertLinearLoss(t, spec, meta, minV, -1, below)
		}
		if hasMax {
			above := make([]float64, len(vals))
			for i, d := range vals {
				above[i] = maxV + d
			}
			assertLinearLoss(t, spec, meta, maxV, 1, above)
		}
	}
}

func TestValidateProfile_RejectsFlatCurve(t *testing.T) {
	p := &Profile{
		Name: "flat",
		Attributes: []AttributeSpec{
			{ID: "hta", Cost: CostEach(1), Ideal: Exact(68), Acceptable: Between(66, 70), EdgeScore: model.Some(100.0), Falloff: FalloffBoth(1, 1)},
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
			Cost:       CostEach(1.2),
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
		Cost:       CostEach(1),
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
	attrMap := map[MetricID]AttributeSpec{}
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

	// Owner decision F-p4c-3: 106 ✓ at the printed bound while 106.2 still pays a
	// sliver past the unrounded free range is intended; Zone must match ✓.
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
	if want := 0.41 * 0.2; math.Abs(ev.PointsLost-want) > 1e-9 {
		t.Errorf("trail(106.2) PointsLost = %.4f, want %.4f (0.2mm past 106 at 0.41/mm)", ev.PointsLost, want)
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

func TestOvalPointsLost(t *testing.T) {
	var spec AttributeSpec
	for _, a := range IdmatchProfile().Attributes {
		if a.ID == MetricHandlebarClampFit {
			spec = a
			break
		}
	}
	meta, _ := LookupAttribute(MetricHandlebarClampFit)

	cases := []struct {
		r, stemPenalty, want float64
	}{
		{0.0, 0, 0},
		{0.97, 0, 0}, // inside the oval is free
		{1.0, 0, 0},
		{1.5, 0, 11},
		{2.8, 0, 39.6},
		{11.07, 0, 221.54}, // linear, no tail
		{2.8, 15, 39.6},    // the selector's stem penalty costs no points
	}
	for _, tc := range cases {
		row := model.CanonicalComparisonRow{Canonical: model.BikeFitAnalysis{
			CockpitFitResult: &model.CockpitFitResult{BestR: tc.r, StemPenalty: tc.stemPenalty},
		}}
		ev := Evaluate(spec, meta, 0, &row)
		if math.Abs(ev.PointsLost-tc.want) > 1e-9 {
			t.Errorf("R=%.2f stem=%.0f: PointsLost = %.4f, want %.4f", tc.r, tc.stemPenalty, ev.PointsLost, tc.want)
		}
		if ev.DistanceToIdeal != tc.r {
			t.Errorf("R=%.2f: DistanceToIdeal = %v, want R", tc.r, ev.DistanceToIdeal)
		}
	}
}

func TestLimitPointsLost_LinearToHardwareLimit(t *testing.T) {
	stem := AttributeSpec{ID: MetricStemLength, LimitPoints: 8, Ideal: Between(70, 80)}
	cases := []struct {
		val, lo, hi, want float64
	}{
		{75, 40, 100, 0},
		{70, 40, 100, 0},
		{80, 40, 100, 0},
		{40, 40, 100, 8},               // mtb catalog limit
		{55, 40, 100, 4},               // halfway to the limit
		{100, 40, 100, 8},              // mtb catalog limit
		{90, 40, 100, 4},               // halfway to the limit
		{60, 60, 110, 8},               // gravel catalog limit
		{110, 60, 110, 8},              // gravel catalog limit
		{30, 40, 100, 8 * 40.0 / 30.0}, // past the limit keeps the same slope
	}
	for _, tc := range cases {
		if got := LimitPointsLost(stem, tc.val, tc.lo, tc.hi); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("stem %.0f in [%.0f, %.0f]: lost %.4f, want %.4f", tc.val, tc.lo, tc.hi, got, tc.want)
		}
	}
	// A side whose hardware limit sits inside the free range is never priced.
	rail := AttributeSpec{ID: MetricRailOffset, LimitPoints: 8, Ideal: Between(-5, 5)}
	if got := LimitPointsLost(rail, 5, -5, 5); got != 0 {
		t.Errorf("rail at the edge of a ±5 window lost %v, want 0", got)
	}
	if got := LimitPointsLost(rail, 15, -15, 15); got != 8 {
		t.Errorf("rail at +15 of a ±15 window lost %v, want 8", got)
	}
	angle := AttributeSpec{ID: MetricStemAngle, LimitPoints: 1, Ideal: Exact(0)}
	for _, a := range []float64{-6, 6} {
		if got := LimitPointsLost(angle, a, -6, 6); got != 1 {
			t.Errorf("stem angle %v° lost %v, want 1", a, got)
		}
	}
}

func TestHardwareLimits_FromRow(t *testing.T) {
	row := &model.CanonicalComparisonRow{
		Bike: model.BikeGeometry{
			MaxSpacerMm: 40,
			Stems: []model.StemSpec{
				{LengthMm: 60, AngleDeg: -6}, {LengthMm: 110, AngleDeg: 6}, {LengthMm: 80, AngleDeg: 0},
			},
		},
		Canonical: model.BikeFitAnalysis{
			PossibleBRPs: []model.BRPPoint{{RailOffsetMm: -15}, {RailOffsetMm: 0}, {RailOffsetMm: 15}},
		},
	}
	for _, tc := range []struct {
		id     MetricID
		lo, hi float64
	}{
		{MetricRailOffset, -15, 15},
		{MetricStemLength, 60, 110},
		{MetricStemAngle, -6, 6},
		{MetricSpacers, 0, 40},
	} {
		lo, hi, ok := HardwareLimits(tc.id, row)
		if !ok || lo != tc.lo || hi != tc.hi {
			t.Errorf("%s: limits (%v, %v, %v), want (%v, %v)", tc.id, lo, hi, ok, tc.lo, tc.hi)
		}
	}
	if _, _, ok := HardwareLimits(MetricStemLength, nil); ok {
		t.Error("nil row must have no hardware limits")
	}
}
