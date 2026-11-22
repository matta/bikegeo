package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"bikegeo/internal/catalog"
	"github.com/PuerkitoBio/goquery"
)

func loadCompareDoc(t *testing.T, rawPath string) (int, *goquery.Document) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+rawPath, nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)
	doc, err := goquery.NewDocumentFromReader(rec.Body)
	if err != nil {
		t.Fatalf("parse HTML for %s: %v", rawPath, err)
	}
	return rec.Code, doc
}

func TestCompareDefaultAll19Frames(t *testing.T) {
	status, doc := loadCompareDoc(t, "/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	if got := strings.TrimSpace(doc.Find("title").Text()); got != "Compare 18 Frames — bikegeo" {
		t.Errorf("title = %q, want Compare 18 Frames — bikegeo", got)
	}
	if doc.Find(`[data-compare-mode="all"]`).Length() != 1 {
		t.Error("expected [data-compare-mode=all]")
	}
	if got := doc.Find(`[data-compare-mode="all"]`).Text(); !strings.Contains(got, "all 18 catalog frames") {
		t.Errorf("compare-mode note = %q, want substring %q", got, "all 18 catalog frames")
	}
	wantSpeedNote := fmt.Sprintf("All %d handlebars evaluates ~15× more builds (about 2 s per frame).", len(catalog.Handlebars()))
	if got := strings.TrimSpace(doc.Find(`[data-note="bar-options-speed"]`).Text()); got != wantSpeedNote {
		t.Errorf("bar-options-speed note = %q, want %q", got, wantSpeedNote)
	}
	if n := doc.Find(`[data-selection-row]`).Length(); n != 0 {
		t.Errorf("got %d selection rows, want 0", n)
	}
	if n := doc.Find(`#preset-all-frames`).Length(); n != 0 {
		t.Errorf("got %d #preset-all-frames links, want 0", n)
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
	for _, want := range []string{"18", "2889", "2163861"} {
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
		if n := doc.Find(sel).Length(); n != 18 {
			t.Errorf("%s row count = %d, want 18", sel, n)
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
		"oval-r":      "0.95",
		"brp-miss":    "(-5.3, +0.0) mm",
		"stem":        "100 mm, -7°",
		"spacers":     "0 mm",
		"rail":        "+15.0 mm",
		"post-insert": "164.3 mm",
	} {
		if got := strings.TrimSpace(fit1.Find(fmt.Sprintf(`[data-compare-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("row 1 %s = %q, want %q", cell, got, want)
		}
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
	for cell, want := range map[string]string{"score": "76.2", "oval-r": "0.89", "brp-miss": "(-9.6, +0.0) mm"} {
		if got := strings.TrimSpace(fit2.Find(fmt.Sprintf(`[data-compare-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("row 2 %s = %q, want %q", cell, got, want)
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
	if got := strings.TrimSpace(fit3.Find(`[data-compare-cell="oval-r"]`).Text()); got != "2.09" {
		t.Errorf("row 3 oval-r = %q, want 2.09", got)
	}
	if got := strings.TrimSpace(fit3.Find(`[data-compare-cell="brp-miss"]`).Text()); got != "(-9.7, +0.0) mm" {
		t.Errorf("row 3 brp-miss = %q, want (-9.7, +0.0) mm", got)
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

	// Row #6: Seven Expedition 926d (or sliding dropouts check)
	fit6 := doc.Find(`tr[data-compare-rank="6"]`)
	if got := strings.TrimSpace(fit6.Find(`[data-compare-cell="frame"]`).Text()); !strings.Contains(got, "Seven Expedition 926d") {
		t.Errorf("row 6 frame = %q, want Seven Expedition 926d", got)
	}
	if got := strings.TrimSpace(doc.Find(`tr[data-geometry-rank="6"] [data-geometry-cell="chainstay"]`).Text()); got != "453.0" {
		t.Errorf("row 6 chainstay = %q, want 453.0", got)
	}

	// Row #8: Esker Hayduke S3 (sliding dropouts)
	fit8 := doc.Find(`tr[data-compare-rank="8"]`)
	if got, _ := fit8.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_grip" {
		t.Errorf("row 8 tier = %q, want fits_grip", got)
	}
	if got := strings.TrimSpace(fit8.Find(`[data-compare-cell="frame"] abbr.sliding-mark`).Text()); got != "*" {
		t.Errorf("row 8 sliding-mark = %q, want *", got)
	}
	if got := strings.TrimSpace(doc.Find(`tr[data-geometry-rank="8"] [data-geometry-cell="chainstay"]`).Text()); got != "454.0*" {
		t.Errorf("row 8 chainstay = %q, want 454.0*", got)
	}
	if got := strings.TrimSpace(fit8.Find(`[data-compare-cell="score"]`).Text()); got != "81.3" {
		t.Errorf("row 8 score = %q, want 81.3", got)
	}
	if got := strings.TrimSpace(fit8.Find(`[data-compare-cell="brp-miss"]`).Text()); got != "(-37.0, +0.0) mm" {
		t.Errorf("row 8 brp-miss = %q, want (-37.0, +0.0) mm", got)
	}

	// Row #16: Otso Fenrir Stainless XL (Tuning Chip frame, non-sliding)
	fit16 := doc.Find(`tr[data-compare-rank="16"]`)
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
	if got := strings.TrimSpace(fit16.Find(`[data-compare-cell="oval-r"]`).Text()); got != "0.96" {
		t.Errorf("row 16 oval-r = %q, want 0.96", got)
	}
	if got := strings.TrimSpace(doc.Find(`tr[data-geometry-rank="16"] [data-geometry-cell="chainstay"]`).Text()); got != "440.0" {
		t.Errorf("row 16 chainstay = %q, want 440.0", got)
	}

	// Row #18: Salsa Cutthroat 60cm
	fit18 := doc.Find(`tr[data-compare-rank="18"]`)
	if got, _ := fit18.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_neither" {
		t.Errorf("row 18 tier = %q, want fits_neither", got)
	}
	if got := strings.TrimSpace(fit18.Find(`[data-compare-cell="score"]`).Text()); got != "50.3" {
		t.Errorf("row 18 score = %q, want 50.3", got)
	}
	if got := strings.TrimSpace(fit18.Find(`[data-compare-cell="oval-r"]`).Text()); got != "3.45" {
		t.Errorf("row 18 oval-r = %q, want 3.45", got)
	}

	slidingNote := doc.Find(`[data-note="sliding-dropouts"]`).Text()
	for _, name := range []string{
		"Binary Havok", "Binary Kinetik", "Esker Hayduke",
		"Revel El Jefe", "Salsa Fargo", "Sklar PBJ",
	} {
		if !strings.Contains(slidingNote, name) {
			t.Errorf("sliding-dropouts note %q missing %q", slidingNote, name)
		}
	}
	if strings.Contains(slidingNote, "Otso Fenrir Stainless") {
		t.Errorf("sliding-dropouts note %q must not include Otso Fenrir Stainless (Q140)", slidingNote)
	}
}

func TestCompareCustomSelectionsAndAutoTopPerFrame(t *testing.T) {
	// Untouched Add-a-frame form stays in all-frames mode.
	status, doc := loadCompareDoc(t, "/?frame=&size=&fork=&sag=")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if doc.Find(`table[data-table="fit"] tbody tr`).Length() != 18 {
		t.Errorf("expected 18 rows for blank frame query")
	}
	if doc.Find(`[data-selection-row]`).Length() != 0 {
		t.Errorf("expected 0 selection rows for blank frame query")
	}

	// Single frame+size selection expands all forks/sags with auto top_per_frame=0.
	status, doc = loadCompareDoc(t, "/?frame=otso-fenrir-stainless&size=L")
	if status != http.StatusOK {
		t.Fatalf("fenrir status = %d, want 200", status)
	}
	if doc.Find(`[data-selection-row]`).Length() != 1 {
		t.Errorf("fenrir selection rows = %d, want 1", doc.Find(`[data-selection-row]`).Length())
	}
	for _, tbl := range []string{"fit", "geometry", "breakdown"} {
		if n := doc.Find(fmt.Sprintf(`table[data-table="%s"] tbody tr`, tbl)).Length(); n != 32 {
			t.Errorf("fenrir %s rows = %d, want 32", tbl, n)
		}
	}
	if !strings.Contains(doc.Find(`[data-summary="configs"]`).Text(), "23968") {
		t.Errorf("fenrir configs summary %q missing 23968", doc.Find(`[data-summary="configs"]`).Text())
	}
	if val, _ := doc.Find(`#ctrl-top-per-frame`).Attr("value"); val != "" {
		t.Errorf("#ctrl-top-per-frame value = %q, want empty", val)
	}
	if href, _ := doc.Find(`#preset-all-frames`).Attr("href"); href != "/" {
		t.Errorf("#preset-all-frames href = %q, want /", href)
	}
	if href, _ := doc.Find(`a[data-remove-selection="1"]`).Attr("href"); href != "/" {
		t.Errorf("remove-selection 1 href = %q, want /", href)
	}
	fit1 := doc.Find(`tr[data-compare-rank="1"]`)
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="fork"]`).Text()); got != "Salsa Firestarter / Cutthroat Carbon Deluxe 483mm" {
		t.Errorf("fenrir row 1 fork = %q", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="score"]`).Text()); got != "65.7" {
		t.Errorf("fenrir row 1 score = %q, want 65.7", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="oval-r"]`).Text()); got != "1.10" {
		t.Errorf("fenrir row 1 oval-r = %q, want 1.10", got)
	}
	geom1 := doc.Find(`tr[data-geometry-rank="1"]`)
	for cell, want := range map[string]string{"reach": "429.3", "stack": "641.1", "ett": "599.7", "chainstay": "440.0"} {
		if got := strings.TrimSpace(geom1.Find(fmt.Sprintf(`[data-geometry-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("fenrir row 1 %s = %q, want %q", cell, got, want)
		}
	}

	// Multi-selection + Remove + All 18 Frames links.
	status, multi := loadCompareDoc(t, "/?frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20&frame=binary-havok&size=L&fork=sklar_steel_485&sag=&post=setback_25")
	if status != http.StatusOK {
		t.Fatalf("multi status = %d, want 200", status)
	}
	if multi.Find(`[data-selection-row]`).Length() != 2 {
		t.Fatalf("multi selection rows = %d, want 2", multi.Find(`[data-selection-row]`).Length())
	}
	if multi.Find(`table[data-table="fit"] tbody tr`).Length() != 2 {
		t.Fatalf("multi ranked rows = %d, want 2", multi.Find(`table[data-table="fit"] tbody tr`).Length())
	}
	if got, _ := multi.Find(`tr[data-compare-rank="1"]`).Attr("data-frame-id"); got != "surly-krampus" {
		t.Errorf("multi row 1 frame = %q, want surly-krampus", got)
	}
	if got, _ := multi.Find(`tr[data-compare-rank="2"]`).Attr("data-frame-id"); got != "binary-havok" {
		t.Errorf("multi row 2 frame = %q, want binary-havok", got)
	}
	removeHref, _ := multi.Find(`a[data-remove-selection="1"]`).Attr("href")
	if !strings.Contains(removeHref, "binary-havok") || !strings.Contains(removeHref, "post=setback_25") {
		t.Errorf("remove 1 href = %q, want binary-havok and post=setback_25", removeHref)
	}
	rStatus, rDoc := loadCompareDoc(t, removeHref)
	if rStatus != http.StatusOK || rDoc.Find(`table[data-table="fit"] tbody tr`).Length() != 1 {
		t.Errorf("after remove 1: status %d, rows %d", rStatus, rDoc.Find(`table[data-table="fit"] tbody tr`).Length())
	}
	allHref, _ := multi.Find(`#preset-all-frames`).Attr("href")
	if got := strings.TrimSpace(multi.Find("a#preset-all-frames").Text()); got != "All 18 Frames" {
		t.Errorf("a#preset-all-frames text = %q, want %q", got, "All 18 Frames")
	}
	if !strings.Contains(allHref, "post=setback_25") || strings.Contains(allHref, "frame=") || strings.Contains(allHref, "top_per_frame=") {
		t.Errorf("all-frames href = %q", allHref)
	}
	aStatus, aDoc := loadCompareDoc(t, allHref)
	if aStatus != http.StatusOK || aDoc.Find(`table[data-table="fit"] tbody tr`).Length() != 18 {
		t.Errorf("after all-frames: status %d, rows %d", aStatus, aDoc.Find(`table[data-table="fit"] tbody tr`).Length())
	}
	if val, _ := aDoc.Find(`#ctrl-post option[selected]`).Attr("value"); val != "setback_25" {
		t.Errorf("after all-frames: post = %q, want setback_25", val)
	}

	// Custom URL sag 18%
	status, sagDoc := loadCompareDoc(t, "/?frame=surly-krampus&size=XL&fork=fox34_120&sag=18")
	if status != http.StatusOK {
		t.Fatalf("custom sag status = %d, want 200", status)
	}
	if val, _ := sagDoc.Find(`tr[data-selection-row="1"] select[name="sag"] option[selected]`).Attr("value"); val != "18" {
		t.Errorf("custom sag selected value = %q, want 18", val)
	}

	// Explicit limits
	status, limDoc := loadCompareDoc(t, "/?top_per_frame=1&top=3")
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
	status, doc := loadCompareDoc(t, "/?frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20&bar_options=all")
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
	fit1 := doc.Find(`tr[data-compare-rank="1"]`)
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="bar"]`).Text()); got != "SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm)" {
		t.Errorf("bar = %q", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="score"]`).Text()); got != "87.1" {
		t.Errorf("score = %q, want 87.1", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="oval-r"]`).Text()); got != "0.58" {
		t.Errorf("oval-r = %q, want 0.58", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="stem"]`).Text()); got != "100 mm, -7°" {
		t.Errorf("stem = %q, want 100 mm, -7°", got)
	}
	if got := doc.Find(`[data-summary="configs"]`).Text(); !strings.Contains(got, "11235") {
		t.Errorf("configs summary %q missing 11235", got)
	}
	if n := doc.Find(`[data-summary="bar"]`).Length(); n != 0 {
		t.Errorf("got %d [data-summary=bar] when bar_options=all, want 0", n)
	}
}

func TestCompareNonRedshiftPostOmitsCaveat(t *testing.T) {
	status, doc := loadCompareDoc(t, "/?post=setback_0")
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

func TestCompareErrorPagesAndRecoveryQ76Q112(t *testing.T) {
	assertCommonError := func(t *testing.T, raw string) *goquery.Document {
		t.Helper()
		status, doc := loadCompareDoc(t, raw)
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

	// Rigid fork + 20% sag recovery
	doc = assertCommonError(t, "/?frame=surly-krampus&size=XL&fork=surly_krampus_483&sag=20")
	row1 := doc.Find(`tr[data-selection-row="1"]`)
	if !strings.Contains(row1.Text(), "Surly Krampus") {
		t.Errorf("row 1 missing Surly Krampus")
	}
	if val, _ := row1.Find(`select[name="size"] option[selected]`).Attr("value"); val != "XL" {
		t.Errorf("row 1 size = %q, want XL", val)
	}
	if val, _ := row1.Find(`select[name="fork"] option[selected]`).Attr("value"); val != "surly_krampus_483" {
		t.Errorf("row 1 fork = %q, want surly_krampus_483", val)
	}
	if inv, _ := row1.Find(`select[name="sag"]`).Attr("aria-invalid"); inv != "true" {
		t.Errorf("row 1 sag aria-invalid = %q, want true", inv)
	}
	if val, _ := row1.Find(`select[name="sag"] option[selected]`).Attr("value"); val != "20" {
		t.Errorf("row 1 sag selected = %q, want 20", val)
	}
	if href, _ := row1.Find(`a[data-remove-selection="1"]`).Attr("href"); href != "/" {
		t.Errorf("row 1 remove href = %q, want /", href)
	}
	if rStatus, _ := loadCompareDoc(t, "/?frame=surly-krampus&size=XL&fork=surly_krampus_483&sag="); rStatus != http.StatusOK {
		t.Errorf("recovered rigid fork status = %d, want 200", rStatus)
	}

	// Any fork + 20% sag
	doc = assertCommonError(t, "/?frame=surly-krampus&size=XL&fork=&sag=20")
	row1 = doc.Find(`tr[data-selection-row="1"]`)
	if inv, _ := row1.Find(`select[name="sag"]`).Attr("aria-invalid"); inv != "true" {
		t.Errorf("any-fork sag aria-invalid = %q, want true", inv)
	}
	if val, _ := row1.Find(`select[name="sag"] option[selected]`).Attr("value"); val != "20" {
		t.Errorf("any-fork sag selected = %q, want 20", val)
	}

	// Invalid size
	doc = assertCommonError(t, "/?frame=surly-krampus&size=XXL")
	sel := doc.Find(`tr[data-selection-row="1"] select[name="size"]`)
	if inv, _ := sel.Attr("aria-invalid"); inv != "true" {
		t.Errorf("invalid size aria-invalid = %q, want true", inv)
	}
	if val, _ := sel.Find(`option[selected]`).Attr("value"); val != "XXL" {
		t.Errorf("invalid size option value = %q, want XXL", val)
	}
	if txt := strings.TrimSpace(sel.Find(`option[selected]`).Text()); txt != "Invalid: XXL" {
		t.Errorf("invalid size option text = %q, want Invalid: XXL", txt)
	}

	// Invalid fork (a catalog fork whose 130 mm class the frame does not unlock, plus deleted fox36_150)
	for _, badFork := range []string{"fox34_130", "fox36_150"} {
		doc = assertCommonError(t, "/?frame=surly-krampus&fork="+badFork)
		sel = doc.Find(`tr[data-selection-row="1"] select[name="fork"]`)
		if inv, _ := sel.Attr("aria-invalid"); inv != "true" {
			t.Errorf("invalid fork %s aria-invalid = %q, want true", badFork, inv)
		}
		if val, _ := sel.Find(`option[selected]`).Attr("value"); val != badFork {
			t.Errorf("invalid fork option value = %q, want %s", val, badFork)
		}
		if txt := strings.TrimSpace(sel.Find(`option[selected]`).Text()); txt != "Invalid: "+badFork {
			t.Errorf("invalid fork option text = %q, want Invalid: %s", txt, badFork)
		}
	}

	// A tapered fork does not fit the Surly ECR's 34 mm bore (Q130).
	assertCommonError(t, "/?frame=surly-ecr&fork=firestarter_deluxe_483")
	// An unlisted compatible fork is accepted (Q128).
	if status, _ := loadCompareDoc(t, "/?frame=surly-krampus&fork=pike_120"); status != http.StatusOK {
		t.Errorf("/?frame=surly-krampus&fork=pike_120 status = %d, want 200", status)
	}

	// Unknown frame
	doc = assertCommonError(t, "/?frame=nope")
	row1 = doc.Find(`tr[data-selection-row="1"]`)
	if row1.Find(`[aria-invalid="true"]`).Length() == 0 {
		t.Error("unknown frame row missing [aria-invalid=true]")
	}
	if href, _ := row1.Find(`a[data-remove-selection="1"]`).Attr("href"); href != "/" {
		t.Errorf("unknown frame remove href = %q, want /", href)
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
		{"/?bar_roll=NaN", `#ctrl-bar-roll`, "NaN"},
		{"/?rail_pitch=Inf", `#ctrl-rail-pitch`, "Inf"},
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

	// Misaligned columns
	doc = assertCommonError(t, "/?frame=surly-krampus&size=XL&size=L")
	if !strings.Contains(doc.Find(`#form-error`).Text(), "must match") {
		t.Errorf("misaligned error %q missing 'must match'", doc.Find(`#form-error`).Text())
	}
	if n := doc.Find(`[data-selection-row]`).Length(); n != 0 {
		t.Errorf("misaligned query rendered %d selection rows, want 0", n)
	}
}

func TestCompareToBikeLosslessEndToEndQ67Q77Q110(t *testing.T) {
	scenarios := []string{
		"frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20",
		"frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20&bar_options=all",
		"frame=binary-havok&size=L&fork=sklar_steel_485",
		"frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20&post=setback_25&bar=sqlab311_20_16_50&bar_roll=10&rail_pitch=-2&brp_x=-210&brp_y=720&grip_x=470&grip_y=730",
	}

	for _, raw := range scenarios {
		t.Run(raw, func(t *testing.T) {
			status, cmpDoc := loadCompareDoc(t, "/?"+raw)
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
			for _, pair := range []struct {
				bikeMetric string
				cmpCell    string
			}{
				{"submitted-score", "score"},
				{"submitted-oval-r", "oval-r"},
				{"submitted-brp-miss", "brp-miss"},
			} {
				got := strings.TrimSpace(bikeDoc.Find(fmt.Sprintf(`[data-metric="%s"]`, pair.bikeMetric)).Text())
				want := strings.TrimSpace(row1.Find(fmt.Sprintf(`[data-compare-cell="%s"]`, pair.cmpCell)).Text())
				if got != want {
					t.Errorf("%s = %q, want %q", pair.bikeMetric, got, want)
				}
			}

			// All 12 attribute scores match breakdown row #1.
			bd1 := cmpDoc.Find(`tr[data-breakdown-rank="1"]`)
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
		{"/?frame=surly-krampus", http.StatusOK},
		{"/?brp=off", http.StatusBadRequest},
	} {
		t.Run(tc.rawPath, func(t *testing.T) {
			status, doc := loadCompareDoc(t, tc.rawPath)
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
