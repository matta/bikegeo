package web

import (
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

func TestStemWorkbenchRendersSVGAndMatchingTrace(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081/", nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}

	doc, err := goquery.NewDocumentFromReader(rec.Body)
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	if title := doc.Find("h1").Text(); !strings.Contains(title, "bikegeo-reboot") {
		t.Errorf("heading = %q, want bikegeo-reboot branding", title)
	}

	svg := doc.Find("svg.diagram")
	if svg.Length() != 1 {
		t.Fatalf("expected 1 svg.diagram, got %d", svg.Length())
	}
	if vb, ok := svg.Attr("viewBox"); !ok || strings.TrimSpace(vb) == "" {
		t.Errorf("svg.diagram missing viewBox attribute")
	}

	for _, name := range []string{"HeadTubeTop", "CoverTop", "SpacerTop", "SteererClamp", "BarClamp"} {
		sel := fmt.Sprintf(`svg.diagram [data-landmark="%s"]`, name)
		if doc.Find(sel).Length() != 1 {
			t.Errorf("expected 1 element matching %s, got %d", sel, doc.Find(sel).Length())
		}
	}

	// Verify BarClamp, leveled HTA, and traced steps match fixture frameset.HeadTubeTop + PlaceStem output.
	fix := otsoFenrirFixture()
	defaultFork, ok := findForkOption(fix.Forks, fix.DefaultForkID, fix.DefaultForkID)
	if !ok {
		t.Fatalf("missing default fork option %q", fix.DefaultForkID)
	}
	forkSetup, err := defaultFork.Spec.Setup(defaultFork.DefaultSagPct, fix.DefaultLowerCupMm)
	if err != nil {
		t.Fatalf("defaultFork.Spec.Setup: %v", err)
	}
	frameset, err := assembly.PlaceFrameset(fix.Chart.Frame, forkSetup, fix.Wheels, nil)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}
	tr := pose.NewTrace()
	steererUp := pose.NewPose(frameset.HeadTubeTop.Origin(), frameset.HeadTubeTop.HeadingDeg(), tr)
	placed, err := assembly.PlaceStem(steererUp, fix.DefaultStem)
	tr.Freeze()
	if err != nil {
		t.Fatalf("PlaceStem: %v", err)
	}

	wantHTA := fmt.Sprintf("%.2f°", frameset.LeveledHeadTubeAngleDeg())
	gotHTA := strings.TrimSpace(doc.Find(`[data-metric="hta"]`).Text())
	if gotHTA != wantHTA {
		t.Errorf("hta metric = %q, want %q", gotHTA, wantHTA)
	}
	if gotHTA == "68.25°" {
		t.Errorf("hta metric = %q, expected leveled HTA rather than unpitched 68.25°", gotHTA)
	}

	wantBarClamp := fmt.Sprintf("(%.2f, %.2f) mm @ %.2f°",
		placed.BarClamp.Origin().X, placed.BarClamp.Origin().Y, placed.BarClamp.HeadingDeg())
	if got := strings.TrimSpace(doc.Find(`[data-metric="bar-clamp"]`).Text()); got != wantBarClamp {
		t.Errorf("bar-clamp metric = %q, want %q", got, wantBarClamp)
	}

	stemBody := doc.Find(`svg.diagram rect[data-component="stem-body"]`)
	if stemBody.Length() != 1 {
		t.Fatalf("expected 1 stem-body rect, got %d", stemBody.Length())
	}
	if x, _ := stemBody.Attr("x"); x != "-60.00" {
		t.Errorf("stem-body x = %q, want -60.00", x)
	}
	wantTransform := fmt.Sprintf("translate(%.2f %.2f) rotate(%.2f)",
		placed.BarClamp.Origin().X, -placed.BarClamp.Origin().Y, -float64(placed.BarClamp.HeadingDeg()))
	if tf, _ := stemBody.Attr("transform"); tf != wantTransform {
		t.Errorf("stem-body transform = %q, want %q", tf, wantTransform)
	}

	// Verify every trace table row matches expected steps and has a matching SVG step element.
	wantSteps := tr.Steps()
	rows := doc.Find("table tbody tr[data-step-id]")
	if rows.Length() != len(wantSteps) {
		t.Fatalf("trace table rows = %d, want %d", rows.Length(), len(wantSteps))
	}
	rows.Each(func(i int, row *goquery.Selection) {
		id, ok := row.Attr("data-step-id")
		if !ok || id == "" {
			t.Errorf("table row missing data-step-id")
			return
		}
		svgMatch := doc.Find(fmt.Sprintf(`svg.diagram .trace-steps [data-step-id="%s"]`, id))
		if svgMatch.Length() != 1 {
			t.Errorf("step %s: expected 1 SVG trace element, got %d", id, svgMatch.Length())
		}
		cells := row.Find("td")
		wantParent := "root"
		if wantSteps[i].ParentID != 0 {
			wantParent = fmt.Sprintf("#%d", wantSteps[i].ParentID)
		}
		if gotParent := strings.TrimSpace(cells.Eq(1).Text()); gotParent != wantParent {
			t.Errorf("step %s parent cell = %q, want %q", id, gotParent, wantParent)
		}
		wantTo := fmt.Sprintf("(%.2f, %.2f) @ %.2f°",
			wantSteps[i].To.Origin.X, wantSteps[i].To.Origin.Y, wantSteps[i].To.HeadingDeg)
		if gotTo := strings.TrimSpace(cells.Eq(6).Text()); gotTo != wantTo {
			t.Errorf("step %s To cell = %q, want %q", id, gotTo, wantTo)
		}
	})

	// Verify every SVG [data-step-id] (steps and landmarks) corresponds to a table row.
	doc.Find("svg.diagram [data-step-id]").Each(func(_ int, el *goquery.Selection) {
		id, _ := el.Attr("data-step-id")
		if doc.Find(fmt.Sprintf(`table tbody tr[data-step-id="%s"]`, id)).Length() != 1 {
			t.Errorf("SVG element with data-step-id=%q has no matching trace table row", id)
		}
	})
	if _, hasID := doc.Find(`svg.diagram [data-landmark="HeadTubeTop"]`).Attr("data-step-id"); hasID {
		t.Errorf("root landmark HeadTubeTop should not have data-step-id")
	}

	// Turn steps (#4 and #5) must render as distinct directed arcs, not identical circles.
	turn4 := doc.Find(`svg.diagram .trace-steps path[data-step-id="4"]`)
	turn5 := doc.Find(`svg.diagram .trace-steps path[data-step-id="5"]`)
	if turn4.Length() != 1 || turn5.Length() != 1 {
		t.Fatalf("expected 1 <path> each for turn steps 4 and 5, got %d and %d", turn4.Length(), turn5.Length())
	}
	d4, _ := turn4.Attr("d")
	d5, _ := turn5.Attr("d")
	if d4 == "" || d5 == "" || d4 == d5 {
		t.Errorf("turn step paths must be non-empty and distinct: step 4=%q, step 5=%q", d4, d5)
	}
}

func TestUnknownPathReturnsNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081/nope", nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /nope status = %d, want 404", rec.Code)
	}
}

func TestBikeWorkbenchRendersAndRespondsToForkQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081/bike?fork=fox34_120&cup=12&sag=20", nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /bike status = %d, want 200", rec.Code)
	}

	doc, err := goquery.NewDocumentFromReader(rec.Body)
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	if got := strings.TrimSpace(doc.Find(`[data-metric="effective-a2c"]`).Text()); got != "506.0 mm" {
		t.Errorf("effective-a2c = %q, want 506.0 mm", got)
	}

	for _, name := range []string{
		"RearAxle", "RearContact", "BB", "SeatTubeTop",
		"SeatpostClamp", "RailCenter", "BRP", "SitBones",
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
	fix := otsoFenrirFixture()
	fox34, ok := findForkOption(fix.Forks, "fox34_120", fix.DefaultForkID)
	if !ok {
		t.Fatalf("missing fox34_120 fixture option")
	}
	forkSetup, err := fox34.Spec.Setup(20.0, 12.0)
	if err != nil {
		t.Fatalf("fox34 setup: %v", err)
	}
	bike, err := assembly.PlaceBike(assembly.BikeSetup{
		Frame:     fix.Chart.Frame,
		Fork:      forkSetup,
		Wheels:    fix.Wheels,
		Seatpost:  fix.DefaultSeatpost,
		Saddle:    fix.DefaultSaddle,
		Stem:      fix.DefaultStem,
		Handlebar: fix.DefaultHandlebar,
	}, nil)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}
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
	if got := strings.TrimSpace(doc.Find(`[data-metric="seat-axis-post-top"]`).Text()); got != "753.5 mm" {
		t.Errorf("seat-axis-post-top = %q, want 753.5 mm", got)
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

	cadBaselineFork, err := assembly.PlaceCadBaselineFork(fix.Chart, fix.Wheels)
	if err != nil {
		t.Fatalf("PlaceCadBaselineFork: %v", err)
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="delta-pitch"]`).Text()); got != fmt.Sprintf("%+.2f°", cadBaselineFork.DeltaPitchDeg(placed)) {
		t.Errorf("delta-pitch = %q, want %+.2f°", got, cadBaselineFork.DeltaPitchDeg(placed))
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="residual-axle-y"]`).Text()); got != fmt.Sprintf("%+.2f mm", cadBaselineFork.Residuals.AxleHeightDiffMm) {
		t.Errorf("residual-axle-y = %q, want %+.2f mm", got, cadBaselineFork.Residuals.AxleHeightDiffMm)
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="residual-wheelbase"]`).Text()); got != fmt.Sprintf("%+.2f mm", cadBaselineFork.Residuals.WheelbaseDiffMm) {
		t.Errorf("residual-wheelbase = %q, want %+.2f mm", got, cadBaselineFork.Residuals.WheelbaseDiffMm)
	}
	if prevFork, _ := doc.Find(`input[name="prev_fork"]`).Attr("value"); prevFork != "fox34_120" {
		t.Errorf("prev_fork hidden input = %q, want fox34_120", prevFork)
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

	if facePaths := doc.Find(`svg.diagram .trace-steps path[data-op="face"]`).Length(); facePaths != 4 {
		t.Errorf("expected 4 face step paths in trace-steps, got %d", facePaths)
	}

	rows := doc.Find("table tbody tr[data-step-id]")
	if rows.Length() == 0 {
		t.Fatalf("expected non-empty trace table on /bike")
	}
	rows.Each(func(_ int, row *goquery.Selection) {
		id, _ := row.Attr("data-step-id")
		if doc.Find(fmt.Sprintf(`svg.diagram .trace-steps [data-step-id="%s"]`, id)).Length() != 1 {
			t.Errorf("step %s: expected 1 SVG trace element", id)
		}
	})
	doc.Find("svg.diagram [data-step-id]").Each(func(_ int, el *goquery.Selection) {
		id, _ := el.Attr("data-step-id")
		if doc.Find(fmt.Sprintf(`table tbody tr[data-step-id="%s"]`, id)).Length() != 1 {
			t.Errorf("SVG element with data-step-id=%q has no matching trace table row", id)
		}
	})
}

func TestBikeWorkbenchForkSwitchAndCadBaselineFork(t *testing.T) {
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
			query:       "/bike?prev_fork=fox34_120&fork=lithic_carbon_mountain&fork_a2c=530&cup=12&sag=20",
			wantCode:    http.StatusOK,
			wantA2C:     "495.0 mm",
			wantForkA2C: "495.0",
			wantSag:     "0.0",
			wantCup:     "12.0",
		},
		{
			name:        "switch rigid to SID applies SID default A2C and 20% sag",
			query:       "/bike?prev_fork=lithic_carbon_mountain&fork=sid_100&fork_a2c=495&cup=12&sag=0",
			wantCode:    http.StatusOK,
			wantA2C:     "486.0 mm",
			wantForkA2C: "506.0",
			wantSag:     "20.0",
			wantCup:     "12.0",
		},
		{
			name:        "staying on SID honors explicit sag=0",
			query:       "/bike?prev_fork=sid_100&fork=sid_100&cup=12&sag=0",
			wantCode:    http.StatusOK,
			wantA2C:     "506.0 mm",
			wantForkA2C: "506.0",
			wantSag:     "0.0",
			wantCup:     "12.0",
		},
		{
			name:        "staying on SID honors explicit fork_a2c=520 and sag=25",
			query:       "/bike?prev_fork=sid_100&fork=sid_100&fork_a2c=520&cup=12&sag=25",
			wantCode:    http.StatusOK,
			wantA2C:     "495.0 mm",
			wantForkA2C: "520.0",
			wantSag:     "25.0",
			wantCup:     "12.0",
		},
		{
			name:     "direct rigid fork with non-zero sag returns 400",
			query:    "/bike?fork=lithic_carbon_mountain&sag=20",
			wantCode: http.StatusBadRequest,
		},
		{
			name:        "CAD baseline fork preset uses fixture CadFork and 0 cup",
			query:       defaultWorkbenchState(otsoFenrirFixture()).withCadBaselineFork(otsoFenrirFixture()).URL(),
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
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081"+tc.query, nil)
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
				if val, _ := selectedOpt.Attr("value"); val != "lithic_carbon_mountain" {
					t.Errorf("CAD baseline fork selected fork option = %q, want lithic_carbon_mountain", val)
				}
				if _, disabled := selectedOpt.Attr("disabled"); disabled {
					t.Errorf("CAD baseline fork selected fork option should not be disabled")
				}
			}
		})
	}
}

func TestBikeWorkbenchCockpitControlsUpdateBuildAndTrace(t *testing.T) {
	query := "/bike?fork=sid_100&prev_fork=sid_100&fork_a2c=515&cup=10&sag=20" +
		"&spacers=35&stem_length=80&stem_angle=7&bar_roll=5" +
		"&post_ext=250&post_setback=20&rail_offset=-10&rail_pitch=-2"
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081"+query, nil)
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

	fix := otsoFenrirFixture()
	sid, ok := findForkOption(fix.Forks, "sid_100", fix.DefaultForkID)
	if !ok {
		t.Fatalf("missing sid_100 fixture option")
	}
	spec := sid.Spec
	spec.StaticA2cMm = 515.0
	forkSetup, err := spec.Setup(20.0, 10.0)
	if err != nil {
		t.Fatalf("spec.Setup: %v", err)
	}
	stemSetup := fix.DefaultStem
	stemSetup.SpacersMm = 35.0
	stemSetup.LengthMm = 80.0
	stemSetup.AngleDeg = 7.0
	barSetup := fix.DefaultHandlebar
	barSetup.BarRollDeg = 5.0
	postSetup := fix.DefaultSeatpost
	postSetup.ExtensionMm = 250.0
	postSetup.PostSetbackMm = 20.0
	saddleSetup := fix.DefaultSaddle
	saddleSetup.RailOffsetMm = -10.0
	saddleSetup.RailPitchDeg = -2.0

	bike, err := assembly.PlaceBike(assembly.BikeSetup{
		Frame:     fix.Chart.Frame,
		Fork:      forkSetup,
		Wheels:    fix.Wheels,
		Seatpost:  postSetup,
		Saddle:    saddleSetup,
		Stem:      stemSetup,
		Handlebar: barSetup,
	}, nil)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}

	for _, metric := range []struct {
		key  string
		want string
	}{
		{"effective-a2c", fmt.Sprintf("%.1f mm", forkSetup.A2cMm)},
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
	if w, _ := doc.Find(`svg.diagram rect[data-component="spacers"]`).Attr("width"); w != "35.00" {
		t.Errorf("spacers width = %q, want 35.00", w)
	}

	for name, want := range map[string]string{
		"prev_fork":    "sid_100",
		"fork_a2c":     "515.0",
		"cup":          "10.0",
		"sag":          "20.0",
		"spacers":      "35.0",
		"stem_length":  "80.0",
		"stem_angle":   "7.0",
		"bar_roll":     "5.0",
		"post_ext":     "250.0",
		"post_setback": "20.0",
		"rail_offset":  "-10.0",
		"rail_pitch":   "-2.0",
	} {
		got, _ := doc.Find(fmt.Sprintf(`input[name="%s"]`, name)).Attr("value")
		if got != want {
			t.Errorf("input[name=%q] value = %q, want %q", name, got, want)
		}
	}

	labels := doc.Find("form.controls label")
	if labels.Length() != 12 {
		t.Errorf("form label count = %d, want 12", labels.Length())
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

	rows := doc.Find("table tbody tr[data-step-id]")
	if rows.Length() == 0 {
		t.Fatalf("expected non-empty trace table")
	}
	rows.Each(func(_ int, row *goquery.Selection) {
		id, _ := row.Attr("data-step-id")
		if doc.Find(fmt.Sprintf(`svg.diagram .trace-steps [data-step-id="%s"]`, id)).Length() != 1 {
			t.Errorf("step %s: expected 1 SVG trace element", id)
		}
	})
	doc.Find("svg.diagram [data-step-id]").Each(func(_ int, el *goquery.Selection) {
		id, _ := el.Attr("data-step-id")
		if doc.Find(fmt.Sprintf(`table tbody tr[data-step-id="%s"]`, id)).Length() != 1 {
			t.Errorf("SVG element with data-step-id=%q has no matching trace table row", id)
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
		{"/bike?cup=-1", "cup", "-1"},
		{"/bike?cup=NaN", "cup", "NaN"},
		{"/bike?sag=150", "sag", "150"},
		{"/bike?sag=Inf", "sag", "Inf"},
		{"/bike?fork_a2c=0", "fork_a2c", "0"},
		{"/bike?fork_a2c=abc", "fork_a2c", "abc"},
		{"/bike?fork=fox34_120&fork_a2c=50&sag=50", "fork_a2c", "50"},
		{"/bike?fork=fox34_120&fork_a2c=50&sag=50&cup=-1", "fork_a2c", "50"},
		{"/bike?spacers=-1", "spacers", "-1"},
		{"/bike?stem_length=0", "stem_length", "0"},
		{"/bike?stem_angle=NaN", "stem_angle", "NaN"},
		{"/bike?bar_roll=Inf", "bar_roll", "Inf"},
		{"/bike?post_ext=-1", "post_ext", "-1"},
		{"/bike?post_ext=-1&spacers=-1", "post_ext", "-1"},
		{"/bike?post_ext=109.5", "post_ext", "109.5"},
		{"/bike?post_ext=300.5", "post_ext", "300.5"},
		{"/bike?post_setback=-2", "post_setback", "-2"},
		{"/bike?rail_offset=NaN", "rail_offset", "NaN"},
		{"/bike?rail_pitch=Inf", "rail_pitch", "Inf"},
		{"/bike?fork=lithic_carbon_mountain&fork_a2c=495.0&cup=0.0&sag=0.0&spacers=-1", "spacers", "-1"},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081"+tc.query, nil)
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
			if tc.wantVal != "" {
				if val, _ := target.Attr("value"); val != tc.wantVal {
					t.Errorf("input[name=%q] value = %q, want %q", tc.invalidField, val, tc.wantVal)
				}
			}
		})
	}
}

func TestBikeWorkbenchAcceptsSeatpostExtensionLimits(t *testing.T) {
	for _, extension := range []string{"110", "300"} {
		t.Run(extension, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081/bike?post_ext="+extension, nil)
			rec := httptest.NewRecorder()
			NewHandler().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("extension %s returned %d, want 200", extension, rec.Code)
			}
			doc, err := goquery.NewDocumentFromReader(rec.Body)
			if err != nil {
				t.Fatalf("parse HTML: %v", err)
			}
			input := doc.Find(`input[name="post_ext"]`)
			for attr, want := range map[string]string{"min": "110", "max": "300", "value": extension + ".0"} {
				if got, _ := input.Attr(attr); got != want {
					t.Errorf("seatpost extension %s = %q, want %q", attr, got, want)
				}
			}
		})
	}
}

func TestBuildBikeViewUsesPlacedDimensions(t *testing.T) {
	fix := otsoFenrirFixture()
	forkSetup := assembly.ForkSetup{A2cMm: 506.0, RakeMm: 44.0, LowerCupMm: 12.0}
	customWheels := assembly.WheelSetup{RadiusMm: 340.0}
	customSeatpost := assembly.SeatpostSetup{ExtensionMm: 240.0, PostSetbackMm: 15.0, ClampLengthMm: 30.0}
	customSaddle := fix.DefaultSaddle
	customSaddle.RailOffsetMm = 12.0
	customSaddle.RailPitchDeg = -1.5
	customStem := fix.DefaultStem
	customStem.SpacersMm = 15.0
	customStem.LengthMm = 70.0
	customStem.AngleDeg = 6.0
	customBar := fix.DefaultHandlebar
	customBar.BarRollDeg = -4.0

	tr := pose.NewTrace()
	bike, err := assembly.PlaceBike(assembly.BikeSetup{
		Frame:     fix.Chart.Frame,
		Fork:      forkSetup,
		Wheels:    customWheels,
		Seatpost:  customSeatpost,
		Saddle:    customSaddle,
		Stem:      customStem,
		Handlebar: customBar,
	}, tr)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}
	cadBaselineFork, err := assembly.PlaceCadBaselineFork(fix.Chart, customWheels)
	if err != nil {
		t.Fatalf("PlaceCadBaselineFork: %v", err)
	}

	view := buildBikeView(fix, "fox34_120", 530.0, 20.0, bike, cadBaselineFork, tr.Steps())

	if view.WheelRadiusMm != customWheels.RadiusMm {
		t.Errorf("view.WheelRadiusMm = %.1f, want placed radius %.1f (not fixture %.1f)", view.WheelRadiusMm, customWheels.RadiusMm, fix.Wheels.RadiusMm)
	}
	if wantGroundY := -bike.Frameset.RearTireContact.Origin().Y; math.Abs(view.GroundY1-wantGroundY) > 1e-12 {
		t.Errorf("view.GroundY1 = %.6f, want %.6f", view.GroundY1, wantGroundY)
	}
	if gotSpan := view.GroundY1 - view.RearAxleSVG.Y; math.Abs(gotSpan-customWheels.RadiusMm) > 1e-9 {
		t.Errorf("ground-to-axle span = %.6f, want %.6f", gotSpan, customWheels.RadiusMm)
	}
	if view.LowerCupMm != forkSetup.LowerCupMm {
		t.Errorf("view.LowerCupMm = %.1f, want %.1f", view.LowerCupMm, forkSetup.LowerCupMm)
	}
	if view.EffectiveA2cMm != forkSetup.A2cMm {
		t.Errorf("view.EffectiveA2cMm = %.1f, want %.1f", view.EffectiveA2cMm, forkSetup.A2cMm)
	}
	wantForm := bikeFormValues{
		Fork:        "fox34_120",
		PrevFork:    "fox34_120",
		ForkA2c:     "530.0",
		Cup:         "12.0",
		Sag:         "20.0",
		Spacers:     "15.0",
		StemLength:  "70.0",
		StemAngle:   "6.0",
		BarRoll:     "-4.0",
		PostExt:     "240.0",
		PostSetback: "15.0",
		RailOffset:  "12.0",
		RailPitch:   "-1.5",
	}
	if view.Form != wantForm {
		t.Errorf("view.Form = %+v, want %+v", view.Form, wantForm)
	}

	cadURL, err := url.Parse(view.CadBaselineForkURL)
	if err != nil {
		t.Fatalf("parse CadBaselineForkURL %q: %v", view.CadBaselineForkURL, err)
	}
	cadDecoded, inputErr := decodeWorkbenchState(fix, cadURL.Query())
	if inputErr != nil {
		t.Fatalf("decodeWorkbenchState(CadBaselineForkURL): %v", inputErr)
	}
	wantCadState := workbenchState{
		ForkID:        fix.CadForkID,
		ForkA2cMm:     fix.Chart.CadFork.A2cMm,
		LowerCupMm:    fix.Chart.CadFork.LowerCupMm,
		SagPct:        0.0,
		SpacersMm:     15.0,
		StemLengthMm:  70.0,
		StemAngleDeg:  6.0,
		BarRollDeg:    -4.0,
		PostExtMm:     240.0,
		PostSetbackMm: 15.0,
		RailOffsetMm:  12.0,
		RailPitchDeg:  -1.5,
	}
	if cadDecoded.State != wantCadState {
		t.Errorf("decoded CadBaselineForkURL state = %+v, want %+v", cadDecoded.State, wantCadState)
	}
}

func TestWorkbenchStateCodecRoundTrip(t *testing.T) {
	fix := otsoFenrirFixture()
	want := workbenchState{
		ForkID:        "fox34_120",
		ForkA2cMm:     535.0,
		LowerCupMm:    12.5,
		SagPct:        22.0,
		SpacersMm:     35.0,
		StemLengthMm:  80.0,
		StemAngleDeg:  7.0,
		BarRollDeg:    -5.0,
		PostExtMm:     250.0,
		PostSetbackMm: 20.0,
		RailOffsetMm:  10.0,
		RailPitchDeg:  -2.0,
	}

	u, err := url.Parse(want.URL())
	if err != nil {
		t.Fatalf("parse encoded URL %q: %v", want.URL(), err)
	}
	if u.Query().Has("prev_fork") || u.Query().Has("baseline") {
		t.Errorf("encoded URL %q should not include prev_fork or baseline", want.URL())
	}

	decoded, inputErr := decodeWorkbenchState(fix, u.Query())
	if inputErr != nil {
		t.Fatalf("decodeWorkbenchState error: %v", inputErr)
	}
	if decoded.State != want {
		t.Errorf("decoded State = %+v, want %+v", decoded.State, want)
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
	target := "http://127.0.0.1:8081" + action + "?" + vals.Encode()
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
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081"+path, nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200 (body: %s)", path, rec.Code, rec.Body.String())
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
	wantSteps := doc.Find("table tbody tr[data-step-id]").Length()

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
	if gotSteps := nextDoc.Find("table tbody tr[data-step-id]").Length(); gotSteps != wantSteps {
		t.Errorf("%s unchanged submit trace rows = %d, want %d", label, gotSteps, wantSteps)
	}
	return nextDoc
}

func TestBikeWorkbenchFormRoundTripPreservesEveryBuild(t *testing.T) {
	// 1. Default build.
	defaultDoc := loadBikeDoc(t, "/bike")
	assertUnchangedSubmitIdempotent(t, "default build", defaultDoc)

	// 2. Follow CAD Baseline Fork preset from default build.
	cadHref, ok := defaultDoc.Find(`a[data-preset="cad-baseline-fork"]`).Attr("href")
	if !ok || cadHref == "" {
		t.Fatalf("missing a[data-preset=cad-baseline-fork] href")
	}
	cadDoc := loadBikeDoc(t, cadHref)
	if got, _ := cadDoc.Find(`input[name="cup"]`).Attr("value"); got != "0.0" {
		t.Errorf("cad preset cup = %q, want 0.0", got)
	}
	if got := strings.TrimSpace(cadDoc.Find(`[data-metric="delta-pitch"]`).Text()); got != "+0.00°" {
		t.Errorf("cad preset delta-pitch = %q, want +0.00°", got)
	}
	assertUnchangedSubmitIdempotent(t, "cad preset from default", cadDoc)

	// 3. Fully customized build.
	customURL := "/bike?fork=fox34_120&fork_a2c=530&cup=12&sag=20" +
		"&spacers=35&stem_length=80&stem_angle=7&bar_roll=-5" +
		"&post_ext=250&post_setback=20&rail_offset=10&rail_pitch=-2"
	customDoc := loadBikeDoc(t, customURL)
	assertUnchangedSubmitIdempotent(t, "custom build", customDoc)

	// 4. Follow CAD Baseline Fork from customized build: preserves cockpit, sets cup=0,
	//    round-trips unchanged, and keeps CAD baseline fork when spacers are later edited.
	customCadHref, ok := customDoc.Find(`a[data-preset="cad-baseline-fork"]`).Attr("href")
	if !ok || customCadHref == "" {
		t.Fatalf("missing a[data-preset=cad-baseline-fork] href on custom build")
	}
	customCadDoc := loadBikeDoc(t, customCadHref)
	if got, _ := customCadDoc.Find(`input[name="spacers"]`).Attr("value"); got != "35.0" {
		t.Errorf("cad preset spacers = %q, want 35.0", got)
	}
	if got, _ := customCadDoc.Find(`input[name="stem_length"]`).Attr("value"); got != "80.0" {
		t.Errorf("cad preset stem_length = %q, want 80.0", got)
	}
	if got, _ := customCadDoc.Find(`input[name="cup"]`).Attr("value"); got != "0.0" {
		t.Errorf("cad preset cup = %q, want 0.0", got)
	}
	if got := strings.TrimSpace(customCadDoc.Find(`[data-metric="delta-pitch"]`).Text()); got != "+0.00°" {
		t.Errorf("cad preset delta-pitch = %q, want +0.00°", got)
	}
	assertUnchangedSubmitIdempotent(t, "cad preset from custom build", customCadDoc)

	rec, editedDoc := submitForm(t, customCadDoc, map[string]string{"spacers": "30"})
	if rec.Code != http.StatusOK {
		t.Fatalf("editing spacers on cad build status = %d, want 200", rec.Code)
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
	defaultsHref, ok := customCadDoc.Find(`a[data-preset="defaults"]`).Attr("href")
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
	cadHref, _ := defaultDoc.Find(`a[data-preset="cad-baseline-fork"]`).Attr("href")
	cadDoc := loadBikeDoc(t, cadHref)
	cadMetrics := collectMetrics(cadDoc)

	// On the rigid CAD baseline fork, changing sag to 10 must return 400 (not silently revert).
	rec, errDoc := submitForm(t, cadDoc, map[string]string{"sag": "10"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mutating sag on rigid CAD baseline fork status = %d, want 400", rec.Code)
	}
	if inv, _ := errDoc.Find(`[name="sag"]`).Attr("aria-invalid"); inv != "true" {
		t.Errorf("sag aria-invalid = %q, want true", inv)
	}

	// Every other enabled control on the CAD baseline fork page must take effect and change the build.
	mutations := []struct {
		field    string
		val      string
		isSelect bool
	}{
		{field: "fork", val: "sid_100", isSelect: true},
		{field: "fork_a2c", val: "510.0"},
		{field: "cup", val: "12.0"},
		{field: "spacers", val: "30.0"},
		{field: "stem_length", val: "80.0"},
		{field: "stem_angle", val: "-12.0"},
		{field: "bar_roll", val: "8.0"},
		{field: "post_ext", val: "250.0"},
		{field: "post_setback", val: "20.0"},
		{field: "rail_offset", val: "15.0"},
		{field: "rail_pitch", val: "-4.0"},
	}
	for _, m := range mutations {
		t.Run("cad/"+m.field, func(t *testing.T) {
			rec, nextDoc := submitForm(t, cadDoc, map[string]string{m.field: m.val})
			if rec.Code != http.StatusOK {
				t.Fatalf("submit %s=%s status = %d, want 200", m.field, m.val, rec.Code)
			}
			if m.isSelect {
				got, _ := nextDoc.Find(fmt.Sprintf(`select[name="%s"] option[selected]`, m.field)).Attr("value")
				if got != m.val {
					t.Errorf("select[name=%q] selected = %q, want %q", m.field, got, m.val)
				}
			} else {
				got, _ := nextDoc.Find(fmt.Sprintf(`input[name="%s"]`, m.field)).Attr("value")
				if got != m.val {
					t.Errorf("input[name=%q] value = %q, want %q", m.field, got, m.val)
				}
			}
			if maps.Equal(collectMetrics(nextDoc), cadMetrics) {
				t.Errorf("mutating %s=%s left all rendered metrics unchanged", m.field, m.val)
			}
		})
	}

	// On a suspension fork build, sag is also live and changes effective A2C.
	sidDoc := loadBikeDoc(t, "/bike?fork=sid_100&cup=12&sag=20")
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
