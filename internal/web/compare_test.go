package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/catalog"
	"github.com/matta/bikegeo/internal/compare"
	"github.com/matta/bikegeo/internal/query"
)

// frameSpec names a catalog frame trimmed to one size and one fork; a blank
// size or fork keeps them all.
type frameSpec struct{ frame, size, fork string }

// krampusXLFox34 ranks the Surly Krampus XL on the Fox 34 534x44: three sag rows.
var krampusXLFox34 = frameSpec{"surly-krampus", "XL", "fox34_120-534x44"}

// havokLSklar ranks the sliding-dropout Binary Havok L on its rigid Sklar fork: one row.
var havokLSklar = frameSpec{"binary-havok", "L", "sklar_steel_485"}

// fullCatalogWinners trims six catalog frames to the size and fork that win
// each of them on the full-catalog default page (query's
// TestParseCompareDefaultAllFrames), so their rows match that page's rows.
var fullCatalogWinners = []frameSpec{
	{"surly-krampus", "XL", "fox34_120-534x44"},
	{"sklar-pbj", "L", "fox36_sl_120"},
	{"seven-custom", "Custom", "fox34_120-534x44"},
	{"esker-hayduke", "S3", "enve_mountain_boost-492x52"},
	{"otso-fenrir-stainless", "XL", "firestarter_deluxe_483"},
	{"salsa-cutthroat", "60cm", "firestarter_deluxe_483"},
}

// trimmedFrames returns a compareFrames func ranking only specs' catalog
// frames, each trimmed to its size and fork, so a compare page ranks a few
// rows instead of the whole catalog.
func trimmedFrames(t *testing.T, specs ...frameSpec) func() []compare.Frame {
	t.Helper()
	catalogFrames := query.CatalogFrames()
	frames := make([]compare.Frame, 0, len(specs))
	for _, spec := range specs {
		i := slices.IndexFunc(catalogFrames, func(f compare.Frame) bool { return f.ID == spec.frame })
		if i < 0 {
			t.Fatalf("unknown frame %q", spec.frame)
		}
		f := catalogFrames[i]
		if spec.size != "" {
			f.Sizes = slices.DeleteFunc(f.Sizes, func(sz compare.Size) bool { return sz.Name != spec.size })
		}
		if spec.fork != "" {
			f.Forks = slices.DeleteFunc(f.Forks, func(fk compare.Fork) bool { return fk.ID != spec.fork })
		}
		if len(f.Sizes) == 0 || len(f.Forks) == 0 {
			t.Fatalf("frame %q has no size %q or fork %q", spec.frame, spec.size, spec.fork)
		}
		frames = append(frames, f)
	}
	return func() []compare.Frame { return frames }
}

// loadCompareDoc serves rawPath with / ranking specs' trimmed frames.
func loadCompareDoc(t *testing.T, rawPath string, specs ...frameSpec) (int, *goquery.Document) {
	t.Helper()
	if len(specs) == 0 {
		t.Fatalf("loadCompareDoc(%q) needs at least one frame", rawPath)
	}
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+rawPath, nil)
	rec := httptest.NewRecorder()
	newHandler(trimmedFrames(t, specs...)).ServeHTTP(rec, req)
	doc, err := goquery.NewDocumentFromReader(rec.Body)
	if err != nil {
		t.Fatalf("parse HTML for %s: %v", rawPath, err)
	}
	return rec.Code, doc
}

// TestCompareDefaultPage renders the default page over fullCatalogWinners, so
// its six rows are rows 1, 2, 3, 8, 16, and 18 of the full-catalog page.
func TestCompareDefaultPage(t *testing.T) {
	status, doc := loadCompareDoc(t, "/", fullCatalogWinners...)
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	if got := strings.TrimSpace(doc.Find("title").Text()); got != "Compare 6 Frames — bikegeo" {
		t.Errorf("title = %q, want Compare 6 Frames — bikegeo", got)
	}
	if got := strings.TrimSpace(doc.Find(".badge").First().Text()); got != "All 6 frames" {
		t.Errorf("header badge = %q, want All 6 frames", got)
	}
	if doc.Find(`[data-compare-mode="all"]`).Length() != 1 {
		t.Error("expected [data-compare-mode=all]")
	}
	if got := doc.Find(`[data-compare-mode="all"]`).Text(); !strings.Contains(got, "all 6 catalog frames") {
		t.Errorf("compare-mode note = %q, want substring %q", got, "all 6 catalog frames")
	}
	wantSpeedNote := fmt.Sprintf("All %d handlebars evaluates ~15× more builds (about 2 s per frame).", len(catalog.Handlebars()))
	if got := strings.TrimSpace(doc.Find(`[data-note="bar-options-speed"]`).Text()); got != wantSpeedNote {
		t.Errorf("bar-options-speed note = %q, want %q", got, wantSpeedNote)
	}
	notesDrawer := doc.Find(`[data-notes-drawer]`)
	if notesDrawer.Length() != 1 {
		t.Fatalf("got %d [data-notes-drawer], want 1", notesDrawer.Length())
	}
	if _, open := notesDrawer.Attr("open"); open {
		t.Error("[data-notes-drawer] should be closed by default")
	}
	if summary := strings.TrimSpace(notesDrawer.Find("summary").Text()); !strings.Contains(summary, "Parameter & model notes") || !strings.Contains(summary, "(Redshift caveat)") {
		t.Errorf("notes drawer summary = %q, want Parameter & model notes + (Redshift caveat)", summary)
	}
	if n := notesDrawer.Find(`[data-note="redshift-mixed-state"]`).Length(); n != 1 {
		t.Errorf("got %d redshift-mixed-state notes, want 1", n)
	}
	if n := doc.Find(`.tier-strip-badges .tier-badge`).Length(); n != 4 {
		t.Errorf("got %d tier strip badges, want 4", n)
	}
	tierDefs := doc.Find(`[data-tier-definitions]`)
	if tierDefs.Length() != 1 {
		t.Fatalf("got %d [data-tier-definitions], want 1", tierDefs.Length())
	}
	if _, open := tierDefs.Attr("open"); open {
		t.Error("[data-tier-definitions] should be closed by default")
	}
	if n := tierDefs.Find(`[data-fit-tier-legend] dt`).Length(); n != 4 {
		t.Errorf("got %d tier legend items, want 4", n)
	}
	if _, open := doc.Find(`[data-score-breakdown]`).Attr("open"); open {
		t.Error("[data-score-breakdown] should be closed by default")
	}
	if href, _ := doc.Find(`a#preset-defaults`).Attr("href"); href != "/" {
		t.Errorf("a#preset-defaults href = %q, want /", href)
	}

	configsText := doc.Find(`[data-summary="configs"]`).Text()
	for _, want := range []string{"6", "12", "8988"} {
		if !strings.Contains(configsText, want) {
			t.Errorf("configs summary %q missing %q", configsText, want)
		}
	}
	if targetsText := doc.Find(`[data-summary="targets"]`).Text(); !strings.Contains(targetsText, "BRP (-219.0, 713.1) mm") || !strings.Contains(targetsText, "HandGrip (481.0, 726.0) mm") {
		t.Errorf("targets summary = %q", targetsText)
	}
	if postSel := strings.TrimSpace(doc.Find(`#ctrl-post option[selected]`).Text()); !strings.Contains(postSel, "Redshift ShockStop PRO") {
		t.Errorf("selected post option %q missing Redshift ShockStop PRO", postSel)
	}
	if barText := doc.Find(`[data-summary="bar"]`).Text(); !strings.Contains(barText, "Persuader") {
		t.Errorf("bar summary %q missing Persuader", barText)
	}

	for _, tableName := range []string{"fit", "geometry", "breakdown"} {
		sel := fmt.Sprintf(`table[data-table="%s"] tbody tr`, tableName)
		if n := doc.Find(sel).Length(); n != 6 {
			t.Errorf("%s row count = %d, want 6", sel, n)
		}
	}

	// Row #1: Surly Krampus XL, Fox Float 34 120mm (534/44), 25% sag.
	fit1 := doc.Find(`tr[data-compare-rank="1"]`)
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="frame"]`).Text()); got != "Surly Krampus" {
		t.Errorf("row 1 frame = %q, want Surly Krampus", got)
	}
	if fit1.Find(`abbr.sliding-mark`).Length() != 0 {
		t.Error("row 1 should not have sliding-mark")
	}
	for cell, want := range map[string]string{
		"size":        "XL",
		"fork":        "Fox Float 34 120mm (534/44), 25% sag",
		"score":       "85.1",
		"grip-fit":    "8.8 mm back, 3.6 mm high",
		"stem":        "100 mm, -7°",
		"spacers":     "0 mm",
		"post-insert": "164.3 mm",
	} {
		if got := strings.TrimSpace(fit1.Find(fmt.Sprintf(`[data-compare-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("row 1 %s = %q, want %q", cell, got, want)
		}
	}
	rail1 := fit1.Find(`[data-compare-cell="rail"]`)
	if got := strings.TrimSpace(rail1.Find(`.fit-primary`).Text()); got != "Max back" {
		t.Errorf("row 1 rail primary = %q, want Max back", got)
	}
	if got := strings.TrimSpace(rail1.Find(`.fit-note`).Text()); got != "+15.0 mm" {
		t.Errorf("row 1 rail note = %q, want +15.0 mm", got)
	}
	if got, _ := rail1.Attr("title"); got != "Rail offset +15.0 mm (range ±15.0 mm)" {
		t.Errorf("row 1 rail title = %q, want Rail offset +15.0 mm (range ±15.0 mm)", got)
	}
	if got, _ := fit1.Find(`[data-compare-cell="grip-fit"]`).Attr("title"); got != "Oval R 0.95 · target − placed (+8.8, -3.6) mm (9.5 mm)" {
		t.Errorf("row 1 grip-fit title = %q, want %q", got, "Oval R 0.95 · target − placed (+8.8, -3.6) mm (9.5 mm)")
	}
	brp1 := fit1.Find(`[data-compare-cell="brp-fit"]`)
	if got := strings.TrimSpace(brp1.Find(`.fit-primary`).Text()); got != "5.3 mm fwd" {
		t.Errorf("row 1 brp-fit primary = %q, want 5.3 mm fwd", got)
	}
	if got := strings.TrimSpace(brp1.Find(`.fit-note`).Text()); got != "rail maxed back" {
		t.Errorf("row 1 brp-fit note = %q, want rail maxed back", got)
	}
	if got, _ := brp1.Attr("title"); got != "target − placed (-5.3, +0.0) mm" {
		t.Errorf("row 1 brp-fit title = %q, want target − placed (-5.3, +0.0) mm", got)
	}
	if got, _ := fit1.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_all" {
		t.Errorf("row 1 tier attr = %q, want fits_all", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-fit-tier]`).Text()); got != "Fits all" {
		t.Errorf("row 1 tier text = %q, want Fits all", got)
	}
	if n := doc.Find(`[data-compare-cell="bar"]`).Length(); n != 0 {
		t.Errorf("got %d bar cells when bar_options=selected, want 0", n)
	}

	geom1 := doc.Find(`tr[data-geometry-rank="1"]`)
	for cell, want := range map[string]string{
		"reach":     "458.2",
		"stack":     "629.7",
		"ett":       "663.8",
		"hta":       "67.9°",
		"sta":       "71.9°",
		"chainstay": "435.0",
		"wheelbase": "1169.3",
		"rc-wb":     "36.9%",
		"bb-drop":   "56.9",
		"trail":     "103.4",
	} {
		if got := strings.TrimSpace(geom1.Find(fmt.Sprintf(`[data-geometry-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("row 1 geom %s = %q, want %q", cell, got, want)
		}
	}

	bd1 := doc.Find(`tr[data-breakdown-rank="1"]`)
	for cell, want := range map[string]string{
		"oval-r":    "0.95",
		"grip-miss": "(+8.8, -3.6) mm",
		"brp-miss":  "(-5.3, +0.0) mm",
	} {
		if got := strings.TrimSpace(bd1.Find(fmt.Sprintf(`[data-breakdown-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("row 1 breakdown %s = %q, want %q", cell, got, want)
		}
	}
	for attr, want := range map[string]string{
		"HandGrip":           "90.5",
		"Saddle BRP Y":       "100.0",
		"Saddle BRP X":       "76.1",
		"Stem":               "73.0",
		"Handlebar sweep":    "100.0",
		"Fork sag":           "100.0",
		"Spacers":            "100.0",
		"Saddle rail offset": "0.0",
		"RC / WB ratio":      "96.0",
		"Seatpost insertion": "100.0",
		"Leveled BB drop":    "59.5",
		"Trail":              "100.0",
	} {
		if got := strings.TrimSpace(bd1.Find(fmt.Sprintf(`[data-breakdown-attr="%s"]`, attr)).Text()); got != want {
			t.Errorf("row 1 breakdown %s = %q, want %q", attr, got, want)
		}
	}

	// Row #2: Sklar PBJ L, Fox 36 SL 120mm at 15% sag (Slice 16d open fork choice)
	fit2 := doc.Find(`tr[data-compare-rank="2"]`)
	if got := strings.TrimSpace(fit2.Find(`[data-compare-cell="frame"]`).Text()); !strings.Contains(got, "Sklar PBJ") {
		t.Errorf("row 2 frame = %q, want Sklar PBJ", got)
	}
	if got := strings.TrimSpace(fit2.Find(`[data-compare-cell="fork"]`).Text()); got != "Fox 36 SL 120mm, 15% sag" {
		t.Errorf("row 2 fork = %q, want Fox 36 SL 120mm, 15%% sag", got)
	}
	if got, _ := fit2.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_all" {
		t.Errorf("row 2 tier = %q, want fits_all", got)
	}
	if got := strings.TrimSpace(fit2.Find(`[data-compare-cell="score"]`).Text()); got != "76.2" {
		t.Errorf("row 2 score = %q, want 76.2", got)
	}
	if got := strings.TrimSpace(fit2.Find(`[data-compare-cell="grip-fit"]`).Text()); got != "0.7 mm fwd, 13.9 mm high" {
		t.Errorf("row 2 grip-fit = %q, want 0.7 mm fwd, 13.9 mm high", got)
	}
	if got := strings.TrimSpace(fit2.Find(`[data-compare-cell="brp-fit"] .fit-primary`).Text()); got != "9.6 mm fwd" {
		t.Errorf("row 2 brp-fit primary = %q, want 9.6 mm fwd", got)
	}
	if got := strings.TrimSpace(fit2.Find(`[data-compare-cell="brp-fit"] .fit-note`).Text()); got != "rail maxed back" {
		t.Errorf("row 2 brp-fit note = %q, want rail maxed back", got)
	}
	bd2 := doc.Find(`tr[data-breakdown-rank="2"]`)
	for cell, want := range map[string]string{"oval-r": "0.89", "grip-miss": "(-0.7, -13.9) mm", "brp-miss": "(-9.6, +0.0) mm"} {
		if got := strings.TrimSpace(bd2.Find(fmt.Sprintf(`[data-breakdown-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("row 2 breakdown %s = %q, want %q", cell, got, want)
		}
	}
	geom2 := doc.Find(`tr[data-geometry-rank="2"]`)
	for cell, want := range map[string]string{"reach": "455.2", "stack": "655.2", "ett": "664.9", "chainstay": "452.0*"} {
		if got := strings.TrimSpace(geom2.Find(fmt.Sprintf(`[data-geometry-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("row 2 geom %s = %q, want %q", cell, got, want)
		}
	}

	// Row #3: Seven Custom
	fit3 := doc.Find(`tr[data-compare-rank="3"]`)
	if got, _ := fit3.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_brp" {
		t.Errorf("row 3 tier = %q, want fits_brp", got)
	}
	if got := strings.TrimSpace(fit3.Find(`[data-compare-cell="score"]`).Text()); got != "53.1" {
		t.Errorf("row 3 score = %q, want 53.1", got)
	}
	if got := strings.TrimSpace(fit3.Find(`[data-compare-cell="grip-fit"]`).Text()); got != "27.1 mm back, 13.9 mm low" {
		t.Errorf("row 3 grip-fit = %q, want 27.1 mm back, 13.9 mm low", got)
	}
	if got := strings.TrimSpace(fit3.Find(`[data-compare-cell="brp-fit"] .fit-primary`).Text()); got != "9.7 mm fwd" {
		t.Errorf("row 3 brp-fit primary = %q, want 9.7 mm fwd", got)
	}
	if got := strings.TrimSpace(fit3.Find(`[data-compare-cell="brp-fit"] .fit-note`).Text()); got != "rail maxed back" {
		t.Errorf("row 3 brp-fit note = %q, want rail maxed back", got)
	}
	bd3 := doc.Find(`tr[data-breakdown-rank="3"]`)
	for cell, want := range map[string]string{"oval-r": "2.09", "grip-miss": "(+27.1, +13.9) mm", "brp-miss": "(-9.7, +0.0) mm"} {
		if got := strings.TrimSpace(bd3.Find(fmt.Sprintf(`[data-breakdown-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("row 3 breakdown %s = %q, want %q", cell, got, want)
		}
	}
	geom3 := doc.Find(`tr[data-geometry-rank="3"]`)
	if got := strings.TrimSpace(geom3.Find(`[data-geometry-cell="reach"]`).Text()); got != "415.8" {
		t.Errorf("row 3 reach = %q, want 415.8", got)
	}
	if got := strings.TrimSpace(geom3.Find(`[data-geometry-cell="stack"]`).Text()); got != "636.4" {
		t.Errorf("row 3 stack = %q, want 636.4", got)
	}
	if got := strings.TrimSpace(geom3.Find(`[data-geometry-cell="ett"]`).Text()); got != "619.4" {
		t.Errorf("row 3 ett = %q, want 619.4", got)
	}

	// Row #4 (full-catalog #8): Esker Hayduke S3 (sliding dropouts)
	fit8 := doc.Find(`tr[data-compare-rank="4"]`)
	if got, _ := fit8.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_grip" {
		t.Errorf("row 8 tier = %q, want fits_grip", got)
	}
	if got := strings.TrimSpace(fit8.Find(`[data-compare-cell="frame"] abbr.sliding-mark`).Text()); got != "*" {
		t.Errorf("row 8 sliding-mark = %q, want *", got)
	}
	if got := strings.TrimSpace(doc.Find(`tr[data-geometry-rank="4"] [data-geometry-cell="chainstay"]`).Text()); got != "454.0*" {
		t.Errorf("row 8 chainstay = %q, want 454.0*", got)
	}
	if got := strings.TrimSpace(fit8.Find(`[data-compare-cell="score"]`).Text()); got != "81.3" {
		t.Errorf("row 8 score = %q, want 81.3", got)
	}
	if got := strings.TrimSpace(fit8.Find(`[data-compare-cell="brp-fit"] .fit-primary`).Text()); got != "37.0 mm fwd" {
		t.Errorf("row 8 brp-fit primary = %q, want 37.0 mm fwd", got)
	}
	if got := strings.TrimSpace(fit8.Find(`[data-compare-cell="brp-fit"] .fit-note`).Text()); got != "rail maxed back" {
		t.Errorf("row 8 brp-fit note = %q, want rail maxed back", got)
	}
	if got := strings.TrimSpace(doc.Find(`tr[data-breakdown-rank="4"] [data-breakdown-cell="brp-miss"]`).Text()); got != "(-37.0, +0.0) mm" {
		t.Errorf("row 8 breakdown brp-miss = %q, want (-37.0, +0.0) mm", got)
	}

	// Row #5 (full-catalog #16): Otso Fenrir Stainless XL (Tuning Chip frame, non-sliding)
	fit16 := doc.Find(`tr[data-compare-rank="5"]`)
	if got := strings.TrimSpace(fit16.Find(`[data-compare-cell="frame"]`).Text()); got != "Otso Fenrir Stainless" {
		t.Errorf("row 16 frame = %q, want Otso Fenrir Stainless", got)
	}
	if fit16.Find(`abbr.sliding-mark`).Length() != 0 {
		t.Error("row 16 should not have sliding-mark")
	}
	if got, _ := fit16.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_grip" {
		t.Errorf("row 16 tier = %q, want fits_grip", got)
	}
	if got := strings.TrimSpace(fit16.Find(`[data-compare-cell="score"]`).Text()); got != "69.1" {
		t.Errorf("row 16 score = %q, want 69.1", got)
	}
	if got := strings.TrimSpace(fit16.Find(`[data-compare-cell="grip-fit"]`).Text()); got != "12.6 mm back, 8.3 mm low" {
		t.Errorf("row 16 grip-fit = %q, want 12.6 mm back, 8.3 mm low", got)
	}
	if got := strings.TrimSpace(doc.Find(`tr[data-breakdown-rank="5"] [data-breakdown-cell="oval-r"]`).Text()); got != "0.96" {
		t.Errorf("row 16 breakdown oval-r = %q, want 0.96", got)
	}
	if got := strings.TrimSpace(doc.Find(`tr[data-geometry-rank="5"] [data-geometry-cell="chainstay"]`).Text()); got != "440.0" {
		t.Errorf("row 16 chainstay = %q, want 440.0", got)
	}

	// Row #6 (full-catalog #18): Salsa Cutthroat 60cm
	fit18 := doc.Find(`tr[data-compare-rank="6"]`)
	if got, _ := fit18.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_neither" {
		t.Errorf("row 18 tier = %q, want fits_neither", got)
	}
	if got := strings.TrimSpace(fit18.Find(`[data-compare-cell="score"]`).Text()); got != "50.3" {
		t.Errorf("row 18 score = %q, want 50.3", got)
	}
	if got := strings.TrimSpace(fit18.Find(`[data-compare-cell="grip-fit"]`).Text()); got != "40.3 mm back, 5.4 mm low" {
		t.Errorf("row 18 grip-fit = %q, want 40.3 mm back, 5.4 mm low", got)
	}
	if got := strings.TrimSpace(doc.Find(`tr[data-breakdown-rank="6"] [data-breakdown-cell="oval-r"]`).Text()); got != "3.45" {
		t.Errorf("row 18 breakdown oval-r = %q, want 3.45", got)
	}

	slidingNote := doc.Find(`[data-note="sliding-dropouts"]`).Text()
	for _, name := range []string{"Esker Hayduke", "Sklar PBJ"} {
		if !strings.Contains(slidingNote, name) {
			t.Errorf("sliding-dropouts note %q missing %q", slidingNote, name)
		}
	}
	if strings.Contains(slidingNote, "Otso Fenrir Stainless") {
		t.Errorf("sliding-dropouts note %q must not include Otso Fenrir Stainless (Q140)", slidingNote)
	}
}

// TestCompareFormHasOnlyCompareKeys checks that every control on / submits a
// key the compare query accepts: no custom selections, bar roll, or rail
// pitch (Q158).
func TestCompareFormHasOnlyCompareKeys(t *testing.T) {
	status, doc := loadCompareDoc(t, "/", krampusXLFox34)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	allowed := []string{"top_per_frame", "top", "post", "bar", "bar_options", "brp_x", "brp_y", "grip_x", "grip_y"}
	doc.Find(`form.controls [name]`).Each(func(_ int, el *goquery.Selection) {
		name, _ := el.Attr("name")
		if !slices.Contains(allowed, name) {
			t.Errorf("form control name=%q is not a compare query key", name)
		}
	})
	for _, sel := range []string{`[data-selection-row]`, `#ctrl-add-frame`, `#preset-all-frames`, `#ctrl-bar-roll`, `#ctrl-rail-pitch`, `[data-summary="posture"]`} {
		if n := doc.Find(sel).Length(); n != 0 {
			t.Errorf("got %d %s, want 0", n, sel)
		}
	}
	for _, raw := range []string{"/?frame=surly-krampus", "/?frame=&size=&fork=&sag=", "/?bar_roll=10", "/?rail_pitch=-2"} {
		status, doc := loadCompareDoc(t, raw, krampusXLFox34)
		if status != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", raw, status)
		}
		if got := doc.Find(`#form-error`).Text(); !strings.Contains(got, "unknown query parameter") {
			t.Errorf("%s error = %q, want unknown query parameter", raw, got)
		}
	}
}

func TestCompareTopLimits(t *testing.T) {
	status, doc := loadCompareDoc(t, "/", krampusXLFox34)
	if status != http.StatusOK {
		t.Fatalf("default status = %d, want 200", status)
	}
	if n := doc.Find(`table[data-table="fit"] tbody tr`).Length(); n != 1 {
		t.Errorf("default rows = %d, want 1 (best per frame)", n)
	}
	if val, _ := doc.Find(`#ctrl-top-per-frame`).Attr("value"); val != "" {
		t.Errorf("default #ctrl-top-per-frame value = %q, want empty", val)
	}

	status, doc = loadCompareDoc(t, "/?top_per_frame=0", krampusXLFox34)
	if status != http.StatusOK {
		t.Fatalf("top_per_frame=0 status = %d, want 200", status)
	}
	if n := doc.Find(`table[data-table="fit"] tbody tr`).Length(); n != 3 {
		t.Errorf("top_per_frame=0 rows = %d, want 3 (one per sag)", n)
	}

	status, limDoc := loadCompareDoc(t, "/?top_per_frame=1&top=3", fullCatalogWinners...)
	if status != http.StatusOK {
		t.Fatalf("limits status = %d, want 200", status)
	}
	if limDoc.Find(`table[data-table="fit"] tbody tr`).Length() != 3 {
		t.Errorf("top=3 rows = %d, want 3", limDoc.Find(`table[data-table="fit"] tbody tr`).Length())
	}
	if val, _ := limDoc.Find(`#ctrl-top-per-frame`).Attr("value"); val != "1" {
		t.Errorf("#ctrl-top-per-frame = %q, want 1", val)
	}
	if val, _ := limDoc.Find(`#ctrl-top`).Attr("value"); val != "3" {
		t.Errorf("#ctrl-top = %q, want 3", val)
	}
}

func TestCompareAllBarOptionsShowsHandlebarColumn(t *testing.T) {
	status, doc := loadCompareDoc(t, "/?bar_options=all&top_per_frame=0", krampusXLFox34)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var headers []string
	doc.Find(`table[data-table="fit"] thead th`).Each(func(_ int, s *goquery.Selection) {
		headers = append(headers, strings.TrimSpace(s.Text()))
	})
	if !strings.Contains(strings.Join(headers, "|"), "Handlebar") {
		t.Errorf("fit table headers %v missing Handlebar", headers)
	}
	fit1 := doc.Find(`table[data-table="fit"] tbody tr`).FilterFunction(func(_ int, tr *goquery.Selection) bool {
		return strings.HasSuffix(strings.TrimSpace(tr.Find(`[data-compare-cell="fork"]`).Text()), ", 20% sag")
	})
	if fit1.Length() != 1 {
		t.Fatalf("got %d 20%% sag rows, want 1", fit1.Length())
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="bar"]`).Text()); got != "SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm)" {
		t.Errorf("bar = %q", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="score"]`).Text()); got != "87.1" {
		t.Errorf("score = %q, want 87.1", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="grip-fit"]`).Text()); got != "7.5 mm back, 3.7 mm low" {
		t.Errorf("grip-fit = %q, want 7.5 mm back, 3.7 mm low", got)
	}
	rank, _ := fit1.Attr("data-compare-rank")
	bd := doc.Find(fmt.Sprintf(`tr[data-breakdown-rank="%s"]`, rank))
	if got := strings.TrimSpace(bd.Find(`[data-breakdown-cell="oval-r"]`).Text()); got != "0.58" {
		t.Errorf("oval-r = %q, want 0.58", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="stem"]`).Text()); got != "100 mm, -7°" {
		t.Errorf("stem = %q, want 100 mm, -7°", got)
	}
	if got := doc.Find(`[data-summary="configs"]`).Text(); !strings.Contains(got, "33705") {
		t.Errorf("configs summary %q missing 33705 (3 sags x 11235)", got)
	}
	if n := doc.Find(`[data-summary="bar"]`).Length(); n != 0 {
		t.Errorf("got %d [data-summary=bar] when bar_options=all, want 0", n)
	}
}

func TestCompareNonRedshiftPostOmitsCaveat(t *testing.T) {
	status, doc := loadCompareDoc(t, "/?post=setback_0", krampusXLFox34)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	notesDrawer := doc.Find(`[data-notes-drawer]`)
	if notesDrawer.Length() != 1 {
		t.Fatalf("got %d [data-notes-drawer], want 1", notesDrawer.Length())
	}
	if summary := strings.TrimSpace(notesDrawer.Find("summary").Text()); strings.Contains(summary, "Redshift caveat") {
		t.Errorf("notes drawer summary = %q, must omit Redshift caveat for setback_0", summary)
	}
	if n := notesDrawer.Find(`[data-note="redshift-mixed-state"]`).Length(); n != 0 {
		t.Errorf("got %d redshift-mixed-state notes, want 0 for setback_0", n)
	}
}

func TestCompareErrorPagesAndRecoveryQ76(t *testing.T) {
	assertCommonError := func(t *testing.T, raw string) *goquery.Document {
		t.Helper()
		status, doc := loadCompareDoc(t, raw, krampusXLFox34)
		if status != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", raw, status)
		}
		if n := doc.Find(`[data-compare-table]`).Length(); n != 0 {
			t.Errorf("%s rendered %d compare tables, want 0", raw, n)
		}
		if n := doc.Find(`.telemetry-strip`).Length(); n != 0 {
			t.Errorf("%s rendered %d telemetry strips, want 0", raw, n)
		}
		if doc.Find(`#form-error[role="alert"]`).Length() != 1 {
			t.Errorf("%s missing #form-error[role=alert]", raw)
		}
		if doc.Find(`a[data-preset="error-reset"][href="/"]`).Length() != 1 {
			t.Errorf("%s missing error-reset link", raw)
		}
		if doc.Find(`a#preset-defaults[href="/"]`).Length() != 1 {
			t.Errorf("%s missing #preset-defaults link", raw)
		}
		return doc
	}

	for _, raw := range []string{"/?brp=off", "/?brp_x=&brp_y="} {
		doc := assertCommonError(t, raw)
		for _, id := range []string{"#ctrl-brp-x", "#ctrl-brp-y"} {
			if val, _ := doc.Find(id).Attr("value"); val != "" {
				t.Errorf("%s %s value = %q, want empty", raw, id, val)
			}
			if inv, _ := doc.Find(id).Attr("aria-invalid"); inv != "true" {
				t.Errorf("%s %s aria-invalid = %q, want true", raw, id, inv)
			}
		}
	}

	for _, raw := range []string{"/?grip=off", "/?grip_x=&grip_y="} {
		doc := assertCommonError(t, raw)
		for _, id := range []string{"#ctrl-grip-x", "#ctrl-grip-y"} {
			if val, _ := doc.Find(id).Attr("value"); val != "" {
				t.Errorf("%s %s value = %q, want empty", raw, id, val)
			}
			if inv, _ := doc.Find(id).Attr("aria-invalid"); inv != "true" {
				t.Errorf("%s %s aria-invalid = %q, want true", raw, id, inv)
			}
		}
	}

	doc := assertCommonError(t, "/?brp_x=-219")
	if val, _ := doc.Find(`#ctrl-brp-x`).Attr("value"); val != "-219" {
		t.Errorf("brp_x = %q, want -219", val)
	}
	if val, _ := doc.Find(`#ctrl-brp-y`).Attr("value"); val != "" {
		t.Errorf("brp_y = %q, want empty", val)
	}
	if inv, _ := doc.Find(`#ctrl-brp-y`).Attr("aria-invalid"); inv != "true" {
		t.Errorf("brp_y aria-invalid = %q, want true", inv)
	}

	// Invalid global selects
	for _, tc := range []struct {
		raw  string
		sel  string
		want string
	}{
		{"/?post=nope", `#ctrl-post`, "Invalid: nope"},
		{"/?bar=nope", `#ctrl-bar`, "Invalid: nope"},
		{"/?bar_options=some", `#ctrl-bar-options`, "Invalid: some"},
	} {
		doc = assertCommonError(t, tc.raw)
		s := doc.Find(tc.sel)
		if inv, _ := s.Attr("aria-invalid"); inv != "true" {
			t.Errorf("%s %s aria-invalid = %q, want true", tc.raw, tc.sel, inv)
		}
		if txt := strings.TrimSpace(s.Find(`option[selected]`).Text()); txt != tc.want {
			t.Errorf("%s %s selected text = %q, want %q", tc.raw, tc.sel, txt, tc.want)
		}
	}

	// Invalid numeric inputs
	for _, tc := range []struct {
		raw string
		sel string
		val string
	}{
		{"/?top=-1", `#ctrl-top`, "-1"},
		{"/?top_per_frame=abc", `#ctrl-top-per-frame`, "abc"},
	} {
		doc = assertCommonError(t, tc.raw)
		inp := doc.Find(tc.sel)
		if inv, _ := inp.Attr("aria-invalid"); inv != "true" {
			t.Errorf("%s %s aria-invalid = %q, want true", tc.raw, tc.sel, inv)
		}
		if got, _ := inp.Attr("value"); got != tc.val {
			t.Errorf("%s %s value = %q, want %q", tc.raw, tc.sel, got, tc.val)
		}
	}

	// Unknown key
	doc = assertCommonError(t, "/?bar_option=all")
	if !strings.Contains(doc.Find(`#form-error`).Text(), "bar_option") {
		t.Errorf("unknown key error %q missing bar_option", doc.Find(`#form-error`).Text())
	}

}

func TestCompareToBikeLosslessEndToEndQ67Q77Q110(t *testing.T) {
	scenarios := []struct {
		raw   string
		frame frameSpec
	}{
		{"", krampusXLFox34},
		{"bar_options=all", krampusXLFox34},
		{"", havokLSklar},
		{"post=setback_25&bar=sqlab311_20_16_50&brp_x=-210&brp_y=720&grip_x=470&grip_y=730", krampusXLFox34},
	}

	for _, sc := range scenarios {
		raw := sc.raw
		t.Run(fmt.Sprintf("%s/%s", sc.frame.frame, raw), func(t *testing.T) {
			status, cmpDoc := loadCompareDoc(t, "/?"+raw, sc.frame)
			if status != http.StatusOK {
				t.Fatalf("GET /?%s status = %d, want 200", raw, status)
			}
			row1 := cmpDoc.Find(`tr[data-compare-rank="1"]`)
			inspectHref, ok := row1.Find(`a[data-compare-inspect]`).Attr("href")
			if !ok || inspectHref == "" {
				t.Fatalf("missing inspect link")
			}

			bikeDoc := loadBikeDoc(t, inspectHref)

			cmpTier, _ := row1.Find(`[data-fit-tier]`).Attr("data-fit-tier")
			bikeTier, _ := bikeDoc.Find(`[data-submitted-tier]`).Attr("data-submitted-tier")
			if bikeTier != cmpTier {
				t.Errorf("submitted tier = %q, want %q", bikeTier, cmpTier)
			}
			bd1 := cmpDoc.Find(`tr[data-breakdown-rank="1"]`)
			if got, want := strings.TrimSpace(bikeDoc.Find(`[data-metric="submitted-score"]`).Text()), strings.TrimSpace(row1.Find(`[data-compare-cell="score"]`).Text()); got != want {
				t.Errorf("submitted-score = %q, want %q", got, want)
			}
			for _, pair := range []struct {
				bikeMetric string
				bdCell     string
			}{
				{"submitted-oval-r", "oval-r"},
				{"submitted-brp-miss", "brp-miss"},
			} {
				got := strings.TrimSpace(bikeDoc.Find(fmt.Sprintf(`[data-metric="%s"]`, pair.bikeMetric)).Text())
				want := strings.TrimSpace(bd1.Find(fmt.Sprintf(`[data-breakdown-cell="%s"]`, pair.bdCell)).Text())
				if got != want {
					t.Errorf("%s = %q, want %q", pair.bikeMetric, got, want)
				}
			}

			// All 12 attribute scores match breakdown row #1.
			if bikeDoc.Find(`[data-submitted-attr]`).Length() != 12 {
				t.Errorf("bike submitted attrs = %d, want 12", bikeDoc.Find(`[data-submitted-attr]`).Length())
			}
			bd1.Find(`[data-breakdown-attr]`).Each(func(_ int, s *goquery.Selection) {
				attr, _ := s.Attr("data-breakdown-attr")
				want := strings.TrimSpace(s.Text())
				got := strings.TrimSpace(bikeDoc.Find(fmt.Sprintf(`[data-submitted-attr="%s"]`, attr)).Text())
				if got != want {
					t.Errorf("attr %s = %q, want %q", attr, got, want)
				}
			})

			// Ranked builds row #1 on /bike also matches / row #1.
			build1 := bikeDoc.Find(`tr[data-build-rank="1"]`)
			if build1.Length() != 1 {
				t.Fatalf("missing tr[data-build-rank=1] on /bike")
			}
			if got, _ := build1.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != cmpTier {
				t.Errorf("build #1 tier = %q, want %q", got, cmpTier)
			}
			if got := strings.TrimSpace(build1.Find(`[data-build-cell="score"]`).Text()); got != strings.TrimSpace(row1.Find(`[data-compare-cell="score"]`).Text()) {
				t.Errorf("build #1 score = %q, want %q", got, row1.Find(`[data-compare-cell="score"]`).Text())
			}
			if got := strings.TrimSpace(build1.Find(`[data-build-cell="stem"]`).Text()); got != strings.TrimSpace(row1.Find(`[data-compare-cell="stem"]`).Text()) {
				t.Errorf("build #1 stem = %q, want %q", got, row1.Find(`[data-compare-cell="stem"]`).Text())
			}
			if got := strings.TrimSpace(build1.Find(`[data-build-cell="spacers"]`).Text()); got != strings.TrimSpace(row1.Find(`[data-compare-cell="spacers"]`).Text()) {
				t.Errorf("build #1 spacers = %q, want %q", got, row1.Find(`[data-compare-cell="spacers"]`).Text())
			}
			if strings.Contains(raw, "bar_options=all") {
				parsedInspect, _ := url.Parse(inspectHref)
				if got := parsedInspect.Query().Get("bar_options"); got != "all" {
					t.Errorf("inspect bar_options = %q, want all", got)
				}
				if got := strings.TrimSpace(build1.Find(`[data-build-cell="bar"]`).Text()); got != strings.TrimSpace(row1.Find(`[data-compare-cell="bar"]`).Text()) {
					t.Errorf("build #1 bar = %q, want %q", got, row1.Find(`[data-compare-cell="bar"]`).Text())
				}
			}
		})
	}
}

func TestCompareHTMLAccessibilityAndNoJS(t *testing.T) {
	for _, tc := range []struct {
		rawPath    string
		wantStatus int
	}{
		{"/", http.StatusOK},
		{"/?bar_options=all", http.StatusOK},
		{"/?brp=off", http.StatusBadRequest},
	} {
		t.Run(tc.rawPath, func(t *testing.T) {
			status, doc := loadCompareDoc(t, tc.rawPath, fullCatalogWinners...)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d", status, tc.wantStatus)
			}
			if n := doc.Find("script").Length(); n != 0 {
				t.Errorf("found %d <script> tags, want 0", n)
			}
			if n := doc.Find("[style]").Length(); n != 0 {
				t.Errorf("found %d [style] attributes, want 0", n)
			}
			for _, attr := range []string{"onclick", "onchange", "onsubmit"} {
				if n := doc.Find(fmt.Sprintf("[%s]", attr)).Length(); n != 0 {
					t.Errorf("found %d [%s] attributes, want 0", n, attr)
				}
			}
			doc.Find("form.controls fieldset").Each(func(i int, fs *goquery.Selection) {
				leg := fs.ChildrenFiltered("legend")
				if leg.Length() != 1 || strings.TrimSpace(leg.Text()) == "" {
					t.Errorf("fieldset %d missing non-empty direct <legend>", i)
				}
			})
			doc.Find("[aria-describedby]").Each(func(i int, el *goquery.Selection) {
				rawIDs, _ := el.Attr("aria-describedby")
				for id := range strings.FieldsSeq(rawIDs) {
					if doc.Find("#"+id).Length() == 0 {
						t.Errorf("element %d references missing aria-describedby #%s", i, id)
					}
				}
			})
			doc.Find("table.data-table").Each(func(i int, tbl *goquery.Selection) {
				capSel := tbl.Find("caption")
				if capSel.Length() != 1 || strings.TrimSpace(capSel.Text()) == "" {
					t.Errorf("table %d missing non-empty <caption>", i)
				}
				tbl.Find("thead th").Each(func(j int, th *goquery.Selection) {
					if scope, _ := th.Attr("scope"); scope != "col" {
						t.Errorf("table %d th %d scope = %q, want col", i, j, scope)
					}
					if strings.TrimSpace(th.Text()) == "" {
						t.Errorf("table %d th %d is empty", i, j)
					}
				})
			})
			doc.Find(".table-scroll").Each(func(i int, sc *goquery.Selection) {
				if ti, _ := sc.Attr("tabindex"); ti != "0" {
					t.Errorf(".table-scroll %d tabindex = %q, want 0", i, ti)
				}
				if role, _ := sc.Attr("role"); role != "region" {
					t.Errorf(".table-scroll %d role = %q, want region", i, role)
				}
				lb, _ := sc.Attr("aria-labelledby")
				lbl, _ := sc.Attr("aria-label")
				if strings.TrimSpace(lb) == "" && strings.TrimSpace(lbl) == "" {
					t.Errorf(".table-scroll %d missing aria-labelledby or aria-label", i)
				}
			})
			if n := doc.Find("details summary").Find("h1, h2, h3, h4, h5, h6").Length(); n != 0 {
				t.Errorf("found %d heading(s) inside details summary, want 0", n)
			}
		})
	}
}

func TestFormatPlacedOffset(t *testing.T) {
	for _, tc := range []struct {
		missX, missY float64
		want         string
	}{
		{0, 0, "On target"},
		{0.04, -0.04, "On target"},
		{-5.3, 0, "5.3 mm fwd"},
		{4.2, 0, "4.2 mm back"},
		{0, -2.0, "2.0 mm high"},
		{0, 6.5, "6.5 mm low"},
		{-10.0, -2.0, "10.0 mm fwd, 2.0 mm high"},
		{8.8, -3.6, "8.8 mm back, 3.6 mm high"},
		{27.1, 13.9, "27.1 mm back, 13.9 mm low"},
		{-1.5, 3.2, "1.5 mm fwd, 3.2 mm low"},
	} {
		if got := formatPlacedOffset(tc.missX, tc.missY); got != tc.want {
			t.Errorf("formatPlacedOffset(%v, %v) = %q, want %q", tc.missX, tc.missY, got, tc.want)
		}
	}
}

func TestBrpFitNote(t *testing.T) {
	for _, tc := range []struct {
		name         string
		railOffset   float64
		insertion    float64
		missX, missY float64
		want         string
	}{
		{"on target", 0, 165, 0, 0, ""},
		{"sub-threshold noise", 15, 165, -0.02, 0.01, ""},
		{"rail maxed back", 15, 165, -5.3, 0, "rail maxed back"},
		{"rail maxed fwd", -15, 165, 5.3, 0, "rail maxed fwd"},
		{"raised for setback", 15, 165, -10.0, -2.0, "raised for setback"},
		{"lowered for setback", -15, 165, 10.0, 2.0, "lowered for setback"},
		{"post maxed out", 3, 100, 0, 6.0, "post maxed out"},
		{"post bottomed out", 3, 250, 0, -4.0, "post bottomed out"},
		{"rail and post combined", 15, 100, -4.0, 6.0, "rail maxed back, post maxed out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			placement := assembly.Placement{
				Bounds: assembly.BuildBounds{
					SeatpostInsertion: assembly.BoundStatus{Value: tc.insertion, Min: 100, Max: 250},
					SaddleRailOffset:  assembly.BoundStatus{Value: tc.railOffset, Min: -15, Max: 15},
				},
			}
			got := brpFitNote(placement, tc.missX, tc.missY)
			if got != tc.want {
				t.Errorf("brpFitNote = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompareBinaryHavokGateTradeoff(t *testing.T) {
	status, doc := loadCompareDoc(t, "/", frameSpec{"binary-havok", "L", "fox36_sl_120"})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	fit1 := doc.Find(`tr[data-compare-rank="1"]`)
	brp := fit1.Find(`[data-compare-cell="brp-fit"]`)
	if got := strings.TrimSpace(brp.Find(`.fit-primary`).Text()); got != "10.0 mm fwd, 2.0 mm high" {
		t.Errorf("brp-fit primary = %q, want 10.0 mm fwd, 2.0 mm high", got)
	}
	if got := strings.TrimSpace(brp.Find(`.fit-note`).Text()); got != "raised for setback" {
		t.Errorf("brp-fit note = %q, want raised for setback", got)
	}
	if got, _ := brp.Attr("title"); got != "target − placed (-10.0, -2.0) mm" {
		t.Errorf("brp-fit title = %q, want target − placed (-10.0, -2.0) mm", got)
	}
	grip := fit1.Find(`[data-compare-cell="grip-fit"]`)
	if got := strings.TrimSpace(grip.Text()); got != "66.1 mm back, 0.3 mm low" {
		t.Errorf("grip-fit = %q, want 66.1 mm back, 0.3 mm low", got)
	}
	if got, _ := grip.Attr("title"); got != "Oval R 5.95 · target − placed (+66.1, +0.3) mm (66.1 mm)" {
		t.Errorf("grip-fit title = %q, want Oval R 5.95 · target − placed (+66.1, +0.3) mm (66.1 mm)", got)
	}
}

func TestFormatRailPosition(t *testing.T) {
	for _, tc := range []struct {
		offset, limit float64
		want          string
	}{
		{0, 15, "Centered"},
		{5.0, 15, "Centered"},
		{-5.0, 15, "Centered"},
		{5.1, 15, "Shifted back"},
		{14.9, 15, "Shifted back"},
		{-5.1, 15, "Shifted fwd"},
		{-14.9, 15, "Shifted fwd"},
		{15.0, 15, "Max back"},
		{-15.0, 15, "Max fwd"},
	} {
		if got := formatRailPosition(tc.offset, tc.limit); got != tc.want {
			t.Errorf("formatRailPosition(%v, %v) = %q, want %q", tc.offset, tc.limit, got, tc.want)
		}
	}
}
