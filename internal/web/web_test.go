package web

import (
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"bikegeo/internal/assembly"
	"bikegeo/internal/catalog"
	"bikegeo/internal/query"
)

// mustBike parses a /bike query string.
func mustBike(t *testing.T, raw string) query.Bike {
	t.Helper()
	q, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", raw, err)
	}
	return mustBikeValues(t, q)
}

// mustBikeValues parses /bike query values.
func mustBikeValues(t *testing.T, q url.Values) query.Bike {
	t.Helper()
	b, err := query.ParseBike(q)
	if err != nil {
		t.Fatalf("ParseBike(%v): %v", q, err)
	}
	return b
}

// mustPlace places a parsed bike.
func mustPlace(t *testing.T, b query.Bike) assembly.BikePlacement {
	t.Helper()
	placed, err := assembly.PlaceBike(b.Setup)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}
	return placed
}

// legacyBuild is the build earlier slices defaulted to on /bike: the generic
// zero-setback post at its Q92 insertion, with both fit targets cleared (Q70).
// Tests that pin numbers for that build pass it explicitly.
const legacyBuild = legacyPost + "&brp=off&grip=off"

// legacyPost is the zero-setback post at its Q92 insertion.
const legacyPost = "post=setback_0&post_insert=180.576"

func TestRootServesComparePageAndNavLinks(t *testing.T) {
	status, rootDoc := loadCompareDoc(t, "/", krampusXLFox34)
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	if title := rootDoc.Find("h1").Text(); !strings.Contains(title, "bikegeo") {
		t.Errorf("root h1 = %q, want bikegeo branding", title)
	}
	rootCurrent := rootDoc.Find(`nav.site-nav a[aria-current="page"]`)
	if rootCurrent.Length() != 1 || strings.TrimSpace(rootCurrent.Text()) != "Compare" {
		t.Errorf("root current nav = %d %q, want 1 Compare", rootCurrent.Length(), rootCurrent.Text())
	}
	if href, _ := rootCurrent.Attr("href"); href != "/" {
		t.Errorf("root current nav href = %q, want /", href)
	}
	rootBikeLink := rootDoc.Find(`nav.site-nav a[href="/bike"]`)
	if rootBikeLink.Length() != 1 || strings.TrimSpace(rootBikeLink.Text()) != "Bike Inspector" {
		t.Errorf("root /bike nav = %d %q, want 1 Bike Inspector", rootBikeLink.Length(), rootBikeLink.Text())
	}

	bikeDoc := loadBikeDoc(t, "/bike")
	if title := bikeDoc.Find("h1").Text(); !strings.Contains(title, "bikegeo") {
		t.Errorf("bike h1 = %q, want bikegeo branding", title)
	}
	bikeCurrent := bikeDoc.Find(`nav.site-nav a[aria-current="page"]`)
	if bikeCurrent.Length() != 1 || strings.TrimSpace(bikeCurrent.Text()) != "Bike Inspector" {
		t.Errorf("bike current nav = %d %q, want 1 Bike Inspector", bikeCurrent.Length(), bikeCurrent.Text())
	}
	if href, _ := bikeCurrent.Attr("href"); href != "/bike" {
		t.Errorf("bike current nav href = %q, want /bike", href)
	}
	bikeCompareLink := bikeDoc.Find(`nav.site-nav a[href="/"]`)
	if bikeCompareLink.Length() != 1 || strings.TrimSpace(bikeCompareLink.Text()) != "Compare" {
		t.Errorf("bike / nav = %d %q, want 1 Compare", bikeCompareLink.Length(), bikeCompareLink.Text())
	}
}

func TestStemCloseUpDrawsStemBodyFromBarClamp(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?view=stem")
	bike := mustPlace(t, query.DefaultBike())
	stemBody := doc.Find(`svg.diagram rect[data-component="stem-body"]`)
	if stemBody.Length() != 1 {
		t.Fatalf("expected 1 stem-body rect, got %d", stemBody.Length())
	}
	if x, _ := stemBody.Attr("x"); x != "-60.00" {
		t.Errorf("stem-body x = %q, want -60.00", x)
	}
	clamp := bike.Stem.BarClamp
	wantTransform := fmt.Sprintf("translate(%.2f %.2f) rotate(%.2f)",
		clamp.Origin().X, -clamp.Origin().Y, -float64(clamp.HeadingDeg()))
	if tf, _ := stemBody.Attr("transform"); tf != wantTransform {
		t.Errorf("stem-body transform = %q, want %q", tf, wantTransform)
	}
}

func TestUnknownPathReturnsNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/nope", nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /nope status = %d, want 404", rec.Code)
	}
}

func TestBikeWorkbenchRendersAndRespondsToForkQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/bike?fork=fox34_120-534x44&cup=12&sag=20", nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /bike status = %d, want 200", rec.Code)
	}

	doc, err := goquery.NewDocumentFromReader(rec.Body)
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	if got := strings.TrimSpace(doc.Find(`[data-metric="effective-a2c"]`).Text()); got != "510.0 mm" {
		t.Errorf("effective-a2c = %q, want 510.0 mm", got)
	}

	for _, name := range []string{
		"RearAxle", "RearContact", "BB", "SeatTubeTop",
		"RailPivot", "RailCenter", "BRP", "SitBones",
		"HeadTubeTop", "CoverTop", "SpacerTop", "SteererClamp", "BarClamp", "HandGrip",
		"HeadTubeBottom", "LowerCupBottom", "FrontAxle", "FrontContact",
	} {
		sel := fmt.Sprintf(`svg.diagram [data-landmark="%s"]`, name)
		if doc.Find(sel).Length() != 1 {
			t.Errorf("expected 1 element matching %s, got %d", sel, doc.Find(sel).Length())
		}
	}

	for _, comp := range []string{"upper-cover", "spacers", "steerer-clamp", "stem-body", "seatpost-clamp", "saddle-rail"} {
		sel := fmt.Sprintf(`svg.diagram rect[data-component="%s"]`, comp)
		if doc.Find(sel).Length() != 1 {
			t.Errorf("expected 1 element matching %s, got %d", sel, doc.Find(sel).Length())
		}
	}

	// Verify the ground line matches the placed RearContact/FrontContact landmarks from assembly.
	parsed := mustBike(t, "fork=fox34_120-534x44&cup=12&sag=20")
	bike := mustPlace(t, parsed)
	placed := bike.Frameset
	wantGroundY := fmt.Sprintf("%.2f", -placed.RearTireContact.Origin().Y)
	groundLine := doc.Find(`svg.diagram line[data-element="ground-line"]`)
	if groundLine.Length() != 1 {
		t.Fatalf("expected 1 ground-line element, got %d", groundLine.Length())
	}
	if y1, _ := groundLine.Attr("y1"); y1 != wantGroundY {
		t.Errorf("ground-line y1 = %q, want %q", y1, wantGroundY)
	}
	if y2, _ := groundLine.Attr("y2"); y2 != wantGroundY {
		t.Errorf("ground-line y2 = %q, want %q", y2, wantGroundY)
	}

	if got := strings.TrimSpace(doc.Find(`[data-metric="frame-pitch"]`).Text()); got != fmt.Sprintf("%+.2f°", placed.PitchDeg()) {
		t.Errorf("frame-pitch = %q, want %+.2f°", got, placed.PitchDeg())
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="leveled-hta"]`).Text()); got != fmt.Sprintf("%.2f°", placed.LeveledHeadTubeAngleDeg()) {
		t.Errorf("leveled-hta = %q, want %.2f°", got, placed.LeveledHeadTubeAngleDeg())
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="leveled-sta"]`).Text()); got != fmt.Sprintf("%.2f°", placed.LeveledSeatTubeAngleDeg()) {
		t.Errorf("leveled-sta = %q, want %.2f°", got, placed.LeveledSeatTubeAngleDeg())
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="seat-axis-post-top"]`).Text()); got != "665.0 mm" {
		t.Errorf("seat-axis-post-top = %q, want 665.0 mm", got)
	}
	wantSaddleHeight := fmt.Sprintf("%.1f mm", bike.SaddleHeightMm())
	if got := strings.TrimSpace(doc.Find(`[data-metric="brp-height"]`).Text()); got != wantSaddleHeight {
		t.Errorf("brp-height = %q, want %s", got, wantSaddleHeight)
	}
	wantBrpGripDist := fmt.Sprintf("%.1f mm", bike.BrpToGripDistMm())
	if got := strings.TrimSpace(doc.Find(`[data-metric="brp-grip-dist"]`).Text()); got != wantBrpGripDist {
		t.Errorf("brp-grip-dist = %q, want %s", got, wantBrpGripDist)
	}
	wantSitBonesGripRise := fmt.Sprintf("%+.1f mm", bike.SitBonesToGripRiseMm())
	if got := strings.TrimSpace(doc.Find(`[data-metric="sitbones-grip-rise"]`).Text()); got != wantSitBonesGripRise {
		t.Errorf("sitbones-grip-rise = %q, want %s", got, wantSitBonesGripRise)
	}
	wantHandLateral := fmt.Sprintf("%.1f mm", bike.Handlebar.HandLateralMm)
	if got := strings.TrimSpace(doc.Find(`[data-metric="hand-lateral"]`).Text()); got != wantHandLateral {
		t.Errorf("hand-lateral = %q, want %s", got, wantHandLateral)
	}

	basis, err := parsed.BasisFrameset()
	if err != nil {
		t.Fatalf("BasisFrameset: %v", err)
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="delta-pitch"]`).Text()); got != fmt.Sprintf("%+.2f°", placed.PitchDeltaDeg(basis)) {
		t.Errorf("delta-pitch = %q, want %+.2f°", got, placed.PitchDeltaDeg(basis))
	}
	if prevFork, _ := doc.Find(`input[name="prev_fork"]`).Attr("value"); prevFork != "fox34_120-534x44" {
		t.Errorf("prev_fork hidden input = %q, want fox34_120-534x44", prevFork)
	}

	for _, name := range []string{"seat-tube", "seatstay", "saddle-profile", "handlebar-rise-sweep"} {
		sel := fmt.Sprintf(`svg.diagram line[data-segment="%s"][data-illustrative="true"]`, name)
		if doc.Find(sel).Length() != 1 {
			t.Errorf("expected 1 illustrative segment matching %s, got %d", sel, doc.Find(sel).Length())
		}
	}
	if _, hasIll := doc.Find(`svg.diagram line[data-segment="head-tube"]`).Attr("data-illustrative"); hasIll {
		t.Errorf("head-tube should not be marked data-illustrative")
	}

	if n := doc.Find(`script, table.trace-table, [data-step-inspector], [data-step-id], .trace-steps, .step-frames`).Length(); n != 0 {
		t.Errorf("found %d construction-trace or script elements, want none", n)
	}
	for _, sel := range []string{`g[data-element="world-axes"]`, `g.component-outlines`, `form.controls[method="get"] button[type="submit"]`} {
		if doc.Find(sel).Length() != 1 {
			t.Errorf("expected 1 element matching %s, got %d", sel, doc.Find(sel).Length())
		}
	}
	if doc.Find("svg.diagram").Length() != 1 {
		t.Errorf("expected 1 svg.diagram, got %d", doc.Find("svg.diagram").Length())
	}
}

func TestBikeWorkbenchForkSwitchAndBasisFork(t *testing.T) {
	cases := []struct {
		name        string
		query       string
		wantCode    int
		wantA2C     string
		wantForkA2C string
		wantSag     string
		wantCup     string
		wantDelta   string
	}{
		{
			name:        "switch suspension to rigid resets fork_a2c and sag",
			query:       "/bike?prev_fork=fox34_120-534x44&fork=lithic_carbon_mountain-495x44&fork_a2c=534&cup=12&sag=20",
			wantCode:    http.StatusOK,
			wantA2C:     "495.0 mm",
			wantForkA2C: "495.0",
			wantSag:     "0.0",
			wantCup:     "12.0",
		},
		{
			name:        "switch rigid to SID applies SID default A2C and 20% sag",
			query:       "/bike?prev_fork=lithic_carbon_mountain-495x44&fork=sid_100-506x44&fork_a2c=495&cup=12&sag=0",
			wantCode:    http.StatusOK,
			wantA2C:     "486.0 mm",
			wantForkA2C: "506.0",
			wantSag:     "20.0",
			wantCup:     "12.0",
		},
		{
			name:        "staying on SID honors explicit sag=0",
			query:       "/bike?prev_fork=sid_100-506x44&fork=sid_100-506x44&cup=12&sag=0",
			wantCode:    http.StatusOK,
			wantA2C:     "506.0 mm",
			wantForkA2C: "506.0",
			wantSag:     "0.0",
			wantCup:     "12.0",
		},
		{
			name:        "staying on SID honors explicit fork_a2c=520 and sag=25",
			query:       "/bike?prev_fork=sid_100-506x44&fork=sid_100-506x44&fork_a2c=520&cup=12&sag=25",
			wantCode:    http.StatusOK,
			wantA2C:     "495.0 mm",
			wantForkA2C: "520.0",
			wantSag:     "25.0",
			wantCup:     "12.0",
		},
		{
			name:     "direct rigid fork with non-zero sag returns 400",
			query:    "/bike?fork=lithic_carbon_mountain-495x44&sag=20",
			wantCode: http.StatusBadRequest,
		},
		{
			name:        "basis fork preset uses workbench BasisFork and 0 cup",
			query:       query.DefaultBike().WithBasisFork().URL(),
			wantCode:    http.StatusOK,
			wantA2C:     "495.0 mm",
			wantForkA2C: "495.0",
			wantSag:     "0.0",
			wantCup:     "0.0",
			wantDelta:   "+0.00°",
		},
		{
			name:        "switch basis reference to Lithic restores physical 12 mm cup",
			query:       "/bike?prev_fork=basis&fork=lithic_carbon_mountain-495x44&fork_a2c=495&cup=0&sag=0",
			wantCode:    http.StatusOK,
			wantA2C:     "495.0 mm",
			wantForkA2C: "495.0",
			wantSag:     "0.0",
			wantCup:     "12.0",
		},
		{
			name:        "switch basis reference to Fox 34 restores physical 12 mm cup and Fox 34 defaults",
			query:       "/bike?prev_fork=basis&fork=fox34_120-534x44&fork_a2c=495&cup=0&sag=0",
			wantCode:    http.StatusOK,
			wantA2C:     "504.0 mm",
			wantForkA2C: "534.0",
			wantSag:     "25.0",
			wantCup:     "12.0",
		},
		{
			name:        "switch Fox 34 to basis reference adopts 0 mm basis cup and 495/0 fork",
			query:       "/bike?prev_fork=fox34_120-534x44&fork=basis&fork_a2c=534&cup=12&sag=20",
			wantCode:    http.StatusOK,
			wantA2C:     "495.0 mm",
			wantForkA2C: "495.0",
			wantSag:     "0.0",
			wantCup:     "0.0",
			wantDelta:   "+0.00°",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+tc.query, nil)
			rec := httptest.NewRecorder()
			NewHandler().ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("GET %s status = %d, want %d (body: %s)", tc.query, rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantCode != http.StatusOK {
				return
			}
			doc, err := goquery.NewDocumentFromReader(rec.Body)
			if err != nil {
				t.Fatalf("parse HTML: %v", err)
			}
			if got := strings.TrimSpace(doc.Find(`[data-metric="effective-a2c"]`).Text()); got != tc.wantA2C {
				t.Errorf("effective-a2c = %q, want %q", got, tc.wantA2C)
			}
			if got, _ := doc.Find(`input[name="fork_a2c"]`).Attr("value"); got != tc.wantForkA2C {
				t.Errorf("fork_a2c input value = %q, want %q", got, tc.wantForkA2C)
			}
			if got, _ := doc.Find(`input[name="sag"]`).Attr("value"); got != tc.wantSag {
				t.Errorf("sag input value = %q, want %q", got, tc.wantSag)
			}
			if got, _ := doc.Find(`input[name="cup"]`).Attr("value"); got != tc.wantCup {
				t.Errorf("cup input value = %q, want %q", got, tc.wantCup)
			}
			if tc.wantDelta != "" {
				if got := strings.TrimSpace(doc.Find(`[data-metric="delta-pitch"]`).Text()); got != tc.wantDelta {
					t.Errorf("delta-pitch = %q, want %q", got, tc.wantDelta)
				}
				selectedOpt := doc.Find(`select[name="fork"] option[selected]`)
				if val, _ := selectedOpt.Attr("value"); val != query.BasisForkID {
					t.Errorf("basis fork selected fork option = %q, want %q", val, query.BasisForkID)
				}
				if _, disabled := selectedOpt.Attr("disabled"); disabled {
					t.Errorf("basis fork selected fork option should not be disabled")
				}
			}
		})
	}
}

func TestBikeWorkbenchCockpitControlsUpdateBuild(t *testing.T) {
	query := "/bike?fork=sid_100-506x44&prev_fork=sid_100-506x44&fork_a2c=515&cup=10&sag=20" +
		"&spacers=25&stem_length=80&stem_angle=7&bar_roll=5" +
		"&post=setback_25&post_insert=150&rail_offset=-10&rail_pitch=-2"
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+query, nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200 (body: %s)", query, rec.Code, rec.Body.String())
	}
	doc, err := goquery.NewDocumentFromReader(rec.Body)
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	if errs := doc.Find("#form-error").Length(); errs != 0 {
		t.Errorf("expected no #form-error on valid build, got %d", errs)
	}
	if invalid := doc.Find("[aria-invalid]").Length(); invalid != 0 {
		t.Errorf("expected no [aria-invalid] on valid build, got %d", invalid)
	}

	parsedBike := mustBike(t, strings.TrimPrefix(query, "/bike?"))
	forkSetup := parsedBike.Setup.Fork
	bike := mustPlace(t, parsedBike)

	for _, metric := range []struct {
		key  string
		want string
	}{
		{"effective-a2c", fmt.Sprintf("%.1f mm", forkSetup.EffectiveA2cMm())},
		{"frame-pitch", fmt.Sprintf("%+.2f°", bike.Frameset.PitchDeg())},
		{"seat-axis-post-top", fmt.Sprintf("%.1f mm", bike.SeatAxisToPostTopMm())},
		{"brp-height", fmt.Sprintf("%.1f mm", bike.SaddleHeightMm())},
		{"brp-grip-dist", fmt.Sprintf("%.1f mm", bike.BrpToGripDistMm())},
		{"sitbones-grip-rise", fmt.Sprintf("%+.1f mm", bike.SitBonesToGripRiseMm())},
	} {
		got := strings.TrimSpace(doc.Find(fmt.Sprintf(`[data-metric="%s"]`, metric.key)).Text())
		if got != metric.want {
			t.Errorf("metric %s = %q, want %q", metric.key, got, metric.want)
		}
	}

	if w, _ := doc.Find(`svg.diagram rect[data-component="stem-body"]`).Attr("width"); w != "80.00" {
		t.Errorf("stem-body width = %q, want 80.00", w)
	}
	if w, _ := doc.Find(`svg.diagram rect[data-component="spacers"]`).Attr("width"); w != "25.00" {
		t.Errorf("spacers width = %q, want 25.00", w)
	}

	for name, want := range map[string]string{
		"prev_fork":   "sid_100-506x44",
		"fork_a2c":    "515.0",
		"cup":         "10.0",
		"sag":         "20.0",
		"spacers":     "25.0",
		"stem_length": "80.0",
		"stem_angle":  "7.0",
		"bar_roll":    "5.0",
		"post_insert": "150.0",
		"rail_offset": "-10.0",
		"rail_pitch":  "-2.0",
	} {
		got, _ := doc.Find(fmt.Sprintf(`input[name="%s"]`, name)).Attr("value")
		if got != want {
			t.Errorf("input[name=%q] value = %q, want %q", name, got, want)
		}
	}
	if got, _ := doc.Find(`select[name="post"] option[selected]`).Attr("value"); got != "setback_25" {
		t.Errorf("select[name=post] selected = %q, want setback_25", got)
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="post-setback"]`).Text()); got != "25.0 mm" {
		t.Errorf("metric post-setback = %q, want 25.0 mm", got)
	}

	labels := doc.Find("form.controls label")
	if labels.Length() != 21 {
		t.Errorf("form label count = %d, want 21", labels.Length())
	}
	labels.Each(func(_ int, lbl *goquery.Selection) {
		forID, ok := lbl.Attr("for")
		if !ok || forID == "" {
			t.Errorf("label %q missing for attribute", strings.TrimSpace(lbl.Text()))
			return
		}
		if doc.Find("#"+forID).Length() != 1 {
			t.Errorf("label for=%q does not match exactly 1 control", forID)
		}
	})
}

func TestBikeWorkbenchRejectsInvalidQuery(t *testing.T) {
	cases := []struct {
		query        string
		invalidField string
		wantVal      string
	}{
		{"/bike?fork=unknown", "fork", ""},
		{"/bike?bar=unknown", "bar", ""},
		{"/bike?bar_options=some", "bar_options", ""},
		{"/bike?fork_options=some", "fork_options", ""},
		{"/bike?cup=-1", "cup", "-1"},
		{"/bike?cup=NaN", "cup", "NaN"},
		{"/bike?sag=150", "sag", "150"},
		{"/bike?sag=Inf", "sag", "Inf"},
		{"/bike?fork_a2c=0", "fork_a2c", "0"},
		{"/bike?fork_a2c=abc", "fork_a2c", "abc"},
		{"/bike?fork=fox34_120-534x44&fork_a2c=50&sag=50", "fork_a2c", "50"},
		{"/bike?fork=fox34_120-534x44&fork_a2c=50&sag=50&cup=-1", "fork_a2c", "50"},
		{"/bike?spacers=-1", "spacers", "-1"},
		{"/bike?stem_length=0", "stem_length", "0"},
		{"/bike?stem_angle=NaN", "stem_angle", "NaN"},
		{"/bike?bar_roll=Inf", "bar_roll", "Inf"},
		{"/bike?post_insert=-1", "post_insert", "-1"},
		{"/bike?post_insert=-1&spacers=-1", "post_insert", "-1"},
		{"/bike?post_insert=290.5", "post_insert", "290.5"},
		{"/bike?post_insert=99.5", "post_insert", "99.5"},
		{"/bike?post=unknown", "post", ""},
		{"/bike?rail_offset=NaN", "rail_offset", "NaN"},
		{"/bike?rail_pitch=Inf", "rail_pitch", "Inf"},
		{"/bike?fork=lithic_carbon_mountain-495x44&fork_a2c=495.0&cup=0.0&sag=0.0&spacers=-1", "cup", "0.0"},
		{"/bike?fork=basis&fork_a2c=495.0&cup=0.0&sag=0.0&spacers=-1", "spacers", "-1"},
		{"/bike?fork=lithic_carbon_mountain-495x44&cup=0", "cup", "0"},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+tc.query, nil)
			rec := httptest.NewRecorder()
			NewHandler().ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("GET %s status = %d, want 400", tc.query, rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
				t.Fatalf("Content-Type = %q, want text/html", ct)
			}

			doc, err := goquery.NewDocumentFromReader(rec.Body)
			if err != nil {
				t.Fatalf("parse HTML: %v", err)
			}
			banner := doc.Find(`#form-error[role="alert"]`)
			if banner.Length() != 1 || strings.TrimSpace(banner.Text()) == "" {
				t.Fatalf("expected non-empty #form-error[role=alert], got %d", banner.Length())
			}
			if doc.Find("svg.diagram").Length() != 0 {
				t.Errorf("expected no svg.diagram on HTTP 400 response")
			}

			invalidEls := doc.Find(`[aria-invalid="true"]`)
			if invalidEls.Length() != 1 {
				t.Fatalf("expected 1 [aria-invalid=true] element, got %d", invalidEls.Length())
			}
			target := doc.Find(fmt.Sprintf(`[name="%s"]`, tc.invalidField))
			if inv, _ := target.Attr("aria-invalid"); inv != "true" {
				t.Errorf("[name=%q] aria-invalid = %q, want true", tc.invalidField, inv)
			}
			if desc, _ := target.Attr("aria-describedby"); desc != "form-error" {
				t.Errorf("[name=%q] aria-describedby = %q, want form-error", tc.invalidField, desc)
			}
			if _, ro := target.Attr("readonly"); ro {
				t.Errorf("invalid field [name=%q] must not be readonly", tc.invalidField)
			}
			if tc.wantVal != "" {
				if val, _ := target.Attr("value"); val != tc.wantVal {
					t.Errorf("input[name=%q] value = %q, want %q", tc.invalidField, val, tc.wantVal)
				}
			}
		})
	}
}

func TestBikeWorkbenchAcceptsSeatpostInsertionLimits(t *testing.T) {
	for _, extension := range []string{"100", "290"} {
		t.Run(extension, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/bike?post=setback_0&post_insert="+extension, nil)
			rec := httptest.NewRecorder()
			NewHandler().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("extension %s returned %d, want 200", extension, rec.Code)
			}
			doc, err := goquery.NewDocumentFromReader(rec.Body)
			if err != nil {
				t.Fatalf("parse HTML: %v", err)
			}
			input := doc.Find(`input[name="post_insert"]`)
			for attr, want := range map[string]string{"min": "100", "max": "290", "value": extension + ".0"} {
				if got, _ := input.Attr(attr); got != want {
					t.Errorf("seatpost insertion %s = %q, want %q", attr, got, want)
				}
			}
		})
	}
}

func TestBuildBikeViewUsesPlacedDimensions(t *testing.T) {
	parsed := mustBike(t, "fork=fox34_120-534x44&cup=12&sag=20&post=setback_15&post_insert=160&rail_offset=12&rail_pitch=-1.5&spacers=15&stem_length=70&stem_angle=6&bar_roll=-4")
	forkSetup := parsed.Setup.Fork
	customWheels := assembly.WheelSetup{RadiusMm: 340.0}
	parsed.Setup.Wheels = customWheels

	bike := mustPlace(t, parsed)
	basis, err := parsed.BasisFrameset()
	if err != nil {
		t.Fatalf("BasisFrameset: %v", err)
	}

	view := buildBikeView(parsed, bike, basis)

	if view.WheelRadiusMm != customWheels.RadiusMm {
		t.Errorf("view.WheelRadiusMm = %.1f, want placed radius %.1f (not catalog %.1f)", view.WheelRadiusMm, customWheels.RadiusMm, catalog.WheelRadiusMm)
	}
	if wantGroundY := -bike.Frameset.RearTireContact.Origin().Y; math.Abs(view.GroundY1-wantGroundY) > 1e-12 {
		t.Errorf("view.GroundY1 = %.6f, want %.6f", view.GroundY1, wantGroundY)
	}
	if gotSpan := view.GroundY1 - view.RearAxleSVG.Y; math.Abs(gotSpan-customWheels.RadiusMm) > 1e-9 {
		t.Errorf("ground-to-axle span = %.6f, want %.6f", gotSpan, customWheels.RadiusMm)
	}
	if view.LowerCupMm != bike.Setup.LowerCup.StackMm {
		t.Errorf("view.LowerCupMm = %.1f, want %.1f", view.LowerCupMm, bike.Setup.LowerCup.StackMm)
	}
	if view.SagPct != forkSetup.SagPct {
		t.Errorf("view.SagPct = %.1f, want %.1f", view.SagPct, forkSetup.SagPct)
	}
	if view.EffectiveA2cMm != forkSetup.EffectiveA2cMm() {
		t.Errorf("view.EffectiveA2cMm = %.1f, want %.1f", view.EffectiveA2cMm, forkSetup.EffectiveA2cMm())
	}
	wantForm := bikeFormValues{
		View:        "bike",
		Frame:       "otso-fenrir-stainless",
		Size:        "L",
		Fork:        "fox34_120-534x44",
		Bar:         "persuader20_800",
		Post:        "setback_15",
		PrevFrame:   "otso-fenrir-stainless",
		PrevSize:    "L",
		PrevFork:    "fox34_120-534x44",
		PrevPost:    "setback_15",
		ForkA2c:     "534.0",
		Cup:         "12.0",
		Sag:         "20.0",
		Spacers:     "15.0",
		StemLength:  "70.0",
		StemAngle:   "6.0",
		BarRoll:     "-4.0",
		PostInsert:  "160.0",
		RailOffset:  "12.0",
		RailPitch:   "-1.5",
		BrpX:        "-219.0",
		BrpY:        "713.1",
		GripX:       "481.0",
		GripY:       "726.0",
		BarOptions:  query.BarOptionsSelected,
		ForkOptions: query.ForkOptionsSelected,
	}
	if view.Form != wantForm {
		t.Errorf("view.Form = %+v, want %+v", view.Form, wantForm)
	}

	basisURL, err := url.Parse(view.BasisForkURL)
	if err != nil {
		t.Fatalf("parse BasisForkURL %q: %v", view.BasisForkURL, err)
	}
	basisDecoded := mustBikeValues(t, basisURL.Query())
	wantBasis := parsed.WithBasisFork()
	if basisDecoded.URL() != wantBasis.URL() {
		t.Errorf("decoded BasisForkURL = %s, want %s", basisDecoded.URL(), wantBasis.URL())
	}
	if basisDecoded.ForkID != query.BasisForkID || basisDecoded.Setup.Fork.Spec.StaticA2cMm != parsed.Frame.BasisFork.StaticA2cMm ||
		basisDecoded.Setup.LowerCup.StackMm != parsed.Frame.BasisLowerCup.StackMm || basisDecoded.Setup.Fork.SagPct != 0 {
		t.Errorf("decoded basis build = fork %q a2c %v cup %v sag %v", basisDecoded.ForkID, basisDecoded.Setup.Fork.Spec.StaticA2cMm, basisDecoded.Setup.LowerCup.StackMm, basisDecoded.Setup.Fork.SagPct)
	}
	if s := basisDecoded.Setup.Stem; s.SpacersMm != 15 || s.LengthMm != 70 || s.AngleDeg != 6 || basisDecoded.Setup.Seatpost.InsertionMm != 160 {
		t.Errorf("decoded basis build lost the cockpit: stem %+v insertion %v", s, basisDecoded.Setup.Seatpost.InsertionMm)
	}
}

func TestFormatFormFloat(t *testing.T) {
	if got := formatFormFloat(math.Copysign(0, -1)); got != "0.0" {
		t.Errorf("formatFormFloat(-0.0) = %q, want 0.0", got)
	}
	if got := formatFormFloat(12.5000000001); got != "12.5000000001" {
		t.Errorf("formatFormFloat(12.5000000001) = %q, want 12.5000000001", got)
	}
}

func serializeForm(t *testing.T, form *goquery.Selection) url.Values {
	t.Helper()
	if form.Length() != 1 {
		t.Fatalf("expected 1 form, got %d", form.Length())
	}
	vals := url.Values{}
	form.Find("input, select, textarea").Each(func(_ int, el *goquery.Selection) {
		if _, disabled := el.Attr("disabled"); disabled {
			return
		}
		if el.ParentsFiltered("fieldset[disabled]").Length() > 0 {
			return
		}
		name, hasName := el.Attr("name")
		if !hasName || name == "" {
			return
		}
		switch goquery.NodeName(el) {
		case "select":
			opt := el.Find("option[selected]").First()
			if opt.Length() == 0 {
				opt = el.Find("option:not([disabled])").First()
			}
			if opt.Length() == 0 {
				return
			}
			if _, disabled := opt.Attr("disabled"); disabled {
				return
			}
			val, _ := opt.Attr("value")
			vals.Add(name, val)
		case "textarea":
			vals.Add(name, el.Text())
		default:
			typ, _ := el.Attr("type")
			switch strings.ToLower(typ) {
			case "submit", "button", "reset", "image":
				return
			case "checkbox", "radio":
				if _, checked := el.Attr("checked"); !checked {
					return
				}
			}
			val, _ := el.Attr("value")
			vals.Add(name, val)
		}
	})
	return vals
}

func submitForm(t *testing.T, doc *goquery.Document, override map[string]string) (*httptest.ResponseRecorder, *goquery.Document) {
	t.Helper()
	form := doc.Find("form.controls")
	if form.Length() != 1 {
		t.Fatalf("expected 1 form.controls, got %d", form.Length())
	}
	action, ok := form.Attr("action")
	if !ok || action == "" {
		action = "/bike"
	}
	vals := serializeForm(t, form)
	for k, v := range override {
		vals.Set(k, v)
	}
	target := "http://127.0.0.1:8080" + action + "?" + vals.Encode()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)

	nextDoc, err := goquery.NewDocumentFromReader(rec.Body)
	if err != nil {
		t.Fatalf("parse response HTML for %s: %v", target, err)
	}
	return rec, nextDoc
}

func collectMetrics(doc *goquery.Document) map[string]string {
	metrics := map[string]string{}
	doc.Find("[data-metric]").Each(func(_ int, el *goquery.Selection) {
		if key, ok := el.Attr("data-metric"); ok {
			metrics[key] = strings.TrimSpace(el.Text())
		}
	})
	return metrics
}

func loadBikeDoc(t *testing.T, path string) *goquery.Document {
	t.Helper()
	return loadBikeDocStatus(t, path, http.StatusOK)
}

func loadBikeDocStatus(t *testing.T, path string, wantStatus int) *goquery.Document {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+path, nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("GET %s status = %d, want %d (body: %s)", path, rec.Code, wantStatus, rec.Body.String())
	}
	doc, err := goquery.NewDocumentFromReader(rec.Body)
	if err != nil {
		t.Fatalf("parse HTML for %s: %v", path, err)
	}
	return doc
}

func assertUnchangedSubmitIdempotent(t *testing.T, label string, doc *goquery.Document) *goquery.Document {
	t.Helper()
	wantMetrics := collectMetrics(doc)
	wantForm := serializeForm(t, doc.Find("form.controls"))

	rec, nextDoc := submitForm(t, doc, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s unchanged submit status = %d, want 200 (body: %s)", label, rec.Code, rec.Body.String())
	}
	gotMetrics := collectMetrics(nextDoc)
	if !maps.Equal(gotMetrics, wantMetrics) {
		t.Errorf("%s unchanged submit changed metrics:\n got: %v\nwant: %v", label, gotMetrics, wantMetrics)
	}
	gotForm := serializeForm(t, nextDoc.Find("form.controls"))
	if gotForm.Encode() != wantForm.Encode() {
		t.Errorf("%s unchanged submit changed form values:\n got: %s\nwant: %s", label, gotForm.Encode(), wantForm.Encode())
	}
	return nextDoc
}

func TestBikeWorkbenchFormRoundTripPreservesEveryBuild(t *testing.T) {
	// 1. Default build.
	defaultDoc := loadBikeDoc(t, "/bike")
	assertUnchangedSubmitIdempotent(t, "default build", defaultDoc)

	// 2. Follow Chart Basis Fork preset from default build.
	basisHref, ok := defaultDoc.Find(`a[data-preset="basis-fork"]`).Attr("href")
	if !ok || basisHref == "" {
		t.Fatalf("missing a[data-preset=basis-fork] href")
	}
	basisDoc := loadBikeDoc(t, basisHref)
	if got, _ := basisDoc.Find(`input[name="cup"]`).Attr("value"); got != "0.0" {
		t.Errorf("basis preset cup = %q, want 0.0", got)
	}
	if got := strings.TrimSpace(basisDoc.Find(`[data-metric="delta-pitch"]`).Text()); got != "+0.00°" {
		t.Errorf("basis preset delta-pitch = %q, want +0.00°", got)
	}
	assertUnchangedSubmitIdempotent(t, "basis preset from default", basisDoc)

	// 3. Fully customized build (targets off: ranking is covered by its own tests).
	customURL := "/bike?fork=fox34_120-534x44&fork_a2c=534&cup=12&sag=20&bar=sqlab3ox_16_45" +
		"&spacers=25&stem_length=80&stem_angle=7&bar_roll=-5" +
		"&post=setback_25&post_insert=150&rail_offset=10&rail_pitch=-2&bar_options=all&fork_options=all&brp=off&grip=off"
	customDoc := loadBikeDoc(t, customURL)
	assertUnchangedSubmitIdempotent(t, "custom build", customDoc)

	// 4. Follow Chart Basis Fork from customized build: preserves cockpit, sets cup=0,
	//    round-trips unchanged, and keeps basis fork when spacers are later edited.
	customBasisHref, ok := customDoc.Find(`a[data-preset="basis-fork"]`).Attr("href")
	if !ok || customBasisHref == "" {
		t.Fatalf("missing a[data-preset=basis-fork] href on custom build")
	}
	customBasisDoc := loadBikeDoc(t, customBasisHref)
	if got, _ := customBasisDoc.Find(`input[name="spacers"]`).Attr("value"); got != "25.0" {
		t.Errorf("basis preset spacers = %q, want 25.0", got)
	}
	if got, _ := customBasisDoc.Find(`input[name="stem_length"]`).Attr("value"); got != "80.0" {
		t.Errorf("basis preset stem_length = %q, want 80.0", got)
	}
	if got, _ := customBasisDoc.Find(`input[name="cup"]`).Attr("value"); got != "0.0" {
		t.Errorf("basis preset cup = %q, want 0.0", got)
	}
	if got := strings.TrimSpace(customBasisDoc.Find(`[data-metric="delta-pitch"]`).Text()); got != "+0.00°" {
		t.Errorf("basis preset delta-pitch = %q, want +0.00°", got)
	}
	assertUnchangedSubmitIdempotent(t, "basis preset from custom build", customBasisDoc)

	rec, editedDoc := submitForm(t, customBasisDoc, map[string]string{"spacers": "30"})
	if rec.Code != http.StatusOK {
		t.Fatalf("editing spacers on basis build status = %d, want 200", rec.Code)
	}
	if got, _ := editedDoc.Find(`input[name="cup"]`).Attr("value"); got != "0.0" {
		t.Errorf("after spacer edit cup = %q, want 0.0", got)
	}
	if got, _ := editedDoc.Find(`input[name="spacers"]`).Attr("value"); got != "30.0" {
		t.Errorf("after spacer edit spacers = %q, want 30.0", got)
	}
	if got := strings.TrimSpace(editedDoc.Find(`[data-metric="delta-pitch"]`).Text()); got != "+0.00°" {
		t.Errorf("after spacer edit delta-pitch = %q, want +0.00°", got)
	}

	// 5. Follow Reset Defaults preset link.
	defaultsHref, ok := customBasisDoc.Find(`a[data-preset="defaults"]`).Attr("href")
	if !ok || defaultsHref == "" {
		t.Fatalf("missing a[data-preset=defaults] href")
	}
	resetDoc := loadBikeDoc(t, defaultsHref)
	if got, _ := resetDoc.Find(`input[name="cup"]`).Attr("value"); got != "12.0" {
		t.Errorf("reset defaults cup = %q, want 12.0", got)
	}
	if got, _ := resetDoc.Find(`input[name="spacers"]`).Attr("value"); got != "20.0" {
		t.Errorf("reset defaults spacers = %q, want 20.0", got)
	}
	assertUnchangedSubmitIdempotent(t, "reset defaults", resetDoc)
}

func TestBikeWorkbenchEveryEnabledControlIsLive(t *testing.T) {
	defaultDoc := loadBikeDoc(t, "/bike")
	basisHref, _ := defaultDoc.Find(`a[data-preset="basis-fork"]`).Attr("href")
	basisDoc := loadBikeDoc(t, basisHref)
	basisMetrics := collectMetrics(basisDoc)

	// Every editable control on the basis fork page must take effect and change the build.
	mutations := []struct {
		field    string
		val      string
		isSelect bool
	}{
		{field: "frame", val: "surly-krampus", isSelect: true},
		{field: "size", val: "XL", isSelect: true},
		{field: "fork", val: "sid_100-506x44", isSelect: true},
		{field: "bar", val: "sqlab311_flx_12_30", isSelect: true},
		{field: "spacers", val: "30.0"},
		{field: "stem_length", val: "80.0"},
		{field: "stem_angle", val: "-12.0"},
		{field: "bar_roll", val: "8.0"},
		{field: "post_insert", val: "150.0"},
		{field: "post", val: "setback_25", isSelect: true},
		{field: "rail_offset", val: "15.0"},
		{field: "rail_pitch", val: "-4.0"},
	}
	for _, m := range mutations {
		t.Run("basis/"+m.field, func(t *testing.T) {
			rec, nextDoc := submitForm(t, basisDoc, map[string]string{m.field: m.val})
			if rec.Code != http.StatusOK {
				t.Fatalf("submit %s=%s status = %d, want 200", m.field, m.val, rec.Code)
			}
			if m.isSelect {
				got, _ := nextDoc.Find(fmt.Sprintf(`select[name="%s"] option[selected]`, m.field)).Attr("value")
				if got != m.val {
					t.Errorf("select[name=%q] selected = %q, want %q", m.field, got, m.val)
				}
				if m.field == "fork" {
					if cup, _ := nextDoc.Find(`input[name="cup"]`).Attr("value"); cup != "12.0" {
						t.Errorf("cup after switching from basis to %s = %q, want 12.0", m.val, cup)
					}
				}
			} else {
				got, _ := nextDoc.Find(fmt.Sprintf(`input[name="%s"]`, m.field)).Attr("value")
				if got != m.val {
					t.Errorf("input[name=%q] value = %q, want %q", m.field, got, m.val)
				}
			}
			if maps.Equal(collectMetrics(nextDoc), basisMetrics) {
				t.Errorf("mutating %s=%s left all rendered metrics unchanged", m.field, m.val)
			}
		})
	}

	// On an EC44 physical fork build, cup is live for positive stacks and rejects 0.
	defaultMetrics := collectMetrics(defaultDoc)
	rec, nextCupDoc := submitForm(t, defaultDoc, map[string]string{"cup": "14.0"})
	if rec.Code != http.StatusOK {
		t.Fatalf("submit cup=14.0 on EC44 build status = %d, want 200", rec.Code)
	}
	if got, _ := nextCupDoc.Find(`input[name="cup"]`).Attr("value"); got != "14.0" {
		t.Errorf("EC44 cup input after submit = %q, want 14.0", got)
	}
	if maps.Equal(collectMetrics(nextCupDoc), defaultMetrics) {
		t.Errorf("mutating cup=14.0 left all rendered metrics unchanged")
	}
	rec, errCupDoc := submitForm(t, defaultDoc, map[string]string{"cup": "0"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mutating cup=0 on EC44 build status = %d, want 400", rec.Code)
	}
	if inv, _ := errCupDoc.Find(`[name="cup"]`).Attr("aria-invalid"); inv != "true" {
		t.Errorf("EC44 cup=0 aria-invalid = %q, want true", inv)
	}

	// On a suspension fork build, sag is also live and changes effective A2C.
	sidDoc := loadBikeDoc(t, "/bike?fork=sid_100-506x44&cup=12&sag=20")
	if got := strings.TrimSpace(sidDoc.Find(`[data-metric="effective-a2c"]`).Text()); got != "486.0 mm" {
		t.Fatalf("initial SID effective-a2c = %q, want 486.0 mm", got)
	}
	rec, nextSidDoc := submitForm(t, sidDoc, map[string]string{"sag": "15"})
	if rec.Code != http.StatusOK {
		t.Fatalf("submit sag=15 status = %d, want 200", rec.Code)
	}
	if got, _ := nextSidDoc.Find(`input[name="sag"]`).Attr("value"); got != "15.0" {
		t.Errorf("sag input after submit = %q, want 15.0", got)
	}
	if got := strings.TrimSpace(nextSidDoc.Find(`[data-metric="effective-a2c"]`).Text()); got != "491.0 mm" {
		t.Errorf("SID effective-a2c after sag=15 = %q, want 491.0 mm", got)
	}
}

func TestBikeBasisForkURLDecodesToChartFork(t *testing.T) {
	base := query.DefaultBike()
	basisURL, err := url.Parse(base.WithBasisFork().URL())
	if err != nil {
		t.Fatalf("parse basis URL: %v", err)
	}
	decoded, err := query.ParseBike(basisURL.Query())
	if err != nil {
		t.Fatalf("ParseBike: %v", err)
	}
	if decoded.ForkID != query.BasisForkID {
		t.Errorf("decoded ForkID = %q, want %q", decoded.ForkID, query.BasisForkID)
	}
	if decoded.Setup.Frame != decoded.Size.ChartFrame {
		t.Errorf("decoded Frame = %+v, want %+v", decoded.Setup.Frame, decoded.Size.ChartFrame)
	}
	if decoded.Setup.LowerCup != decoded.Frame.BasisLowerCup {
		t.Errorf("decoded LowerCup = %+v, want %+v", decoded.Setup.LowerCup, decoded.Frame.BasisLowerCup)
	}
	if err := decoded.Setup.Fork.Validate(); err != nil {
		t.Fatalf("decoded Fork.Validate: %v", err)
	}
	if decoded.Setup.Fork.Spec != decoded.Frame.BasisFork {
		t.Errorf("decoded Fork.Spec = %+v, want %+v", decoded.Setup.Fork.Spec, decoded.Frame.BasisFork)
	}
	basis, err := decoded.BasisFrameset()
	if err != nil {
		t.Fatalf("BasisFrameset: %v", err)
	}
	placed, err := assembly.PlaceFrameset(decoded.Setup.Frame, decoded.Setup.LowerCup, decoded.Setup.Fork, decoded.Setup.Wheels)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}
	if gotDelta := placed.PitchDeltaDeg(basis); math.Abs(float64(gotDelta)) > 1e-12 {
		t.Errorf("basis PitchDeltaDeg = %.6f°, want 0°", gotDelta)
	}
}

func TestBikeWorkbenchBasisForkLockAndLabels(t *testing.T) {
	hasReadOnly := func(doc *goquery.Document, name string) bool {
		_, exists := doc.Find(fmt.Sprintf(`input[name=%q]`, name)).Attr("readonly")
		return exists
	}

	// 1. Overrides on fork=basis are ignored (200, no error, chart values locked & readonly).
	for _, query := range []string{
		"/bike?fork=basis&cup=12",
		"/bike?fork=basis&fork_a2c=530",
		"/bike?fork=basis&sag=10",
		"/bike?fork=basis&fork_a2c=530&cup=12&sag=20&spacers=25",
	} {
		t.Run("locked/"+query, func(t *testing.T) {
			doc := loadBikeDoc(t, query)
			if doc.Find(`#form-error`).Length() != 0 {
				t.Fatalf("unexpected #form-error: %s", doc.Find(`#form-error`).Text())
			}
			for field, want := range map[string]string{
				"fork_a2c": "495.0",
				"cup":      "0.0",
				"sag":      "0.0",
			} {
				if got, _ := doc.Find(fmt.Sprintf(`input[name=%q]`, field)).Attr("value"); got != want {
					t.Errorf("%s value = %q, want %q", field, got, want)
				}
				if !hasReadOnly(doc, field) {
					t.Errorf("expected %s to be readonly under fork=basis", field)
				}
			}
			if got := strings.TrimSpace(doc.Find(`[data-metric="delta-pitch"]`).Text()); got != "+0.00°" {
				t.Errorf("delta-pitch = %q, want +0.00°", got)
			}
			if hasReadOnly(doc, "spacers") {
				t.Errorf("cockpit field spacers should remain editable under fork=basis")
			}
		})
	}

	// 2. Rigid physical fork has readonly sag only; suspension fork has all three editable.
	rigidDoc := loadBikeDoc(t, "/bike")
	if hasReadOnly(rigidDoc, "fork_a2c") || hasReadOnly(rigidDoc, "cup") {
		t.Errorf("rigid physical fork should keep fork_a2c and cup editable")
	}
	if !hasReadOnly(rigidDoc, "sag") {
		t.Errorf("rigid physical fork should mark sag readonly")
	}
	suspDoc := loadBikeDoc(t, "/bike?fork=fox34_120-534x44")
	if hasReadOnly(suspDoc, "fork_a2c") || hasReadOnly(suspDoc, "cup") || hasReadOnly(suspDoc, "sag") {
		t.Errorf("suspension fork should keep fork_a2c, cup, and sag editable")
	}

	// 3. Single "Chart Basis Fork" naming, cup standard label, and data-driven note.
	basisDoc := loadBikeDoc(t, "/bike?fork=basis")
	basisHTML, err := basisDoc.Html()
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	for _, banned := range []string{"Chart Inspection", "basis Chart Reference", "chart only"} {
		if strings.Contains(basisHTML, banned) {
			t.Errorf("HTML unexpectedly contains legacy label %q", banned)
		}
	}
	if got, _ := basisDoc.Find(`select[name="fork"] optgroup`).Last().Attr("label"); got != "Chart Basis Fork" {
		t.Errorf("last optgroup label = %q, want %q", got, "Chart Basis Fork")
	}
	if got := strings.TrimSpace(basisDoc.Find(`label[for="ctrl-cup"]`).Text()); got != "No lower cup (bare head tube)" {
		t.Errorf("basis cup label = %q, want %q", got, "No lower cup (bare head tube)")
	}
	if got := strings.TrimSpace(rigidDoc.Find(`label[for="ctrl-cup"]`).Text()); got != "EC44 lower cup (mm, > 0)" {
		t.Errorf("physical cup label = %q, want %q", got, "EC44 lower cup (mm, > 0)")
	}
	note := strings.TrimSpace(basisDoc.Find(`[data-note="basis-fork"]`).Text())
	if !strings.Contains(note, "Chart Basis Fork") || !strings.Contains(note, "bare head tube") || !strings.Contains(note, "EC44 (12 mm)") {
		t.Errorf("unexpected basis-fork note: %q", note)
	}
	if rigidDoc.Find(`[data-note="basis-fork"]`).Length() != 0 {
		t.Errorf("basis-fork note should not render for physical forks")
	}

	// 4. Switching both ways between fork=basis and physical forks updates cup, A2C, and sag.
	rec, fromBasis := submitForm(t, basisDoc, map[string]string{"fork": "fox34_120-534x44"})
	if rec.Code != http.StatusOK {
		t.Fatalf("switch basis -> fox34_120 status = %d, want 200", rec.Code)
	}
	if got, _ := fromBasis.Find(`input[name="cup"]`).Attr("value"); got != "12.0" {
		t.Errorf("switch basis -> fox34_120 cup = %q, want 12.0", got)
	}
	if got, _ := fromBasis.Find(`input[name="sag"]`).Attr("value"); got != "25.0" {
		t.Errorf("switch basis -> fox34_120 sag = %q, want 25.0", got)
	}
	rec, backToBasis := submitForm(t, fromBasis, map[string]string{"fork": "basis"})
	if rec.Code != http.StatusOK {
		t.Fatalf("switch fox34_120 -> basis status = %d, want 200", rec.Code)
	}
	if got, _ := backToBasis.Find(`input[name="cup"]`).Attr("value"); got != "0.0" {
		t.Errorf("switch fox34_120 -> basis cup = %q, want 0.0", got)
	}
	if got, _ := backToBasis.Find(`input[name="fork_a2c"]`).Attr("value"); got != "495.0" {
		t.Errorf("switch fox34_120 -> basis fork_a2c = %q, want 495.0", got)
	}
}

func TestBikeWorkbenchRecoversFromInvalidRigidForkSag(t *testing.T) {
	hasReadOnly := func(doc *goquery.Document, name string) bool {
		_, exists := doc.Find(fmt.Sprintf(`input[name=%q]`, name)).Attr("readonly")
		return exists
	}

	// Single-error recovery: invalid/non-zero sag on rigid fork stays editable so
	// the user can reset it to 0 without losing other settings, while unchanged
	// resubmission continues to fail physical-fork validation.
	for _, sagVal := range []string{"150", "10", "Inf"} {
		t.Run("single/"+sagVal, func(t *testing.T) {
			errDoc := loadBikeDocStatus(t, fmt.Sprintf("/bike?sag=%s&spacers=25&stem_length=80", sagVal), http.StatusBadRequest)
			if errDoc.Find(`#form-error[role="alert"]`).Length() != 1 {
				t.Fatalf("expected #form-error[role=alert]")
			}
			sagInput := errDoc.Find(`input[name="sag"]`)
			if got, _ := sagInput.Attr("value"); got != sagVal {
				t.Errorf("sag value = %q, want %q", got, sagVal)
			}
			if inv, _ := sagInput.Attr("aria-invalid"); inv != "true" {
				t.Errorf("sag aria-invalid = %q, want true", inv)
			}
			if hasReadOnly(errDoc, "sag") {
				t.Errorf("invalid sag on rigid fork must not be readonly")
			}
			if got, _ := errDoc.Find(`input[name="spacers"]`).Attr("value"); got != "25" {
				t.Errorf("spacers value = %q, want 25", got)
			}
			if got, _ := errDoc.Find(`input[name="stem_length"]`).Attr("value"); got != "80" {
				t.Errorf("stem_length value = %q, want 80", got)
			}

			// Resubmitting unchanged preserves physical-fork validation failure.
			rec, stillErr := submitForm(t, errDoc, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("unchanged resubmit status = %d, want 400", rec.Code)
			}
			if hasReadOnly(stillErr, "sag") {
				t.Errorf("sag after unchanged resubmit must remain editable")
			}

			// Resetting sag to 0 recovers the build, restores readonly, and keeps other settings.
			rec, okDoc := submitForm(t, errDoc, map[string]string{"sag": "0"})
			if rec.Code != http.StatusOK {
				t.Fatalf("recovery submit status = %d, want 200", rec.Code)
			}
			if okDoc.Find(`#form-error`).Length() != 0 {
				t.Errorf("unexpected #form-error after recovery: %s", okDoc.Find(`#form-error`).Text())
			}
			if got, _ := okDoc.Find(`input[name="sag"]`).Attr("value"); got != "0.0" {
				t.Errorf("recovered sag value = %q, want 0.0", got)
			}
			if !hasReadOnly(okDoc, "sag") {
				t.Errorf("recovered rigid fork sag should be readonly again")
			}
			if got, _ := okDoc.Find(`input[name="spacers"]`).Attr("value"); got != "25.0" {
				t.Errorf("recovered spacers = %q, want 25.0", got)
			}
			if got, _ := okDoc.Find(`input[name="stem_length"]`).Attr("value"); got != "80.0" {
				t.Errorf("recovered stem_length = %q, want 80.0", got)
			}
		})
	}

	// Multi-error recovery: cup fails first so invalidField is "cup", yet invalid
	// sag on the rigid fork still stays editable so both can be fixed together.
	multiErr := loadBikeDocStatus(t, "/bike?cup=abc&sag=150&spacers=25", http.StatusBadRequest)
	if inv, _ := multiErr.Find(`input[name="cup"]`).Attr("aria-invalid"); inv != "true" {
		t.Errorf("cup aria-invalid = %q, want true", inv)
	}
	if got, _ := multiErr.Find(`input[name="sag"]`).Attr("value"); got != "150" {
		t.Errorf("multi-error sag value = %q, want 150", got)
	}
	if hasReadOnly(multiErr, "sag") {
		t.Errorf("multi-error invalid sag on rigid fork must not be readonly")
	}
	rec, multiOK := submitForm(t, multiErr, map[string]string{"cup": "12", "sag": "0"})
	if rec.Code != http.StatusOK {
		t.Fatalf("multi-error recovery status = %d, want 200", rec.Code)
	}
	if !hasReadOnly(multiOK, "sag") {
		t.Errorf("recovered multi-error rigid fork sag should be readonly")
	}
	if got, _ := multiOK.Find(`input[name="spacers"]`).Attr("value"); got != "25.0" {
		t.Errorf("multi-error recovered spacers = %q, want 25.0", got)
	}

	// Control cases: valid 0 sag stays readonly on error pages when another field fails.
	otherErr := loadBikeDocStatus(t, "/bike?spacers=-1", http.StatusBadRequest)
	if !hasReadOnly(otherErr, "sag") {
		t.Errorf("valid zero sag on rigid fork error page should stay readonly")
	}
	basisErr := loadBikeDocStatus(t, "/bike?fork=basis&spacers=-1", http.StatusBadRequest)
	for _, field := range []string{"fork_a2c", "cup", "sag"} {
		if !hasReadOnly(basisErr, field) {
			t.Errorf("basis baseline field %q on error page should stay readonly", field)
		}
	}
}

// activeViewHref is the resolved build URL carried by the current diagram
// view link on a successful page.
func activeViewHref(t *testing.T, doc *goquery.Document) string {
	t.Helper()
	links := doc.Find("nav.diagram-views a[aria-current]")
	if links.Length() != 1 {
		t.Fatalf("expected 1 nav.diagram-views a[aria-current], got %d", links.Length())
	}
	href, ok := links.Attr("href")
	if !ok || href == "" {
		t.Fatalf("active diagram view link has no href")
	}
	return href
}

func TestRemovedRoutesReturnNotFound(t *testing.T) {
	for _, path := range []string{"/static/workbench.js", "/static/workbench_test.js", "/static/missing.js", "/nope"} {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+path, nil)
		rec := httptest.NewRecorder()
		NewHandler().ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, rec.Code)
		}
	}
}

func TestBikePagesHaveNoScriptOrConstructionTrace(t *testing.T) {
	const removed = `script, table.trace-table, [data-step-inspector], [data-step-id], .trace-steps, .step-frames`
	for _, path := range []string{"/bike", "/bike?view=seat", "/bike?view=stem", "/bike?brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726"} {
		doc := loadBikeDoc(t, path)
		if n := doc.Find(removed).Length(); n != 0 {
			t.Errorf("%s: found %d script or construction-trace elements, want none", path, n)
		}
		if n := doc.Find("svg.diagram").Length(); n != 1 {
			t.Errorf("%s: %d schematics, want 1", path, n)
		}
		if n := doc.Find(`form.controls[method="get"][action="/bike"] button[type="submit"]`).Length(); n != 1 {
			t.Errorf("%s: %d GET form submit buttons, want 1", path, n)
		}
		if n := doc.Find(`g[data-element="world-axes"]`).Length(); n != 1 {
			t.Errorf("%s: %d world-axes groups, want 1", path, n)
		}
		if n := doc.Find(`g.component-outlines rect`).Length(); n == 0 {
			t.Errorf("%s: no component outlines", path)
		}
	}
	for _, path := range []string{"/bike?cup=0", "/bike?spacers=-1"} {
		doc := loadBikeDocStatus(t, path, http.StatusBadRequest)
		if n := doc.Find(removed).Length(); n != 0 {
			t.Errorf("%s: 400 page has %d script or trace elements, want none", path, n)
		}
	}
}

func TestBikeWorkbenchActiveViewLinkCarriesResolvedBuild(t *testing.T) {
	okCases := []struct {
		name string
		path string
		want query.Bike
	}{
		{
			name: "default",
			path: "/bike",
			want: query.DefaultBike(),
		},
		{
			name: "fork-switch-omits-prev-fork",
			path: "/bike?prev_fork=lithic_carbon_mountain-495x44&fork=fox34_120-534x44&fork_a2c=495&sag=0",
			want: mustBike(t, "fork=fox34_120-534x44"),
		},
		{
			name: "basis-fork-overrides-locked",
			path: "/bike?fork=basis&cup=12&fork_a2c=530&sag=20",
			want: query.DefaultBike().WithBasisFork(),
		},
	}

	for _, tc := range okCases {
		t.Run("ok/"+tc.name, func(t *testing.T) {
			doc := loadBikeDoc(t, tc.path)
			href := activeViewHref(t, doc)
			u, err := url.Parse(href)
			if err != nil {
				t.Fatalf("parse active view href %q: %v", href, err)
			}
			if u.Path != "/bike" {
				t.Errorf("active view href path = %q, want /bike", u.Path)
			}
			if u.Query().Has("prev_fork") {
				t.Errorf("active view href %q must omit transient prev_fork", href)
			}
			decoded := mustBikeValues(t, u.Query())
			if decoded.URL() != tc.want.URL() {
				t.Errorf("decoded active view build = %s, want %s", decoded.URL(), tc.want.URL())
			}

			form := doc.Find("form.controls")
			if form.Find(`button#ctrl-submit[type="submit"]`).Length() != 1 {
				t.Errorf("expected 1 button#ctrl-submit[type=submit] inside form.controls")
			}
			if href, ok := form.Find(`a#preset-defaults[data-preset="defaults"]`).Attr("href"); !ok || href != query.DefaultBike().URL() {
				t.Errorf("a#preset-defaults href = %q (ok=%v), want %q", href, ok, query.DefaultBike().URL())
			}
			if href, ok := form.Find(`a#preset-basis-fork[data-preset="basis-fork"]`).Attr("href"); !ok || !strings.HasPrefix(href, "/bike?") {
				t.Errorf("a#preset-basis-fork href = %q (ok=%v), want /bike?...", href, ok)
			}
		})
	}

	for _, errPath := range []string{"/bike?cup=0", "/bike?spacers=-1"} {
		t.Run("err/"+errPath, func(t *testing.T) {
			doc := loadBikeDocStatus(t, errPath, http.StatusBadRequest)
			if n := doc.Find("nav.diagram-views").Length(); n != 0 {
				t.Errorf("HTTP 400 page renders %d diagram view navs, want none", n)
			}
		})
	}
}

func parseViewBoxAttr(t *testing.T, doc *goquery.Document) svgViewBox {
	t.Helper()
	raw, ok := doc.Find("svg.diagram").Attr("viewBox")
	if !ok {
		t.Fatalf("svg.diagram missing viewBox")
	}
	fields := strings.Fields(raw)
	if len(fields) != 4 {
		t.Fatalf("viewBox %q: want 4 numbers", raw)
	}
	var nums [4]float64
	for i, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			t.Fatalf("viewBox %q: %v", raw, err)
		}
		nums[i] = v
	}
	return svgViewBox{X: nums[0], Y: nums[1], W: nums[2], H: nums[3]}
}

func landmarkCentre(t *testing.T, doc *goquery.Document, name string) svgPoint {
	t.Helper()
	circle := doc.Find(fmt.Sprintf(`.landmarks [data-landmark=%q] circle`, name))
	if circle.Length() != 1 {
		t.Fatalf("landmark %s: want 1 circle, got %d", name, circle.Length())
	}
	cx, _ := circle.Attr("cx")
	cy, _ := circle.Attr("cy")
	x, errX := strconv.ParseFloat(cx, 64)
	y, errY := strconv.ParseFloat(cy, 64)
	if errX != nil || errY != nil {
		t.Fatalf("landmark %s: bad centre (%q, %q)", name, cx, cy)
	}
	return svgPoint{X: x, Y: y}
}

func (b svgViewBox) contains(p svgPoint) bool {
	return p.X >= b.X && p.X <= b.X+b.W && p.Y >= b.Y && p.Y <= b.Y+b.H
}

// unitsPerPx reads the server's nominal --units-per-px from the diagram.
func unitsPerPx(t *testing.T, doc *goquery.Document) float64 {
	t.Helper()
	style, _ := doc.Find("svg.diagram").Attr("style")
	const key = "--units-per-px:"
	_, after, ok := strings.Cut(style, key)
	if !ok {
		t.Fatalf("svg.diagram style %q has no %s", style, key)
	}
	raw := strings.TrimSpace(strings.TrimSuffix(strings.Fields(after)[0], ";"))
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		t.Fatalf("--units-per-px %q: %v", raw, err)
	}
	return v
}

func hiddenViewValue(doc *goquery.Document) string {
	v, _ := doc.Find(`form.controls input[type="hidden"][name="view"]`).Attr("value")
	return v
}

func decodeHref(t *testing.T, href string) query.Bike {
	t.Helper()
	u, err := url.Parse(href)
	if err != nil {
		t.Fatalf("parse %q: %v", href, err)
	}
	return mustBikeValues(t, u.Query())
}

func TestBikeWorkbenchDefaultViewKeepsWholeBikeDrawing(t *testing.T) {
	doc := loadBikeDoc(t, "/bike")

	canonical := activeViewHref(t, doc)
	if got := decodeHref(t, canonical).View; got != query.ViewBike {
		t.Errorf("canonical URL %q view = %q, want bike", canonical, got)
	}
	if !strings.Contains(canonical, "view=bike") {
		t.Errorf("canonical URL %q must carry view=bike", canonical)
	}
	if got := hiddenViewValue(doc); got != "bike" {
		t.Errorf("hidden view input = %q, want bike", got)
	}

	// The whole-bike frame and drawing sizes are unchanged by the view work.
	parsed := query.DefaultBike()
	bike := mustPlace(t, parsed)
	basis, err := parsed.BasisFrameset()
	if err != nil {
		t.Fatalf("BasisFrameset: %v", err)
	}
	view := buildBikeView(parsed, bike, basis)
	if view.DrawScale != 1 {
		t.Errorf("whole-bike DrawScale = %v, want exactly 1", view.DrawScale)
	}
	if got, _ := doc.Find("svg.diagram").Attr("viewBox"); got != "-902.07 -850.54 2079.62 1256.87" {
		t.Errorf("whole-bike viewBox = %q, want the pre-view framing -902.07 -850.54 2079.62 1256.87", got)
	}

	checks := []struct{ sel, attr, want string }{
		{`.landmarks [data-landmark="BB"] circle`, "r", "5"},
		{`.landmarks [data-landmark="BB"] circle`, "stroke-width", "1.5"},
		{`.landmarks [data-landmark="BB"] text`, "x", "0.00"},
		{`.landmarks [data-landmark="BB"] text`, "dy", "0.35em"},
		{`.landmarks [data-landmark="BB"] text`, "text-anchor", "middle"},
		{`[data-element="world-axes"]`, "stroke-width", "1.8"},
		{`[data-element="world-axes"]`, "stroke-dasharray", "6 4"},
		{`[data-element="ground-line"]`, "stroke-dasharray", "10 6"},
		{`.wheels`, "stroke-width", "3"},
		{`[data-segment="chainstay"]`, "stroke-width", "6.0"},
		{`[data-segment="seat-tube"]`, "stroke-dasharray", "12 6"},
		{`.component-outlines`, "stroke-width", "1.5"},
		{`.component-outlines rect`, "rx", "2"},
		{`svg.diagram`, "style", fmt.Sprintf("--units-per-px: %.4f;", 2079.62/nominalDiagramWidthPx)},
	}
	for _, c := range checks {
		if got, _ := doc.Find(c.sel).First().Attr(c.attr); got != c.want {
			t.Errorf("%s [%s] = %q, want %q", c.sel, c.attr, got, c.want)
		}
	}
	// Label font sizes come from CSS in rem, not from SVG attributes, so they
	// do not shrink with the diagram.
	doc.Find(`.landmarks text, .target-labels text, [data-element="world-axes"] text`).Each(func(_ int, el *goquery.Selection) {
		if fs, ok := el.Attr("font-size"); ok {
			t.Errorf("label %q has font-size attribute %q; size labels with CSS rem", el.Text(), fs)
		}
	})
}

func TestBikeWorkbenchCloseUpViewsFrameTheirLandmarks(t *testing.T) {
	bikeDoc := loadBikeDoc(t, "/bike")
	whole := parseViewBoxAttr(t, bikeDoc)
	wholeUnitsPerPx := unitsPerPx(t, bikeDoc)

	cases := []struct {
		view    string
		framing []string
		far     string
		label   string
	}{
		{"seat", []string{"RailPivot", "RailCenter", "BRP", "SitBones"}, "SeatTubeTop", "seat close-up"},
		{"stem", []string{"CoverTop", "SpacerTop", "SteererClamp", "BarClamp", "HandGrip"}, "HeadTubeBottom", "stem close-up"},
	}
	for _, tc := range cases {
		t.Run(tc.view, func(t *testing.T) {
			doc := loadBikeDoc(t, "/bike?view="+tc.view)
			box := parseViewBoxAttr(t, doc)
			for _, name := range tc.framing {
				if c := landmarkCentre(t, doc, name); !box.contains(c) {
					t.Errorf("viewBox %+v does not contain %s at %+v", box, name, c)
				}
			}
			if c := landmarkCentre(t, doc, tc.far); box.contains(c) {
				t.Errorf("viewBox %+v should leave out far landmark %s at %+v", box, tc.far, c)
			}
			if got, want := box.W/box.H, whole.W/whole.H; math.Abs(got-want) > 0.01 {
				t.Errorf("aspect ratio = %.4f, want whole-bike %.4f", got, want)
			}
			if box.W >= whole.W/2 {
				t.Errorf("close-up width %.1f should be well under whole-bike width %.1f", box.W, whole.W)
			}
			// Labels keep their on-screen size: SVG units per px scale with the
			// view width.
			if got, want := unitsPerPx(t, doc), wholeUnitsPerPx*box.W/whole.W; math.Abs(got-want) > 0.001 {
				t.Errorf("--units-per-px = %.4f, want %.4f (scaled by view width)", got, want)
			}
			if got, _ := doc.Find("svg.diagram").Attr("aria-label"); got != "Leveled bike geometry, "+tc.label {
				t.Errorf("svg aria-label = %q", got)
			}
			if got := hiddenViewValue(doc); got != tc.view {
				t.Errorf("hidden view input = %q, want %q", got, tc.view)
			}
		})
	}
}

func TestBikeWorkbenchViewLinks(t *testing.T) {
	path := "/bike?view=seat&fork=fox34_120-534x44&cup=12&sag=20&spacers=30&stem_length=70"
	doc := loadBikeDoc(t, path)
	current := decodeHref(t, path)

	links := doc.Find("nav.diagram-views a")
	if links.Length() != 3 {
		t.Fatalf("view links = %d, want 3", links.Length())
	}
	if doc.Find("form.controls a[data-diagram-view]").Length() != 0 {
		t.Errorf("view links must sit in the diagram card, not the controls form")
	}
	for i, view := range []diagramView{diagramViewBike, diagramViewSeat, diagramViewStem} {
		link := links.Eq(i)
		if id, _ := link.Attr("id"); id != "view-"+string(view) {
			t.Errorf("link %d id = %q, want view-%s", i, id, view)
		}
		if dv, _ := link.Attr("data-diagram-view"); dv != string(view) {
			t.Errorf("link %d data-diagram-view = %q, want %s", i, dv, view)
		}
		if _, hasPreset := link.Attr("data-preset"); hasPreset {
			t.Errorf("view link %s must not be a data-preset link", view)
		}
		href, _ := link.Attr("href")
		if got, want := decodeHref(t, href), current.WithView(string(view)); got.URL() != want.URL() {
			t.Errorf("view link %s href build = %s, want %s", view, got.URL(), want.URL())
		}
		_, isCurrent := link.Attr("aria-current")
		if isCurrent != (view == diagramViewSeat) {
			t.Errorf("view link %s aria-current = %v, want %v", view, isCurrent, view == diagramViewSeat)
		}
	}

	// The error page has no diagram and no view links.
	errDoc := loadBikeDocStatus(t, "/bike?view=seat&spacers=-1", http.StatusBadRequest)
	if errDoc.Find("a[data-diagram-view]").Length() != 0 {
		t.Errorf("400 page must not render view links")
	}
}

func TestBikeWorkbenchControlChangesAndPresetsKeepView(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?view=stem&spacers=30")
	canonical := activeViewHref(t, doc)
	state := decodeHref(t, canonical)
	if state.View != query.ViewStem || state.Setup.Stem.SpacersMm != 30 {
		t.Errorf("canonical %q: view=%q spacers=%v, want stem and 30", canonical, state.View, state.Setup.Stem.SpacersMm)
	}
	if got := hiddenViewValue(doc); got != "stem" {
		t.Errorf("hidden view input = %q, want stem", got)
	}

	// Submitting another control change from that page keeps the view.
	rec, next := submitForm(t, doc, map[string]string{"stem_length": "80"})
	if rec.Code != http.StatusOK {
		t.Fatalf("submit status = %d", rec.Code)
	}
	if got := hiddenViewValue(next); got != "stem" {
		t.Errorf("after control change hidden view = %q, want stem", got)
	}

	defaultsHref, _ := doc.Find("a#preset-defaults").Attr("href")
	if got, want := decodeHref(t, defaultsHref), query.DefaultBike().WithView(query.ViewStem); got.URL() != want.URL() {
		t.Errorf("Reset Defaults build = %s, want %s", got.URL(), want.URL())
	}
	basisHref, _ := doc.Find("a#preset-basis-fork").Attr("href")
	if got, want := decodeHref(t, basisHref), state.WithBasisFork(); got.URL() != want.URL() {
		t.Errorf("Chart Basis Fork build = %s, want %s", got.URL(), want.URL())
	}
	for _, href := range []string{defaultsHref, basisHref} {
		if got := hiddenViewValue(loadBikeDoc(t, href)); got != "stem" {
			t.Errorf("following preset %q: hidden view = %q, want stem", href, got)
		}
	}
}

func TestBikeWorkbenchUnknownViewRecoversThroughPresets(t *testing.T) {
	doc := loadBikeDocStatus(t, "/bike?view=zoom", http.StatusBadRequest)
	if msg := doc.Find("#form-error").Text(); !strings.Contains(msg, "unknown diagram view") {
		t.Errorf("form error = %q, want unknown diagram view", msg)
	}
	if doc.Find("svg.diagram").Length() != 0 {
		t.Errorf("400 page must not render the diagram")
	}
	if got := hiddenViewValue(doc); got != "zoom" {
		t.Errorf("hidden view input = %q, want the submitted zoom", got)
	}
	for _, id := range []string{"preset-defaults", "preset-basis-fork"} {
		href, _ := doc.Find("a#" + id).Attr("href")
		if got := decodeHref(t, href).View; got != query.ViewBike {
			t.Errorf("%s href %q view = %q, want bike", id, href, got)
		}
		next := loadBikeDoc(t, href)
		if next.Find("svg.diagram").Length() != 1 {
			t.Errorf("following %s must render the diagram", id)
		}
		if got := hiddenViewValue(next); got != "bike" {
			t.Errorf("following %s: hidden view = %q, want bike", id, got)
		}
	}

	// A bad control with a valid view echoes and keeps the view.
	badDoc := loadBikeDocStatus(t, "/bike?view=seat&spacers=abc", http.StatusBadRequest)
	if got := hiddenViewValue(badDoc); got != "seat" {
		t.Errorf("bad control: hidden view = %q, want seat", got)
	}
	for _, id := range []string{"preset-defaults", "preset-basis-fork"} {
		href, _ := badDoc.Find("a#" + id).Attr("href")
		if got := decodeHref(t, href).View; got != query.ViewSeat {
			t.Errorf("bad control: %s view = %q, want seat", id, got)
		}
		if got := hiddenViewValue(loadBikeDoc(t, href)); got != "seat" {
			t.Errorf("bad control: following %s: hidden view = %q, want seat", id, got)
		}
	}
}

func TestCloseUpViewBoxAndSizeFormatting(t *testing.T) {
	whole := svgViewBox{X: 0, Y: 0, W: 2000, H: 1000}
	pts := []svgPoint{{X: 900, Y: 400}, {X: 950, Y: 600}}
	box := closeUpViewBox(pts, whole, 100, 100)
	if math.Abs(box.W/box.H-2) > 1e-9 {
		t.Errorf("aspect = %v, want 2", box.W/box.H)
	}
	// Height-limited: the vertical pad is padMm scaled by the zoom.
	pad := 100 * box.W / whole.W
	if math.Abs(box.H-(200+2*pad)) > 1e-9 {
		t.Errorf("H = %v, want span 200 + 2×%v", box.H, pad)
	}
	if cx, cy := box.X+box.W/2, box.Y+box.H/2; math.Abs(cx-925) > 1e-9 || math.Abs(cy-500) > 1e-9 {
		t.Errorf("centre = (%v, %v), want (925, 500)", cx, cy)
	}
	// A framing as large as the whole view falls back to the whole view.
	if got := closeUpViewBox([]svgPoint{{X: 0, Y: 0}, {X: 2000, Y: 10}}, whole, 100, 100); got != whole {
		t.Errorf("oversized framing = %+v, want whole %+v", got, whole)
	}
	// Width-limited: the side pad uses padX, independent of padY.
	wide := closeUpViewBox([]svgPoint{{X: 900, Y: 500}, {X: 1100, Y: 510}}, whole, 300, 10)
	if padX := 300 * wide.W / whole.W; math.Abs(wide.W-(200+2*padX)) > 1e-9 {
		t.Errorf("W = %v, want span 200 + 2×%v", wide.W, padX)
	}

	for in, want := range map[float64]string{18: "18", 1.5: "1.5", 0: "0", -110: "-110", 5.142857: "5.14", 0.001: "0"} {
		if got := formatSVGSize(in); got != want {
			t.Errorf("formatSVGSize(%v) = %q, want %q", in, got, want)
		}
	}
}

// renderedRectCorners returns the four corners, in SVG user units, of a
// rendered rect with a "translate(x y) rotate(a)" transform.
func renderedRectCorners(t *testing.T, rect *goquery.Selection) []svgPoint {
	t.Helper()
	attr := func(name string) float64 {
		raw, _ := rect.Attr(name)
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			t.Fatalf("rect %s = %q: %v", name, raw, err)
		}
		return v
	}
	x, y, w, h := attr("x"), attr("y"), attr("width"), attr("height")
	transform, _ := rect.Attr("transform")
	var tx, ty, deg float64
	if _, err := fmt.Sscanf(transform, "translate(%g %g) rotate(%g)", &tx, &ty, &deg); err != nil {
		t.Fatalf("parse transform %q: %v", transform, err)
	}
	sin, cos := math.Sincos(deg * math.Pi / 180)
	var pts []svgPoint
	for _, lx := range []float64{x, x + w} {
		for _, ly := range []float64{y, y + h} {
			pts = append(pts, svgPoint{X: tx + lx*cos - ly*sin, Y: ty + lx*sin + ly*cos})
		}
	}
	return pts
}

func TestBikeWorkbenchCloseUpsContainTheirOutlines(t *testing.T) {
	cases := []struct {
		view       string
		components []string
		landmarks  []string
	}{
		{"seat", []string{"seatpost-clamp", "saddle-rail"}, nil},
		// The upper cover outline starts at HeadTubeTop, so that end of the
		// stack stays in view even though the head tube is not framed.
		{"stem", []string{"upper-cover", "spacers", "steerer-clamp", "stem-body"}, []string{"HeadTubeTop"}},
	}
	for _, tc := range cases {
		for _, query := range []string{"", "&spacers=0", "&spacers=30&stem_length=90&stem_angle=10"} {
			t.Run(tc.view+query, func(t *testing.T) {
				doc := loadBikeDoc(t, "/bike?view="+tc.view+query)
				box := parseViewBoxAttr(t, doc)
				for _, name := range tc.components {
					rect := doc.Find(fmt.Sprintf(`.component-outlines rect[data-component=%q]`, name))
					if rect.Length() != 1 {
						t.Fatalf("component %s: want 1 rect, got %d", name, rect.Length())
					}
					for _, c := range renderedRectCorners(t, rect) {
						if !box.contains(c) {
							t.Errorf("viewBox %+v cuts off %s corner %+v", box, name, c)
						}
					}
				}
				for _, name := range tc.landmarks {
					if c := landmarkCentre(t, doc, name); !box.contains(c) {
						t.Errorf("viewBox %+v does not contain %s at %+v", box, name, c)
					}
				}
			})
		}
	}
}

// Every view draws the same build: the readouts match across views, and only
// the framing changes.
func TestBikeWorkbenchViewsShareReadoutsAndDifferInFraming(t *testing.T) {
	cases := []struct{ path, view, viewBox string }{
		{"/bike?view=bike", "bike", "-902.07 -850.54 2079.62 1256.87"},
		{"/bike?view=seat", "seat", "-281.92 -724.42 167.71 101.36"},
		{"/bike?view=stem", "stem", "318.47 -768.85 239.50 144.75"},
		{"/bike?brp=off&grip=off&view=seat", "seat", "-266.63 -703.82 137.14 82.89"},
		{"/bike?brp=off&grip=off&view=stem", "stem", "300.54 -768.85 239.50 144.75"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			doc := loadBikeDoc(t, tc.path)
			if got := metricText(doc, "wheelbase"); got != "1145.6 mm" {
				t.Errorf("wheelbase = %q, want 1145.6 mm", got)
			}
			if got := metricText(doc, "brp-grip-dist"); got != "551.5 mm" {
				t.Errorf("brp-grip-dist = %q, want 551.5 mm", got)
			}
			if got, _ := doc.Find("svg.diagram").Attr("viewBox"); got != tc.viewBox {
				t.Errorf("viewBox = %q, want %q", got, tc.viewBox)
			}
			if got := hiddenViewValue(doc); got != tc.view {
				t.Errorf("hidden view input = %q, want %q", got, tc.view)
			}
		})
	}
}

func fittingTestTarget(t *testing.T, q url.Values) (x, y float64) {
	t.Helper()
	bike := mustPlace(t, mustBikeValues(t, q))
	brp := bike.Saddle.BRP.Origin()
	return brp.X, brp.Y
}

func hrefQuery(t *testing.T, doc *goquery.Document, selector string) url.Values {
	t.Helper()
	href, ok := doc.Find(selector).Attr("href")
	if !ok {
		t.Fatalf("%s has no href", selector)
	}
	u, err := url.Parse(href)
	if err != nil {
		t.Fatalf("parse %q: %v", href, err)
	}
	return u.Query()
}

func TestBikeWorkbenchWithoutBrpTargetHasNoFitting(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp=off&grip=off")
	if n := doc.Find("[data-fitting]").Length(); n != 0 {
		t.Errorf("fitting rows = %d, want 0", n)
	}
	if n := doc.Find("#apply-fitting").Length(); n != 0 {
		t.Errorf("apply-fitting links = %d, want 0", n)
	}
	for _, id := range []string{"ctrl-brp-x", "ctrl-brp-y"} {
		if v, _ := doc.Find("#" + id).Attr("value"); v != "" {
			t.Errorf("#%s value = %q, want blank", id, v)
		}
		if _, ok := doc.Find("#" + id).Attr("placeholder"); ok {
			t.Errorf("#%s has a placeholder; the example belongs in the note", id)
		}
	}
}

func TestBikeWorkbenchFitsFitReportBrpAtRailLimit(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?"+legacyBuild+"&brp_x=-219&brp_y=713.1&view=seat")
	metric := func(name string) string {
		return strings.TrimSpace(doc.Find(`[data-metric="` + name + `"]`).Text())
	}
	if got := metric("fit-saddle-outcome"); got != "limited by saddle rail offset" {
		t.Errorf("fit-saddle-outcome = %q", got)
	}
	if got := metric("fit-rail-offset"); got != "15.0 mm" {
		t.Errorf("fit-rail-offset = %q, want 15.0 mm", got)
	}
	if got := metric("fit-insertion"); got != "196.4 mm" {
		t.Errorf("fit-insertion = %q, want 196.4 mm", got)
	}
	q := hrefQuery(t, doc, "a#apply-fitting")
	if got := q.Get("rail_offset"); got != "15" {
		t.Errorf("apply rail_offset = %q, want 15", got)
	}
	if ext, err := strconv.ParseFloat(q.Get("post_insert"), 64); err != nil || math.Abs(ext-196.447506) > 1e-6 {
		t.Errorf("apply post_insert = %q (%v), want ≈196.447506", q.Get("post_insert"), err)
	}
	if q.Get("brp_x") != "-219" || q.Get("brp_y") != "713.1" || q.Get("grip") != "off" {
		t.Errorf("apply link lost the targets: %v", q)
	}
	if got, _ := doc.Find("a#apply-fitting").Attr("data-preset"); got != "apply-fitting" {
		t.Errorf("apply-fitting data-preset = %q", got)
	}
	// The diagram and form still show the submitted build, not the fitted one.
	if got, _ := doc.Find("#ctrl-post-insert").Attr("value"); got != "180.576" {
		t.Errorf("post_insert echo = %q, want the submitted 180.576", got)
	}
}

func TestBikeWorkbenchApplyFittingReachesReachableTarget(t *testing.T) {
	x, y := fittingTestTarget(t, url.Values{"post_insert": {"200"}, "rail_offset": {"5"}})
	path := "/bike?brp_x=" + formatFormFloat(x) + "&brp_y=" + formatFormFloat(y)
	doc := loadBikeDoc(t, path)
	if got := strings.TrimSpace(doc.Find(`[data-metric="fit-saddle-outcome"]`).Text()); got != "reached" {
		t.Fatalf("fit-saddle-outcome = %q, want reached", got)
	}
	applied, _ := doc.Find("a#apply-fitting").Attr("href")
	doc = loadBikeDoc(t, applied)
	if got := strings.TrimSpace(doc.Find(`[data-metric="fit-brp-miss"]`).Text()); got != "0.0 mm" {
		t.Errorf("fit-brp-miss after applying = %q, want 0.0 mm", got)
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="fit-saddle-outcome"]`).Text()); got != "reached" {
		t.Errorf("fit-saddle-outcome after applying = %q, want reached", got)
	}
}

// A half-entered target is rejected (Q70): the missing coordinate is marked
// invalid and the typed value stays in its box.
func TestBikeWorkbenchHalfEnteredTargetIsRejected(t *testing.T) {
	for _, c := range []struct{ path, typedID, typed, missingID string }{
		{"/bike?brp_y=710", "ctrl-brp-y", "710", "ctrl-brp-x"},
		{"/bike?brp_x=-219", "ctrl-brp-x", "-219", "ctrl-brp-y"},
		{"/bike?grip_y=726", "ctrl-grip-y", "726", "ctrl-grip-x"},
		{"/bike?grip_x=481", "ctrl-grip-x", "481", "ctrl-grip-y"},
		{"/bike?brp_y=710&grip_x=481&grip_y=726", "ctrl-brp-y", "710", "ctrl-brp-x"},
	} {
		doc := loadBikeDocStatus(t, c.path, http.StatusBadRequest)
		if n := doc.Find("[data-fitting]").Length(); n != 0 {
			t.Errorf("%s: fitting shown for a half target", c.path)
		}
		if v, _ := doc.Find("#" + c.typedID).Attr("value"); v != c.typed {
			t.Errorf("%s: #%s value = %q, want %q", c.path, c.typedID, v, c.typed)
		}
		if n := doc.Find("[aria-invalid]").Length(); n != 1 {
			t.Errorf("%s: %d invalid controls, want 1", c.path, n)
		}
		if v, _ := doc.Find("#" + c.missingID).Attr("aria-invalid"); v != "true" {
			t.Errorf("%s: #%s aria-invalid = %q, want true", c.path, c.missingID, v)
		}
	}
}

func TestBikeWorkbenchNonFiniteTargetStillErrors(t *testing.T) {
	doc := loadBikeDocStatus(t, "/bike?brp_x=NaN&brp_y=1", http.StatusBadRequest)
	if v, _ := doc.Find("#ctrl-brp-x").Attr("aria-invalid"); v != "true" {
		t.Errorf("non-finite ctrl-brp-x aria-invalid = %q, want true", v)
	}
}

func TestBikeWorkbenchTargetExampleIsInNote(t *testing.T) {
	doc := loadBikeDoc(t, "/bike")
	notes := doc.Find(`[data-notes-drawer] [data-note="targets-help"]`).Text()
	for _, want := range []string{"-219.0, 713.1", "481.0, 726.0"} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes %q missing example %q", notes, want)
		}
	}
}

func TestBikeWorkbenchBrpTargetPresetLinks(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp_x=-200&brp_y=710")
	if q := hrefQuery(t, doc, "a#preset-defaults"); q.Get("brp_x") != "-219" || q.Get("brp_y") != "713.1" {
		t.Errorf("Reset Defaults restores the default target: %v", q)
	}
	q := hrefQuery(t, doc, "a#preset-basis-fork")
	if q.Get("brp_x") != "-200" || q.Get("brp_y") != "710" {
		t.Errorf("Chart Basis Fork lost the target: %v", q)
	}
	if v := activeViewHref(t, doc); !strings.Contains(v, "brp_x=-200") {
		t.Errorf("canonical URL = %q, want it to keep the target", v)
	}

	cleared := loadBikeDoc(t, "/bike?brp=off")
	for _, sel := range []string{"a#preset-basis-fork", "a#view-seat", "a#apply-fitting"} {
		q := hrefQuery(t, cleared, sel)
		if q.Get("brp") != "off" || q.Has("brp_x") {
			t.Errorf("%s dropped the cleared BRP target: %v", sel, q)
		}
	}
}

func TestBikeWorkbenchRailOffsetRange(t *testing.T) {
	doc := loadBikeDoc(t, "/bike")
	rail := doc.Find("#ctrl-rail-offset")
	if min, _ := rail.Attr("min"); min != "-15" {
		t.Errorf("rail offset min = %q, want -15", min)
	}
	if max, _ := rail.Attr("max"); max != "15" {
		t.Errorf("rail offset max = %q, want 15", max)
	}
	loadBikeDoc(t, "/bike?rail_offset=15")
	doc = loadBikeDocStatus(t, "/bike?rail_offset=20", http.StatusBadRequest)
	if v, _ := doc.Find("#ctrl-rail-offset").Attr("aria-invalid"); v != "true" {
		t.Errorf("ctrl-rail-offset aria-invalid = %q, want true", v)
	}
}

func TestBikeWorkbenchSpacersMax(t *testing.T) {
	doc := loadBikeDoc(t, "/bike")
	if v, _ := doc.Find("#ctrl-spacers").Attr("max"); v != "30" {
		t.Errorf("ctrl-spacers max = %q, want 30", v)
	}
	loadBikeDocStatus(t, "/bike?spacers=30", http.StatusOK)
	doc = loadBikeDocStatus(t, "/bike?spacers=31", http.StatusBadRequest)
	if v, _ := doc.Find("#ctrl-spacers").Attr("aria-invalid"); v != "true" {
		t.Errorf("ctrl-spacers aria-invalid = %q, want true", v)
	}
	if v, _ := doc.Find("#ctrl-spacers").Attr("max"); v != "30" {
		t.Errorf("error page ctrl-spacers max = %q, want 30", v)
	}
}

func gripTestTarget(t *testing.T, q url.Values) (x, y float64) {
	t.Helper()
	bike := mustPlace(t, mustBikeValues(t, q))
	grip := bike.Handlebar.HandGrip.Origin()
	return grip.X, grip.Y
}

func metricText(doc *goquery.Document, name string) string {
	return strings.TrimSpace(doc.Find(`[data-metric="` + name + `"]`).Text())
}

func TestBikeWorkbenchWithoutGripTargetHasNoCockpitFitting(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp=off&grip=off")
	if n := doc.Find("[data-fitting]").Length() + doc.Find("#apply-fitting").Length(); n != 0 {
		t.Errorf("fitting elements = %d, want 0", n)
	}
	for _, id := range []string{"ctrl-grip-x", "ctrl-grip-y"} {
		if v, _ := doc.Find("#" + id).Attr("value"); v != "" {
			t.Errorf("#%s value = %q, want blank", id, v)
		}
		if _, ok := doc.Find("#" + id).Attr("placeholder"); ok {
			t.Errorf("#%s has a placeholder; the example belongs in the note", id)
		}
	}
}

func TestBikeWorkbenchFitsFitReportGripAtLimits(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp=off&grip_x=481&grip_y=726&view=stem")
	for name, want := range map[string]string{
		"fit-cockpit-outcome": "limited by stem spacers, stem length",
		"fit-spacers":         "0.0 mm",
		"fit-stem-length":     "130.0 mm",
		"fit-grip-miss":       "97.1 mm",
	} {
		if got := metricText(doc, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if n := doc.Find(`[data-metric="fit-saddle-outcome"]`).Length(); n != 0 {
		t.Errorf("saddle block present without a BRP target")
	}
	q := hrefQuery(t, doc, "a#apply-fitting")
	for k, want := range map[string]string{"spacers": "0", "stem_length": "130", "grip_x": "481", "grip_y": "726", "brp": "off"} {
		if q.Get(k) != want {
			t.Errorf("apply %s = %q, want %q", k, q.Get(k), want)
		}
	}
	if q.Has("brp_x") {
		t.Errorf("apply link has brp_x: %v", q)
	}
}

func TestBikeWorkbenchApplyFittingReachesReachableGrip(t *testing.T) {
	x, y := gripTestTarget(t, url.Values{"spacers": {"10"}, "stem_length": {"90"}})
	doc := loadBikeDoc(t, "/bike?grip_x="+formatFormFloat(x)+"&grip_y="+formatFormFloat(y))
	if got := metricText(doc, "fit-cockpit-outcome"); got != "reached" {
		t.Fatalf("fit-cockpit-outcome = %q, want reached", got)
	}
	applied, _ := doc.Find("a#apply-fitting").Attr("href")
	doc = loadBikeDoc(t, applied)
	if got := metricText(doc, "fit-grip-miss"); got != "0.0 mm" {
		t.Errorf("fit-grip-miss after applying = %q, want 0.0 mm", got)
	}
	if got := metricText(doc, "fit-cockpit-outcome"); got != "reached" {
		t.Errorf("fit-cockpit-outcome after applying = %q, want reached", got)
	}
}

func TestBikeWorkbenchBothFitTargets(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?"+legacyPost+"&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726")
	if doc.Find(`[data-metric="fit-saddle-outcome"]`).Length() != 1 || doc.Find(`[data-metric="fit-cockpit-outcome"]`).Length() != 1 {
		t.Fatal("want both fitting blocks")
	}
	if n := doc.Find("a#apply-fitting").Length(); n != 1 {
		t.Fatalf("apply links = %d, want 1", n)
	}
	q := hrefQuery(t, doc, "a#apply-fitting")
	if q.Get("rail_offset") != "15" || q.Get("spacers") != "0" || q.Get("stem_length") != "130" {
		t.Errorf("apply link values: %v", q)
	}
	if ext, err := strconv.ParseFloat(q.Get("post_insert"), 64); err != nil || math.Abs(ext-196.447506) > 1e-6 {
		t.Errorf("apply post_insert = %q, want ≈196.447506", q.Get("post_insert"))
	}
	for _, k := range []string{"brp_x", "brp_y", "grip_x", "grip_y"} {
		if !q.Has(k) {
			t.Errorf("apply link lost %s: %v", k, q)
		}
	}
}

func TestBikeWorkbenchFitTargetsPresetLinks(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp_x=-200&brp_y=710&grip_x=470&grip_y=720")
	q := hrefQuery(t, doc, "a#preset-defaults")
	for k, want := range map[string]string{"brp_x": "-219", "brp_y": "713.1", "grip_x": "481", "grip_y": "726"} {
		if q.Get(k) != want {
			t.Errorf("Reset Defaults %s = %q, want the default %q", k, q.Get(k), want)
		}
	}
	for _, sel := range []string{"a#preset-basis-fork", "a#view-seat"} {
		q = hrefQuery(t, doc, sel)
		if q.Get("grip_x") != "470" || q.Get("grip_y") != "720" || q.Get("brp_x") != "-200" {
			t.Errorf("%s lost a target: %v", sel, q)
		}
	}
	if v := activeViewHref(t, doc); !strings.Contains(v, "grip_x=470") {
		t.Errorf("canonical URL = %q, want it to keep the grip target", v)
	}
}

func buildCell(row *goquery.Selection, name string) string {
	return strings.TrimSpace(row.Find(`[data-build-cell="` + name + `"]`).Text())
}

func buildAttr(doc *goquery.Document, rank int, attr string) string {
	return strings.TrimSpace(doc.Find(fmt.Sprintf(`tr[data-build-breakdown-rank="%d"] [data-build-attr="%s"]`, rank, attr)).Text())
}

func TestBikeWorkbenchBuildRankingRequiresGrip(t *testing.T) {
	for _, path := range []string{"/bike?brp=off&grip=off", "/bike?grip=off", "/bike?brp_x=-219&brp_y=713.1&grip=off"} {
		if n := loadBikeDoc(t, path).Find("[data-build-ranking]").Length(); n != 0 {
			t.Errorf("%s: got %d build ranking blocks, want none", path, n)
		}
	}
}

func TestBikeWorkbenchRanksBuilds(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp=off&view=stem")
	rows := doc.Find("tr[data-build-rank]")
	if rows.Length() != 5 {
		t.Fatalf("build rows = %d, want 5", rows.Length())
	}
	if got := doc.Find("[data-build-evaluated]").Text(); got != "749" {
		t.Errorf("evaluated = %q, want 749", got)
	}
	const summary = "749 stem and spacer combinations placed; top 5 by fit tier and 0–100 weighted score (total weight 23; 4 saddle attributes omitted without BRP target). The saddle fit is kept."
	if got := doc.Find("[data-build-summary]").Text(); got != summary {
		t.Errorf("build summary = %q, want %q", got, summary)
	}
	if n := doc.Find("[data-fit-tier-legend] dt").Length(); n != 4 {
		t.Errorf("tier legend items = %d, want 4", n)
	}
	for name, want := range map[string]string{
		"tier": "Fits neither", "score": "57.1", "stem": "120 mm, -30°", "spacers": "0 mm", "grip-miss": "(+23.3, +21.6) mm · 31.7 mm",
		"oval-r": "1.78", "fork": "Lithic Carbon Mountain (495/44)", "brp-miss": "—",
	} {
		if got := buildCell(rows.First(), name); got != want {
			t.Errorf("row 1 %s = %q, want %q", name, got, want)
		}
	}
	for i, want := range []string{"57.1", "54.1", "52.0"} {
		if got := buildCell(rows.Eq(i), "score"); got != want {
			t.Errorf("row %d score = %q, want %q", i+1, got, want)
		}
	}
	headers := doc.Find("table.build-table thead th")
	if headers.Length() != 11 {
		t.Fatalf("build table headers = %d, want 11", headers.Length())
	}
	if got := headers.Eq(1).Text(); got != "Action" {
		t.Errorf("header 2 = %q, want Action", got)
	}
	if got := headers.Eq(2).Text(); got != "Tier" {
		t.Errorf("header 3 = %q, want Tier", got)
	}
	if got := headers.Eq(3).Text(); got != "Score" {
		t.Errorf("header 4 = %q, want Score", got)
	}
	if rows.First().Find("[data-build-attr]").Length() != 0 {
		t.Errorf("primary build table should not inline [data-build-attr] cells")
	}

	breakdownRows := doc.Find(`tr[data-build-breakdown-rank]`)
	if breakdownRows.Length() != 5 {
		t.Fatalf("breakdown rows = %d, want 5", breakdownRows.Length())
	}
	attrs := doc.Find(`tr[data-build-breakdown-rank="1"] [data-build-attr]`)
	breakdownHeaders := doc.Find("table.build-breakdown-table thead th")
	if attrs.Length() != 12 || breakdownHeaders.Length() != 17 {
		t.Fatalf("build attribute cells = %d, breakdown headers = %d; want 12 and 17", attrs.Length(), breakdownHeaders.Length())
	}
	for i, want := range []string{"Grip (6)", "BRP Y (6)", "BRP X (4)", "Stem (5)", "Sweep (3)", "Sag (3)", "Spacers (2)", "Rail (2)", "RC/WB (2)", "Post (1)", "BB drop (1)", "Trail (1)"} {
		if got := breakdownHeaders.Eq(5 + i).Text(); got != want {
			t.Errorf("attribute header %d = %q, want %q", i, got, want)
		}
	}
	for i, want := range []string{"19.5", "—", "—", "0.0", "100.0", "100.0", "100.0", "—", "100.0", "—", "100.0", "95.1"} {
		if got := strings.TrimSpace(attrs.Eq(i).Text()); got != want {
			t.Errorf("row 1 attribute %d = %q, want %q", i, got, want)
		}
	}
	firstCells := rows.First().Children()
	if firstCells.Eq(1).Find("a[data-build-apply]").Length() != 1 {
		t.Error("row 1 cell 2 has no apply link")
	}
	if got, _ := firstCells.Eq(2).Attr("data-build-cell"); got != "tier" {
		t.Errorf("row 1 cell 3 = %q, want tier", got)
	}
	if got, _ := firstCells.Eq(3).Attr("data-build-cell"); got != "score" {
		t.Errorf("row 1 cell 4 = %q, want score", got)
	}
	links := doc.Find("a[data-build-apply]")
	if links.Length() != 5 {
		t.Fatalf("build apply links = %d, want 5", links.Length())
	}
	links.Each(func(i int, link *goquery.Selection) {
		if got, _ := link.Attr("data-preset"); got != "apply-build" {
			t.Errorf("link %d data-preset = %q, want apply-build", i+1, got)
		}
	})
	q := hrefQuery(t, doc, `tr[data-build-rank="1"] a[data-build-apply]`)
	for k, want := range map[string]string{
		"stem_angle": "-30", "stem_length": "120", "spacers": "0", "grip_x": "481", "grip_y": "726", "view": "stem",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("build apply %s = %q, want %q", k, got, want)
		}
	}
	if q.Has("brp_x") || q.Has("brp_y") {
		t.Errorf("build apply link contains a BRP target: %v", q)
	}
	if got := metricText(doc, "fit-cockpit-outcome"); got != "limited by stem spacers, stem length" {
		t.Errorf("continuous outcome = %q, want limited by stem spacers, stem length", got)
	}
}

func assertSelectedBarOptions(t *testing.T, doc *goquery.Document, want string) {
	t.Helper()
	selected := doc.Find(`select[name="bar_options"] option[selected]`)
	if got, _ := selected.Attr("value"); selected.Length() != 1 || got != want {
		t.Errorf("selected bar_options = %q (%d selected options), want %q", got, selected.Length(), want)
	}
}

func TestBikeWorkbenchBarOptionsDefaultKeepsSubmittedBar(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp=off")
	if n := doc.Find(`select[name="bar_options"] option`).Length(); n != 2 {
		t.Errorf("bar_options options = %d, want 2", n)
	}
	assertSelectedBarOptions(t, doc, "selected")
	if got := doc.Find("[data-build-evaluated]").Text(); got != "749" {
		t.Errorf("evaluated = %q, want 749", got)
	}
	const summary = "749 stem and spacer combinations placed; top 5 by fit tier and 0–100 weighted score (total weight 23; 4 saddle attributes omitted without BRP target). The saddle fit is kept."
	if got := doc.Find("[data-build-summary]").Text(); got != summary {
		t.Errorf("build summary = %q, want %q", got, summary)
	}
	rows := doc.Find("tr[data-build-rank]")
	if rows.Length() != 5 {
		t.Fatalf("build rows = %d, want 5", rows.Length())
	}
	rows.Each(func(i int, row *goquery.Selection) {
		if got := buildCell(row, "bar"); got != "Tumbleweed Persuader 20° (800 mm)" {
			t.Errorf("row %d bar = %q, want Tumbleweed Persuader 20° (800 mm)", i+1, got)
		}
		q := hrefQuery(t, doc, fmt.Sprintf(`tr[data-build-rank="%d"] a[data-build-apply]`, i+1))
		if q.Get("bar") != "persuader20_800" || q.Has("bar_options") {
			t.Errorf("row %d apply query = %v, want submitted bar and omitted bar_options", i+1, q)
		}
	})
	if q := workbenchCanonicalQuery(t, doc); q.Has("bar_options") {
		t.Errorf("canonical query contains bar_options: %v", q)
	}
}

func workbenchCanonicalQuery(t *testing.T, doc *goquery.Document) url.Values {
	t.Helper()
	u, err := url.Parse(activeViewHref(t, doc))
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

const (
	barAlloy50  = "SQlab 311 2.0 Alloy 16°, 50 mm rise (740 mm)"
	barFLX12x15 = "SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm)"
	barFLX12x30 = "SQlab 311 FL-X Carbon 12°, 30 mm rise (740 mm)"
	bar3OX16x45 = "SQlab 3OX 16°, 45 mm rise (780 mm)"
	barFLX16x30 = "SQlab 311 FL-X Carbon 16°, 30 mm rise (740 mm)"
)

type barOptionRow struct {
	bar, stem, spacers, ovalR, score string
}

func assertBarOptionRows(t *testing.T, doc *goquery.Document, want []barOptionRow) {
	t.Helper()
	rows := doc.Find("tr[data-build-rank]")
	if rows.Length() != len(want) {
		t.Fatalf("build rows = %d, want %d", rows.Length(), len(want))
	}
	for i, row := range want {
		for cell, value := range map[string]string{
			"bar": row.bar, "stem": row.stem, "spacers": row.spacers, "oval-r": row.ovalR, "score": row.score,
		} {
			if got := buildCell(rows.Eq(i), cell); got != value {
				t.Errorf("row %d %s = %q, want %q", i+1, cell, got, value)
			}
		}
	}
}

func TestBikeWorkbenchFitsBuildsAcrossAllBars(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?"+legacyPost+"&bar_options=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726&view=stem")
	if got := doc.Find("[data-build-evaluated]").Text(); got != "11235" {
		t.Errorf("evaluated = %q, want 11235", got)
	}
	if got := doc.Find("[data-build-evaluated]").Parent().Text(); !strings.Contains(got, "handlebar, stem, and spacer combinations") {
		t.Errorf("build count note = %q", got)
	}
	want := []barOptionRow{
		{barAlloy50, "120 mm, -30°", "0 mm", "0.90", "67.8"},
		{barFLX12x15, "120 mm, -10°", "0 mm", "0.63", "67.5"},
		{barFLX12x15, "110 mm, -17°", "0 mm", "0.84", "67.4"},
		{barFLX12x30, "110 mm, -17°", "0 mm", "0.95", "67.2"},
		{barFLX12x15, "120 mm, -17°", "5 mm", "0.25", "66.7"},
	}
	assertBarOptionRows(t, doc, want)
	if got := buildCell(doc.Find(`tr[data-build-rank="1"]`), "tier"); got != "Fits grip" {
		t.Errorf("row 1 tier = %q, want Fits grip", got)
	}

	const rowTwoApply = `tr[data-build-rank="2"] a[data-build-apply]`
	q := hrefQuery(t, doc, rowTwoApply)
	for name, value := range map[string]string{
		"bar": "sqlab311_flx_12_15", "bar_options": "all", "stem_length": "120", "stem_angle": "-10",
		"spacers": "0", "bar_roll": "0", "rail_offset": "15", "brp_x": "-219", "grip_x": "481",
	} {
		if got := q.Get(name); got != value {
			t.Errorf("row 2 apply %s = %q, want %q", name, got, value)
		}
	}
	if q := hrefQuery(t, doc, "#apply-fitting"); q.Get("bar") != "persuader20_800" || q.Get("bar_options") != "all" {
		t.Errorf("continuous apply query = %v, want submitted bar and all bar options", q)
	}
	selected := loadBikeDoc(t, "/bike?"+legacyPost+"&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726&view=stem")
	for _, metric := range []string{"fit-spacers", "fit-stem-length", "fit-cockpit-outcome"} {
		if got, want := metricText(doc, metric), metricText(selected, metric); got != want {
			t.Errorf("continuous %s = %q, want %q from selected mode", metric, got, want)
		}
	}

	t.Run("apply row two", func(t *testing.T) {
		href, _ := doc.Find(rowTwoApply).Attr("href")
		applied := loadBikeDoc(t, href)
		assertSelectedHandlebar(t, applied, "sqlab311_flx_12_15")
		assertSelectedBarOptions(t, applied, "all")
		assertBarOptionRows(t, applied, want)
	})
	t.Run("different submitted bar", func(t *testing.T) {
		other := loadBikeDoc(t, "/bike?bar_options=all&bar=sqlab3ox_12_15&brp=off")
		for i, score := range []string{"75.7", "75.2", "75.0", "74.8", "73.9"} {
			row := other.Find("tr[data-build-rank]").Eq(i)
			if got := buildCell(row, "score"); got != score {
				t.Errorf("row %d score = %q, want %s", i+1, got, score)
			}
			if got := buildCell(row, "tier"); got != "Fits grip" {
				t.Errorf("row %d tier = %q, want Fits grip", i+1, got)
			}
		}
	})
}

func TestBikeWorkbenchFitsBuildsKeepsSubmittedBarRoll(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?bar_options=all&bar_roll=5&brp=off")
	assertBarOptionRows(t, doc, []barOptionRow{
		{barAlloy50, "120 mm, -30°", "0 mm", "0.61", "76.5"},
		{barFLX12x15, "110 mm, -17°", "0 mm", "0.62", "75.6"},
		{barFLX16x30, "120 mm, -30°", "5 mm", "0.96", "75.5"},
		{barFLX16x30, "120 mm, -30°", "0 mm", "0.97", "75.5"},
		{bar3OX16x45, "120 mm, -30°", "0 mm", "0.99", "75.5"},
	})
	for i := range 5 {
		q := hrefQuery(t, doc, fmt.Sprintf(`tr[data-build-rank="%d"] a[data-build-apply]`, i+1))
		if got := q.Get("bar_roll"); got != "5" {
			t.Errorf("row %d apply bar_roll = %q, want 5", i+1, got)
		}
	}
}

func TestBikeWorkbenchBuildSweepColumn(t *testing.T) {
	for _, tc := range []struct {
		name, query, score string
	}{
		{"all bars", "bar_options=all", "100.0"},
		{"selected bar", "bar=persuader20_800", "100.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := loadBikeDoc(t, "/bike?"+tc.query+"&brp=off")
			rows := doc.Find("tr[data-build-rank]")
			if rows.Length() != 5 {
				t.Fatalf("build rows = %d, want 5", rows.Length())
			}
			checked := doc.Find("tr[data-build-breakdown-rank]")
			if tc.query == "bar_options=all" {
				checked = checked.First() // later rows may install other bars
			}
			checked.Each(func(i int, row *goquery.Selection) {
				cells := row.Find(`[data-build-attr="Handlebar sweep"]`)
				if cells.Length() != 1 || strings.TrimSpace(cells.Text()) != tc.score {
					t.Errorf("row %d sweep score = %q (%d cells), want %s in one cell", i+1, cells.Text(), cells.Length(), tc.score)
				}
			})
		})
	}
}

func TestBikeWorkbenchKeepsBarOptionsWithoutTarget(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?bar_options=all&view=seat&brp=off&grip=off")
	if n := doc.Find("[data-build-ranking]").Length(); n != 0 {
		t.Errorf("build ranking blocks = %d, want none", n)
	}
	assertSelectedBarOptions(t, doc, "all")
	if got := workbenchCanonicalQuery(t, doc).Get("bar_options"); got != "all" {
		t.Errorf("canonical bar_options = %q, want all", got)
	}
	for _, sel := range []string{"#view-bike", "#view-seat", "#view-stem", "#preset-basis-fork"} {
		if got := hrefQuery(t, doc, sel).Get("bar_options"); got != "all" {
			t.Errorf("%s bar_options = %q, want all", sel, got)
		}
	}
	if q := hrefQuery(t, doc, "#preset-defaults"); q.Has("bar_options") {
		t.Errorf("Reset Defaults query contains bar_options: %v", q)
	}
}

func TestBikeWorkbenchSubmitsBarOptionsSelect(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp=off")
	rec, next := submitForm(t, doc, map[string]string{"bar_options": "all"})
	if rec.Code != http.StatusOK {
		t.Fatalf("bar_options submission status = %d, want 200", rec.Code)
	}
	if got := next.Find("[data-build-evaluated]").Text(); got != "11235" {
		t.Errorf("evaluated = %q, want 11235", got)
	}
	if got := buildCell(next.Find(`tr[data-build-rank="1"]`), "bar"); got != barAlloy50 {
		t.Errorf("row 1 bar = %q, want %q", got, barAlloy50)
	}
}

func TestBikeWorkbenchBarOptionsErrorPage(t *testing.T) {
	doc := loadBikeDocStatus(t, "/bike?bar_options=some", http.StatusBadRequest)
	if got, _ := doc.Find(`select[name="bar_options"]`).Attr("aria-invalid"); got != "true" {
		t.Errorf("bar_options aria-invalid = %q, want true", got)
	}
	if got := strings.TrimSpace(doc.Find("#form-error").Text()); !strings.Contains(got, `unknown handlebar options value "some"`) {
		t.Errorf("bar_options error message = %q", got)
	}

	doc = loadBikeDocStatus(t, "/bike?bar_options=all&spacers=-1", http.StatusBadRequest)
	assertSelectedBarOptions(t, doc, "all")
	if got := hrefQuery(t, doc, "#preset-basis-fork").Get("bar_options"); got != "all" {
		t.Errorf("error-page basis preset bar_options = %q, want all", got)
	}
}

func TestBikeWorkbenchAppliesFreeScoredBuild(t *testing.T) {
	x, y := gripTestTarget(t, url.Values{"stem_angle": {"0"}, "stem_length": {"70"}, "spacers": {"10"}})
	doc := loadBikeDoc(t, "/bike?brp=off&grip_x="+formatFormFloat(x)+"&grip_y="+formatFormFloat(y))
	const rankOne = `tr[data-build-rank="1"]`
	row := doc.Find(rankOne)
	for name, want := range map[string]string{"stem": "70 mm, +0°", "spacers": "10 mm", "oval-r": "0.00", "score": "99.8", "tier": "Fits grip"} {
		if got := buildCell(row, name); got != want {
			t.Errorf("row 1 %s = %q, want %q", name, got, want)
		}
	}
	applied, ok := row.Find("a[data-build-apply]").Attr("href")
	if !ok {
		t.Fatal("missing rank 1 apply link")
	}
	doc = loadBikeDoc(t, applied)
	if got := metricText(doc, "fit-cockpit-outcome"); got != "reached" {
		t.Errorf("continuous outcome after applying = %q, want reached", got)
	}
	if got := metricText(doc, "fit-grip-miss"); got != "0.0 mm" {
		t.Errorf("HandGrip miss after applying = %q, want 0.0 mm", got)
	}
	if got := buildCell(doc.Find(rankOne), "stem"); got != "70 mm, +0°" {
		t.Errorf("row 1 stem after applying = %q, want 70 mm, +0°", got)
	}
}

func TestBikeWorkbenchBuildRankingKeepsSaddleFitAndTargets(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?"+legacyPost+"&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726")
	row := doc.Find(`tr[data-build-rank="1"]`)
	for name, want := range map[string]string{"tier": "Fits neither", "score": "55.9", "brp-miss": "(-49.8, +0.0) mm"} {
		if got := buildCell(row, name); got != want {
			t.Errorf("row 1 %s = %q, want %q", name, got, want)
		}
	}
	q := hrefQuery(t, doc, `tr[data-build-rank="1"] a[data-build-apply]`)
	for k, want := range map[string]string{
		"rail_offset": "15", "stem_angle": "-30", "stem_length": "120", "spacers": "0",
		"brp_x": "-219", "brp_y": "713.1", "grip_x": "481", "grip_y": "726",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("build apply %s = %q, want %q", k, got, want)
		}
	}
	if ext, err := strconv.ParseFloat(q.Get("post_insert"), 64); err != nil || math.Abs(ext-196.447506) > 1e-6 {
		t.Errorf("build apply post_insert = %q, want ≈196.447506", q.Get("post_insert"))
	}
	if n := doc.Find("#apply-fitting").Length(); n != 1 {
		t.Errorf("continuous apply links = %d, want 1", n)
	}
}

func TestWorkbenchHandlebarReferenceHandGrips(t *testing.T) {
	bars := catalog.Handlebars()
	want := []struct {
		id            string
		x, y, lateral float64
	}{
		{"persuader20_800", 388.546580, 755.539727, 330},
		{"persuader20_840", 381.267176, 755.539727, 350},
		{"sqlab3ox_16_15", 387.404312, 743.021197, 320},
		{"sqlab3ox_16_30", 391.275374, 757.077185, 320},
		{"sqlab3ox_16_45", 395.146437, 771.133174, 320},
		{"sqlab3ox_12_15", 406.062801, 743.021197, 320},
		{"sqlab3ox_12_30", 408.932314, 757.077185, 320},
		{"sqlab3ox_12_45", 411.801828, 771.133174, 320},
		{"sqlab311_20_16_15", 393.139219, 745.688230, 300},
		{"sqlab311_20_16_25", 395.719928, 754.900832, 300},
		{"sqlab311_20_16_50", 402.171699, 777.932337, 300},
		{"sqlab311_flx_16_15", 393.139219, 741.622661, 300},
		{"sqlab311_flx_16_30", 397.010282, 755.678649, 300},
		{"sqlab311_flx_12_15", 410.313932, 741.622661, 300},
		{"sqlab311_flx_12_30", 413.183446, 755.678649, 300},
	}
	if len(bars) != len(want) {
		t.Fatalf("catalog has %d handlebars, want %d", len(bars), len(want))
	}
	for i, ref := range want {
		t.Run(ref.id, func(t *testing.T) {
			opt := bars[i]
			if opt.ID != ref.id {
				t.Fatalf("bar %d = %q, want %q", i, opt.ID, ref.id)
			}
			placed := mustPlace(t, mustBikeValues(t, url.Values{"bar": {opt.ID}}))
			grip := placed.Handlebar.HandGrip.Origin()
			if math.Abs(grip.X-ref.x) > 1e-3 || math.Abs(grip.Y-ref.y) > 1e-3 {
				t.Errorf("HandGrip = %v, want (%.3f, %.3f) within 0.001 mm", grip, ref.x, ref.y)
			}
			if got := placed.Handlebar.HandLateralMm; got != ref.lateral {
				t.Errorf("HandLateralMm = %v, want %v", got, ref.lateral)
			}
		})
	}
}

func assertSelectedHandlebar(t *testing.T, doc *goquery.Document, want string) {
	t.Helper()
	selected := doc.Find(`select[name="bar"] option[selected]`)
	if got, _ := selected.Attr("value"); selected.Length() != 1 || got != want {
		t.Errorf("selected handlebar = %q (%d selected options), want %q", got, selected.Length(), want)
	}
}

func TestBikeWorkbenchDefaultHandlebar(t *testing.T) {
	doc := loadBikeDoc(t, "/bike")
	if got := doc.Find(`select[name="bar"] option`).Length(); got != 15 {
		t.Errorf("handlebar options = %d, want 15", got)
	}
	assertSelectedHandlebar(t, doc, "persuader20_800")
	canonical := activeViewHref(t, doc)
	u, err := url.Parse(canonical)
	if err != nil {
		t.Fatalf("parse canonical URL: %v", err)
	}
	if got := u.Query().Get("bar"); got != "persuader20_800" {
		t.Errorf("canonical bar = %q, want persuader20_800", got)
	}
	if got := metricText(doc, "brp-grip-dist"); got != "551.5 mm" {
		t.Errorf("BRP to HandGrip = %q, want 551.5 mm", got)
	}
}

func TestBikeWorkbenchChosenHandlebarChangesBuild(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?"+legacyPost+"&bar=sqlab3ox_12_15&view=stem")
	assertSelectedHandlebar(t, doc, "sqlab3ox_12_15")
	for metric, want := range map[string]string{
		"hand-lateral": "320.0 mm", "brp-grip-dist": "564.7 mm", "sitbones-grip-rise": "+4.6 mm",
	} {
		if got := metricText(doc, metric); got != want {
			t.Errorf("%s = %q, want %q", metric, got, want)
		}
	}
	const name = "SQlab 3OX 12°, 15 mm rise (780 mm)"
	if label := doc.Find(`[data-metric="brp-grip-dist"]`).Parent().Text(); !strings.Contains(label, name) {
		t.Errorf("BRP to HandGrip label = %q, want it to contain %q", label, name)
	}
}

func TestBikeWorkbenchChangingHandlebarKeepsRoll(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?bar_roll=5")
	rec, next := submitForm(t, doc, map[string]string{"bar": "sqlab3ox_16_30"})
	if rec.Code != http.StatusOK {
		t.Fatalf("changing bar: status = %d, want 200", rec.Code)
	}
	assertSelectedHandlebar(t, next, "sqlab3ox_16_30")
	if got, _ := next.Find(`input[name="bar_roll"]`).Attr("value"); got != "5.0" {
		t.Errorf("bar_roll = %q, want 5.0", got)
	}
}

func TestBikeWorkbenchPresetAndViewLinksKeepHandlebar(t *testing.T) {
	const bar = "sqlab3ox_16_30"
	doc := loadBikeDoc(t, "/bike?bar="+bar+"&view=seat")
	for _, selector := range []string{
		`a[data-diagram-view="bike"]`, `a[data-diagram-view="seat"]`, `a[data-diagram-view="stem"]`,
		`a[data-preset="basis-fork"]`,
	} {
		if got := hrefQuery(t, doc, selector).Get("bar"); got != bar {
			t.Errorf("%s bar = %q, want %q", selector, got, bar)
		}
	}
	const defaults = `a[data-preset="defaults"]`
	if got := hrefQuery(t, doc, defaults).Get("bar"); got != "persuader20_800" {
		t.Errorf("Reset Defaults bar = %q, want persuader20_800", got)
	}

	errorDoc := loadBikeDocStatus(t, "/bike?bar="+bar+"&stem_length=0", http.StatusBadRequest)
	assertSelectedHandlebar(t, errorDoc, bar)
	basisQuery := hrefQuery(t, errorDoc, `a[data-preset="basis-fork"]`)
	if got := basisQuery.Get("bar"); got != bar {
		t.Errorf("Chart Basis Fork bar on error page = %q, want %q", got, bar)
	}
	badBarDoc := loadBikeDocStatus(t, "/bike?bar=unknown", http.StatusBadRequest)
	selBar := badBarDoc.Find(`select[name="bar"] option[selected]`)
	if selBar.Length() != 1 || strings.TrimSpace(selBar.Text()) != "Invalid: unknown" {
		t.Errorf("unknown bar selected = %d %q, want 1 'Invalid: unknown'", selBar.Length(), selBar.Text())
	}
}

func TestBikeWorkbenchFitsAroundChosenHandlebar(t *testing.T) {
	const bar = "sqlab3ox_12_15"
	doc := loadBikeDoc(t, "/bike?bar="+bar+"&brp=off&view=stem")
	for metric, want := range map[string]string{
		"fit-cockpit-outcome": "limited by stem spacers", "fit-spacers": "0.0 mm", "fit-stem-length": "124.9 mm",
	} {
		if got := metricText(doc, metric); got != want {
			t.Errorf("%s = %q, want %q", metric, got, want)
		}
	}
	if got := hrefQuery(t, doc, "#apply-fitting").Get("bar"); got != bar {
		t.Errorf("Apply fitted build bar = %q, want %q", got, bar)
	}
	const firstRow = `tr[data-build-rank="1"]`
	for cell, want := range map[string]string{
		"tier": "Fits grip", "stem": "120 mm, -17°", "spacers": "0 mm", "oval-r": "0.45", "score": "73.4",
	} {
		if got := buildCell(doc.Find(firstRow), cell); got != want {
			t.Errorf("row 1 %s = %q, want %q", cell, got, want)
		}
	}
	const firstApply = firstRow + " a[data-build-apply]"
	q := hrefQuery(t, doc, firstApply)
	for field, want := range map[string]string{
		"bar": bar, "stem_length": "120", "stem_angle": "-17", "spacers": "0",
	} {
		if got := q.Get(field); got != want {
			t.Errorf("row 1 Apply %s = %q, want %q", field, got, want)
		}
	}
	doc.Find("a[data-build-apply]").Each(func(i int, link *goquery.Selection) {
		href, _ := link.Attr("href")
		if u, err := url.Parse(href); err != nil || u.Query().Get("bar") != bar {
			t.Errorf("row %d Apply URL = %q, want bar=%s", i+1, href, bar)
		}
	})
	href, _ := doc.Find(firstApply).Attr("href")
	applied := loadBikeDoc(t, href)
	assertSelectedHandlebar(t, applied, bar)
	if got := buildCell(applied.Find(firstRow), "stem"); got != "120 mm, -17°" {
		t.Errorf("applied row 1 stem = %q, want 120 mm, -17°", got)
	}
}

func TestBikeWorkbenchSeatpostSelect(t *testing.T) {
	posts := catalog.Seatposts()
	doc := loadBikeDoc(t, "/bike?post=setback_0")

	options := doc.Find(`select[name="post"] option`)
	if options.Length() != len(posts) {
		t.Fatalf("seatpost option count = %d, want %d", options.Length(), len(posts))
	}
	for i, want := range posts {
		opt := options.Eq(i)
		if got, _ := opt.Attr("value"); got != want.ID {
			t.Errorf("option %d value = %q, want %q", i, got, want.ID)
		}
	}
	if got, _ := doc.Find(`select[name="post"] option[selected]`).Attr("value"); got != "setback_0" {
		t.Errorf("selected seatpost = %q, want setback_0", got)
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="post-setback"]`).Text()); got != "0.0 mm" {
		t.Errorf("post-setback = %q, want 0.0 mm", got)
	}
	if note := doc.Find(`[data-note="post-setback"]`).Text(); !strings.Contains(note, "insertion 100–290 mm (frame and post) · length 400 mm · clamp 30 mm") {
		t.Errorf("post-setback note = %q, want insertion 100–290 mm (frame and post) · length 400 mm · clamp 30 mm", note)
	}
	if doc.Find(`[data-note="redshift-mixed-state"]`).Length() != 0 {
		t.Errorf("setback_0 page renders the Redshift mixed-state note")
	}
	canonHref := activeViewHref(t, doc)
	if canonURL, err := url.Parse(canonHref); err != nil || canonURL.Query().Get("post") != "setback_0" || canonURL.Query().Has("post_setback") {
		t.Errorf("canonical URL = %q, want post=setback_0 and no post_setback", canonHref)
	}

	defaultDoc := loadBikeDoc(t, "/bike")
	if got, _ := defaultDoc.Find(`select[name="post"] option[selected]`).Attr("value"); got != "redshift_shockstop_pro" {
		t.Errorf("default selected seatpost = %q, want redshift_shockstop_pro", got)
	}
	redshiftDoc := loadBikeDoc(t, "/bike?post=redshift_shockstop_pro")
	if got := strings.TrimSpace(redshiftDoc.Find(`[data-metric="post-setback"]`).Text()); got != "12.0 mm" {
		t.Errorf("redshift post-setback = %q, want 12.0 mm", got)
	}
	bike := mustPlace(t, mustBike(t, "post=redshift_shockstop_pro"))
	wantHeight := fmt.Sprintf("%.1f mm", bike.SaddleHeightMm())
	if got := strings.TrimSpace(redshiftDoc.Find(`[data-metric="brp-height"]`).Text()); got != wantHeight {
		t.Errorf("redshift brp-height = %q, want %q", got, wantHeight)
	}
}

func TestBikeWorkbenchSeatpostSurvivesLinks(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?post=setback_15&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726&view=seat")
	assertHasPost := func(label, href string) {
		t.Helper()
		u, err := url.Parse(href)
		if err != nil {
			t.Fatalf("%s invalid URL %q: %v", label, href, err)
		}
		if got := u.Query().Get("post"); got != "setback_15" {
			t.Errorf("%s post = %q, want setback_15 (%s)", label, got, href)
		}
	}
	canon := activeViewHref(t, doc)
	assertHasPost("canonical", canon)
	doc.Find(`a[data-diagram-view]`).Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		assertHasPost(fmt.Sprintf("diagram-view %d", i), href)
	})
	basisHref, _ := doc.Find(`a[data-preset="basis-fork"]`).Attr("href")
	assertHasPost("basis-fork", basisHref)
	applyHref, _ := doc.Find(`#apply-fitting`).Attr("href")
	assertHasPost("apply-fitting", applyHref)
	buildLinks := doc.Find(`[data-build-apply]`)
	if buildLinks.Length() == 0 {
		t.Fatal("expected [data-build-apply] links")
	}
	buildLinks.Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		assertHasPost(fmt.Sprintf("build-apply %d", i), href)
	})
	defaultsHref, _ := doc.Find(`a[data-preset="defaults"]`).Attr("href")
	if u, err := url.Parse(defaultsHref); err != nil || u.Query().Get("post") != catalog.DefaultSeatpostID {
		t.Errorf("defaults href = %q, want post=%s", defaultsHref, catalog.DefaultSeatpostID)
	}
}

// Retired and unknown query keys are rejected, not silently ignored (Q111).
func TestBikeWorkbenchRejectsUnknownAndRetiredKeys(t *testing.T) {
	for _, key := range []string{"post_setback", "stock_bars", "stock_forks", "post_ext", "fram", "bar_option"} {
		t.Run(key, func(t *testing.T) {
			doc := loadBikeDocStatus(t, "/bike?"+key+"=all", http.StatusBadRequest)
			msg := strings.TrimSpace(doc.Find("#form-error").Text())
			if !strings.Contains(msg, fmt.Sprintf("unknown query parameter %q", key)) {
				t.Errorf("#form-error = %q, want unknown query parameter %q", msg, key)
			}
			if doc.Find("svg.diagram").Length() != 0 {
				t.Errorf("400 page must not render the diagram")
			}
			if doc.Find("form.controls").Length() != 1 {
				t.Errorf("400 page must keep the editable controls form")
			}
		})
	}
}

func TestBikeWorkbenchUnknownSeatpost(t *testing.T) {
	doc := loadBikeDocStatus(t, "/bike?post=unknown", http.StatusBadRequest)
	if inv, _ := doc.Find(`select[name="post"]`).Attr("aria-invalid"); inv != "true" {
		t.Errorf("select[name=post] aria-invalid = %q, want true", inv)
	}
	if msg := doc.Find("#form-error").Text(); !strings.Contains(msg, `unknown seatpost "unknown"`) {
		t.Errorf("#form-error = %q, want unknown seatpost %q", msg, "unknown")
	}
}

func TestBikeWorkbenchSetbackNotesFitReport(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?"+legacyPost+"&grip=off&brp_x=-219&brp_y=713.1")
	if got := metricText(doc, "fit-saddle-outcome"); got != "limited by saddle rail offset" {
		t.Errorf("fit-saddle-outcome = %q, want limited by saddle rail offset", got)
	}
	for k, want := range map[string]string{
		"fit-setback-centered":           "62.5 mm",
		"fit-setback-centered-insertion": "179.0 mm",
		"fit-setback-centered-outcome":   "reached",
		"fit-setback-reach-rail":         "+15.0 mm",
		"fit-setback-reach":              "48.0 mm",
		"fit-setback-reach-insertion":    "183.1 mm",
		"fit-setback-reach-outcome":      "reached",
	} {
		if got := metricText(doc, k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}

	// Notes are never applied: #apply-fitting keeps post=setback_0.
	applyHref, _ := doc.Find(`#apply-fitting`).Attr("href")
	if u, err := url.Parse(applyHref); err != nil || u.Query().Get("post") != "setback_0" {
		t.Errorf("#apply-fitting href = %q, want post=setback_0", applyHref)
	}
	applied := loadBikeDoc(t, applyHref)
	if got := metricText(applied, "post-setback"); got != "0.0 mm" {
		t.Errorf("applied post-setback = %q, want 0.0 mm", got)
	}

	// Changing the post to setback_25 keeps the same note values while fit-brp-miss changes.
	doc25 := loadBikeDoc(t, "/bike?post=setback_25&post_insert=180.576&grip=off&brp_x=-219&brp_y=713.1")
	for _, k := range []string{
		"fit-setback-centered", "fit-setback-centered-insertion", "fit-setback-centered-outcome",
		"fit-setback-reach-rail", "fit-setback-reach", "fit-setback-reach-insertion", "fit-setback-reach-outcome",
	} {
		if got, want := metricText(doc25, k), metricText(doc, k); got != want {
			t.Errorf("post=setback_25 %s = %q, want %q", k, got, want)
		}
	}
	if metricText(doc25, "fit-brp-miss") == metricText(doc, "fit-brp-miss") {
		t.Errorf("fit-brp-miss did not change between 0 mm and 25 mm posts (%q)", metricText(doc25, "fit-brp-miss"))
	}
}

func TestBikeWorkbenchSetbackNotesReachedFit(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?post=setback_25&brp_x=-189&brp_y=713.1")
	if got := metricText(doc, "fit-saddle-outcome"); got != "reached" {
		t.Errorf("fit-saddle-outcome = %q, want reached", got)
	}
	if got := metricText(doc, "fit-rail-offset"); got != "8.9 mm" {
		t.Errorf("fit-rail-offset = %q, want 8.9 mm", got)
	}
	if doc.Find(`[data-metric="fit-setback-reach"]`).Length() != 0 {
		t.Errorf("expected no fit-setback-reach when saddle fit is reached")
	}
	for k, want := range map[string]string{
		"fit-setback-centered":           "33.6 mm",
		"fit-setback-centered-insertion": "187.1 mm",
		"fit-setback-centered-outcome":   "reached",
	} {
		if got := metricText(doc, k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestBikeWorkbenchSetbackNotesForwardTarget(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?post=redshift_shockstop_pro&brp_x=-150&brp_y=758.1")
	if got := metricText(doc, "fit-saddle-outcome"); got != "limited by seatpost insertion, saddle rail offset" {
		t.Errorf("fit-saddle-outcome = %q, want limited by seatpost insertion, saddle rail offset", got)
	}
	if got := metricText(doc, "fit-rail-offset"); got != "-15.0 mm" {
		t.Errorf("fit-rail-offset = %q, want -15.0 mm", got)
	}
	for k, want := range map[string]string{
		"fit-setback-reach-rail":         "-15.0 mm",
		"fit-setback-reach":              "0.0 mm",
		"fit-setback-reach-insertion":    "100.2 mm",
		"fit-setback-reach-outcome":      "limited by seatpost setback",
		"fit-setback-centered":           "0.0 mm",
		"fit-setback-centered-insertion": "104.2 mm",
		"fit-setback-centered-outcome":   "limited by seatpost setback",
	} {
		if got := metricText(doc, k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	centeredSpan := doc.Find(`[data-metric="fit-setback-centered"]`).Parent().Text()
	if !strings.Contains(centeredSpan, "16.1 mm short") {
		t.Errorf("centered span = %q, want 16.1 mm short", centeredSpan)
	}
}

func TestBikeWorkbenchSetbackNotesInsertionLimit(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?"+legacyPost+"&grip=off&brp_x=-219&brp_y=945")
	if got := metricText(doc, "fit-saddle-outcome"); got != "limited by seatpost insertion, saddle rail offset" {
		t.Errorf("fit-saddle-outcome = %q, want limited by seatpost insertion, saddle rail offset", got)
	}
	for k, want := range map[string]string{
		"fit-setback-centered":           "0.2 mm",
		"fit-setback-centered-insertion": "100.0 mm",
		"fit-setback-centered-outcome":   "limited by seatpost insertion",
		"fit-setback-reach-rail":         "+15.0 mm",
		"fit-setback-reach":              "0.0 mm",
		"fit-setback-reach-insertion":    "100.0 mm",
		"fit-setback-reach-outcome":      "limited by seatpost insertion, seatpost setback",
	} {
		if got := metricText(doc, k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	centeredSpan := doc.Find(`[data-metric="fit-setback-centered"]`).Parent().Text()
	if !strings.Contains(centeredSpan, "144.3 mm short") {
		t.Errorf("centered span = %q, want 144.3 mm short", centeredSpan)
	}
	reachSpan := doc.Find(`[data-metric="fit-setback-reach"]`).Parent().Text()
	if !strings.Contains(reachSpan, "141.0 mm short") {
		t.Errorf("reach span = %q, want 141.0 mm short", reachSpan)
	}
}

func TestBikeWorkbenchSetbackNotesAbsentWithoutBrpTarget(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp=off")
	if n := doc.Find(`[data-metric^="fit-setback"]`).Length(); n != 0 {
		t.Errorf("expected no [data-metric^=fit-setback] without BRP target, got %d", n)
	}
}

func assertSelectedForkOptions(t *testing.T, doc *goquery.Document, want string) {
	t.Helper()
	selected := doc.Find(`select[name="fork_options"] option[selected]`)
	if got, _ := selected.Attr("value"); got != want || selected.Length() != 1 {
		t.Errorf("selected fork_options = %q (%d selected options), want %q", got, selected.Length(), want)
	}
}

func TestBikeWorkbenchForkOptionsDefaultKeepsSubmittedFork(t *testing.T) {
	for _, tc := range []struct{ name, query, fork, sagScore string }{
		{"rigid", "", "Lithic Carbon Mountain (495/44)", "100.0"},
		{"suspension", "&fork=sid_100-506x44&sag=25", "RockShox SID 100mm (506/44), 25% sag", "50.0"},
		{"Basis", "&fork=basis", "", "100.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := loadBikeDoc(t, "/bike?brp=off"+tc.query)
			options := doc.Find(`select[name="fork_options"] option`)
			if options.Length() != 2 || options.Eq(1).Text() != "All 32 fork setups" {
				t.Errorf("fork_options options = %v, want Selected fork only and All 32 fork setups", options.Map(func(_ int, option *goquery.Selection) string { return option.Text() }))
			}
			assertSelectedForkOptions(t, doc, "selected")
			if got := doc.Find("[data-build-evaluated]").Text(); got != "749" {
				t.Errorf("evaluated = %q, want 749", got)
			}
			rows := doc.Find("tr[data-build-rank]")
			if rows.Length() != 5 {
				t.Fatalf("build rows = %d, want 5", rows.Length())
			}
			first := buildCell(rows.First(), "fork")
			rows.Each(func(i int, row *goquery.Selection) {
				if got := buildCell(row, "fork"); got != first || (tc.fork != "" && got != tc.fork) {
					t.Errorf("row %d fork = %q, want %q", i+1, got, first)
				}
				if got := buildAttr(doc, i+1, "Fork sag"); got != tc.sagScore {
					t.Errorf("row %d sag score = %q, want %q", i+1, got, tc.sagScore)
				}
				q := hrefQuery(t, doc, fmt.Sprintf(`tr[data-build-rank="%d"] a[data-build-apply]`, i+1))
				if q.Has("fork_options") {
					t.Errorf("row %d Apply includes fork_options: %v", i+1, q)
				}
			})
			if tc.name == "Basis" {
				if got := buildCell(rows.First(), "score"); got != "72.4" {
					t.Errorf("basis row 1 score = %q, want 72.4", got)
				}
				if got := buildCell(rows.First(), "stem"); got != "120 mm, -20°" {
					t.Errorf("basis row 1 stem = %q, want 120 mm, -20°", got)
				}
			}
		})
	}
}

const (
	forkLithic      = "Lithic Carbon Mountain (495/44)"
	forkSID15       = "RockShox SID 100mm (506/44), 15% sag"
	forkSID20       = "RockShox SID 100mm (506/44), 20% sag"
	forkSID51_20    = "RockShox SID 100mm (506/51), 20% sag"
	forkFirestarter = "Salsa Firestarter / Cutthroat Carbon Deluxe 483mm"
	forkSklar       = "Sklar Chromoly Steel 485mm"
	forkKrampus483  = "Surly Krampus 4130 Chromoly 483mm"
	forkFox20       = "Fox Float 34 120mm (534/44), 20% sag"
	firstBuildApply = `tr[data-build-rank="1"] a[data-build-apply]`
)

func assertForkOptionRows(t *testing.T, doc *goquery.Document, cells []string, attr string, want [][]string) {
	t.Helper()
	rows := doc.Find("tr[data-build-rank]")
	if rows.Length() != len(want) {
		t.Fatalf("build rows = %d, want %d", rows.Length(), len(want))
	}
	for i, values := range want {
		row := rows.Eq(i)
		for j, cell := range cells {
			if got := buildCell(row, cell); got != values[j] {
				t.Errorf("row %d %s = %q, want %q", i+1, cell, got, values[j])
			}
		}
		if attr != "" {
			if got := buildAttr(doc, i+1, attr); got != values[len(cells)] {
				t.Errorf("row %d %s score = %q, want %q", i+1, attr, got, values[len(cells)])
			}
		}
	}
}

func TestBikeWorkbenchFitsBuildsAcrossAllForks(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?fork_options=all&brp=off&view=stem")
	evaluated := doc.Find("[data-build-evaluated]")
	if got := evaluated.Text(); got != "23968" {
		t.Errorf("evaluated = %q, want 23968", got)
	}
	if got := evaluated.Parent().Text(); !strings.Contains(got, "fork, stem, and spacer combinations") || !strings.Contains(got, "The saddle is refitted for each fork.") {
		t.Errorf("build count note = %q", got)
	}
	assertForkOptionRows(t, doc, []string{"fork", "stem", "spacers", "oval-r", "score"}, "Fork sag", [][]string{
		{forkFirestarter, "120 mm, -20°", "0 mm", "1.10", "72.4", "100.0"},
		{forkSklar, "120 mm, -20°", "0 mm", "1.19", "70.3", "100.0"},
		{forkKrampus483, "120 mm, -20°", "0 mm", "1.19", "71.2", "100.0"},
		{forkSID51_20, "120 mm, -20°", "0 mm", "1.27", "68.8", "100.0"},
		{forkFirestarter, "120 mm, -17°", "0 mm", "1.29", "73.2", "100.0"},
	})
	q := hrefQuery(t, doc, firstBuildApply)
	for key, want := range map[string]string{
		"fork": "firestarter_deluxe_483", "fork_a2c": "483", "sag": "0", "cup": "12", "stem_length": "120",
		"stem_angle": "-20", "spacers": "0", "fork_options": "all", "post": "redshift_shockstop_pro",
	} {
		if got := q.Get(key); got != want {
			t.Errorf("row 1 Apply %s = %q, want %q", key, got, want)
		}
	}
}

func TestBikeWorkbenchForkOptionsWithSaddle(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?"+legacyPost+"&fork_options=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726")
	cells := []string{"fork", "stem", "spacers", "score"}
	want := [][]string{
		{forkFirestarter, "120 mm, -20°", "0 mm", "65.7", "0.0"},
		{forkSklar, "120 mm, -20°", "0 mm", "64.4", "0.0"},
		{forkKrampus483, "120 mm, -20°", "0 mm", "64.9", "0.0"},
		{forkSID51_20, "120 mm, -20°", "0 mm", "63.4", "0.0"},
		{forkFirestarter, "120 mm, -17°", "0 mm", "66.2", "0.0"},
	}
	assertForkOptionRows(t, doc, cells, "Saddle BRP X", want)
	if got := buildCell(doc.Find(`tr[data-build-rank="1"]`), "brp-miss"); got != "(-58.4, +0.0) mm" {
		t.Errorf("row 1 BRP miss = %q, want (-58.4, +0.0) mm", got)
	}
	q := hrefQuery(t, doc, firstBuildApply)
	for key, value := range map[string]string{"fork": "firestarter_deluxe_483", "sag": "0", "rail_offset": "15"} {
		if got := q.Get(key); got != value {
			t.Errorf("row 1 Apply %s = %q, want %q", key, got, value)
		}
	}
	if ins, err := strconv.ParseFloat(q.Get("post_insert"), 64); err != nil || math.Abs(ins-198.708763) > 1e-6 {
		t.Errorf("row 1 Apply post_insert = %q, want 198.708763", q.Get("post_insert"))
	}
	href, _ := doc.Find(firstBuildApply).Attr("href")
	applied := loadBikeDoc(t, href)
	if got, _ := applied.Find(`select[name="fork"] option[selected]`).Attr("value"); got != "firestarter_deluxe_483" {
		t.Errorf("applied fork = %q, want firestarter_deluxe_483", got)
	}
	assertSelectedForkOptions(t, applied, "all")
	assertForkOptionRows(t, applied, cells, "Saddle BRP X", want)
}

func TestBikeWorkbenchForkOptionsAndBarOptions(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?fork_options=all&bar_options=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726")
	evaluated := doc.Find("[data-build-evaluated]")
	if got := evaluated.Text(); got != "359520" {
		t.Errorf("evaluated = %q, want 359520", got)
	}
	if got := evaluated.Parent().Text(); !strings.Contains(got, "fork, handlebar, stem, and spacer combinations") {
		t.Errorf("build count note = %q", got)
	}
	const summary = "359520 fork, handlebar, stem, and spacer combinations placed; top 5 by fit tier and 0–100 weighted score (total weight 36). The saddle is refitted for each fork."
	if got := doc.Find("[data-build-summary]").Text(); got != summary {
		t.Errorf("build summary = %q, want %q", got, summary)
	}
	const (
		alloy16x25 = "SQlab 311 2.0 Alloy 16°, 25 mm rise (740 mm)"
		alloy16x15 = "SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm)"
		flx16x15   = "SQlab 311 FL-X Carbon 16°, 15 mm rise (740 mm)"
	)
	assertForkOptionRows(t, doc, []string{"bar", "fork", "stem", "spacers", "score"}, "", [][]string{
		{barFLX16x30, forkKrampus483, "120 mm, -17°", "0 mm", "71.7"},
		{alloy16x25, forkKrampus483, "120 mm, -17°", "0 mm", "71.6"},
		{barFLX16x30, forkFirestarter, "120 mm, -17°", "0 mm", "71.4"},
		{alloy16x15, forkKrampus483, "120 mm, -17°", "0 mm", "71.4"},
		{flx16x15, forkKrampus483, "120 mm, -17°", "0 mm", "71.3"},
	})
}

func TestBikeWorkbenchKeepsForkOptionsWithoutTarget(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?fork_options=all&view=seat&brp=off&grip=off")
	if n := doc.Find("[data-build-ranking]").Length(); n != 0 {
		t.Errorf("build ranking blocks = %d, want none", n)
	}
	assertSelectedForkOptions(t, doc, "all")
	if got := workbenchCanonicalQuery(t, doc).Get("fork_options"); got != "all" {
		t.Errorf("canonical fork_options = %q, want all", got)
	}
	for _, sel := range []string{"#view-bike", "#view-seat", "#view-stem", "#preset-basis-fork"} {
		if got := hrefQuery(t, doc, sel).Get("fork_options"); got != "all" {
			t.Errorf("%s fork_options = %q, want all", sel, got)
		}
	}
	if q := hrefQuery(t, doc, "#preset-defaults"); q.Has("fork_options") {
		t.Errorf("Reset Defaults query contains fork_options: %v", q)
	}
}

func TestBikeWorkbenchForkOptionsErrorPage(t *testing.T) {
	doc := loadBikeDocStatus(t, "/bike?fork_options=some", http.StatusBadRequest)
	if got, _ := doc.Find(`select[name="fork_options"]`).Attr("aria-invalid"); got != "true" {
		t.Errorf("fork_options aria-invalid = %q, want true", got)
	}
	if got := strings.TrimSpace(doc.Find("#form-error").Text()); !strings.Contains(got, `unknown fork options value "some"`) {
		t.Errorf("fork_options error message = %q", got)
	}
	doc = loadBikeDocStatus(t, "/bike?fork_options=all&spacers=-1", http.StatusBadRequest)
	assertSelectedForkOptions(t, doc, "all")
	if got := hrefQuery(t, doc, "#preset-basis-fork").Get("fork_options"); got != "all" {
		t.Errorf("error-page basis preset fork_options = %q, want all", got)
	}
}

func TestBikeWorkbenchShowsInsertionBoundsAndExposedLength(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?post=setback_0")
	input := doc.Find("#ctrl-post-insert")
	for attr, want := range map[string]string{"min": "100", "max": "290", "value": "195.0"} {
		if got, _ := input.Attr(attr); got != want {
			t.Errorf("generic post insertion %s = %q, want %q", attr, got, want)
		}
	}
	if got := metricText(doc, "post-exposed"); got != "205.0 mm" {
		t.Errorf("exposed length = %q, want 205.0 mm", got)
	}

	redshift := loadBikeDoc(t, "/bike")
	input = redshift.Find("#ctrl-post-insert")
	for attr, want := range map[string]string{"min": "100", "max": "250", "value": "175.0"} {
		if got, _ := input.Attr(attr); got != want {
			t.Errorf("Redshift insertion %s = %q, want %q", attr, got, want)
		}
	}
	if got := metricText(redshift, "post-exposed"); got != "175.0 mm" {
		t.Errorf("Redshift exposed length = %q, want 175.0 mm", got)
	}
}

// Switching to a shorter-travel post keeps the submitted insertion and
// validates it against the new post; the correction link path uses its bounds.
func TestBikeWorkbenchPostSwitchKeepsInsertionAndRecovers(t *testing.T) {
	// 275 mm is legal for the generic post (100–290) but not the Redshift (100–250).
	generic := loadBikeDoc(t, "/bike?post=setback_0&post_insert=275")
	if got := metricText(generic, "post-exposed"); got != "125.0 mm" {
		t.Errorf("generic exposed length = %q, want 125.0 mm", got)
	}
	errDoc := loadBikeDocStatus(t, "/bike?post=redshift_shockstop_pro&post_insert=275", http.StatusBadRequest)
	input := errDoc.Find("#ctrl-post-insert")
	if got, _ := input.Attr("value"); got != "275" {
		t.Errorf("error echo = %q, want the submitted 275", got)
	}
	if inv, _ := input.Attr("aria-invalid"); inv != "true" {
		t.Errorf("aria-invalid = %q, want true", inv)
	}
	if max, _ := input.Attr("max"); max != "250" {
		t.Errorf("error page max = %q, want the Redshift's 250", max)
	}
	if got, _ := errDoc.Find(`select[name="post"] option[selected]`).Attr("value"); got != "redshift_shockstop_pro" {
		t.Errorf("selected post = %q, want redshift_shockstop_pro", got)
	}

	fixed := loadBikeDoc(t, "/bike?post=redshift_shockstop_pro&post_insert=250")
	if got := metricText(fixed, "post-exposed"); got != "100.0 mm" {
		t.Errorf("corrected exposed length = %q, want 100.0 mm", got)
	}

	// Switching the post in the form clamps the carried insertion into the new
	// post's bounds (Q109) instead of failing.
	rec, switched := submitForm(t, generic, map[string]string{"post": "redshift_shockstop_pro"})
	if rec.Code != http.StatusOK {
		t.Fatalf("post switch status = %d, want 200", rec.Code)
	}
	if got, _ := switched.Find("#ctrl-post-insert").Attr("value"); got != "250.0" {
		t.Errorf("post switch insertion = %q, want the clamped 250.0", got)
	}
	if got := metricText(switched, "post-exposed"); got != "100.0 mm" {
		t.Errorf("post switch exposed length = %q, want 100.0 mm", got)
	}
}

// An applied fitted build keeps its exact insertion through view links and reload.
func TestBikeWorkbenchAppliedFitSurvivesNavigationAndReload(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp_x=-219&brp_y=713.1")
	applyHref, _ := doc.Find("#apply-fitting").Attr("href")
	applied := loadBikeDoc(t, applyHref)
	want, _ := applied.Find("#ctrl-post-insert").Attr("value")
	if want == "" {
		t.Fatal("applied build has no post_insert value")
	}
	canon := activeViewHref(t, applied)
	for _, href := range []string{canon, applyHref} {
		reloaded := loadBikeDoc(t, href)
		if got, _ := reloaded.Find("#ctrl-post-insert").Attr("value"); got != want {
			t.Errorf("reload of %s: post_insert = %q, want %q", href, got, want)
		}
	}
	applied.Find(`a[data-diagram-view]`).Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		view := loadBikeDoc(t, href)
		if got, _ := view.Find("#ctrl-post-insert").Attr("value"); got != want {
			t.Errorf("view link %d: post_insert = %q, want %q", i, got, want)
		}
	})
}

// railCenterOf reads the saddle-rail outline's origin (its RailCenter) from its transform.
func railCenterOf(t *testing.T, doc *goquery.Document) svgPoint {
	t.Helper()
	rect := doc.Find(`.component-outlines rect[data-component="saddle-rail"]`)
	transform, _ := rect.Attr("transform")
	var p svgPoint
	if _, err := fmt.Sscanf(transform, "translate(%f %f)", &p.X, &p.Y); err != nil {
		t.Fatalf("saddle-rail transform %q: %v", transform, err)
	}
	return p
}

// The seat close-up draws the rail at RailCenter and the saddle profile through
// the raised BRP and SitBones, frames every landmark and outline for level and
// pitched rails, and uses the same coordinates as the whole-bike view.
func TestBikeWorkbenchSeatViewShowsRaisedSaddleAboveRails(t *testing.T) {
	for _, pitch := range []string{"0", "10", "-10"} {
		t.Run("rail_pitch="+pitch, func(t *testing.T) {
			seat := loadBikeDoc(t, "/bike?view=seat&rail_pitch="+pitch)
			whole := loadBikeDoc(t, "/bike?view=bike&rail_pitch="+pitch)
			box := parseViewBoxAttr(t, seat)

			names := []string{"RailPivot", "RailCenter", "BRP", "SitBones"}
			for _, name := range names {
				got, want := landmarkCentre(t, seat, name), landmarkCentre(t, whole, name)
				if got != want {
					t.Errorf("%s: seat view at %+v, whole-bike view at %+v", name, got, want)
				}
				if !box.contains(got) {
					t.Errorf("viewBox %+v does not contain %s at %+v", box, name, got)
				}
			}
			for _, component := range []string{"seatpost-clamp", "saddle-rail"} {
				rect := seat.Find(fmt.Sprintf(`.component-outlines rect[data-component=%q]`, component))
				for _, c := range renderedRectCorners(t, rect) {
					if !box.contains(c) {
						t.Errorf("viewBox %+v cuts off %s corner %+v", box, component, c)
					}
				}
			}

			// The rail outline sits at RailCenter, and BRP is 32 mm along and 45 mm
			// above it along the rails' +Y, whatever the pitch.
			rail := railCenterOf(t, seat)
			center := landmarkCentre(t, seat, "RailCenter")
			if math.Hypot(rail.X-center.X, rail.Y-center.Y) > 0.02 {
				t.Errorf("rail outline at %+v, RailCenter at %+v", rail, center)
			}
			brp := landmarkCentre(t, seat, "BRP")
			assertWebNear(t, "RailCenter to BRP", math.Hypot(brp.X-rail.X, brp.Y-rail.Y), math.Hypot(32, 45), 0.02)
			sit := landmarkCentre(t, seat, "SitBones")
			assertWebNear(t, "RailCenter to SitBones", math.Hypot(sit.X-rail.X, sit.Y-rail.Y), math.Hypot(-48, 55), 0.02)

			if pitch == "0" {
				// Level rails: the whole saddle profile is drawn above (smaller SVG Y than) the rail.
				if brp.Y >= rail.Y || sit.Y >= rail.Y {
					t.Errorf("profile (BRP y %v, SitBones y %v) is not above the rail (y %v)", brp.Y, sit.Y, rail.Y)
				}
				assertWebNear(t, "BRP height above the rail", rail.Y-brp.Y, 45, 0.02)
				assertWebNear(t, "SitBones height above the rail", rail.Y-sit.Y, 55, 0.02)
			}
			profile := seat.Find(`.bike-segments line[data-segment="saddle-profile"]`)
			for attr, want := range map[string]svgPoint{"1": sit, "2": brp} {
				x, _ := profile.Attr("x" + attr)
				y, _ := profile.Attr("y" + attr)
				gx, _ := strconv.ParseFloat(x, 64)
				gy, _ := strconv.ParseFloat(y, 64)
				if math.Hypot(gx-want.X, gy-want.Y) > 0.02 {
					t.Errorf("saddle-profile end %s at (%v, %v), want %+v", attr, gx, gy, want)
				}
			}
		})
	}
}

func assertWebNear(t *testing.T, label string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %v, want %v (±%v)", label, got, want, tol)
	}
}

// Q92: the default build puts the leveled Lithic BRP at (-155.065, 729.317) mm and the basis fork at 746.008 mm.
func TestBikeWorkbenchDefaultInsertionMidpointQ109(t *testing.T) {
	doc := loadBikeDoc(t, "/bike")
	if got, _ := doc.Find(`select[name="post"] option[selected]`).Attr("value"); got != "redshift_shockstop_pro" {
		t.Errorf("selected post = %q, want redshift_shockstop_pro", got)
	}
	if got, _ := doc.Find("#ctrl-post-insert").Attr("value"); got != "175.0" {
		t.Errorf("post_insert = %q, want the 175.0 midpoint", got)
	}
	if got := metricText(doc, "post-exposed"); got != "175.0 mm" {
		t.Errorf("exposed length = %q, want 175.0 mm", got)
	}
	if got := metricText(doc, "brp-height"); got != "700.4 mm" {
		t.Errorf("BRP height = %q, want 700.4 mm", got)
	}
	brp := mustPlace(t, mustBike(t, "")).Saddle.BRP.Origin()
	assertWebNear(t, "BRP.X", brp.X, -158.061, 0.001)
	assertWebNear(t, "BRP.Y", brp.Y, 682.374, 0.001)

	setback := loadBikeDoc(t, "/bike?post=setback_0")
	if got, _ := setback.Find("#ctrl-post-insert").Attr("value"); got != "195.0" {
		t.Errorf("setback_0 post_insert = %q, want the 195.0 midpoint", got)
	}
	if got := metricText(setback, "post-exposed"); got != "205.0 mm" {
		t.Errorf("setback_0 exposed length = %q, want 205.0 mm", got)
	}
}

func TestBikeWorkbenchFrameHandlingReadouts(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		wantTrail string
		wantRC    string
		wantRatio string
	}{
		{
			name:      "default Lithic",
			path:      "/bike",
			wantTrail: "107.0 mm",
			wantRC:    "435.1 mm",
			wantRatio: "38.0%",
		},
		{
			name:      "Fox 34 at 20%",
			path:      "/bike?fork=fox34_120-534x44&sag=20",
			wantTrail: "112.0 mm",
			wantRC:    "435.8 mm",
			wantRatio: "37.9%",
		},
		{
			name:      "basis",
			path:      "/bike?fork=basis",
			wantTrail: "94.5 mm",
			wantRC:    "434.3 mm",
			wantRatio: "37.8%",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := loadBikeDoc(t, tc.path)
			if got := metricText(doc, "trail"); got != tc.wantTrail {
				t.Errorf("trail = %q, want %q", got, tc.wantTrail)
			}
			if got := metricText(doc, "rear-center"); got != tc.wantRC {
				t.Errorf("rear-center = %q, want %q", got, tc.wantRC)
			}
			if got := metricText(doc, "rc-wb"); got != tc.wantRatio {
				t.Errorf("rc-wb = %q, want %q", got, tc.wantRatio)
			}
		})
	}
}

func TestBikeWorkbenchBuildTiersAndLegend(t *testing.T) {
	const prefix = "/bike?post=setback_25&brp_x=-189&brp_y=713.1&grip_x=481&grip_y=726"
	doc := loadBikeDoc(t, prefix)
	row := doc.Find(`tr[data-build-rank="1"]`)
	if got, _ := row.Find("[data-fit-tier]").Attr("data-fit-tier"); got != "fits_brp" {
		t.Errorf("row 1 data-fit-tier = %q, want fits_brp", got)
	}
	for name, want := range map[string]string{"tier": "Fits BRP", "score": "70.4", "brp-miss": "(+0.0, +0.0) mm"} {
		if got := buildCell(row, name); got != want {
			t.Errorf("row 1 %s = %q, want %q", name, got, want)
		}
	}
	if got := buildAttr(doc, 1, "Saddle rail offset"); got != "61.2" {
		t.Errorf("row 1 rail score = %q, want 61.2", got)
	}

	doc = loadBikeDoc(t, prefix+"&bar_options=all")
	row = doc.Find(`tr[data-build-rank="1"]`)
	if got, _ := row.Find("[data-fit-tier]").Attr("data-fit-tier"); got != "fits_all" {
		t.Errorf("all-bars row 1 data-fit-tier = %q, want fits_all", got)
	}
	for name, want := range map[string]string{
		"tier": "Fits all", "score": "82.3", "bar": barAlloy50, "stem": "120 mm, -30°", "spacers": "0 mm", "oval-r": "0.90",
	} {
		if got := buildCell(row, name); got != want {
			t.Errorf("all-bars row 1 %s = %q, want %q", name, got, want)
		}
	}
	if got := buildAttr(doc, 1, "Saddle rail offset"); got != "61.2" {
		t.Errorf("all-bars row 1 rail score = %q, want 61.2", got)
	}
	bdRow := doc.Find(`tr[data-build-breakdown-rank="1"]`)
	if n := bdRow.Find("[data-build-attr]").Length(); n != 12 {
		t.Errorf("attribute cells = %d, want 12", n)
	}
	bdRow.Find("[data-build-attr]").Each(func(i int, cell *goquery.Selection) {
		if strings.TrimSpace(cell.Text()) == "—" {
			t.Errorf("attribute cell %d is blank with a BRP target", i)
		}
	})

	legend := doc.Find("[data-fit-tier-legend]")
	labels := legend.Find("dt").Map(func(_ int, dt *goquery.Selection) string { return strings.TrimSpace(dt.Text()) })
	if want := []string{"Fits all", "Fits BRP", "Fits grip", "Fits neither"}; !slices.Equal(labels, want) {
		t.Errorf("legend labels = %v, want %v", labels, want)
	}
	if got := strings.TrimSpace(legend.Find("dd").First().Text()); !strings.Contains(got, "|ΔY| ≤ 2.0 mm, |ΔX| ≤ 10.0 mm") {
		t.Errorf("Fits all definition = %q", got)
	}

	// No score readout without targets (Q98).
	plain := loadBikeDoc(t, "/bike?brp=off&grip=off")
	if n := plain.Find("[data-build-ranking], [data-fit-tier-legend]").Length(); n != 0 {
		t.Errorf("/bike without targets shows %d build/score blocks, want none", n)
	}
}

// selectedValue returns the value of the selected option of select[name].
func selectedValue(doc *goquery.Document, name string) string {
	v, _ := doc.Find(fmt.Sprintf(`select[name=%q] option[selected]`, name)).Attr("value")
	return v
}

// inputValue returns the value of input[name].
func inputValue(doc *goquery.Document, name string) string {
	v, _ := doc.Find(fmt.Sprintf(`input[name=%q]`, name)).Attr("value")
	return v
}

func TestBikeMultiFrameSelectAndSwitching(t *testing.T) {
	doc := loadBikeDoc(t, "/bike")
	frames := catalog.Frames()
	options := doc.Find(`select[name="frame"] option`)
	if options.Length() != 18 || len(frames) != 18 {
		t.Fatalf("frame options = %d (catalog %d), want 18", options.Length(), len(frames))
	}
	for i, f := range frames {
		opt := options.Eq(i)
		if got, _ := opt.Attr("value"); got != f.ID {
			t.Errorf("frame option %d = %q, want %q", i, got, f.ID)
		}
		if want := fmt.Sprintf("%s %s (%d)", f.Manufacturer, f.Model, f.Year); strings.TrimSpace(opt.Text()) != want {
			t.Errorf("frame option %d label = %q, want %q", i, strings.TrimSpace(opt.Text()), want)
		}
	}
	if got := selectedValue(doc, "frame"); got != "otso-fenrir-stainless" {
		t.Errorf("selected frame = %q, want otso-fenrir-stainless", got)
	}
	fenrir, _ := catalog.FrameByID("otso-fenrir-stainless")
	sizeOptions := doc.Find(`select[name="size"] option`)
	if sizeOptions.Length() != len(fenrir.Sizes) {
		t.Fatalf("size options = %d, want %d", sizeOptions.Length(), len(fenrir.Sizes))
	}
	for i, sz := range fenrir.Sizes {
		if got, _ := sizeOptions.Eq(i).Attr("value"); got != sz.Name {
			t.Errorf("size option %d = %q, want %q", i, got, sz.Name)
		}
	}
	if got := selectedValue(doc, "size"); got != "L" {
		t.Errorf("selected size = %q, want L", got)
	}
	for name, want := range map[string]string{"prev_frame": "otso-fenrir-stainless", "prev_size": "L", "prev_fork": "lithic_carbon_mountain-495x44", "prev_post": "redshift_shockstop_pro"} {
		if got := inputValue(doc, name); got != want {
			t.Errorf("hidden %s = %q, want %q", name, got, want)
		}
	}

	t.Run("switch to esker-hayduke preserves lithic", func(t *testing.T) {
		rec, next := submitForm(t, doc, map[string]string{"frame": "esker-hayduke", "bar": "sqlab3ox_16_30", "stem_length": "80"})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		for name, want := range map[string]string{"frame": "esker-hayduke", "size": "S3", "fork": "lithic_carbon_mountain-495x44", "bar": "sqlab3ox_16_30"} {
			if got := selectedValue(next, name); got != want {
				t.Errorf("selected %s = %q, want %q", name, got, want)
			}
		}
		for name, want := range map[string]string{"fork_a2c": "495.0", "cup": "12.0", "sag": "0.0", "stem_length": "80.0"} {
			if got := inputValue(next, name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		if got := next.Find(`select[name="size"] option`).Map(func(_ int, o *goquery.Selection) string { return strings.TrimSpace(o.Text()) }); !slices.Equal(got, []string{"S1", "S2", "S3", "S4"}) {
			t.Errorf("esker sizes = %v, want S1–S4", got)
		}
	})
	t.Run("switch to esker-hayduke from sid falls back to pike", func(t *testing.T) {
		sidDoc := loadBikeDoc(t, "/bike?fork=sid_100-506x44&cup=12&sag=20")
		rec, next := submitForm(t, sidDoc, map[string]string{"frame": "esker-hayduke", "bar": "sqlab3ox_16_30", "stem_length": "80"})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		for name, want := range map[string]string{"frame": "esker-hayduke", "size": "S3", "fork": "pike_120", "bar": "sqlab3ox_16_30"} {
			if got := selectedValue(next, name); got != want {
				t.Errorf("selected %s = %q, want %q", name, got, want)
			}
		}
		for name, want := range map[string]string{"fork_a2c": "531.0", "cup": "12.0", "sag": "25.0", "stem_length": "80.0"} {
			if got := inputValue(next, name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
	})
	t.Run("switch to surly-krampus keeps size and the compatible fork", func(t *testing.T) {
		rec, next := submitForm(t, doc, map[string]string{"frame": "surly-krampus"})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := selectedValue(next, "size"); got != "L" {
			t.Errorf("selected size = %q, want L", got)
		}
		// Q135: the Krampus lists fox34_120, which unlocks the 495 rigid class.
		if got := selectedValue(next, "fork"); got != "lithic_carbon_mountain-495x44" {
			t.Errorf("selected fork = %q, want lithic_carbon_mountain-495x44", got)
		}
		if got := inputValue(next, "fork_a2c"); got != "495.0" {
			t.Errorf("fork_a2c = %q, want 495.0", got)
		}
		if got := inputValue(next, "sag"); got != "0.0" {
			t.Errorf("sag = %q, want 0.0", got)
		}
	})
	t.Run("switch to surly-ecr falls back to its default fork", func(t *testing.T) {
		rec, next := submitForm(t, doc, map[string]string{"frame": "surly-ecr"})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := selectedValue(next, "fork"); got != "surly_ecr_468" {
			t.Errorf("selected fork = %q, want surly_ecr_468 (tapered forks do not fit 34 mm, Q130)", got)
		}
	})
	t.Run("switch to salsa-cutthroat keeps the shared fork", func(t *testing.T) {
		sid := loadBikeDoc(t, "/bike?fork=sid_100-506x44&cup=12&sag=25")
		rec, next := submitForm(t, sid, map[string]string{"frame": "salsa-cutthroat"})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := selectedValue(next, "size"); got != "56cm" {
			t.Errorf("selected size = %q, want 56cm", got)
		}
		if got := selectedValue(next, "fork"); got != "sid_100-506x44" {
			t.Errorf("selected fork = %q, want sid_100-506x44", got)
		}
		if got := inputValue(next, "cup"); got != "1.0" {
			t.Errorf("IS52 cup = %q, want 1.0", got)
		}
		if got := inputValue(next, "sag"); got != "20.0" {
			t.Errorf("sag = %q, want 20.0", got)
		}
	})
	t.Run("unknown frame and size", func(t *testing.T) {
		frameDoc := loadBikeDocStatus(t, "/bike?frame=nope", http.StatusBadRequest)
		if got, _ := frameDoc.Find(`select[name="frame"]`).Attr("aria-invalid"); got != "true" {
			t.Errorf("frame aria-invalid = %q, want true", got)
		}
		selFrame := frameDoc.Find(`select[name="frame"] option[selected]`)
		if selFrame.Length() != 1 || strings.TrimSpace(selFrame.Text()) != "Invalid: nope" {
			t.Errorf("unknown frame selected = %d %q, want 1 'Invalid: nope'", selFrame.Length(), selFrame.Text())
		}
		if got := frameDoc.Find(`select[name="frame"] option`).Length(); got != 19 {
			t.Errorf("error page frame options = %d, want 19", got)
		}
		sizeDoc := loadBikeDocStatus(t, "/bike?size=XXL", http.StatusBadRequest)
		if got, _ := sizeDoc.Find(`select[name="size"]`).Attr("aria-invalid"); got != "true" {
			t.Errorf("size aria-invalid = %q, want true", got)
		}
		selSize := sizeDoc.Find(`select[name="size"] option[selected]`)
		if selSize.Length() != 1 || strings.TrimSpace(selSize.Text()) != "Invalid: XXL" {
			t.Errorf("unknown size selected = %d %q, want 1 'Invalid: XXL'", selSize.Length(), selSize.Text())
		}
		if got := sizeDoc.Find(`select[name="size"] option`).Length(); got != len(fenrir.Sizes)+1 {
			t.Errorf("error page size options = %d, want %d", got, len(fenrir.Sizes)+1)
		}
	})
}

func TestBikeSlidingDropoutsAndUnpublishedWheelbase(t *testing.T) {
	physical := loadBikeDoc(t, "/bike?frame=binary-havok&size=L")
	for name, want := range map[string]string{
		"wheelbase": "1125.9 mm", "rear-center": "454.8 mm", "leveled-reach": "396.1 mm",
		"leveled-stack": "657.1 mm", "ett": "586.6 mm", "delta-pitch": "+0.16°",
	} {
		if got := metricText(physical, name); got != want {
			t.Errorf("physical %s = %q, want %q", name, got, want)
		}
	}
	note := physical.Find(`[data-note="sliding-dropouts"]`).Text()
	if !strings.Contains(note, "460 mm") || !strings.Contains(note, "445 mm") {
		t.Errorf("sliding-dropouts note = %q, want it to mention 460 mm and 445 mm", note)
	}

	basis := loadBikeDoc(t, "/bike?frame=binary-havok&size=L&fork=basis")
	for name, want := range map[string]string{"wheelbase": "1110.6 mm", "rear-center": "439.5 mm", "delta-pitch": "+0.00°"} {
		if got := metricText(basis, name); got != want {
			t.Errorf("basis %s = %q, want %q", name, got, want)
		}
	}
	if got := inputValue(basis, "cup"); got != "10.0" {
		t.Errorf("basis cup = %q, want 10.0", got)
	}
	if _, ro := basis.Find(`input[name="cup"]`).Attr("readonly"); !ro {
		t.Error("basis cup must be readonly")
	}
	if strings.TrimSpace(basis.Find(`[data-note="basis-fork"]`).Text()) == "" {
		t.Error("basis-fork note is empty")
	}
	if basis.Find(`[data-note="sliding-dropouts"]`).Length() != 1 {
		t.Error("basis page must explain the published chainstay")
	}

	moots := loadBikeDoc(t, "/bike?frame=moots-mountaineer&size=M")
	if strings.Contains(moots.Text(), "(Published") {
		t.Error("unpublished wheelbase must not print (Published ...) (Q57)")
	}
	fenrir := loadBikeDoc(t, "/bike")
	if strings.Contains(fenrir.Text(), "(Published") {
		t.Error("Fenrir unpublished wheelbase must not print (Published ...) (Q141)")
	}
	if fenrir.Find(`[data-note="sliding-dropouts"]`).Length() != 0 {
		t.Error("Fenrir Tuning Chip frame must not print sliding-dropouts note (Q140)")
	}
	krampus := loadBikeDoc(t, "/bike?frame=surly-krampus&size=L")
	if !strings.Contains(krampus.Text(), "(Published 1138.0 mm)") {
		t.Error("published wheelbase must print (Published 1138.0 mm)")
	}
}

func TestBikeLeveledReadoutsAndRedshiftNote(t *testing.T) {
	doc := loadBikeDoc(t, "/bike")
	for name, want := range map[string]string{"leveled-reach": "421.6 mm", "leveled-stack": "646.2 mm", "ett": "601.7 mm"} {
		if got := metricText(doc, name); got != want {
			t.Errorf("default %s = %q, want %q", name, got, want)
		}
	}
	if doc.Find(`[data-note="redshift-mixed-state"]`).Length() != 1 {
		t.Error("default /bike must render the Redshift mixed-state note (Q78)")
	}
	basis := loadBikeDoc(t, "/bike?fork=basis")
	for name, want := range map[string]string{"leveled-reach": "429.3 mm", "leveled-stack": "641.1 mm", "ett": "599.7 mm"} {
		if got := metricText(basis, name); got != want {
			t.Errorf("basis %s = %q, want %q", name, got, want)
		}
	}
	if loadBikeDoc(t, "/bike?post=setback_0").Find(`[data-note="redshift-mixed-state"]`).Length() != 0 {
		t.Error("setback_0 must not render the Redshift mixed-state note")
	}
}

func TestBikeSubmittedBuildScoreQ110(t *testing.T) {
	attrCells := func(doc *goquery.Document) map[string]string {
		cells := map[string]string{}
		doc.Find(`[data-submitted-score] [data-submitted-attr]`).Each(func(_ int, el *goquery.Selection) {
			name, _ := el.Attr("data-submitted-attr")
			cells[name] = strings.TrimSpace(el.Text())
		})
		return cells
	}

	doc := loadBikeDoc(t, "/bike")
	if doc.Find(`[data-submitted-score]`).Length() != 1 {
		t.Fatal("default /bike must render the submitted build score")
	}
	if got, _ := doc.Find(`[data-submitted-tier]`).Attr("data-submitted-tier"); got != "fits_neither" {
		t.Errorf("tier = %q, want fits_neither", got)
	}
	for name, want := range map[string]string{
		"submitted-score": "53.5", "submitted-oval-r": "9.54", "submitted-brp-miss": "(-60.9, +30.7) mm",
	} {
		if got := metricText(doc, name); got != want {
			t.Errorf("default %s = %q, want %q", name, got, want)
		}
	}
	cells := attrCells(doc)
	if len(cells) != 12 {
		t.Errorf("attribute cells = %d, want 12", len(cells))
	}
	for name, got := range cells {
		if got == "—" {
			t.Errorf("attribute %q is inactive with both targets set", name)
		}
	}
	if cells["Stem"] != "86.5" {
		t.Errorf("Stem attribute = %q, want 86.5", cells["Stem"])
	}

	const winning = "/bike?frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20&post=redshift_shockstop_pro&post_insert=163.13014276565696&rail_offset=15&rail_pitch=0&bar=persuader20_800&bar_roll=0&stem_length=110&stem_angle=-7&spacers=0"
	win := loadBikeDoc(t, winning)
	if got, _ := win.Find(`[data-submitted-tier]`).Attr("data-submitted-tier"); got != "fits_all" {
		t.Errorf("winning tier = %q, want fits_all", got)
	}
	for name, want := range map[string]string{"submitted-score": "84.7", "submitted-oval-r": "0.73", "submitted-brp-miss": "(-1.8, +0.0) mm"} {
		if got := metricText(win, name); got != want {
			t.Errorf("winning %s = %q, want %q", name, got, want)
		}
	}
	top := win.Find(`tr[data-build-rank="1"]`)
	for cell, want := range map[string]string{"tier": "Fits all", "score": "84.7", "oval-r": "0.73", "brp-miss": "(-1.8, +0.0) mm"} {
		if got := buildCell(top, cell); got != want {
			t.Errorf("winning rank 1 %s = %q, want %q", cell, got, want)
		}
	}

	cleared := loadBikeDoc(t, "/bike?brp=off")
	if got, _ := cleared.Find(`[data-submitted-tier]`).Attr("data-submitted-tier"); got != "fits_neither" {
		t.Errorf("cleared BRP tier = %q, want fits_neither", got)
	}
	if got := metricText(cleared, "submitted-score"); got != "70.8" {
		t.Errorf("cleared BRP score = %q, want 70.8", got)
	}
	if got := metricText(cleared, "submitted-brp-miss"); got != "—" {
		t.Errorf("cleared BRP miss = %q, want —", got)
	}
	inactive := 0
	for name, got := range attrCells(cleared) {
		switch name {
		case "Saddle BRP Y", "Saddle BRP X", "Saddle rail offset", "Seatpost insertion":
			if got != "—" {
				t.Errorf("saddle attribute %q = %q, want —", name, got)
			}
			inactive++
		default:
			if got == "—" {
				t.Errorf("attribute %q is inactive without a BRP target", name)
			}
		}
	}
	if inactive != 4 {
		t.Errorf("inactive saddle attributes = %d, want 4", inactive)
	}

	off := loadBikeDoc(t, "/bike?stem_length=65&stem_angle=-6")
	if off.Find(`[data-submitted-score] [data-note="off-catalog-stem"]`).Length() != 1 {
		t.Error("off-catalog stem must explain why there is no score")
	}
	if n := off.Find(`[data-submitted-tier], [data-metric="submitted-score"]`).Length(); n != 0 {
		t.Errorf("off-catalog stem renders %d score elements, want none", n)
	}
	if got := strings.TrimSpace(off.Find(`[data-note="off-catalog-stem"]`).Text()); !strings.Contains(got, "65 mm, -6.0°") {
		t.Errorf("off-catalog note = %q, want it to name 65 mm, -6.0°", got)
	}

	if n := loadBikeDoc(t, "/bike?grip=off").Find(`[data-submitted-score]`).Length(); n != 0 {
		t.Errorf("cleared HandGrip renders %d submitted score blocks, want none", n)
	}
}

func TestBikeDurableClearedTargetsQ70(t *testing.T) {
	clearedState := func(t *testing.T, doc *goquery.Document) {
		t.Helper()
		for _, sel := range []string{"[data-fitting]", "[data-submitted-score]", "[data-build-ranking]", "a#preset-clear-targets"} {
			if n := doc.Find(sel).Length(); n != 0 {
				t.Errorf("cleared page renders %d %s elements, want 0", n, sel)
			}
		}
		for _, id := range []string{"ctrl-brp-x", "ctrl-brp-y", "ctrl-grip-x", "ctrl-grip-y"} {
			if v, ok := doc.Find("#" + id).Attr("value"); !ok || v != "" {
				t.Errorf("#%s value = %q (present %v), want empty", id, v, ok)
			}
		}
	}
	keepsOff := func(t *testing.T, label string, q url.Values, keys ...string) {
		t.Helper()
		for _, k := range keys {
			if q.Get(k) != "off" {
				t.Errorf("%s dropped %s=off: %v", label, k, q)
			}
		}
	}

	def := loadBikeDoc(t, "/bike")
	clearHref, ok := def.Find("a#preset-clear-targets").Attr("href")
	if !ok {
		t.Fatal("default /bike has no Clear Targets link")
	}
	cleared := loadBikeDoc(t, clearHref)
	clearedState(t, cleared)
	keepsOff(t, "clear link", hrefQuery(t, cleared, "#preset-basis-fork"), "brp", "grip")

	rec, edited := submitForm(t, cleared, map[string]string{"stem_length": "80"})
	if rec.Code != http.StatusOK {
		t.Fatalf("edit of cleared page status = %d, want 200", rec.Code)
	}
	clearedState(t, edited)
	if got := inputValue(edited, "stem_length"); got != "80.0" {
		t.Errorf("edited stem_length = %q, want 80.0", got)
	}
	for _, sel := range []string{"#view-seat", "#view-stem", "#preset-basis-fork"} {
		keepsOff(t, sel, hrefQuery(t, edited, sel), "brp", "grip")
	}
	reloaded := loadBikeDoc(t, activeViewHref(t, edited))
	clearedState(t, reloaded)

	t.Run("clear BRP only", func(t *testing.T) {
		doc := loadBikeDoc(t, "/bike?brp_x=&brp_y=&grip_x=481&grip_y=726")
		for _, sel := range []string{"#apply-fitting", `tr[data-build-rank="1"] a[data-build-apply]`, "#view-seat", "#preset-basis-fork"} {
			q := hrefQuery(t, doc, sel)
			keepsOff(t, sel, q, "brp")
			if q.Get("grip_x") != "481" {
				t.Errorf("%s lost the HandGrip target: %v", sel, q)
			}
			next := loadBikeDoc(t, "/bike?"+q.Encode())
			if next.Find(`[data-metric="fit-saddle-outcome"]`).Length() != 0 || next.Find(`[data-metric="fit-cockpit-outcome"]`).Length() != 1 {
				t.Errorf("following %s lost the cleared BRP state", sel)
			}
		}
	})
	t.Run("clear HandGrip only", func(t *testing.T) {
		doc := loadBikeDoc(t, "/bike?grip=off")
		for _, sel := range []string{"#apply-fitting", "#view-seat", "#preset-basis-fork"} {
			q := hrefQuery(t, doc, sel)
			keepsOff(t, sel, q, "grip")
			next := loadBikeDoc(t, "/bike?"+q.Encode())
			if next.Find(`[data-metric="fit-cockpit-outcome"]`).Length() != 0 {
				t.Errorf("following %s lost the cleared HandGrip state", sel)
			}
		}
	})
}

// A rejected frame, size, or fork renders the default's dependent options, so
// choosing another frame afterwards must recover instead of failing again.
func TestBikeWorkbenchRecoversFromRejectedSelects(t *testing.T) {
	for _, path := range []string{"/bike?frame=nope", "/bike?fork=nope", "/bike?size=XXL"} {
		t.Run(path, func(t *testing.T) {
			errDoc := loadBikeDocStatus(t, path, http.StatusBadRequest)
			rec, next := submitForm(t, errDoc, map[string]string{"frame": "esker-hayduke"})
			if rec.Code != http.StatusOK {
				t.Fatalf("switch to esker-hayduke status = %d, want 200", rec.Code)
			}
			if got := selectedValue(next, "size"); got != "S3" {
				t.Errorf("selected size = %q, want S3", got)
			}
		})
	}
}

// optgroupLabels returns the labels of a select's option groups in order.
func optgroupLabels(sel *goquery.Selection) []string {
	return sel.Find("optgroup").Map(func(_ int, g *goquery.Selection) string {
		label, _ := g.Attr("label")
		return label
	})
}

// TestBikeForkSelectGroupsListedAndOtherForks checks Q139 on /bike.
func TestBikeForkSelectGroupsListedAndOtherForks(t *testing.T) {
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"/bike", []string{"Listed Forks", "Other Compatible Forks", "Chart Basis Fork"}},
		{"/bike?frame=surly-krampus&size=L", []string{"Listed Forks", "Other Compatible Forks", "Chart Basis Fork"}},
		{"/bike?frame=surly-ecr&size=L", []string{"Listed Forks", "Chart Basis Fork"}},
		{"/bike?frame=tumbleweed-sunliner&size=L", []string{"Listed Forks", "Chart Basis Fork"}},
	} {
		doc := loadBikeDoc(t, tc.path)
		if got := optgroupLabels(doc.Find(`select[name="fork"]`)); !slices.Equal(got, tc.want) {
			t.Errorf("%s fork optgroups = %v, want %v", tc.path, got, tc.want)
		}
	}
	doc := loadBikeDoc(t, "/bike?frame=surly-krampus&size=L")
	if n := doc.Find(`select[name="fork"] optgroup[label="Other Compatible Forks"] option[value="pike_120"]`).Length(); n != 1 {
		t.Errorf("pike_120 under Other Compatible Forks = %d options, want 1", n)
	}
	if n := doc.Find(`select[name="fork"] optgroup[label="Listed Forks"] option[value="surly_krampus_483"]`).Length(); n != 1 {
		t.Errorf("surly_krampus_483 under Listed Forks = %d options, want 1", n)
	}
	fenrirDoc := loadBikeDoc(t, "/bike")
	if n := fenrirDoc.Find(`select[name="fork"] optgroup[label="Listed Forks"] option[value="enve_mountain_boost-490x44"]`).Length(); n != 1 {
		t.Errorf("enve_mountain_boost-490x44 under Fenrir Listed Forks = %d, want 1 (Q128)", n)
	}
	if n := fenrirDoc.Find(`select[name="fork"] optgroup[label="Other Compatible Forks"] option[value="enve_mountain_boost-490x44"]`).Length(); n != 0 {
		t.Errorf("enve_mountain_boost-490x44 under Fenrir Other Compatible Forks = %d, want 0", n)
	}
}

// TestCompareForkSelectGroupsListedAndOtherForks checks Q139 on /.
func TestBikeWorkbenchLayoutAndDisclosures(t *testing.T) {
	doc := loadBikeDoc(t, "/bike")
	if got := strings.TrimSpace(doc.Find("head title").Text()); got != "Otso Fenrir Stainless (2024, Size L) — bikegeo" {
		t.Errorf("title = %q, want Otso Fenrir Stainless (2024, Size L) — bikegeo", got)
	}
	if n := doc.Find(".diagram-panel nav.diagram-views").Length(); n != 1 {
		t.Errorf("got %d .diagram-panel nav.diagram-views, want 1", n)
	}
	if n := doc.Find("form.controls nav.diagram-views").Length(); n != 0 {
		t.Errorf("got %d nav.diagram-views inside form.controls, want 0", n)
	}

	notesDrawer := doc.Find("[data-notes-drawer]")
	if notesDrawer.Length() != 1 {
		t.Fatalf("got %d [data-notes-drawer], want 1", notesDrawer.Length())
	}
	if _, open := notesDrawer.Attr("open"); open {
		t.Error("[data-notes-drawer] should be closed by default")
	}
	if sum := strings.TrimSpace(notesDrawer.Find("summary").Text()); !strings.Contains(sum, "Build & model notes") || !strings.Contains(sum, "(Redshift caveat)") {
		t.Errorf("notes drawer summary = %q, want Build & model notes + (Redshift caveat)", sum)
	}
	if n := notesDrawer.Find(`[data-note="redshift-mixed-state"]`).Length(); n != 1 {
		t.Errorf("got %d redshift-mixed-state notes, want 1", n)
	}
	if n := notesDrawer.Find(`[data-note="post-setback"]`).Length(); n != 1 {
		t.Errorf("got %d post-setback notes, want 1", n)
	}

	for _, sel := range []string{"[data-submitted-breakdown]", "[data-tier-definitions]", "[data-build-breakdown]"} {
		el := doc.Find(sel)
		if el.Length() != 1 {
			t.Fatalf("got %d %s, want 1", el.Length(), sel)
		}
		if _, open := el.Attr("open"); open {
			t.Errorf("%s should be closed by default", sel)
		}
	}

	nonRedshift := loadBikeDoc(t, "/bike?post=setback_0")
	if sum := nonRedshift.Find("[data-notes-drawer] summary").Text(); strings.Contains(sum, "Redshift caveat") {
		t.Errorf("setback_0 notes drawer summary = %q, must omit Redshift caveat", sum)
	}
	if n := nonRedshift.Find(`[data-note="redshift-mixed-state"]`).Length(); n != 0 {
		t.Errorf("setback_0 got %d redshift-mixed-state notes, want 0", n)
	}

	offCatalog := loadBikeDoc(t, "/bike?stem_length=65&stem_angle=-6")
	if n := offCatalog.Find(`[data-note="off-catalog-stem"]`).Length(); n != 1 {
		t.Errorf("off-catalog stem got %d notes, want 1", n)
	}
	if n := offCatalog.Find(`[data-submitted-breakdown]`).Length(); n != 0 {
		t.Errorf("off-catalog stem got %d [data-submitted-breakdown], want 0", n)
	}
}

func TestBikeWorkbenchErrorInvalidSelectOptions(t *testing.T) {
	for _, key := range []string{"frame", "size", "fork", "bar", "post", "bar_options", "fork_options"} {
		t.Run(key, func(t *testing.T) {
			doc := loadBikeDocStatus(t, "/bike?"+key+"=nope", http.StatusBadRequest)
			if got := strings.TrimSpace(doc.Find("title").Text()); !strings.HasPrefix(got, "Error — ") {
				t.Errorf("400 title = %q, want Error — prefix", got)
			}
			if doc.Find(`#form-error[role="alert"]`).Length() != 1 {
				t.Fatalf("missing #form-error[role=alert]")
			}
			if doc.Find(`#form-error [data-preset="error-reset"]`).Length() != 1 {
				t.Errorf("missing error-reset link in #form-error")
			}
			sel := doc.Find(fmt.Sprintf(`select[name=%q]`, key))
			if inv, _ := sel.Attr("aria-invalid"); inv != "true" {
				t.Errorf("select[name=%q] aria-invalid = %q, want true", key, inv)
			}
			selected := sel.Find("option[selected]")
			if selected.Length() != 1 {
				t.Fatalf("select[name=%q] selected options = %d, want 1", key, selected.Length())
			}
			if val, _ := selected.Attr("value"); val != "nope" {
				t.Errorf("selected value = %q, want nope", val)
			}
			if got := strings.TrimSpace(selected.Text()); got != "Invalid: nope" {
				t.Errorf("selected text = %q, want 'Invalid: nope'", got)
			}
			firstOpt := sel.Find("option").First()
			if val, _ := firstOpt.Attr("value"); val != "nope" {
				t.Errorf("first option value = %q, want nope", val)
			}
		})
	}
}

func TestBikeHTMLAccessibilityAndNoJS(t *testing.T) {
	for _, tc := range []struct {
		rawPath    string
		wantStatus int
	}{
		{"/bike", http.StatusOK},
		{"/bike?view=seat", http.StatusOK},
		{"/bike?brp=off&grip=off", http.StatusOK},
		{"/bike?frame=nope", http.StatusBadRequest},
		{"/bike?spacers=-1", http.StatusBadRequest},
	} {
		t.Run(tc.rawPath, func(t *testing.T) {
			doc := loadBikeDocStatus(t, tc.rawPath, tc.wantStatus)
			if n := doc.Find("script").Length(); n != 0 {
				t.Errorf("found %d <script> tags, want 0", n)
			}
			doc.Find("[style]").Each(func(i int, el *goquery.Selection) {
				if !el.Is("svg.diagram") {
					t.Errorf("non-diagram element %d has inline style attribute", i)
				}
			})
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
				for id := range strings.FieldsSeq(lb) {
					if doc.Find("#"+id).Length() == 0 {
						t.Errorf(".table-scroll %d references missing aria-labelledby #%s", i, id)
					}
				}
			})
			if n := doc.Find("details summary").Find("h1, h2, h3, h4, h5, h6").Length(); n != 0 {
				t.Errorf("found %d heading(s) inside details summary, want 0", n)
			}
			seenLabels := make(map[string]bool)
			doc.Find("a[data-build-apply]").Each(func(i int, a *goquery.Selection) {
				lbl, _ := a.Attr("aria-label")
				lbl = strings.TrimSpace(lbl)
				if lbl == "" || !strings.Contains(lbl, "Apply") {
					t.Errorf("apply link %d aria-label = %q, want non-empty containing 'Apply'", i, lbl)
				}
				if seenLabels[lbl] {
					t.Errorf("duplicate apply link aria-label %q", lbl)
				}
				seenLabels[lbl] = true
			})
		})
	}
}
