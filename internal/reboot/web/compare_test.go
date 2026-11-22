package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func loadCompareDoc(t *testing.T, rawPath string) (int, *goquery.Document) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081"+rawPath, nil)
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
	if got := strings.TrimSpace(doc.Find("title").Text()); got != "Compare 19 Frames — bikegeo-reboot" {
		t.Errorf("title = %q, want Compare 19 Frames — bikegeo-reboot", got)
	}
	if doc.Find(`[data-compare-mode="all"]`).Length() != 1 {
		t.Error("expected [data-compare-mode=all]")
	}
	if n := doc.Find(`[data-selection-row]`).Length(); n != 0 {
		t.Errorf("got %d selection rows, want 0", n)
	}
	if n := doc.Find(`#preset-all-frames`).Length(); n != 0 {
		t.Errorf("got %d #preset-all-frames links, want 0", n)
	}
	if n := doc.Find(`[data-note="redshift-mixed-state"]`).Length(); n != 1 {
		t.Errorf("got %d redshift-mixed-state notes, want 1", n)
	}
	if n := doc.Find(`[data-fit-tier-legend] dt`).Length(); n != 4 {
		t.Errorf("got %d tier legend items, want 4", n)
	}
	if href, _ := doc.Find(`a#preset-defaults`).Attr("href"); href != "/" {
		t.Errorf("a#preset-defaults href = %q, want /", href)
	}

	configsText := doc.Find(`[data-summary="configs"]`).Text()
	for _, want := range []string{"19", "701", "525049"} {
		if !strings.Contains(configsText, want) {
			t.Errorf("configs summary %q missing %q", configsText, want)
		}
	}
	if postText := doc.Find(`[data-summary="post"]`).Text(); !strings.Contains(postText, "Redshift ShockStop PRO") {
		t.Errorf("post summary %q missing Redshift ShockStop PRO", postText)
	}
	if barText := doc.Find(`[data-summary="bar"]`).Text(); !strings.Contains(barText, "Persuader") {
		t.Errorf("bar summary %q missing Persuader", barText)
	}

	for _, tableName := range []string{"fit", "geometry", "breakdown"} {
		sel := fmt.Sprintf(`table[data-table="%s"] tbody tr`, tableName)
		if n := doc.Find(sel).Length(); n != 19 {
			t.Errorf("%s row count = %d, want 19", sel, n)
		}
	}

	// Row #1: Surly Krampus XL, Fox Float 34 120mm, 20% sag.
	fit1 := doc.Find(`tr[data-compare-rank="1"]`)
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="frame"]`).Text()); got != "Surly Krampus" {
		t.Errorf("row 1 frame = %q, want Surly Krampus", got)
	}
	if fit1.Find(`abbr.sliding-mark`).Length() != 0 {
		t.Error("row 1 should not have sliding-mark")
	}
	for cell, want := range map[string]string{
		"size":        "XL",
		"fork":        "Fox Float 34 120mm, 20% sag",
		"score":       "84.6",
		"oval-r":      "0.76",
		"brp-miss":    "(-4.1, +0.0) mm",
		"stem":        "100 mm, -10°",
		"spacers":     "0 mm",
		"rail":        "+15.0 mm",
		"post-insert": "163.9 mm",
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
		"reach":     "457.2",
		"stack":     "630.5",
		"ett":       "664.1",
		"hta":       "67.8°",
		"sta":       "71.8°",
		"chainstay": "435.0",
		"wheelbase": "1170.1",
		"rc-wb":     "36.9%",
		"bb-drop":   "56.2",
		"trail":     "104.1",
	} {
		if got := strings.TrimSpace(geom1.Find(fmt.Sprintf(`[data-geometry-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("row 1 geom %s = %q, want %q", cell, got, want)
		}
	}

	bd1 := doc.Find(`tr[data-breakdown-rank="1"]`)
	for attr, want := range map[string]string{
		"HandGrip":           "92.4",
		"Saddle BRP Y":       "100.0",
		"Saddle BRP X":       "82.7",
		"Stem":               "62.5",
		"Handlebar sweep":    "100.0",
		"Fork sag":           "100.0",
		"Spacers":            "100.0",
		"Saddle rail offset": "0.0",
		"RC / WB ratio":      "95.5",
		"Seatpost insertion": "100.0",
		"Leveled BB drop":    "56.1",
		"Trail":              "100.0",
	} {
		if got := strings.TrimSpace(bd1.Find(fmt.Sprintf(`[data-breakdown-attr="%s"]`, attr)).Text()); got != want {
			t.Errorf("row 1 breakdown %s = %q, want %q", attr, got, want)
		}
	}

	// Row #2: Seven Expedition 926F S 64cm
	fit2 := doc.Find(`tr[data-compare-rank="2"]`)
	if got, _ := fit2.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_brp" {
		t.Errorf("row 2 tier = %q, want fits_brp", got)
	}
	if got := strings.TrimSpace(fit2.Find(`[data-compare-cell="score"]`).Text()); got != "46.9" {
		t.Errorf("row 2 score = %q, want 46.9", got)
	}
	if got := strings.TrimSpace(fit2.Find(`[data-compare-cell="oval-r"]`).Text()); got != "1.99" {
		t.Errorf("row 2 oval-r = %q, want 1.99", got)
	}
	if got := strings.TrimSpace(fit2.Find(`[data-compare-cell="brp-miss"]`).Text()); got != "(-10.0, -1.3) mm" {
		t.Errorf("row 2 brp-miss = %q, want (-10.0, -1.3) mm", got)
	}
	geom2 := doc.Find(`tr[data-geometry-rank="2"]`)
	if got := strings.TrimSpace(geom2.Find(`[data-geometry-cell="reach"]`).Text()); got != "425.0" {
		t.Errorf("row 2 reach = %q, want 425.0", got)
	}
	if got := strings.TrimSpace(geom2.Find(`[data-geometry-cell="stack"]`).Text()); got != "678.1" {
		t.Errorf("row 2 stack = %q, want 678.1", got)
	}
	if got := strings.TrimSpace(geom2.Find(`[data-geometry-cell="ett"]`).Text()); got != "641.1" {
		t.Errorf("row 2 ett = %q, want 641.1", got)
	}

	// Row #5: Salsa Fargo XL (sliding dropouts)
	fit5 := doc.Find(`tr[data-compare-rank="5"]`)
	if got := strings.TrimSpace(fit5.Find(`[data-compare-cell="frame"]`).Text()); !strings.Contains(got, "Salsa Fargo") {
		t.Errorf("row 5 frame = %q, want Salsa Fargo", got)
	}
	if got := strings.TrimSpace(fit5.Find(`[data-compare-cell="frame"] abbr.sliding-mark`).Text()); got != "*" {
		t.Errorf("row 5 sliding-mark = %q, want *", got)
	}
	if got := strings.TrimSpace(doc.Find(`tr[data-geometry-rank="5"] [data-geometry-cell="chainstay"]`).Text()); got != "462.0*" {
		t.Errorf("row 5 chainstay = %q, want 462.0*", got)
	}

	// Row #6: Sour Pasta Party L (-0.0 normalized to +0.0)
	fit6 := doc.Find(`tr[data-compare-rank="6"]`)
	if got, _ := fit6.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_grip" {
		t.Errorf("row 6 tier = %q, want fits_grip", got)
	}
	if got := strings.TrimSpace(fit6.Find(`[data-compare-cell="score"]`).Text()); got != "80.9" {
		t.Errorf("row 6 score = %q, want 80.9", got)
	}
	if got := strings.TrimSpace(fit6.Find(`[data-compare-cell="brp-miss"]`).Text()); got != "(-45.9, +0.0) mm" {
		t.Errorf("row 6 brp-miss = %q, want (-45.9, +0.0) mm", got)
	}

	// Row #19: Salsa Cutthroat 60cm
	fit19 := doc.Find(`tr[data-compare-rank="19"]`)
	if got, _ := fit19.Find(`[data-fit-tier]`).Attr("data-fit-tier"); got != "fits_neither" {
		t.Errorf("row 19 tier = %q, want fits_neither", got)
	}
	if got := strings.TrimSpace(fit19.Find(`[data-compare-cell="score"]`).Text()); got != "45.9" {
		t.Errorf("row 19 score = %q, want 45.9", got)
	}
	if got := strings.TrimSpace(fit19.Find(`[data-compare-cell="oval-r"]`).Text()); got != "3.27" {
		t.Errorf("row 19 oval-r = %q, want 3.27", got)
	}

	slidingNote := doc.Find(`[data-note="sliding-dropouts"]`).Text()
	for _, name := range []string{
		"Binary Havok", "Binary Kinetik", "Esker Hayduke",
		"Otso Fenrir Stainless", "Revel El Jefe", "Salsa Fargo", "Sklar PBJ",
	} {
		if !strings.Contains(slidingNote, name) {
			t.Errorf("sliding-dropouts note %q missing %q", slidingNote, name)
		}
	}
}

func TestCompareCustomSelectionsAndAutoTopPerFrame(t *testing.T) {
	// Untouched Add-a-frame form stays in all-frames mode.
	status, doc := loadCompareDoc(t, "/?frame=&size=&fork=&sag=")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if doc.Find(`table[data-table="fit"] tbody tr`).Length() != 19 {
		t.Errorf("expected 19 rows for blank frame query")
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
		if n := doc.Find(fmt.Sprintf(`table[data-table="%s"] tbody tr`, tbl)).Length(); n != 7 {
			t.Errorf("fenrir %s rows = %d, want 7", tbl, n)
		}
	}
	if !strings.Contains(doc.Find(`[data-summary="configs"]`).Text(), "5243") {
		t.Errorf("fenrir configs summary %q missing 5243", doc.Find(`[data-summary="configs"]`).Text())
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
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="fork"]`).Text()); got != "RockShox SID 100mm, 25% sag" {
		t.Errorf("fenrir row 1 fork = %q", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="score"]`).Text()); got != "58.8" {
		t.Errorf("fenrir row 1 score = %q, want 58.8", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="oval-r"]`).Text()); got != "1.32" {
		t.Errorf("fenrir row 1 oval-r = %q, want 1.32", got)
	}
	geom1 := doc.Find(`tr[data-geometry-rank="1"]`)
	for cell, want := range map[string]string{"reach": "427.1", "stack": "642.6", "ett": "597.2", "chainstay": "450.0*"} {
		if got := strings.TrimSpace(geom1.Find(fmt.Sprintf(`[data-geometry-cell="%s"]`, cell)).Text()); got != want {
			t.Errorf("fenrir row 1 %s = %q, want %q", cell, got, want)
		}
	}

	// Multi-selection + Remove + All 19 Frames links.
	status, multi := loadCompareDoc(t, "/?frame=surly-krampus&size=XL&fork=fox34_120&sag=20&frame=binary-havok&size=L&fork=rigid_485&sag=&post=setback_25")
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
	if !strings.Contains(allHref, "post=setback_25") || strings.Contains(allHref, "frame=") || strings.Contains(allHref, "top_per_frame=") {
		t.Errorf("all-frames href = %q", allHref)
	}
	aStatus, aDoc := loadCompareDoc(t, allHref)
	if aStatus != http.StatusOK || aDoc.Find(`table[data-table="fit"] tbody tr`).Length() != 19 {
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
	status, doc := loadCompareDoc(t, "/?frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all")
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
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="bar"]`).Text()); got != "SQlab 311 FL-X Carbon 16°, 30 mm rise (740 mm)" {
		t.Errorf("bar = %q", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="score"]`).Text()); got != "86.7" {
		t.Errorf("score = %q, want 86.7", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="oval-r"]`).Text()); got != "0.39" {
		t.Errorf("oval-r = %q, want 0.39", got)
	}
	if got := strings.TrimSpace(fit1.Find(`[data-compare-cell="stem"]`).Text()); got != "100 mm, -7°" {
		t.Errorf("stem = %q, want 100 mm, -7°", got)
	}
	if got := doc.Find(`[data-summary="configs"]`).Text(); !strings.Contains(got, "11235") {
		t.Errorf("configs summary %q missing 11235", got)
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

	// Invalid fork
	doc = assertCommonError(t, "/?frame=surly-krampus&fork=fox36_150")
	sel = doc.Find(`tr[data-selection-row="1"] select[name="fork"]`)
	if inv, _ := sel.Attr("aria-invalid"); inv != "true" {
		t.Errorf("invalid fork aria-invalid = %q, want true", inv)
	}
	if val, _ := sel.Find(`option[selected]`).Attr("value"); val != "fox36_150" {
		t.Errorf("invalid fork option value = %q, want fox36_150", val)
	}
	if txt := strings.TrimSpace(sel.Find(`option[selected]`).Text()); txt != "Invalid: fox36_150" {
		t.Errorf("invalid fork option text = %q, want Invalid: fox36_150", txt)
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
		"frame=surly-krampus&size=XL&fork=fox34_120&sag=20",
		"frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all",
		"frame=binary-havok&size=L&fork=rigid_485",
		"frame=surly-krampus&size=XL&fork=fox34_120&sag=20&post=setback_25&bar=sqlab311_20_16_50&bar_roll=10&rail_pitch=-2&brp_x=-210&brp_y=720&grip_x=470&grip_y=730",
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
	for _, rawPath := range []string{"/", "/?frame=surly-krampus"} {
		t.Run(rawPath, func(t *testing.T) {
			status, doc := loadCompareDoc(t, rawPath)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200", status)
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
