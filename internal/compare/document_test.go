package compare

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"

	"bikegeo/internal/catalog"
	"bikegeo/internal/frames"
	"bikegeo/internal/ideal"
	"bikegeo/internal/model"
	"bikegeo/internal/report"
	"bikegeo/internal/scoring"
	"bikegeo/internal/validator"
)

func testBaseline() model.SavedComparisonState {
	return DefaultComparisonState(catalog.DefaultSaddle(), catalog.DefaultHandlebar(), catalog.DefaultRiderProfile())
}

func withDefaults(opts Options) Options {
	if opts.Frames == nil {
		opts.Frames = frames.AllFrames()
	}
	if len(opts.Baseline.Columns) == 0 {
		opts.Baseline = testBaseline()
	}
	if opts.Profile == nil {
		opts.Profile = ideal.IdmatchProfile()
	}
	if len(opts.Catalog.Forks) == 0 && len(opts.Catalog.Stems) == 0 {
		opts.Catalog = validator.Catalog{Forks: catalog.CommonForkCatalog, Stems: catalog.CommonStemCatalog}
	}
	return opts
}

func mustRun(t *testing.T, opts Options) *Result {
	t.Helper()
	res, err := Run(withDefaults(opts))
	if err != nil {
		t.Fatalf("Run(%+v): %v", opts, err)
	}
	return res
}

func TestRunDefaultBaseline(t *testing.T) {
	res := mustRun(t, Options{})
	if got, want := len(res.Rows), len(testBaseline().Columns); got != want {
		t.Errorf("default rows = %d, want %d", got, want)
	}
	if !res.State.Transpose {
		t.Error("default state with >4 columns should be transposed")
	}
}

func TestRunErrors(t *testing.T) {
	for _, opts := range []Options{
		withDefaults(Options{Specs: []string{"nosuchframe"}}),
		withDefaults(Options{Specs: []string{"fenrir@17%"}}),
		withDefaults(Options{Specs: []string{"-all"}}),
		{Frames: nil, Baseline: testBaseline(), Profile: ideal.IdmatchProfile()},
		{Frames: frames.AllFrames(), Baseline: testBaseline(), Profile: nil},
	} {
		if _, err := Run(opts); err == nil {
			t.Errorf("Run(%+v) succeeded, want error", opts)
		}
	}
}

func TestBuildDocumentSplitLayout(t *testing.T) {
	res := mustRun(t, Options{})
	doc, err := BuildDocument(res, DocOptions{Scoring: true, Detail: true, DocFooters: true})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, tb := range doc.Tables() {
		ids = append(ids, tb.ID)
	}
	if got := strings.Join(ids, ","); got != "fit-cockpit,chassis,breakdown" {
		t.Fatalf("table IDs = %s", got)
	}
	if len(doc.Footers) == 0 {
		t.Error("expected document-level footers")
	}
	for _, tb := range doc.Tables() {
		if len(tb.Footers) != 0 {
			t.Errorf("table %s has footers despite DocFooters", tb.ID)
		}
	}

	fit := doc.Tables()[0]
	if fit.Rows[0].Kind != report.KindIdeal || fit.Rows[1].Kind != report.KindData {
		t.Error("only the ideal reference row precedes the data rows (no acceptable row)")
	}
	zones := map[string]int{}
	for _, r := range fit.Rows[1:] {
		if r.Attrs["frame-id"] == "" || r.Attrs["rank"] == "" {
			t.Errorf("data row missing attrs: %+v", r.Attrs)
		}
		for _, c := range r.Cells {
			if c.MetricID != "" {
				zones[c.Zone]++
			}
		}
	}
	if zones[report.ZoneIdeal]+zones[report.ZoneOutside] == 0 {
		t.Errorf("no zoned metric cells found: %v", zones)
	}
	if got := len(fit.Rows) - 1; got != len(res.Rows) {
		t.Errorf("data rows = %d, want %d", got, len(res.Rows))
	}
}

func TestBuildDocumentFooterPlacement(t *testing.T) {
	res := mustRun(t, Options{Specs: []string{"fenrir:L@110mm", "hayduke:S3@110mm"}})

	withDetail, _ := BuildDocument(res, DocOptions{Scoring: true, Detail: true})
	last := withDetail.Tables()[len(withDetail.Tables())-1]
	if last.ID != TableBreakdown || len(last.Footers) == 0 {
		t.Errorf("footers should follow the breakdown table, got %s with %d", last.ID, len(last.Footers))
	}

	flat, _ := BuildDocument(mustRun(t, Options{}), DocOptions{Scoring: true, Flat: true})
	if len(flat.Tables()) != 1 || flat.Tables()[0].ID != TableMatrix {
		t.Errorf("flat layout should be one matrix table, got %d", len(flat.Tables()))
	}
}

func TestBuildDocumentColumnLayoutHasColumnAttrs(t *testing.T) {
	res := mustRun(t, Options{Specs: []string{"fenrir:L@110mm", "hayduke:S3@110mm"}})
	doc, err := BuildDocument(res, DocOptions{Scoring: true})
	if err != nil {
		t.Fatal(err)
	}
	tb := doc.Tables()[0]
	if tb.ID != TableMatrix {
		t.Fatalf("table ID = %s", tb.ID)
	}
	var withAttrs int
	for _, c := range tb.Columns {
		if c.Attrs["frame-id"] != "" {
			withAttrs++
		}
	}
	if withAttrs != 2 {
		t.Errorf("columns with frame-id = %d, want 2", withAttrs)
	}
}

func TestBuildDocumentIncludesAETTInAllLayouts(t *testing.T) {
	hasCol := func(tb report.Table, id string) bool {
		for _, c := range tb.Columns {
			if c.ID == id {
				return true
			}
		}
		return false
	}
	hasRow := func(tb report.Table, id string) bool {
		for _, r := range tb.Rows {
			if r.ID == id {
				return true
			}
		}
		return false
	}

	// Split transposed layout (web rows)
	splitDoc, err := BuildDocument(mustRun(t, Options{}), DocOptions{Scoring: true})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCol(splitDoc.Tables()[1], "aett") {
		t.Errorf("split chassis table missing aett column")
	}

	// Flat transposed layout (CLI)
	flatDoc, err := BuildDocument(mustRun(t, Options{}), DocOptions{Scoring: true, Flat: true})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCol(flatDoc.Tables()[0], "aett") {
		t.Errorf("flat matrix table missing aett column")
	}

	// Non-transposed column layout
	colDoc, err := BuildDocument(mustRun(t, Options{Specs: []string{"fenrir:L@110mm", "hayduke:S3@110mm"}}), DocOptions{Scoring: true})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRow(colDoc.Tables()[0], "aett") {
		t.Errorf("column matrix table missing aett row")
	}
}

func TestOvalFitsReferenceMatchesMarker(t *testing.T) {
	meta, ok := ideal.LookupAttribute("handlebar_clamp_fit")
	if !ok {
		t.Fatal("missing handlebar_clamp_fit meta")
	}
	var clampSpec ideal.AttributeSpec
	for _, a := range ideal.IdmatchProfile().Attributes {
		if a.ID == "handlebar_clamp_fit" {
			clampSpec = a
			break
		}
	}
	wantIdeal := FormatAttrRange(clampSpec, meta)
	if wantIdeal != "≥1" {
		t.Fatalf("FormatAttrRange(handlebar_clamp_fit) = %q, want ≥1", wantIdeal)
	}

	for _, saddleID := range []string{"", "berthoud-aspin-open", "selle-italia-slr-boost-superflow-l3"} {
		var saddle model.Opt[model.SaddleSpec]
		if saddleID != "" {
			s, ok := catalog.GetSaddle(saddleID)
			if !ok {
				t.Fatalf("unknown test saddle %q", saddleID)
			}
			saddle = model.Some(s)
		}
		res := mustRun(t, Options{All: true, Saddle: saddle})
		doc, err := BuildDocument(res, DocOptions{Scoring: true, Detail: true})
		if err != nil {
			t.Fatal(err)
		}
		tables := doc.Tables()
		fit := tables[0]
		var breakdown report.Table
		for _, tb := range tables {
			if tb.ID == TableBreakdown {
				breakdown = tb
				break
			}
		}
		if breakdown.ID == "" {
			t.Fatal("missing breakdown table")
		}

		// The fit-cockpit Ideal reference row stays; it is the only reference row.
		clampCol := -1
		for i, c := range fit.Columns {
			if c.ID == "handlebar_clamp_fit" {
				clampCol = i
				break
			}
		}
		if clampCol < 0 {
			t.Fatal("missing handlebar_clamp_fit column in fit-cockpit")
		}
		if got := fit.Rows[0].Cells[clampCol].Text; got != wantIdeal {
			t.Errorf("saddle=%q Ideal ref = %q, want %q", saddleID, got, wantIdeal)
		}
		if fit.Rows[1].Kind != report.KindData {
			t.Errorf("saddle=%q: the Ideal row must be the only reference row", saddleID)
		}

		bdClampCol := -1
		for i, c := range breakdown.Columns {
			if c.ID == "handlebar_clamp_fit" {
				bdClampCol = i
				break
			}
		}
		if bdClampCol < 0 {
			t.Fatal("missing handlebar_clamp_fit column in breakdown")
		}

		// Breakdown cells print the Oval Fits points lost (0 or negative) first.
		seenBreakdown := map[string]bool{}
		breakdownText := map[string]string{}
		for _, r := range breakdown.Rows {
			fields := strings.Fields(r.Cells[bdClampCol].Text)
			if len(fields) == 0 {
				t.Fatalf("empty breakdown cell for row %s", r.ID)
			}
			lost, err := strconv.Atoi(fields[0])
			if err != nil || lost > 0 {
				t.Fatalf("breakdown Oval Fits cell %q must start with points lost (<= 0): %v", r.Cells[bdClampCol].Text, err)
			}
			seenBreakdown[r.ID] = true
			breakdownText[r.ID] = r.Cells[bdClampCol].Text
		}

		// The matrix marker is ✓ when the selected build is free (R ≤ 1) and otherwise
		// ✗ with the same points lost the breakdown prints.
		for _, r := range fit.Rows[1:] {
			cell := r.Cells[clampCol]
			if !seenBreakdown[r.ID] {
				t.Fatalf("missing breakdown row for %s", r.ID)
			}
			idx, err := strconv.Atoi(r.ID)
			if err != nil || idx < 1 || idx > len(res.Rows) || res.Rows[idx-1].Canonical.CockpitFitResult == nil {
				t.Fatalf("row %s has no cockpit fit result", r.ID)
			}
			bestR := res.Rows[idx-1].Canonical.CockpitFitResult.BestR
			wantZone := report.ZoneOutside
			if bestR <= 1.0 {
				wantZone = report.ZoneIdeal
				if !strings.HasSuffix(cell.Text, " "+ideal.SymbolIdeal) {
					t.Errorf("saddle=%q row=%s R=%.2f: cell=%q, want free marker %s", saddleID, r.ID, bestR, cell.Text, ideal.SymbolIdeal)
				}
			} else {
				fields := strings.Fields(cell.Text)
				if len(fields) < 3 || fields[1] != ideal.SymbolOutside {
					t.Errorf("saddle=%q row=%s R=%.2f: cell=%q, want %s with points lost", saddleID, r.ID, bestR, cell.Text, ideal.SymbolOutside)
				} else if lost := strings.TrimPrefix(strings.Fields(breakdownText[r.ID])[0], "-"); fields[2] != lost && fields[2] != "<1" {
					t.Errorf("saddle=%q row=%s: matrix cell %q vs breakdown points lost -%s", saddleID, r.ID, cell.Text, lost)
				}
			}
			if cell.Zone != wantZone {
				t.Errorf("saddle=%q row=%s R=%.2f: zone=%q, want %q", saddleID, r.ID, bestR, cell.Zone, wantZone)
			}
		}
	}
}

func TestDocument_AllConfigsZonesAndDeltasConsistent(t *testing.T) {
	markerToZone := map[string]string{
		ideal.SymbolIdeal:   report.ZoneIdeal,
		ideal.SymbolOutside: report.ZoneOutside,
	}

	for _, saddleID := range []string{"", "berthoud-aspin-open"} {
		var saddle model.Opt[model.SaddleSpec]
		if saddleID != "" {
			s, ok := catalog.GetSaddle(saddleID)
			if !ok {
				t.Fatalf("unknown test saddle %q", saddleID)
			}
			saddle = model.Some(s)
		}
		res := mustRun(t, Options{All: true, Saddle: saddle})
		doc, err := BuildDocument(res, DocOptions{Scoring: true, Detail: true})
		if err != nil {
			t.Fatal(err)
		}
		fit := doc.Tables()[0]
		idealRow := fit.Rows[0]

		for colIdx, col := range fit.Columns {
			if col.ID == "handlebar_clamp_fit" {
				continue
			}
			idealText := idealRow.Cells[colIdx].Text
			if idealText == "" {
				continue
			}
			meta, ok := ideal.LookupAttribute(ideal.MetricID(col.ID))
			if !ok {
				continue
			}
			// Parse ideal min/max from reference cell text.
			cleanIdeal := strings.TrimSuffix(idealText, meta.Unit)
			var idealMin, idealMax float64
			if parts := strings.Split(cleanIdeal, "–"); len(parts) == 2 {
				idealMin, _ = strconv.ParseFloat(parts[0], 64)
				idealMax, _ = strconv.ParseFloat(parts[1], 64)
			} else {
				idealMin, _ = strconv.ParseFloat(cleanIdeal, 64)
				idealMax = idealMin
			}

			for _, r := range fit.Rows[1:] {
				c := r.Cells[colIdx]
				if c.Zone == report.ZoneMissing {
					continue
				}
				fields := strings.Fields(c.Text)
				if len(fields) < 2 {
					t.Fatalf("row %s col %s unexpected cell %q", r.ID, col.ID, c.Text)
				}
				if wantZone := markerToZone[fields[1]]; c.Zone != wantZone {
					t.Errorf("saddle=%q row=%s col=%s cell=%q: zone=%q, want %q",
						saddleID, r.ID, col.ID, c.Text, c.Zone, wantZone)
				}
				shownVal, err := strconv.ParseFloat(fields[0], 64)
				if err != nil {
					t.Fatalf("parse val %q: %v", fields[0], err)
				}
				var expectedDelta float64
				if shownVal < idealMin {
					expectedDelta = idealMin - shownVal
				} else if shownVal > idealMax {
					expectedDelta = shownVal - idealMax
				}
				if expectedDelta == 0 {
					if len(fields) != 2 || fields[1] != ideal.SymbolIdeal {
						t.Errorf("saddle=%q row=%s col=%s: shown=%v in [%v,%v] but cell=%q",
							saddleID, r.ID, col.ID, shownVal, idealMin, idealMax, c.Text)
					}
				} else {
					if len(fields) != 4 || fields[1] != ideal.SymbolOutside {
						t.Errorf("saddle=%q row=%s col=%s: expected %s with a cost or delta in %q", saddleID, r.ID, col.ID, ideal.SymbolOutside, c.Text)
						continue
					}
					if last := fields[3]; last == "pt" || last == "pts" {
						// Scored band: "✗ N pts", "✗ 1 pt" or "✗ <1 pt".
						continue
					}
					shownDelta, err := strconv.ParseFloat(strings.TrimSuffix(fields[2], meta.Unit), 64)
					if err != nil {
						t.Fatalf("parse delta %q: %v", fields[2], err)
					}
					if math.Abs(shownDelta-expectedDelta) > 1e-6 {
						t.Errorf("saddle=%q row=%s col=%s cell=%q: shown=%v ideal=[%v,%v] delta=%v, want %v",
							saddleID, r.ID, col.ID, c.Text, shownVal, idealMin, idealMax, shownDelta, expectedDelta)
					}
				}
			}
		}
	}
}

func TestBuildFrameListDocument_RejectsUnknownFilters(t *testing.T) {
	allFrames := frames.AllFrames()
	if _, err := BuildFrameListDocument(allFrames, ListFilter{Category: "hardtail"}); err != nil {
		t.Fatalf("valid category hardtail failed: %v", err)
	}
	if _, err := BuildFrameListDocument(allFrames, ListFilter{Size: "L"}); err != nil {
		t.Fatalf("valid size L failed: %v", err)
	}
	for _, f := range []ListFilter{
		{Category: "bogus"},
		{Category: "full-suspension"},
		{Size: "ZZ"},
	} {
		if _, err := BuildFrameListDocument(allFrames, f); err == nil {
			t.Errorf("BuildFrameListDocument(%+v) succeeded, want error", f)
		}
	}
}

func TestParseComparisonSpecs_RejectsInvalidSagAndRigidTypos(t *testing.T) {
	allFrames := frames.AllFrames()
	badSpecs := []string{
		"fenrir@NaN",
		"fenrir@nan",
		"fenrir@NaN%",
		"fenrir@sag=NaN",
		"fenrir@-5",
		"fenrir@-5%",
		"fenrir@-0.1",
		"fenrir@sag=-5",
		"fenrir@-0",
		"fenrir@Inf",
		"fenrir@-Inf",
		"fenrir@110mm@NaN",
		"fenrir@110mm@-5",
		"fenrir@110mm@sag=-5",
		"fenrir@110mm@15%",
		"fenrir@rigidXYZ",
	}
	for _, spec := range badSpecs {
		if _, err := ParseComparisonSpecs(allFrames, catalog.CommonForkCatalog, spec); err == nil {
			t.Errorf("ParseComparisonSpecs(%q) succeeded, want error", spec)
		}
	}
}

func TestResolveValidationTargets_UnifiedUnknownSizeError(t *testing.T) {
	allFrames := frames.AllFrames()
	_, errCmp := ParseComparisonSpecs(allFrames, catalog.CommonForkCatalog, "fenrir:ZZ")
	_, errVal := ResolveValidationTargets(allFrames, []string{"fenrir"}, "ZZ")
	if errCmp == nil || errVal == nil {
		t.Fatalf("expected both compare and validate to reject size ZZ: cmp=%v val=%v", errCmp, errVal)
	}
	if errCmp.Error() != errVal.Error() {
		t.Errorf("unknown size error mismatch:\n  compare:  %s\n  validate: %s", errCmp, errVal)
	}
}

func TestSamePhysicalForkSameLabel(t *testing.T) {
	specs := []string{
		"sklar-pbj",
		"sklar-pbj@stock",
		"sklar-pbj@rigid",
		"sklar-pbj@sklar_steel_485",
		"sklar-pbj@a2c=485/52",
		"sklar-pbj@0%",
	}
	res := mustRun(t, Options{Specs: specs})
	doc, err := BuildDocument(res, DocOptions{Scoring: false, Flat: true})
	if err != nil {
		t.Fatal(err)
	}
	tb := doc.Tables()[0]
	forkCol := -1
	for i, c := range tb.Columns {
		if c.ID == "fork" {
			forkCol = i
			break
		}
	}
	if forkCol < 0 {
		t.Fatal("missing fork column")
	}
	for _, r := range tb.Rows {
		if r.Kind != report.KindData {
			continue
		}
		if got := r.Cells[forkCol].Text; got != "Rigid (485mm)" {
			t.Errorf("row %s fork = %q, want %q", r.ID, got, "Rigid (485mm)")
		}
	}
}

func TestMetricLabelsLayoutIndependent(t *testing.T) {
	specs := []string{"sklar-pbj@110mm@20%", "revel-el-jefe@120mm@20%"}

	colRes := mustRun(t, Options{Specs: specs, Transpose: model.Some(false)})
	colDoc, err := BuildDocument(colRes, DocOptions{Scoring: true})
	if err != nil {
		t.Fatal(err)
	}

	rowRes := mustRun(t, Options{Specs: specs, Transpose: model.Some(true)})
	rowDoc, err := BuildDocument(rowRes, DocOptions{Scoring: true, Flat: true})
	if err != nil {
		t.Fatal(err)
	}

	colLabels := map[string]string{}
	colForkCells := map[string]string{}
	colTable := colDoc.Tables()[0]
	for _, r := range colTable.Rows {
		colLabels[r.ID] = r.Cells[0].Text
		if r.ID == "fork" {
			for i, c := range colTable.Columns {
				if c.Attrs["frame-id"] != "" {
					colForkCells[c.Attrs["col"]] = r.Cells[i].Text
				}
			}
		}
	}

	rowTable := rowDoc.Tables()[0]
	rowLabels := map[string]string{}
	forkIdx := -1
	for i, c := range rowTable.Columns {
		rowLabels[c.ID] = c.Label
		if c.ID == "fork" {
			forkIdx = i
		}
	}
	if forkIdx < 0 {
		t.Fatal("missing fork column in transposed layout")
	}
	for id, wantLabel := range colLabels {
		if got := rowLabels[id]; got != wantLabel {
			t.Errorf("metric %q label mismatch: columns=%q rows=%q", id, wantLabel, got)
		}
	}
	for _, r := range rowTable.Rows {
		if r.Kind != report.KindData {
			continue
		}
		if got, want := r.Cells[forkIdx].Text, colForkCells[r.Attrs["col"]]; got != want {
			t.Errorf("col %s fork cell mismatch: rows=%q columns=%q", r.Attrs["col"], got, want)
		}
	}
}

func TestFooterDefinesOnlyShownMetrics(t *testing.T) {
	res := mustRun(t, Options{Specs: []string{"sklar-pbj", "revel-el-jefe"}})
	doc, err := BuildDocument(res, DocOptions{Scoring: true, DocFooters: true})
	if err != nil {
		t.Fatal(err)
	}
	var refFooter *report.Footer
	for i := range doc.Footers {
		if doc.Footers[i].Title == "Reference Metric Definitions" {
			refFooter = &doc.Footers[i]
			break
		}
	}
	if refFooter == nil {
		t.Fatal("missing Reference Metric Definitions footer")
	}
	joined := strings.Join(refFooter.Lines, "\n")
	for _, unshown := range []string{"Seat Axis Setback", "Wheelbase"} {
		if strings.Contains(joined, unshown) {
			t.Errorf("Reference Metric Definitions mentions unshown metric %q:\n%s", unshown, joined)
		}
	}
}

func TestCompare_BrpSetbackUsesSelectedBRPAndFooterUsesSitBones(t *testing.T) {
	res := mustRun(t, Options{Specs: []string{"surly-ecr:L@rigid"}})
	doc, err := BuildDocument(res, DocOptions{Scoring: true, DocFooters: true, Flat: true})
	if err != nil {
		t.Fatal(err)
	}
	// F-p4ab-1: with the zero-setback dropper baseline, Surly ECR L lands 10mm short of 219 on the rails (+15mm limit) rather than DeadCenterBRP.
	brpCell := ""
	for _, r := range doc.Tables()[0].Rows {
		if r.Kind == report.KindData && r.Attrs["metric"] == "brp_setback" {
			brpCell = r.Cells[2].Text
		}
	}
	if brpCell != "209 ✗ 12 pts" {
		t.Errorf("surly-ecr:L brp_setback cell = %q, want %q", brpCell, "209 ✗ 12 pts")
	}

	// F-p4ab-2: Footer defines HandGrip Rise relative to Sit Bones, not Saddle BRP.
	var refLines []string
	for _, f := range doc.Footers {
		if f.Title == "Reference Metric Definitions" {
			refLines = f.Lines
		}
	}
	joined := strings.Join(refLines, "\n")
	if !strings.Contains(joined, "HandGrip Rise: hand grip height relative to the Sit Bones") {
		t.Errorf("footer should define HandGrip Rise relative to Sit Bones:\n%s", joined)
	}
	if strings.Contains(joined, "HandGrip Rise: hand grip height relative to the Saddle BRP") || strings.Contains(joined, "HB Rise: handlebar height relative to the Saddle BRP") {
		t.Errorf("footer still contains stale HandGrip Rise vs Saddle BRP text:\n%s", joined)
	}
}

func TestCompare_SingleStepIntegerRounding(t *testing.T) {
	res := mustRun(t, Options{
		Specs:       []string{"sklar-pbj:M", "fairlight-holt-2:S", "surly-ecr:L", "sklar-pbj:L", "surly-ecr:M"},
		TopPerFrame: model.Some(0),
	})
	doc, err := BuildDocument(res, DocOptions{Flat: true})
	if err != nil {
		t.Fatalf("BuildDocument: %v", err)
	}
	tables := doc.Tables()
	if len(tables) == 0 {
		t.Fatalf("expected comparison table")
	}
	tbl := tables[0]
	colIdx := map[string]int{}
	for i, c := range tbl.Columns {
		colIdx[c.Label] = i
	}
	findVal := func(framePrefix, col string) string {
		ci, ok := colIdx[col]
		if !ok {
			t.Fatalf("missing header %q", col)
		}
		for _, r := range tbl.Rows {
			if strings.HasPrefix(r.Cells[colIdx["Frame (Size)"]].Text, framePrefix) {
				return r.Cells[ci].Text
			}
		}
		t.Fatalf("missing row %q", framePrefix)
		return ""
	}

	if got := findVal("Sklar PBJ (M)", "Trail"); !strings.HasPrefix(got, "87 ") {
		t.Errorf("Sklar PBJ (M) Trail = %q, want 87 ... (single-step from ~87.45mm)", got)
	}
	if got := findVal("Fairlight Holt 2.0 (S)", "ETT"); got != "587" {
		t.Errorf("Fairlight Holt 2.0 (S) ETT = %q, want 587 (single-step from ~586.55mm)", got)
	}
	if got := findVal("Surly ECR (L)", "BRP–HandGrip"); got != "643" {
		t.Errorf("Surly ECR (L) BRP–HandGrip = %q, want 643 (display-only, with 11 mm HeadsetTopStackMm)", got)
	}
}

func TestCompare_BreakdownSubtitleAndTranslatedH(t *testing.T) {
	saddle, ok := catalog.GetSaddle("berthoud-aspin-open")
	if !ok {
		t.Fatalf("unknown test saddle berthoud-aspin-open")
	}
	res := mustRun(t, Options{
		Specs:  []string{"sklar-pbj:L"},
		Saddle: model.Some(saddle),
	})
	doc, err := BuildDocument(res, DocOptions{Scoring: true, Detail: true})
	if err != nil {
		t.Fatalf("BuildDocument: %v", err)
	}

	tables := doc.Tables()
	if len(tables) < 2 {
		t.Fatalf("expected matrix + breakdown tables, got %d", len(tables))
	}
	matSub := tables[0].Subtitle
	if !strings.Contains(matSub, "H (saddle height): 753.3mm (translated from 746mm)") {
		t.Errorf("matrix subtitle should report translated H for Berthoud Aspin Open:\n%s", matSub)
	}
	if !strings.Contains(matSub, "| Ideal Profile: idmatch | Cockpit Target: HandGrip") {
		t.Errorf("matrix subtitle should disclose active Cockpit Target:\n%s", matSub)
	}

	bdSub := tables[len(tables)-1].Subtitle
	if strings.Contains(bdSub, "±1–2") {
		t.Errorf("breakdown subtitle must not carry the rounding caveat:\n%s", bdSub)
	}
	for _, want := range []string{
		"Total = 100 − Σ points lost",
		"BRP Setback 1.20/mm",
		"Oval Fits 22/R beyond the oval edge",
		"Seatpost Ext 0.18/mm below, 0.21/mm above",
	} {
		if !strings.Contains(bdSub, want) {
			t.Errorf("breakdown subtitle missing %q:\n%s", want, bdSub)
		}
	}
}

func TestRunAll_CadForkSurvivesSagPruningAndRanksByGate(t *testing.T) {
	res := mustRun(t, Options{All: true, TopPerFrame: model.Some(0)})
	cad := map[string]bool{}
	for _, r := range res.Rows {
		if r.Setup.IsCadFork {
			cad[r.Setup.FrameID] = true
		}
	}
	for _, f := range frames.AllFrames() {
		if !cad[f.ID] {
			t.Errorf("%s: the CAD fork was pruned from the candidates", f.ID)
		}
	}

	// Every fitting setup ranks above every one that does not fit.
	sawMiss := false
	for _, r := range res.Rows {
		fits := scoring.Fits(r, res.Profile)
		if !fits {
			sawMiss = true
		} else if sawMiss {
			t.Fatalf("%s %s %s fits but ranks below a setup that does not", r.Setup.FrameID, r.Setup.Size, r.Setup.DisplayLabel)
		}
	}
	if !sawMiss {
		t.Fatal("expected at least one setup that fails the gate")
	}
}

func TestRun_CustomCatalogForkAndStemIndependence(t *testing.T) {
	customCat := validator.Catalog{
		Forks: []model.ForkCatalogEntry{
			{
				ID:       "custom_sid_120",
				Name:     "Custom SID 120mm",
				TravelMm: 120,
				Standard: true,
				Options:  []model.ForkCatalogOption{{A2cMm: 530, RakeMm: 44}},
			},
			{
				ID:       "custom_rigid_510",
				Name:     "Custom Rigid 510mm",
				TravelMm: 0,
				Options:  []model.ForkCatalogOption{{A2cMm: 510, RakeMm: 44}},
			},
		},
		Stems: []model.StemCatalogEntry{
			{
				ID:            "custom_trail",
				LengthRangeMm: []float64{60, 70},
				AnglesDeg:     []model.Deg{-6},
			},
		},
	}

	customFrame := frames.RevelElJefe.Clone()
	customFrame.ID = "custom-el-jefe"
	customFrame.Stems = []string{"custom_trail"}
	customFrame.CadFork = model.ForkRef{Fork: "custom_sid_120", TravelMm: 120, SagPct: 20, A2cMm: 506, RakeMm: 44}
	customFrame.Forks = []model.ForkRef{
		{Fork: "custom_sid_120", TravelMm: 120, A2cMm: 530, RakeMm: 44},
		{Fork: "custom_rigid_510", A2cMm: 510, RakeMm: 44},
	}

	runWith := func(cat validator.Catalog, specs ...string) (*Result, error) {
		return Run(Options{
			Frames:   []model.FrameModel{customFrame},
			Baseline: testBaseline(),
			Profile:  ideal.IdmatchProfile(),
			Catalog:  cat,
			Specs:    specs,
		})
	}

	// Default fork resolves using the injected 120mm entry (506mm sagged), not raw 530mm.
	res, err := runWith(customCat, "custom-el-jefe:L")
	if err != nil {
		t.Fatalf("Run default fork with custom catalog failed: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(res.Rows))
	}
	if math.Abs(res.Rows[0].EffectiveA2c-506.0) > 0.05 {
		t.Errorf("EffectiveA2c = %.1f, want 506.0", res.Rows[0].EffectiveA2c)
	}
	if len(res.Rows[0].Bike.Stems) != 2 {
		t.Errorf("Bike.Stems = %+v, want 2 stems from custom_trail", res.Rows[0].Bike.Stems)
	}

	// Explicit injected fork ID works in compare.Run.
	resByID, err := runWith(customCat, "custom-el-jefe:L@custom_sid_120")
	if err != nil {
		t.Fatalf("Run(@custom_sid_120) failed: %v", err)
	}
	if math.Abs(resByID.Rows[0].EffectiveA2c-506.0) > 0.05 {
		t.Errorf("@custom_sid_120 EffectiveA2c = %.1f, want 506.0", resByID.Rows[0].EffectiveA2c)
	}

	// @all expansion uses injected forks.
	resAll, err := runWith(customCat, "custom-el-jefe:L@all")
	if err != nil {
		t.Fatalf("Run(@all) with custom catalog failed: %v", err)
	}
	if len(resAll.Rows) < 2 {
		t.Errorf("expected multiple @all rows from custom catalog, got %d", len(resAll.Rows))
	}

	// Same frame fails when only the compiled default catalog is supplied.
	defaultCat := validator.Catalog{Forks: catalog.CommonForkCatalog, Stems: catalog.CommonStemCatalog}
	if _, err := runWith(defaultCat, "custom-el-jefe:L@custom_sid_120"); err == nil {
		t.Errorf("expected Run(@custom_sid_120) to fail with CommonForkCatalog")
	}
}

func TestRun_BaselineNotMutated(t *testing.T) {
	baseline := testBaseline()
	if len(baseline.Columns) < 3 {
		t.Fatalf("expected at least 3 baseline columns, got %d", len(baseline.Columns))
	}
	origCols := slices.Clone(baseline.Columns)

	run := func(specs ...string) *Result {
		t.Helper()
		res, err := Run(Options{
			Frames:   frames.AllFrames(),
			Baseline: baseline,
			Profile:  ideal.IdmatchProfile(),
			Catalog:  validator.Catalog{Forks: catalog.CommonForkCatalog, Stems: catalog.CommonStemCatalog},
			Specs:    specs,
		})
		if err != nil {
			t.Fatalf("Run(%v): %v", specs, err)
		}
		return res
	}

	res1 := run("-1")
	if !slices.Equal(baseline.Columns, origCols) {
		t.Fatalf("baseline.Columns mutated after -1:\ngot:  %+v\nwant: %+v", baseline.Columns, origCols)
	}
	if len(res1.Rows) != len(origCols)-1 {
		t.Errorf("len(res1.Rows) = %d, want %d", len(res1.Rows), len(origCols)-1)
	}
	for _, r := range res1.Rows {
		if r.Setup.FrameID == origCols[0].FrameID && r.Setup.Size == origCols[0].Size {
			t.Errorf("removed column %s:%s still present after -1", origCols[0].FrameID, origCols[0].Size)
		}
	}

	_ = run("-2")
	if !slices.Equal(baseline.Columns, origCols) {
		t.Fatalf("baseline.Columns mutated after -2")
	}

	resPlus := run("+fenrir:L")
	if !slices.Equal(baseline.Columns, origCols) {
		t.Fatalf("baseline.Columns mutated after +fenrir:L")
	}
	if len(resPlus.Rows) != len(origCols)+1 {
		t.Errorf("len(resPlus.Rows) = %d, want %d", len(resPlus.Rows), len(origCols)+1)
	}
}

func TestRun_ForkLookupCompatibility(t *testing.T) {
	for _, spec := range []string{
		"fenrir:L@SID_100",
		"fenrir:L@rockshox sid 100mm",
	} {
		res := mustRun(t, Options{Specs: []string{spec}})
		if len(res.Rows) != 1 {
			t.Fatalf("%s: got %d rows, want 1", spec, len(res.Rows))
		}
		if math.Abs(res.Rows[0].EffectiveA2c-486.0) > 0.05 {
			t.Errorf("%s: EffectiveA2c = %.1f, want 486.0", spec, res.Rows[0].EffectiveA2c)
		}
	}
}
