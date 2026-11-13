package compare

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"bikegeo/pkg/frames"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/report"
)

func mustRun(t *testing.T, opts Options) *Result {
	t.Helper()
	res, err := Run(opts)
	if err != nil {
		t.Fatalf("Run(%+v): %v", opts, err)
	}
	return res
}

func TestRunDefaultBaseline(t *testing.T) {
	res := mustRun(t, Options{})
	if got, want := len(res.Rows), len(DefaultComparisonState().Columns); got != want {
		t.Errorf("default rows = %d, want %d", got, want)
	}
	if !res.State.Transpose {
		t.Error("default state with >4 columns should be transposed")
	}
}

func TestRunErrors(t *testing.T) {
	for _, opts := range []Options{
		{Specs: []string{"nosuchframe"}},
		{Specs: []string{"fenrir@17%"}},
		{Saddle: "nope"},
		{Handlebar: "nope"},
		{Specs: []string{"-all"}},
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
	if fit.Rows[0].Kind != report.KindIdeal || fit.Rows[1].Kind != report.KindAcceptable {
		t.Error("first two rows should be ideal/acceptable references")
	}
	zones := map[string]int{}
	for _, r := range fit.Rows[2:] {
		if r.Attrs["frame-id"] == "" || r.Attrs["rank"] == "" {
			t.Errorf("data row missing attrs: %+v", r.Attrs)
		}
		for _, c := range r.Cells {
			if c.MetricID != "" {
				zones[c.Zone]++
			}
		}
	}
	if zones[report.ZoneIdeal]+zones[report.ZoneAcceptable]+zones[report.ZoneOutside] == 0 {
		t.Errorf("no zoned metric cells found: %v", zones)
	}
	if got := len(fit.Rows) - 2; got != len(res.Rows) {
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
	wantIdeal, wantAcc := FormatAttrRanges(clampSpec, meta)
	if wantIdeal != "≥100" || wantAcc != "≥75" {
		t.Fatalf("FormatAttrRanges(handlebar_clamp_fit) = (%q, %q), want (≥100, ≥75)", wantIdeal, wantAcc)
	}

	for _, saddle := range []string{"", "berthoud-aspin-open", "selle-italia-slr-boost-superflow-l3"} {
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

		// Verify reference rows in fit-cockpit show ≥100 / ≥75 for handlebar_clamp_fit.
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
			t.Errorf("saddle=%q Ideal ref = %q, want %q", saddle, got, wantIdeal)
		}
		if got := fit.Rows[1].Cells[clampCol].Text; got != wantAcc {
			t.Errorf("saddle=%q Acceptable ref = %q, want %q", saddle, got, wantAcc)
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

		// Map breakdown row ID -> printed subscore.
		bdByRow := map[string]int{}
		for _, r := range breakdown.Rows {
			fields := strings.Fields(r.Cells[bdClampCol].Text)
			if len(fields) == 0 {
				t.Fatalf("empty breakdown cell for row %s", r.ID)
			}
			sub, err := strconv.Atoi(fields[0])
			if err != nil {
				t.Fatalf("parse breakdown subscore %q: %v", r.Cells[bdClampCol].Text, err)
			}
			bdByRow[r.ID] = sub
		}

		for _, r := range fit.Rows[2:] {
			cell := r.Cells[clampCol]
			sub, ok := bdByRow[r.ID]
			if !ok {
				t.Fatalf("missing breakdown row for %s", r.ID)
			}
			wantMarker := ideal.SymbolOutside
			wantZone := report.ZoneOutside
			if sub >= 100 {
				wantMarker = ideal.SymbolIdeal
				wantZone = report.ZoneIdeal
			} else if sub >= 75 {
				wantMarker = ideal.SymbolAcceptable
				wantZone = report.ZoneAcceptable
			}
			if !strings.HasSuffix(cell.Text, " "+wantMarker) {
				t.Errorf("saddle=%q row=%s sub=%d: cell=%q, want marker %s", saddle, r.ID, sub, cell.Text, wantMarker)
			}
			if cell.Zone != wantZone {
				t.Errorf("saddle=%q row=%s sub=%d: zone=%q, want %q", saddle, r.ID, sub, cell.Zone, wantZone)
			}
		}
	}
}

func TestDocument_AllConfigsZonesAndDeltasConsistent(t *testing.T) {
	markerToZone := map[string]string{
		ideal.SymbolIdeal:      report.ZoneIdeal,
		ideal.SymbolAcceptable: report.ZoneAcceptable,
		ideal.SymbolOutside:    report.ZoneOutside,
	}

	for _, saddle := range []string{"", "berthoud-aspin-open"} {
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
			meta, ok := ideal.LookupAttribute(col.ID)
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

			for _, r := range fit.Rows[2:] {
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
						saddle, r.ID, col.ID, c.Text, c.Zone, wantZone)
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
							saddle, r.ID, col.ID, shownVal, idealMin, idealMax, c.Text)
					}
				} else {
					if len(fields) != 4 {
						t.Errorf("saddle=%q row=%s col=%s: expected delta in %q", saddle, r.ID, col.ID, c.Text)
						continue
					}
					shownDelta, err := strconv.ParseFloat(strings.TrimSuffix(fields[2], meta.Unit), 64)
					if err != nil {
						t.Fatalf("parse delta %q: %v", fields[2], err)
					}
					if math.Abs(shownDelta-expectedDelta) > 1e-6 {
						t.Errorf("saddle=%q row=%s col=%s cell=%q: shown=%v ideal=[%v,%v] delta=%v, want %v",
							saddle, r.ID, col.ID, c.Text, shownVal, idealMin, idealMax, shownDelta, expectedDelta)
					}
				}
			}
		}
	}
}

func TestBuildFrameListDocument_RejectsUnknownFilters(t *testing.T) {
	if _, err := BuildFrameListDocument(ListFilter{Category: "hardtail"}); err != nil {
		t.Fatalf("valid category hardtail failed: %v", err)
	}
	if _, err := BuildFrameListDocument(ListFilter{Size: "L"}); err != nil {
		t.Fatalf("valid size L failed: %v", err)
	}
	for _, f := range []ListFilter{
		{Category: "bogus"},
		{Category: "full-suspension"},
		{Size: "ZZ"},
	} {
		if _, err := BuildFrameListDocument(f); err == nil {
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
		if _, err := ParseComparisonSpecs(allFrames, spec); err == nil {
			t.Errorf("ParseComparisonSpecs(%q) succeeded, want error", spec)
		}
	}
}

func TestResolveValidationTargets_UnifiedUnknownSizeError(t *testing.T) {
	allFrames := frames.AllFrames()
	_, errCmp := ParseComparisonSpecs(allFrames, "fenrir:ZZ")
	_, errVal := ResolveValidationTargets([]string{"fenrir"}, "ZZ")
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

	colRes := mustRun(t, Options{Specs: specs, Transpose: new(false)})
	colDoc, err := BuildDocument(colRes, DocOptions{Scoring: true})
	if err != nil {
		t.Fatal(err)
	}

	rowRes := mustRun(t, Options{Specs: specs, Transpose: new(true)})
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
