package web

import (
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

func defaultHandlebarSetup(t *testing.T, fix bikeFixture) assembly.HandlebarSetup {
	t.Helper()
	opt, ok := fix.resolveHandlebar("")
	if !ok {
		t.Fatalf("missing default handlebar %q", fix.DefaultHandlebarID)
	}
	return opt.Setup
}

// fixtureSeatpost returns the fixture post with id, at extension extMm.
func fixtureSeatpost(t *testing.T, fix bikeFixture, id string, extMm float64) assembly.SeatpostSetup {
	t.Helper()
	opt, ok := fix.resolveSeatpost(id)
	if !ok {
		t.Fatalf("missing seatpost %q", id)
	}
	setup := opt.Setup
	setup.ExtensionMm = extMm
	return setup
}

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
	defaultFork, ok := fix.resolveForkOption("")
	if !ok {
		t.Fatalf("missing default fork option %q", fix.DefaultForkID)
	}
	forkSetup := assembly.ForkSetup{
		Spec:   defaultFork.Spec,
		SagPct: defaultFork.DefaultSagPct,
	}
	frameset, err := assembly.PlaceFrameset(fix.Chart.Frame, defaultFork.DefaultLowerCup, forkSetup, fix.Wheels, nil)
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
	fox34, ok := fix.resolveForkOption("fox34_120")
	if !ok {
		t.Fatalf("missing fox34_120 fixture option")
	}
	forkSetup := assembly.ForkSetup{
		Spec:   fox34.Spec,
		SagPct: 20.0,
	}
	bike, err := assembly.PlaceBike(assembly.BikeSetup{
		Frame:     fix.Chart.Frame,
		LowerCup:  fox34.DefaultLowerCup,
		Fork:      forkSetup,
		Wheels:    fix.Wheels,
		Seatpost:  fixtureSeatpost(t, fix, fix.DefaultSeatpostID, fix.DefaultPostExtMm),
		Saddle:    fix.DefaultSaddle,
		Stem:      fix.DefaultStem,
		Handlebar: defaultHandlebarSetup(t, fix),
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
		{
			name:        "switch CAD reference to Lithic restores physical 12 mm cup",
			query:       "/bike?prev_fork=cad&fork=lithic_carbon_mountain&fork_a2c=495&cup=0&sag=0",
			wantCode:    http.StatusOK,
			wantA2C:     "495.0 mm",
			wantForkA2C: "495.0",
			wantSag:     "0.0",
			wantCup:     "12.0",
		},
		{
			name:        "switch CAD reference to Fox 34 restores physical 12 mm cup and Fox 34 defaults",
			query:       "/bike?prev_fork=cad&fork=fox34_120&fork_a2c=495&cup=0&sag=0",
			wantCode:    http.StatusOK,
			wantA2C:     "506.0 mm",
			wantForkA2C: "530.0",
			wantSag:     "20.0",
			wantCup:     "12.0",
		},
		{
			name:        "switch Fox 34 to CAD reference adopts 0 mm CAD cup and 495/0 fork",
			query:       "/bike?prev_fork=fox34_120&fork=cad&fork_a2c=530&cup=12&sag=20",
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
				if val, _ := selectedOpt.Attr("value"); val != cadForkOptionID {
					t.Errorf("CAD baseline fork selected fork option = %q, want %q", val, cadForkOptionID)
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
		"&post=setback_25&post_ext=250&rail_offset=-10&rail_pitch=-2"
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
	sid, ok := fix.resolveForkOption("sid_100")
	if !ok {
		t.Fatalf("missing sid_100 fixture option")
	}
	spec := sid.Spec
	spec.StaticA2cMm = 515.0
	forkSetup := assembly.ForkSetup{
		Spec:   spec,
		SagPct: 20.0,
	}
	stemSetup := fix.DefaultStem
	stemSetup.SpacersMm = 35.0
	stemSetup.LengthMm = 80.0
	stemSetup.AngleDeg = 7.0
	barSetup := defaultHandlebarSetup(t, fix)
	barSetup.BarRollDeg = 5.0
	postSetup := fixtureSeatpost(t, fix, "setback_25", 250.0)
	saddleSetup := fix.DefaultSaddle
	saddleSetup.RailOffsetMm = -10.0
	saddleSetup.RailPitchDeg = -2.0

	bike, err := assembly.PlaceBike(assembly.BikeSetup{
		Frame:     fix.Chart.Frame,
		LowerCup:  assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 10.0},
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
	if w, _ := doc.Find(`svg.diagram rect[data-component="spacers"]`).Attr("width"); w != "35.00" {
		t.Errorf("spacers width = %q, want 35.00", w)
	}

	for name, want := range map[string]string{
		"prev_fork":   "sid_100",
		"fork_a2c":    "515.0",
		"cup":         "10.0",
		"sag":         "20.0",
		"spacers":     "35.0",
		"stem_length": "80.0",
		"stem_angle":  "7.0",
		"bar_roll":    "5.0",
		"post_ext":    "250.0",
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
	if labels.Length() != 18 {
		t.Errorf("form label count = %d, want 18", labels.Length())
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
		{"/bike?bar=unknown", "bar", ""},
		{"/bike?stock_bars=some", "stock_bars", ""},
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
		{"/bike?post=unknown", "post", ""},
		{"/bike?rail_offset=NaN", "rail_offset", "NaN"},
		{"/bike?rail_pitch=Inf", "rail_pitch", "Inf"},
		{"/bike?fork=lithic_carbon_mountain&fork_a2c=495.0&cup=0.0&sag=0.0&spacers=-1", "cup", "0.0"},
		{"/bike?fork=cad&fork_a2c=495.0&cup=0.0&sag=0.0&spacers=-1", "spacers", "-1"},
		{"/bike?fork=lithic_carbon_mountain&cup=0", "cup", "0"},
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
	forkSetup := assembly.ForkSetup{
		Spec: assembly.ForkSpec{
			StaticA2cMm: 530.0,
			TravelMm:    120.0,
			RakeMm:      44.0,
		},
		SagPct: 20.0,
	}
	lowerCup := assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12.0}
	customWheels := assembly.WheelSetup{RadiusMm: 340.0}
	customSeatpost := fixtureSeatpost(t, fix, "setback_15", 240.0)
	customSaddle := fix.DefaultSaddle
	customSaddle.RailOffsetMm = 12.0
	customSaddle.RailPitchDeg = -1.5
	customStem := fix.DefaultStem
	customStem.SpacersMm = 15.0
	customStem.LengthMm = 70.0
	customStem.AngleDeg = 6.0
	customBar := defaultHandlebarSetup(t, fix)
	customBar.BarRollDeg = -4.0

	tr := pose.NewTrace()
	bike, err := assembly.PlaceBike(assembly.BikeSetup{
		Frame:     fix.Chart.Frame,
		LowerCup:  lowerCup,
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

	view := buildBikeView(fix, workbenchState{View: diagramViewBike, ForkID: "fox34_120", BarID: "persuader20_800", PostID: "setback_15"}, bike, cadBaselineFork, tr.Steps())

	if view.WheelRadiusMm != customWheels.RadiusMm {
		t.Errorf("view.WheelRadiusMm = %.1f, want placed radius %.1f (not fixture %.1f)", view.WheelRadiusMm, customWheels.RadiusMm, fix.Wheels.RadiusMm)
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
		View:       "bike",
		Fork:       "fox34_120",
		Bar:        "persuader20_800",
		Post:       "setback_15",
		PrevFork:   "fox34_120",
		ForkA2c:    "530.0",
		Cup:        "12.0",
		Sag:        "20.0",
		Spacers:    "15.0",
		StemLength: "70.0",
		StemAngle:  "6.0",
		BarRoll:    "-4.0",
		PostExt:    "240.0",
		RailOffset: "12.0",
		RailPitch:  "-1.5",
		StockBars:  stockBarsSelected,
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
		View:         diagramViewBike,
		ForkID:       cadForkOptionID,
		BarID:        "persuader20_800",
		PostID:       "setback_15",
		ForkA2cMm:    fix.Chart.CadFork.A2cMm,
		LowerCupMm:   fix.Chart.CadLowerCup.StackMm,
		SagPct:       0.0,
		SpacersMm:    15.0,
		StemLengthMm: 70.0,
		StemAngleDeg: 6.0,
		BarRollDeg:   -4.0,
		PostExtMm:    240.0,
		RailOffsetMm: 12.0,
		RailPitchDeg: -1.5,
	}
	if cadDecoded.State != wantCadState {
		t.Errorf("decoded CadBaselineForkURL state = %+v, want %+v", cadDecoded.State, wantCadState)
	}
}

func TestWorkbenchStateCodecRoundTrip(t *testing.T) {
	fix := otsoFenrirFixture()
	want := workbenchState{
		View:         diagramViewStem,
		ForkID:       "fox34_120",
		BarID:        "sqlab3ox_16_30",
		PostID:       "setback_25",
		ForkA2cMm:    535.0,
		LowerCupMm:   12.5000000001,
		SagPct:       22.0,
		SpacersMm:    35.0,
		StemLengthMm: 80.0,
		StemAngleDeg: 7.0,
		BarRollDeg:   -5.0,
		PostExtMm:    250.0,
		RailOffsetMm: 10.0,
		RailPitchDeg: -2.0,
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

	if got := formatFormFloat(math.Copysign(0, -1)); got != "0.0" {
		t.Errorf("formatFormFloat(-0.0) = %q, want 0.0", got)
	}
	if got := formatFormFloat(12.5000000001); got != "12.5000000001" {
		t.Errorf("formatFormFloat(12.5000000001) = %q, want 12.5000000001", got)
	}

	t.Run("all-stock-bars", func(t *testing.T) {
		want := defaultWorkbenchState(fix)
		want.AllStockBars = true
		q := url.Values{"stock_bars": {"all"}}
		decoded, inputErr := decodeWorkbenchState(fix, q)
		if inputErr != nil || decoded.State != want {
			t.Fatalf("decoded State = %+v, error = %v, want %+v", decoded.State, inputErr, want)
		}
		encoded, err := url.Parse(decoded.State.URL())
		if err != nil {
			t.Fatal(err)
		}
		if got := encoded.Query().Get("stock_bars"); got != "all" {
			t.Errorf("encoded stock_bars = %q, want all", got)
		}
		roundTrip, inputErr := decodeWorkbenchState(fix, encoded.Query())
		if inputErr != nil || roundTrip.State != want {
			t.Errorf("round-tripped State = %+v, error = %v, want %+v", roundTrip.State, inputErr, want)
		}
	})
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
	return loadBikeDocStatus(t, path, http.StatusOK)
}

func loadBikeDocStatus(t *testing.T, path string, wantStatus int) *goquery.Document {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081"+path, nil)
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
	customURL := "/bike?fork=fox34_120&fork_a2c=530&cup=12&sag=20&bar=sqlab3ox_16_45" +
		"&spacers=35&stem_length=80&stem_angle=7&bar_roll=-5" +
		"&post=setback_25&post_ext=250&rail_offset=10&rail_pitch=-2&stock_bars=all"
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

	// Every editable control on the CAD baseline fork page must take effect and change the build.
	mutations := []struct {
		field    string
		val      string
		isSelect bool
	}{
		{field: "fork", val: "sid_100", isSelect: true},
		{field: "bar", val: "sqlab311_flx_12_30", isSelect: true},
		{field: "spacers", val: "30.0"},
		{field: "stem_length", val: "80.0"},
		{field: "stem_angle", val: "-12.0"},
		{field: "bar_roll", val: "8.0"},
		{field: "post_ext", val: "250.0"},
		{field: "post", val: "setback_25", isSelect: true},
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
				if m.field == "fork" {
					if cup, _ := nextDoc.Find(`input[name="cup"]`).Attr("value"); cup != "12.0" {
						t.Errorf("cup after switching from CAD to %s = %q, want 12.0", m.val, cup)
					}
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

func TestFixtureValidationAndCadBaselineForkInvariant(t *testing.T) {
	fix := otsoFenrirFixture()
	if err := fix.validate(); err != nil {
		t.Fatalf("otsoFenrirFixture().validate(): %v", err)
	}

	// Decoding the CAD baseline fork preset link yields fork=cad, exact bare frame, CadLowerCup, and CAD fork geometry.
	cadURL, err := url.Parse(defaultWorkbenchState(fix).withCadBaselineFork(fix).URL())
	if err != nil {
		t.Fatalf("parse CAD URL: %v", err)
	}
	decoded, inputErr := decodeWorkbenchState(fix, cadURL.Query())
	if inputErr != nil {
		t.Fatalf("decodeWorkbenchState: %v", inputErr)
	}
	if decoded.State.ForkID != cadForkOptionID {
		t.Errorf("decoded ForkID = %q, want %q", decoded.State.ForkID, cadForkOptionID)
	}
	if decoded.Setup.Frame != fix.Chart.Frame {
		t.Errorf("decoded Frame = %+v, want %+v", decoded.Setup.Frame, fix.Chart.Frame)
	}
	if decoded.Setup.LowerCup != fix.Chart.CadLowerCup {
		t.Errorf("decoded LowerCup = %+v, want %+v", decoded.Setup.LowerCup, fix.Chart.CadLowerCup)
	}
	if err := decoded.Setup.Fork.Validate(); err != nil {
		t.Fatalf("decoded Fork.Validate: %v", err)
	}
	if decoded.Setup.Fork.Geometry() != fix.Chart.CadFork {
		t.Errorf("decoded Fork.Geometry() = %+v, want %+v", decoded.Setup.Fork.Geometry(), fix.Chart.CadFork)
	}

	// Fixture whose Chart.CadFork (481 mm / 51 mm) does not match any installable retail fork in Forks,
	// and whose installable forks pair with two different lower-cup standards (ZS44 0 mm vs EC44 12 mm):
	// fork=cad still reproduces Chart.CadFork with 0° delta pitch, and switching standards updates cup automatically.
	ec44Cup := assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12.0}
	zs44Cup := assembly.LowerCupSetup{Standard: assembly.LowerCupZS44, StackMm: 0.0}
	customFix := otsoFenrirFixture()
	customFix.Chart.CadFork = assembly.ForkGeometry{A2cMm: 481.0, RakeMm: 51.0}
	customFix.Forks = []forkOption{
		makeForkOption("rigid_straight_483", "Straight Steerer Rigid", assembly.ForkSpec{
			StaticA2cMm: 483.0,
			TravelMm:    0.0,
			RakeMm:      51.0,
		}, 0.0, zs44Cup),
		makeForkOption("sid_100", "RockShox SID", assembly.ForkSpec{
			StaticA2cMm: 506.0,
			TravelMm:    100.0,
			RakeMm:      51.0,
		}, 20.0, ec44Cup),
	}
	customFix.DefaultForkID = "rigid_straight_483"
	if err := customFix.validate(); err != nil {
		t.Fatalf("customFix.validate(): %v", err)
	}

	customCadURL, err := url.Parse(defaultWorkbenchState(customFix).withCadBaselineFork(customFix).URL())
	if err != nil {
		t.Fatalf("parse customFix CAD URL: %v", err)
	}
	if got := customCadURL.Query().Get("fork"); got != cadForkOptionID {
		t.Errorf("customFix CAD URL fork = %q, want %q", got, cadForkOptionID)
	}
	if got := customCadURL.Query().Get("fork_a2c"); got != "481.0" {
		t.Errorf("customFix CAD URL fork_a2c = %q, want 481.0", got)
	}
	if got := customCadURL.Query().Get("sag"); got != "0.0" {
		t.Errorf("customFix CAD URL sag = %q, want 0.0", got)
	}
	customDecoded, inputErr := decodeWorkbenchState(customFix, customCadURL.Query())
	if inputErr != nil {
		t.Fatalf("decode customFix CAD URL: %v", inputErr)
	}
	if customDecoded.Setup.Frame != customFix.Chart.Frame {
		t.Errorf("customFix decoded Frame = %+v, want %+v", customDecoded.Setup.Frame, customFix.Chart.Frame)
	}
	if customDecoded.Setup.LowerCup != customFix.Chart.CadLowerCup {
		t.Errorf("customFix decoded LowerCup = %+v, want %+v", customDecoded.Setup.LowerCup, customFix.Chart.CadLowerCup)
	}
	if customDecoded.Setup.Fork.Geometry() != customFix.Chart.CadFork {
		t.Errorf("customFix decoded Fork.Geometry() = %+v, want %+v", customDecoded.Setup.Fork.Geometry(), customFix.Chart.CadFork)
	}
	cadBaseline, err := assembly.PlaceCadBaselineFork(customFix.Chart, customFix.Wheels)
	if err != nil {
		t.Fatalf("PlaceCadBaselineFork(customFix): %v", err)
	}
	placedCustom, err := assembly.PlaceFrameset(customDecoded.Setup.Frame, customDecoded.Setup.LowerCup, customDecoded.Setup.Fork, customDecoded.Setup.Wheels, nil)
	if err != nil {
		t.Fatalf("PlaceFrameset(customDecoded): %v", err)
	}
	if gotDelta := cadBaseline.DeltaPitchDeg(placedCustom); math.Abs(float64(gotDelta)) > 1e-12 {
		t.Errorf("customFix CAD DeltaPitchDeg = %.6f°, want 0°", gotDelta)
	}

	switchToEC44, inputErr := decodeWorkbenchState(customFix, url.Values{
		"prev_fork": {"rigid_straight_483"},
		"fork":      {"sid_100"},
		"fork_a2c":  {"483.0"},
		"cup":       {"0.0"},
		"sag":       {"0.0"},
	})
	if inputErr != nil {
		t.Fatalf("decodeWorkbenchState(ZS44 -> EC44): %v", inputErr)
	}
	if switchToEC44.State.LowerCupMm != 12.0 || switchToEC44.Setup.LowerCup != ec44Cup || switchToEC44.State.ForkA2cMm != 506.0 || switchToEC44.State.SagPct != 20.0 {
		t.Errorf("ZS44 -> EC44 state = %+v (cup %+v), want EC44 12.0 mm / 506.0 mm / 20.0%%", switchToEC44.State, switchToEC44.Setup.LowerCup)
	}

	switchToZS44, inputErr := decodeWorkbenchState(customFix, url.Values{
		"prev_fork": {"sid_100"},
		"fork":      {"rigid_straight_483"},
		"fork_a2c":  {"506.0"},
		"cup":       {"12.0"},
		"sag":       {"20.0"},
	})
	if inputErr != nil {
		t.Fatalf("decodeWorkbenchState(EC44 -> ZS44): %v", inputErr)
	}
	if switchToZS44.State.LowerCupMm != 0.0 || switchToZS44.Setup.LowerCup != zs44Cup || switchToZS44.State.ForkA2cMm != 483.0 || switchToZS44.State.SagPct != 0.0 {
		t.Errorf("EC44 -> ZS44 state = %+v (cup %+v), want ZS44 0.0 mm / 483.0 mm / 0.0%%", switchToZS44.State, switchToZS44.Setup.LowerCup)
	}

	// Invalid fixture configuration checks.
	invalidCases := []struct {
		name   string
		mutate func(*bikeFixture)
	}{
		{"empty DefaultForkID", func(f *bikeFixture) { f.DefaultForkID = "" }},
		{"unknown DefaultForkID", func(f *bikeFixture) { f.DefaultForkID = "missing" }},
		{"DefaultForkID uses reserved cad ID", func(f *bikeFixture) { f.DefaultForkID = cadForkOptionID }},
		{"fork option uses reserved cad ID", func(f *bikeFixture) { f.Forks[0].ID = cadForkOptionID }},
		{"fork option with empty ID", func(f *bikeFixture) { f.Forks[0].ID = "" }},
		{"non-positive Chart.CadFork.A2cMm", func(f *bikeFixture) { f.Chart.CadFork.A2cMm = 0 }},
		{"NaN Chart.CadFork.RakeMm", func(f *bikeFixture) { f.Chart.CadFork.RakeMm = math.NaN() }},
		{"rigid fork option with non-zero DefaultSagPct", func(f *bikeFixture) { f.Forks[0].DefaultSagPct = 10.0 }},
		{"installable fork option uses bare_head_tube cup", func(f *bikeFixture) {
			f.Forks[0].DefaultLowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0.0}
		}},
		{"installable fork option missing lower cup standard", func(f *bikeFixture) {
			f.Forks[0].DefaultLowerCup = assembly.LowerCupSetup{}
		}},
		{"installable fork option EC44 zero stack", func(f *bikeFixture) {
			f.Forks[0].DefaultLowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 0.0}
		}},
		{"Chart.CadLowerCup missing standard", func(f *bikeFixture) {
			f.Chart.CadLowerCup = assembly.LowerCupSetup{}
		}},
		{"Chart.CadLowerCup bare_head_tube non-zero stack", func(f *bikeFixture) {
			f.Chart.CadLowerCup = assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 12.0}
		}},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			bad := otsoFenrirFixture()
			tc.mutate(&bad)
			if err := bad.validate(); err == nil {
				t.Fatalf("expected validate() error for %s", tc.name)
			}
		})
	}
}

func TestBikeWorkbenchCadBaselineForkLockAndLabels(t *testing.T) {
	hasReadOnly := func(doc *goquery.Document, name string) bool {
		_, exists := doc.Find(fmt.Sprintf(`input[name=%q]`, name)).Attr("readonly")
		return exists
	}

	// 1. Overrides on fork=cad are ignored (200, no error, chart values locked & readonly).
	for _, query := range []string{
		"/bike?fork=cad&cup=12",
		"/bike?fork=cad&fork_a2c=530",
		"/bike?fork=cad&sag=10",
		"/bike?fork=cad&fork_a2c=530&cup=12&sag=20&spacers=35",
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
					t.Errorf("expected %s to be readonly under fork=cad", field)
				}
			}
			if got := strings.TrimSpace(doc.Find(`[data-metric="delta-pitch"]`).Text()); got != "+0.00°" {
				t.Errorf("delta-pitch = %q, want +0.00°", got)
			}
			if hasReadOnly(doc, "spacers") {
				t.Errorf("cockpit field spacers should remain editable under fork=cad")
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
	suspDoc := loadBikeDoc(t, "/bike?fork=fox34_120")
	if hasReadOnly(suspDoc, "fork_a2c") || hasReadOnly(suspDoc, "cup") || hasReadOnly(suspDoc, "sag") {
		t.Errorf("suspension fork should keep fork_a2c, cup, and sag editable")
	}

	// 3. Single "CAD Baseline Fork" naming, cup standard label, and data-driven note.
	cadDoc := loadBikeDoc(t, "/bike?fork=cad")
	cadHTML, err := cadDoc.Html()
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	for _, banned := range []string{"Chart Inspection", "CAD Chart Reference", "chart only"} {
		if strings.Contains(cadHTML, banned) {
			t.Errorf("HTML unexpectedly contains legacy label %q", banned)
		}
	}
	if got, _ := cadDoc.Find(`select[name="fork"] optgroup`).Last().Attr("label"); got != "CAD Baseline Fork" {
		t.Errorf("last optgroup label = %q, want %q", got, "CAD Baseline Fork")
	}
	if got := strings.TrimSpace(cadDoc.Find(`label[for="ctrl-cup"]`).Text()); got != "No lower cup (bare head tube)" {
		t.Errorf("cad cup label = %q, want %q", got, "No lower cup (bare head tube)")
	}
	if got := strings.TrimSpace(rigidDoc.Find(`label[for="ctrl-cup"]`).Text()); got != "EC44 lower cup (mm, > 0)" {
		t.Errorf("physical cup label = %q, want %q", got, "EC44 lower cup (mm, > 0)")
	}
	note := strings.TrimSpace(cadDoc.Find(`[data-note="cad-baseline-fork"]`).Text())
	if !strings.Contains(note, "CAD Baseline Fork") || !strings.Contains(note, "bare head tube") || !strings.Contains(note, "EC44 (12 mm)") {
		t.Errorf("unexpected cad-baseline-fork note: %q", note)
	}
	if rigidDoc.Find(`[data-note="cad-baseline-fork"]`).Length() != 0 {
		t.Errorf("cad-baseline-fork note should not render for physical forks")
	}

	// 4. Switching both ways between fork=cad and physical forks updates cup, A2C, and sag.
	rec, fromCad := submitForm(t, cadDoc, map[string]string{"fork": "fox34_120"})
	if rec.Code != http.StatusOK {
		t.Fatalf("switch cad -> fox34_120 status = %d, want 200", rec.Code)
	}
	if got, _ := fromCad.Find(`input[name="cup"]`).Attr("value"); got != "12.0" {
		t.Errorf("switch cad -> fox34_120 cup = %q, want 12.0", got)
	}
	if got, _ := fromCad.Find(`input[name="sag"]`).Attr("value"); got != "20.0" {
		t.Errorf("switch cad -> fox34_120 sag = %q, want 20.0", got)
	}
	rec, backToCad := submitForm(t, fromCad, map[string]string{"fork": "cad"})
	if rec.Code != http.StatusOK {
		t.Fatalf("switch fox34_120 -> cad status = %d, want 200", rec.Code)
	}
	if got, _ := backToCad.Find(`input[name="cup"]`).Attr("value"); got != "0.0" {
		t.Errorf("switch fox34_120 -> cad cup = %q, want 0.0", got)
	}
	if got, _ := backToCad.Find(`input[name="fork_a2c"]`).Attr("value"); got != "495.0" {
		t.Errorf("switch fox34_120 -> cad fork_a2c = %q, want 495.0", got)
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
			errDoc := loadBikeDocStatus(t, fmt.Sprintf("/bike?sag=%s&spacers=35&stem_length=80", sagVal), http.StatusBadRequest)
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
			if got, _ := errDoc.Find(`input[name="spacers"]`).Attr("value"); got != "35" {
				t.Errorf("spacers value = %q, want 35", got)
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
			if got, _ := okDoc.Find(`input[name="spacers"]`).Attr("value"); got != "35.0" {
				t.Errorf("recovered spacers = %q, want 35.0", got)
			}
			if got, _ := okDoc.Find(`input[name="stem_length"]`).Attr("value"); got != "80.0" {
				t.Errorf("recovered stem_length = %q, want 80.0", got)
			}
		})
	}

	// Multi-error recovery: cup fails first so invalidField is "cup", yet invalid
	// sag on the rigid fork still stays editable so both can be fixed together.
	multiErr := loadBikeDocStatus(t, "/bike?cup=abc&sag=150&spacers=35", http.StatusBadRequest)
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
	if got, _ := multiOK.Find(`input[name="spacers"]`).Attr("value"); got != "35.0" {
		t.Errorf("multi-error recovered spacers = %q, want 35.0", got)
	}

	// Control cases: valid 0 sag stays readonly on error pages when another field fails.
	otherErr := loadBikeDocStatus(t, "/bike?spacers=-1", http.StatusBadRequest)
	if !hasReadOnly(otherErr, "sag") {
		t.Errorf("valid zero sag on rigid fork error page should stay readonly")
	}
	cadErr := loadBikeDocStatus(t, "/bike?fork=cad&spacers=-1", http.StatusBadRequest)
	for _, field := range []string{"fork_a2c", "cup", "sag"} {
		if !hasReadOnly(cadErr, field) {
			t.Errorf("cad baseline field %q on error page should stay readonly", field)
		}
	}
}

func TestWorkbenchStaticAssetServed(t *testing.T) {
	if len(workbenchJS) == 0 {
		t.Fatalf("embedded workbenchJS must not be empty")
	}

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081/static/workbench.js", nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/workbench.js status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Errorf("GET Content-Type = %q, want text/javascript", ct)
	}
	if nosniff := rec.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Errorf("GET X-Content-Type-Options = %q, want nosniff", nosniff)
	}
	if rec.Body.String() != string(workbenchJS) {
		t.Errorf("GET /static/workbench.js body did not match embedded asset")
	}

	headReq := httptest.NewRequest(http.MethodHead, "http://127.0.0.1:8081/static/workbench.js", nil)
	headRec := httptest.NewRecorder()
	NewHandler().ServeHTTP(headRec, headReq)
	if headRec.Code != http.StatusOK {
		t.Fatalf("HEAD /static/workbench.js status = %d, want 200", headRec.Code)
	}
	if ct := headRec.Header().Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Errorf("HEAD Content-Type = %q, want text/javascript", ct)
	}
	if headRec.Body.Len() != 0 {
		t.Errorf("HEAD /static/workbench.js body length = %d, want 0", headRec.Body.Len())
	}

	postReq := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8081/static/workbench.js", nil)
	postRec := httptest.NewRecorder()
	NewHandler().ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /static/workbench.js status = %d, want 405", postRec.Code)
	}
	if allow := postRec.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Errorf("POST Allow = %q, want %q", allow, "GET, HEAD")
	}

	missingReq := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081/static/missing.js", nil)
	missingRec := httptest.NewRecorder()
	NewHandler().ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("GET /static/missing.js status = %d, want 404", missingRec.Code)
	}

	testAssetReq := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081/static/workbench_test.js", nil)
	testAssetRec := httptest.NewRecorder()
	NewHandler().ServeHTTP(testAssetRec, testAssetReq)
	if testAssetRec.Code != http.StatusNotFound {
		t.Fatalf("GET /static/workbench_test.js status = %d, want 404", testAssetRec.Code)
	}

	for _, path := range []string{"/", "/bike"} {
		doc := loadBikeDoc(t, path)
		script := doc.Find(`script[src="/static/workbench.js"]`)
		if script.Length() != 1 {
			t.Errorf("%s: expected 1 script[src=/static/workbench.js], got %d", path, script.Length())
			continue
		}
		if _, hasDefer := script.Attr("defer"); !hasDefer {
			t.Errorf("%s: workbench.js script should have defer attribute", path)
		}
	}
}

func TestTraceInspectionMarkupAndLocalPoseFrames(t *testing.T) {
	fix := otsoFenrirFixture()

	defaultFork, ok := fix.resolveForkOption("")
	if !ok {
		t.Fatalf("missing default fork option")
	}
	stemFrameset, err := assembly.PlaceFrameset(
		fix.Chart.Frame,
		defaultFork.DefaultLowerCup,
		assembly.ForkSetup{Spec: defaultFork.Spec, SagPct: defaultFork.DefaultSagPct},
		fix.Wheels,
		nil,
	)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}
	stemTrace := pose.NewTrace()
	if _, err := assembly.PlaceStem(pose.NewPose(stemFrameset.HeadTubeTop.Origin(), stemFrameset.HeadTubeTop.HeadingDeg(), stemTrace), fix.DefaultStem); err != nil {
		t.Fatalf("PlaceStem: %v", err)
	}
	stemTrace.Freeze()

	defaultBikeTrace := pose.NewTrace()
	if _, err := assembly.PlaceBike(assembly.BikeSetup{
		Frame:     fix.Chart.Frame,
		LowerCup:  defaultFork.DefaultLowerCup,
		Fork:      assembly.ForkSetup{Spec: defaultFork.Spec, SagPct: defaultFork.DefaultSagPct},
		Wheels:    fix.Wheels,
		Seatpost:  fixtureSeatpost(t, fix, fix.DefaultSeatpostID, fix.DefaultPostExtMm),
		Saddle:    fix.DefaultSaddle,
		Stem:      fix.DefaultStem,
		Handlebar: defaultHandlebarSetup(t, fix),
	}, defaultBikeTrace); err != nil {
		t.Fatalf("PlaceBike default: %v", err)
	}

	fox34, ok := fix.resolveForkOption("fox34_120")
	if !ok {
		t.Fatalf("missing fox34_120 option")
	}
	customBar := defaultHandlebarSetup(t, fix)
	customBar.BarRollDeg = 5.0
	customSaddle := fix.DefaultSaddle
	customSaddle.RailPitchDeg = -2.0
	customBikeTrace := pose.NewTrace()
	if _, err := assembly.PlaceBike(assembly.BikeSetup{
		Frame:     fix.Chart.Frame,
		LowerCup:  assembly.LowerCupSetup{Standard: assembly.LowerCupEC44, StackMm: 12.0},
		Fork:      assembly.ForkSetup{Spec: fox34.Spec, SagPct: 20.0},
		Wheels:    fix.Wheels,
		Seatpost:  fixtureSeatpost(t, fix, fix.DefaultSeatpostID, fix.DefaultPostExtMm),
		Saddle:    customSaddle,
		Stem:      fix.DefaultStem,
		Handlebar: customBar,
	}, customBikeTrace); err != nil {
		t.Fatalf("PlaceBike custom: %v", err)
	}

	parseFloatAttr := func(t *testing.T, sel *goquery.Selection, attr string) float64 {
		t.Helper()
		raw, ok := sel.Attr(attr)
		if !ok {
			t.Fatalf("missing attribute %q", attr)
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			t.Fatalf("parse attribute %s=%q: %v", attr, raw, err)
		}
		return v
	}

	cases := []struct {
		name           string
		path           string
		steps          []pose.Step
		axisLenMm      float64
		expectWorldAxe bool
	}{
		{
			name:           "stem",
			path:           "/",
			steps:          stemTrace.Steps(),
			axisLenMm:      stemAxisLengthMm,
			expectWorldAxe: false,
		},
		{
			name:           "bike-default",
			path:           "/bike",
			steps:          defaultBikeTrace.Steps(),
			axisLenMm:      bikeAxisLengthMm,
			expectWorldAxe: true,
		},
		{
			name:           "bike-pitched-custom",
			path:           "/bike?fork=fox34_120&cup=12&sag=20&bar_roll=5&rail_pitch=-2",
			steps:          customBikeTrace.Steps(),
			axisLenMm:      bikeAxisLengthMm,
			expectWorldAxe: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := loadBikeDoc(t, tc.path)
			if doc.Find("main").Length() != 1 {
				t.Errorf("expected 1 <main> landmark, got %d", doc.Find("main").Length())
			}
			if doc.Find("[data-trace-workbench]").Length() != 1 {
				t.Errorf("expected 1 [data-trace-workbench], got %d", doc.Find("[data-trace-workbench]").Length())
			}
			if doc.Find("[data-step-inspector]").Length() != 1 {
				t.Errorf("expected 1 [data-step-inspector], got %d", doc.Find("[data-step-inspector]").Length())
			}
			if doc.Find("[data-step-clear]").Length() != 1 {
				t.Errorf("expected 1 [data-step-clear], got %d", doc.Find("[data-step-clear]").Length())
			}
			if tc.expectWorldAxe && doc.Find(`svg.diagram [data-element="world-axes"]`).Length() != 1 {
				t.Errorf("expected 1 svg.diagram [data-element=world-axes]")
			}
			if doc.Find("svg.diagram .step-frames [data-step-id]").Length() != 0 {
				t.Errorf("step-frames must not carry data-step-id")
			}

			ths := doc.Find("table.trace-table thead th")
			if ths.Length() == 0 {
				t.Fatalf("expected trace table header cells")
			}
			ths.Each(func(i int, th *goquery.Selection) {
				if scope, _ := th.Attr("scope"); scope != "col" {
					t.Errorf("thead th[%d] scope = %q, want col", i, scope)
				}
			})

			rows := doc.Find("table.trace-table tbody tr[data-step-id]")
			frames := doc.Find("svg.diagram .step-frames g[data-step-frame]")
			if rows.Length() != len(tc.steps) {
				t.Fatalf("trace rows = %d, want %d", rows.Length(), len(tc.steps))
			}
			if frames.Length() != len(tc.steps) {
				t.Fatalf("step-frames = %d, want %d", frames.Length(), len(tc.steps))
			}

			for i, step := range tc.steps {
				row := rows.Eq(i)
				idStr := strconv.Itoa(step.ID)
				if gotID, _ := row.Attr("data-step-id"); gotID != idStr {
					t.Errorf("row %d data-step-id = %q, want %q", i, gotID, idStr)
				}
				if gotParent, _ := row.Attr("data-parent-id"); gotParent != strconv.Itoa(step.ParentID) {
					t.Errorf("step %d data-parent-id = %q, want %d", step.ID, gotParent, step.ParentID)
				}
				if gotOp, _ := row.Attr("data-step-op"); gotOp != step.Op {
					t.Errorf("step %d data-step-op = %q, want %q", step.ID, gotOp, step.Op)
				}
				if gotLabel, _ := row.Attr("data-step-label"); gotLabel != step.Label {
					t.Errorf("step %d data-step-label = %q, want %q", step.ID, gotLabel, step.Label)
				}
				if summary, ok := row.Attr("data-step-summary"); !ok || strings.TrimSpace(summary) == "" {
					t.Errorf("step %d missing non-empty data-step-summary", step.ID)
				}

				btn := row.Find(fmt.Sprintf(`button[type="button"][data-step-select="%s"]`, idStr))
				if btn.Length() != 1 {
					t.Errorf("step %d: expected 1 step-select button, got %d", step.ID, btn.Length())
				} else {
					if pressed, _ := btn.Attr("aria-pressed"); pressed != "false" {
						t.Errorf("step %d button aria-pressed = %q, want false", step.ID, pressed)
					}
					ariaLabel, _ := btn.Attr("aria-label")
					visText := strings.TrimSpace(btn.Text())
					if visText == "" || !strings.Contains(ariaLabel, visText) {
						t.Errorf("step %d button aria-label %q must contain visible text %q", step.ID, ariaLabel, visText)
					}
				}

				frame := doc.Find(fmt.Sprintf(`svg.diagram .step-frames g[data-step-frame="%s"]`, idStr))
				if frame.Length() != 1 {
					t.Errorf("step %d: expected 1 step-frame group, got %d", step.ID, frame.Length())
					continue
				}
				lineX := frame.Find(`line[data-axis="x"]`)
				lineY := frame.Find(`line[data-axis="y"]`)
				if lineX.Length() != 1 || lineY.Length() != 1 {
					t.Errorf("step %d: expected 1 x and 1 y axis line, got %d and %d", step.ID, lineX.Length(), lineY.Length())
					continue
				}

				x1x := parseFloatAttr(t, lineX, "x1")
				y1x := parseFloatAttr(t, lineX, "y1")
				x2x := parseFloatAttr(t, lineX, "x2")
				y2x := parseFloatAttr(t, lineX, "y2")

				x1y := parseFloatAttr(t, lineY, "x1")
				y1y := parseFloatAttr(t, lineY, "y1")
				x2y := parseFloatAttr(t, lineY, "x2")
				y2y := parseFloatAttr(t, lineY, "y2")

				wantOX, wantOY := step.To.Origin.X, -step.To.Origin.Y
				if math.Abs(x1x-wantOX) > 0.02 || math.Abs(y1x-wantOY) > 0.02 {
					t.Errorf("step %d +X origin = (%.2f, %.2f), want (%.2f, %.2f)", step.ID, x1x, y1x, wantOX, wantOY)
				}
				if math.Abs(x1y-wantOX) > 0.02 || math.Abs(y1y-wantOY) > 0.02 {
					t.Errorf("step %d +Y origin = (%.2f, %.2f), want (%.2f, %.2f)", step.ID, x1y, y1y, wantOX, wantOY)
				}

				dxX, dyX := x2x-x1x, y2x-y1x
				dxY, dyY := x2y-x1y, y2y-y1y
				lenX := math.Hypot(dxX, dyX)
				lenY := math.Hypot(dxY, dyY)
				if math.Abs(lenX-tc.axisLenMm) > 0.02 {
					t.Errorf("step %d +X axis length = %.4f, want %.1f", step.ID, lenX, tc.axisLenMm)
				}
				if math.Abs(lenY-tc.axisLenMm) > 0.02 {
					t.Errorf("step %d +Y axis length = %.4f, want %.1f", step.ID, lenY, tc.axisLenMm)
				}

				rad := float64(step.To.HeadingDeg) * math.Pi / 180.0
				uxSVG, uySVG := dxX/lenX, dyX/lenX
				if math.Abs(uxSVG-math.Cos(rad)) > 0.01 || math.Abs(uySVG-(-math.Sin(rad))) > 0.01 {
					t.Errorf("step %d +X unit SVG dir = (%.4f, %.4f), want (%.4f, %.4f)",
						step.ID, uxSVG, uySVG, math.Cos(rad), -math.Sin(rad))
				}

				// Convert SVG deltas back to world (+X right, +Y up) to verify +Y is 90° CCW from +X.
				uxW, uyW := uxSVG, -uySVG
				vxW, vyW := dxY/lenY, -dyY/lenY
				dot := uxW*vxW + uyW*vyW
				cross := uxW*vyW - uyW*vxW
				if math.Abs(dot) > 0.01 || math.Abs(cross-1.0) > 0.01 {
					t.Errorf("step %d world axes (+X,+Y) dot=%.4f cross=%.4f, want dot=0 cross=1", step.ID, dot, cross)
				}
			}
		})
	}

	errDoc := loadBikeDocStatus(t, "/bike?spacers=-1", http.StatusBadRequest)
	if errDoc.Find("[data-trace-workbench]").Length() != 0 {
		t.Errorf("HTTP 400 error page should not render [data-trace-workbench]")
	}
}

func TestBikeWorkbenchFetchedUpdateHooks(t *testing.T) {
	fix := otsoFenrirFixture()

	stemDoc := loadBikeDoc(t, "/")
	stemMain := stemDoc.Find("main")
	if stemMain.Length() != 1 {
		t.Fatalf("/ expected 1 <main>, got %d", stemMain.Length())
	}
	if _, hasBike := stemMain.Attr("data-bike-workbench"); hasBike {
		t.Errorf("/ <main> must not carry data-bike-workbench")
	}
	if _, hasCanonical := stemMain.Attr("data-canonical-url"); hasCanonical {
		t.Errorf("/ <main> must not carry data-canonical-url")
	}

	okCases := []struct {
		name string
		path string
		want workbenchState
	}{
		{
			name: "default",
			path: "/bike",
			want: defaultWorkbenchState(fix),
		},
		{
			name: "fork-switch-omits-prev-fork",
			path: "/bike?prev_fork=lithic_carbon_mountain&fork=fox34_120&fork_a2c=495&sag=0",
			want: func() workbenchState {
				s := defaultWorkbenchState(fix)
				fox, _ := fix.resolveForkOption("fox34_120")
				s.ForkID = fox.ID
				s.ForkA2cMm = fox.Spec.StaticA2cMm
				s.LowerCupMm = fox.DefaultLowerCup.StackMm
				s.SagPct = fox.DefaultSagPct
				return s
			}(),
		},
		{
			name: "cad-baseline-fork-overrides-locked",
			path: "/bike?fork=cad&cup=12&fork_a2c=530&sag=20",
			want: defaultWorkbenchState(fix).withCadBaselineFork(fix),
		},
	}

	for _, tc := range okCases {
		t.Run("ok/"+tc.name, func(t *testing.T) {
			doc := loadBikeDoc(t, tc.path)
			mainEl := doc.Find("main[data-bike-workbench]")
			if mainEl.Length() != 1 {
				t.Fatalf("expected 1 main[data-bike-workbench], got %d", mainEl.Length())
			}
			canonicalRaw, hasCanonical := mainEl.Attr("data-canonical-url")
			if !hasCanonical || canonicalRaw == "" {
				t.Fatalf("expected non-empty data-canonical-url attribute")
			}
			u, err := url.Parse(canonicalRaw)
			if err != nil {
				t.Fatalf("parse data-canonical-url %q: %v", canonicalRaw, err)
			}
			if u.Path != "/bike" {
				t.Errorf("canonical URL path = %q, want /bike", u.Path)
			}
			if u.Query().Has("prev_fork") {
				t.Errorf("canonical URL %q must omit transient prev_fork", canonicalRaw)
			}
			decoded, inputErr := decodeWorkbenchState(fix, u.Query())
			if inputErr != nil {
				t.Fatalf("decode canonical URL %q: %v", canonicalRaw, inputErr)
			}
			if decoded.State != tc.want {
				t.Errorf("decoded canonical URL state = %+v, want %+v", decoded.State, tc.want)
			}

			form := mainEl.Find("form.controls")
			if form.Find(`button#ctrl-submit[type="submit"]`).Length() != 1 {
				t.Errorf("expected 1 button#ctrl-submit[type=submit] inside form.controls")
			}
			if href, ok := form.Find(`a#preset-defaults[data-preset="defaults"]`).Attr("href"); !ok || href != defaultWorkbenchState(fix).URL() {
				t.Errorf("a#preset-defaults href = %q (ok=%v), want %q", href, ok, defaultWorkbenchState(fix).URL())
			}
			if href, ok := form.Find(`a#preset-cad-baseline-fork[data-preset="cad-baseline-fork"]`).Attr("href"); !ok || !strings.HasPrefix(href, "/bike?") {
				t.Errorf("a#preset-cad-baseline-fork href = %q (ok=%v), want /bike?...", href, ok)
			}
		})
	}

	for _, errPath := range []string{"/bike?cup=0", "/bike?spacers=-1"} {
		t.Run("err/"+errPath, func(t *testing.T) {
			doc := loadBikeDocStatus(t, errPath, http.StatusBadRequest)
			mainEl := doc.Find("main[data-bike-workbench]")
			if mainEl.Length() != 1 {
				t.Fatalf("expected 1 main[data-bike-workbench] on 400 page, got %d", mainEl.Length())
			}
			if _, hasCanonical := mainEl.Attr("data-canonical-url"); hasCanonical {
				t.Errorf("HTTP 400 page must not expose data-canonical-url")
			}
			if mainEl.Find("[data-trace-workbench]").Length() != 0 {
				t.Errorf("HTTP 400 page must not render [data-trace-workbench]")
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

func decodeHref(t *testing.T, href string) workbenchState {
	t.Helper()
	u, err := url.Parse(href)
	if err != nil {
		t.Fatalf("parse %q: %v", href, err)
	}
	decoded, inputErr := decodeWorkbenchState(otsoFenrirFixture(), u.Query())
	if inputErr != nil {
		t.Fatalf("decode %q: %v", href, inputErr)
	}
	return decoded.State
}

func TestBikeWorkbenchDefaultViewKeepsWholeBikeDrawing(t *testing.T) {
	fix := otsoFenrirFixture()
	doc := loadBikeDoc(t, "/bike")

	canonical, _ := doc.Find("main[data-bike-workbench]").Attr("data-canonical-url")
	if got := decodeHref(t, canonical).View; got != diagramViewBike {
		t.Errorf("canonical URL %q view = %q, want bike", canonical, got)
	}
	if !strings.Contains(canonical, "view=bike") {
		t.Errorf("canonical URL %q must carry view=bike", canonical)
	}
	if got := hiddenViewValue(doc); got != "bike" {
		t.Errorf("hidden view input = %q, want bike", got)
	}

	// The whole-bike frame and drawing sizes are unchanged by the view work.
	tr := pose.NewTrace()
	resolved, inputErr := decodeWorkbenchState(fix, url.Values{})
	if inputErr != nil {
		t.Fatalf("decode defaults: %v", inputErr)
	}
	bike, err := assembly.PlaceBike(resolved.Setup, tr)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}
	cad, err := assembly.PlaceCadBaselineFork(fix.Chart, fix.Wheels)
	if err != nil {
		t.Fatalf("PlaceCadBaselineFork: %v", err)
	}
	view := buildBikeView(fix, resolved.State, bike, cad, tr.Steps())
	if view.DrawScale != 1 {
		t.Errorf("whole-bike DrawScale = %v, want exactly 1", view.DrawScale)
	}
	if got, _ := doc.Find("svg.diagram").Attr("viewBox"); got != "-911.86 -850.12 2096.16 1254.30" {
		t.Errorf("whole-bike viewBox = %q, want the pre-view framing -911.86 -850.12 2096.16 1254.30", got)
	}

	checks := []struct{ sel, attr, want string }{
		{`.landmarks [data-landmark="BB"] circle`, "r", "5"},
		{`.landmarks [data-landmark="BB"] circle`, "stroke-width", "1.5"},
		{`.landmarks [data-landmark="BB"] text`, "x", "0.00"},
		{`[data-element="world-axes"]`, "stroke-width", "1.8"},
		{`[data-element="world-axes"]`, "stroke-dasharray", "6 4"},
		{`[data-element="ground-line"]`, "stroke-dasharray", "10 6"},
		{`.wheels`, "stroke-width", "3"},
		{`[data-segment="chainstay"]`, "stroke-width", "6.0"},
		{`[data-segment="seat-tube"]`, "stroke-dasharray", "12 6"},
		{`.component-outlines`, "stroke-width", "1.5"},
		{`.component-outlines rect`, "rx", "2"},
		{`.trace-steps line`, "stroke-width", "2.0"},
		{`.trace-steps line`, "stroke-dasharray", "6 4"},
		{`.step-frames text.axis-label`, "font-size", "18.0"},
		{`.step-frames [data-axis="origin"]`, "r", "8.0"},
		{`svg.diagram`, "style", fmt.Sprintf("--active-step-width: 6; --ancestor-step-width: 3.5; --active-landmark-stroke-width: 2.5; --units-per-px: %.4f;", 2096.16/nominalDiagramWidthPx)},
		// Labels sit on their point and are offset in ems, so the offset
		// follows the on-screen (rem) label size set by CSS.
		{`.landmarks [data-landmark="BB"] text`, "dx", "0em"},
		{`.landmarks [data-landmark="BB"] text`, "dy", "1.44em"},
		{`.landmarks [data-landmark="SitBones"] text`, "dx", "-0.78em"},
		{`.landmarks [data-landmark="BB"]`, "data-label-priority", "1"},
		{`.landmarks [data-landmark="SpacerTop"]`, "data-label-priority", "3"},
	}
	for _, c := range checks {
		if got, _ := doc.Find(c.sel).First().Attr(c.attr); got != c.want {
			t.Errorf("%s [%s] = %q, want %q", c.sel, c.attr, got, c.want)
		}
	}
	bb := landmarkCentre(t, doc, "BB")
	labelY, _ := doc.Find(`.landmarks [data-landmark="BB"] text`).Attr("y")
	if y, err := strconv.ParseFloat(labelY, 64); err != nil || math.Abs(y-bb.Y) > 0.005 {
		t.Errorf("BB label y = %q, want the point's %.2f", labelY, bb.Y)
	}
	// Label font sizes come from CSS in rem, not from SVG attributes, so they
	// do not shrink with the diagram.
	if _, ok := doc.Find("svg.diagram[data-label-layout]").Attr("data-label-layout"); !ok {
		t.Errorf("svg.diagram must opt into label layout")
	}
	doc.Find(".landmarks text, [data-element=\"world-axes\"] text").Each(func(_ int, el *goquery.Selection) {
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
		{"seat", []string{"SeatpostClamp", "RailCenter", "BRP", "SitBones"}, "SeatTubeTop", "seat close-up"},
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
	path := "/bike?view=seat&fork=fox34_120&cup=12&sag=20&spacers=30&stem_length=70"
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
		if got, want := decodeHref(t, href), current.withView(view); got != want {
			t.Errorf("view link %s href state = %+v, want %+v", view, got, want)
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
	fix := otsoFenrirFixture()
	doc := loadBikeDoc(t, "/bike?view=stem&spacers=30")
	canonical, _ := doc.Find("main[data-bike-workbench]").Attr("data-canonical-url")
	state := decodeHref(t, canonical)
	if state.View != diagramViewStem || state.SpacersMm != 30 {
		t.Errorf("canonical %q: view=%q spacers=%v, want stem and 30", canonical, state.View, state.SpacersMm)
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
	if got, want := decodeHref(t, defaultsHref), defaultWorkbenchState(fix).withView(diagramViewStem); got != want {
		t.Errorf("Reset Defaults state = %+v, want %+v", got, want)
	}
	cadHref, _ := doc.Find("a#preset-cad-baseline-fork").Attr("href")
	if got, want := decodeHref(t, cadHref), state.withCadBaselineFork(fix); got != want {
		t.Errorf("CAD Baseline Fork state = %+v, want %+v", got, want)
	}
	for _, href := range []string{defaultsHref, cadHref} {
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
	for _, id := range []string{"preset-defaults", "preset-cad-baseline-fork"} {
		href, _ := doc.Find("a#" + id).Attr("href")
		if got := decodeHref(t, href).View; got != diagramViewBike {
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
	for _, id := range []string{"preset-defaults", "preset-cad-baseline-fork"} {
		href, _ := badDoc.Find("a#" + id).Attr("href")
		if got := decodeHref(t, href).View; got != diagramViewSeat {
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
		for _, query := range []string{"", "&spacers=0", "&spacers=40&stem_length=90&stem_angle=10"} {
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

// Every view draws the same build, so the trace rows, per-step SVG elements,
// step frames and landmark step IDs must stay in one-to-one correspondence
// whichever view is selected, and must not differ between views.
func TestBikeWorkbenchStepIDsMatchInEveryView(t *testing.T) {
	stepIDs := func(doc *goquery.Document, sel string) []string {
		var ids []string
		doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
			id, _ := s.Attr("data-step-id")
			ids = append(ids, id)
		})
		return ids
	}
	var wantRows []string
	for _, view := range []string{"bike", "seat", "stem"} {
		t.Run(view, func(t *testing.T) {
			doc := loadBikeDoc(t, "/bike?view="+view)
			rows := stepIDs(doc, "table.trace-table tbody tr[data-step-id]")
			if len(rows) == 0 {
				t.Fatalf("no trace rows")
			}
			if wantRows == nil {
				wantRows = rows
			} else if strings.Join(rows, ",") != strings.Join(wantRows, ",") {
				t.Errorf("trace row IDs differ from whole-bike view: %v vs %v", rows, wantRows)
			}
			for _, id := range rows {
				if n := doc.Find(fmt.Sprintf(`svg.diagram .trace-steps [data-step-id=%q]`, id)).Length(); n != 1 {
					t.Errorf("step %s: %d SVG trace elements, want 1", id, n)
				}
				if n := doc.Find(fmt.Sprintf(`svg.diagram .step-frames g[data-step-frame=%q]`, id)).Length(); n != 1 {
					t.Errorf("step %s: %d step frames, want 1", id, n)
				}
			}
			doc.Find("svg.diagram .landmarks [data-step-id]").Each(func(_ int, s *goquery.Selection) {
				id, _ := s.Attr("data-step-id")
				if doc.Find(fmt.Sprintf(`table.trace-table tbody tr[data-step-id=%q]`, id)).Length() != 1 {
					name, _ := s.Attr("data-landmark")
					t.Errorf("landmark %s step %q has no trace row", name, id)
				}
			})
		})
	}
}

func fittingTestTarget(t *testing.T, query url.Values) (x, y float64) {
	t.Helper()
	fix := otsoFenrirFixture()
	resolved, inputErr := decodeWorkbenchState(fix, query)
	if inputErr != nil {
		t.Fatalf("decode %v: %v", query, inputErr)
	}
	bike, err := assembly.PlaceBike(resolved.Setup, nil)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}
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
	doc := loadBikeDoc(t, "/bike")
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
	doc := loadBikeDoc(t, "/bike?brp_x=-219&brp_y=713.1&view=seat")
	metric := func(name string) string {
		return strings.TrimSpace(doc.Find(`[data-metric="` + name + `"]`).Text())
	}
	if got := metric("fit-saddle-outcome"); got != "limited by saddle rail offset" {
		t.Errorf("fit-saddle-outcome = %q", got)
	}
	if got := metric("fit-rail-offset"); got != "15.0 mm" {
		t.Errorf("fit-rail-offset = %q, want 15.0 mm", got)
	}
	if got := metric("fit-extension"); got != "260.1 mm" {
		t.Errorf("fit-extension = %q, want 260.1 mm", got)
	}
	q := hrefQuery(t, doc, "a#apply-fitting")
	if got := q.Get("rail_offset"); got != "15.0" {
		t.Errorf("apply rail_offset = %q, want 15.0", got)
	}
	if ext, err := strconv.ParseFloat(q.Get("post_ext"), 64); err != nil || math.Abs(ext-260.092) > 0.01 {
		t.Errorf("apply post_ext = %q (%v), want ≈260.092", q.Get("post_ext"), err)
	}
	if q.Get("brp_x") != "-219.0" || q.Get("brp_y") != "713.1" {
		t.Errorf("apply link lost the target: %v", q)
	}
	if got, _ := doc.Find("a#apply-fitting").Attr("data-preset"); got != "apply-fitting" {
		t.Errorf("apply-fitting data-preset = %q", got)
	}
	// The diagram and form still show the submitted build, not the fitted one.
	if got, _ := doc.Find("#ctrl-post-ext").Attr("value"); got != "263.5" {
		t.Errorf("post_ext echo = %q, want the submitted 263.5", got)
	}
}

func TestBikeWorkbenchApplyFittingReachesReachableTarget(t *testing.T) {
	x, y := fittingTestTarget(t, url.Values{"post_ext": {"200"}, "rail_offset": {"5"}})
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

// A half-entered target is not an error: it is quietly not applied and the
// typed value stays in its box.
func TestBikeWorkbenchHalfEnteredTargetIsQuietlyIgnored(t *testing.T) {
	for _, c := range []struct{ path, id, want string }{
		{"/bike?brp_y=710", "ctrl-brp-y", "710"},
		{"/bike?brp_x=-219", "ctrl-brp-x", "-219"},
		{"/bike?grip_y=726", "ctrl-grip-y", "726"},
		{"/bike?grip_x=481", "ctrl-grip-x", "481"},
	} {
		doc := loadBikeDoc(t, c.path)
		if n := doc.Find("[data-fitting]").Length(); n != 0 {
			t.Errorf("%s: fitting shown for a half target", c.path)
		}
		if v, _ := doc.Find("#" + c.id).Attr("value"); v != c.want {
			t.Errorf("%s: #%s value = %q, want %q", c.path, c.id, v, c.want)
		}
		if n := doc.Find("[aria-invalid]").Length(); n != 0 {
			t.Errorf("%s: %d invalid controls, want 0", c.path, n)
		}
	}
	// A complete grip pair still fits while a half BRP pair is ignored and kept.
	doc := loadBikeDoc(t, "/bike?brp_y=710&grip_x=481&grip_y=726")
	if doc.Find(`[data-metric="fit-cockpit-outcome"]`).Length() != 1 || doc.Find(`[data-metric="fit-saddle-outcome"]`).Length() != 0 {
		t.Error("want only the cockpit block")
	}
	if v, _ := doc.Find("#ctrl-brp-y").Attr("value"); v != "710" {
		t.Errorf("half BRP echo = %q, want 710", v)
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
	notes := doc.Find(".fit-note").Text()
	for _, want := range []string{"-219.0, 713.1", "481.0, 726.0"} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes %q missing example %q", notes, want)
		}
	}
}

func TestBikeWorkbenchBrpTargetPresetLinks(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp_x=-219&brp_y=713.1")
	if q := hrefQuery(t, doc, "a#preset-defaults"); q.Has("brp_x") || q.Has("brp_y") {
		t.Errorf("Reset Defaults keeps the target: %v", q)
	}
	q := hrefQuery(t, doc, "a#preset-cad-baseline-fork")
	if q.Get("brp_x") != "-219.0" || q.Get("brp_y") != "713.1" {
		t.Errorf("CAD Baseline Fork lost the target: %v", q)
	}
	if v, _ := doc.Find("main").Attr("data-canonical-url"); !strings.Contains(v, "brp_x=-219.0") {
		t.Errorf("canonical URL = %q, want it to keep the target", v)
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
	if v, _ := doc.Find("#ctrl-spacers").Attr("max"); v != "40" {
		t.Errorf("ctrl-spacers max = %q, want 40", v)
	}
	loadBikeDocStatus(t, "/bike?spacers=40", http.StatusOK)
	doc = loadBikeDocStatus(t, "/bike?spacers=45", http.StatusBadRequest)
	if v, _ := doc.Find("#ctrl-spacers").Attr("aria-invalid"); v != "true" {
		t.Errorf("ctrl-spacers aria-invalid = %q, want true", v)
	}
	if v, _ := doc.Find("#ctrl-spacers").Attr("max"); v != "40" {
		t.Errorf("error page ctrl-spacers max = %q, want 40", v)
	}
}

func gripTestTarget(t *testing.T, query url.Values) (x, y float64) {
	t.Helper()
	fix := otsoFenrirFixture()
	resolved, inputErr := decodeWorkbenchState(fix, query)
	if inputErr != nil {
		t.Fatalf("decode %v: %v", query, inputErr)
	}
	bike, err := assembly.PlaceBike(resolved.Setup, nil)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}
	grip := bike.Handlebar.HandGrip.Origin()
	return grip.X, grip.Y
}

func metricText(doc *goquery.Document, name string) string {
	return strings.TrimSpace(doc.Find(`[data-metric="` + name + `"]`).Text())
}

func TestBikeWorkbenchWithoutGripTargetHasNoCockpitFitting(t *testing.T) {
	doc := loadBikeDoc(t, "/bike")
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
	doc := loadBikeDoc(t, "/bike?grip_x=481&grip_y=726&view=stem")
	for name, want := range map[string]string{
		"fit-cockpit-outcome": "limited by stem spacers, stem length",
		"fit-spacers":         "0.0 mm",
		"fit-stem-length":     "130.0 mm",
		"fit-grip-miss":       "96.3 mm",
	} {
		if got := metricText(doc, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if n := doc.Find(`[data-metric="fit-saddle-outcome"]`).Length(); n != 0 {
		t.Errorf("saddle block present without a BRP target")
	}
	q := hrefQuery(t, doc, "a#apply-fitting")
	for k, want := range map[string]string{"spacers": "0.0", "stem_length": "130.0", "grip_x": "481.0", "grip_y": "726.0"} {
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
	doc := loadBikeDoc(t, "/bike?brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726")
	if doc.Find(`[data-metric="fit-saddle-outcome"]`).Length() != 1 || doc.Find(`[data-metric="fit-cockpit-outcome"]`).Length() != 1 {
		t.Fatal("want both fitting blocks")
	}
	if n := doc.Find("a#apply-fitting").Length(); n != 1 {
		t.Fatalf("apply links = %d, want 1", n)
	}
	q := hrefQuery(t, doc, "a#apply-fitting")
	if q.Get("rail_offset") != "15.0" || q.Get("spacers") != "0.0" || q.Get("stem_length") != "130.0" {
		t.Errorf("apply link values: %v", q)
	}
	if ext, err := strconv.ParseFloat(q.Get("post_ext"), 64); err != nil || math.Abs(ext-260.092) > 0.01 {
		t.Errorf("apply post_ext = %q, want ≈260.092", q.Get("post_ext"))
	}
	for _, k := range []string{"brp_x", "brp_y", "grip_x", "grip_y"} {
		if !q.Has(k) {
			t.Errorf("apply link lost %s: %v", k, q)
		}
	}
}

func TestBikeWorkbenchFitTargetsPresetLinks(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726")
	q := hrefQuery(t, doc, "a#preset-defaults")
	for _, k := range []string{"brp_x", "brp_y", "grip_x", "grip_y"} {
		if q.Has(k) {
			t.Errorf("Reset Defaults keeps %s", k)
		}
	}
	for _, sel := range []string{"a#preset-cad-baseline-fork", "a#view-seat"} {
		q = hrefQuery(t, doc, sel)
		if q.Get("grip_x") != "481.0" || q.Get("grip_y") != "726.0" || q.Get("brp_x") != "-219.0" {
			t.Errorf("%s lost a target: %v", sel, q)
		}
	}
	if v, _ := doc.Find("main").Attr("data-canonical-url"); !strings.Contains(v, "grip_x=481.0") {
		t.Errorf("canonical URL = %q, want it to keep the grip target", v)
	}
}

func stockCell(row *goquery.Selection, name string) string {
	return strings.TrimSpace(row.Find(`[data-stock-cell="` + name + `"]`).Text())
}

func TestBikeWorkbenchStockFittingRequiresGrip(t *testing.T) {
	for _, path := range []string{"/bike", "/bike?brp_x=-219&brp_y=713.1"} {
		if n := loadBikeDoc(t, path).Find("[data-stock-fitting]").Length(); n != 0 {
			t.Errorf("%s: got %d stock fitting blocks, want none", path, n)
		}
	}
}

func TestBikeWorkbenchRanksStockFitting(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?grip_x=481&grip_y=726&view=stem")
	rows := doc.Find("tr[data-stock-rank]")
	if rows.Length() != 5 {
		t.Fatalf("stock rows = %d, want 5", rows.Length())
	}
	if got := doc.Find("[data-stock-evaluated]").Text(); got != "450" {
		t.Errorf("evaluated = %q, want 450", got)
	}
	const preferences = "Free: HandGrip inside the ±10 × ±20 mm oval, stem length 70–80 mm, stem angle 0°, spacers 0–20 mm, bar sweep 16–20°. Penalties are illustrative except bar sweep (Q30)."
	if got := doc.Find("[data-stock-preferences]").Text(); got != preferences {
		t.Errorf("preference note = %q, want %q", got, preferences)
	}
	for name, want := range map[string]string{"stem": "130 mm, -17°", "spacers": "0 mm", "grip-miss": "13.6 mm", "oval-r": "1.36"} {
		if got := stockCell(rows.First(), name); got != want {
			t.Errorf("row 1 %s = %q, want %q", name, got, want)
		}
	}
	for i, want := range []string{"78.7", "70.0", "62.2"} {
		if got := stockCell(rows.Eq(i), "total"); got != want {
			t.Errorf("row %d total = %q, want %q", i+1, got, want)
		}
	}
	points := rows.First().Find("[data-stock-points]")
	headers := doc.Find("table.stock-table thead th")
	if points.Length() != 5 || headers.Length() != 13 {
		t.Fatalf("stock points cells = %d, headers = %d; want 5 and 13", points.Length(), headers.Length())
	}
	if got := headers.Last().Text(); got != "Handlebar" {
		t.Errorf("last stock header = %q, want Handlebar", got)
	}
	for i, want := range []struct{ band, points, header string }{
		{"HandGrip oval", "7.9", "Oval pts"},
		{"stem length", "10.0", "Length pts"},
		{"stem angle", "3.4", "Angle pts"},
		{"spacers", "0.0", "Spacer pts"},
		{"bar sweep", "0.0", "Sweep pts"},
	} {
		cell := points.Eq(i)
		if band, _ := cell.Attr("data-stock-points"); band != want.band || cell.Text() != want.points {
			t.Errorf("ledger cell %d = %q %q, want %q %q", i, band, cell.Text(), want.band, want.points)
		}
		if got := headers.Eq(7 + i).Text(); got != want.header {
			t.Errorf("ledger header %d = %q, want %q", i, got, want.header)
		}
	}
	// Apply and Total sit next to the rank so they stay visible when the table scrolls.
	if got := headers.Eq(2).Text(); got != "Total" {
		t.Errorf("header 3 = %q, want Total", got)
	}
	firstCells := rows.First().Children()
	if firstCells.Eq(1).Find("a[data-stock-apply]").Length() != 1 {
		t.Error("row 1 cell 2 has no apply link")
	}
	if got, _ := firstCells.Eq(2).Attr("data-stock-cell"); got != "total" {
		t.Errorf("row 1 cell 3 = %q, want total", got)
	}
	links := doc.Find("a[data-stock-apply]")
	if links.Length() != 5 {
		t.Fatalf("stock apply links = %d, want 5", links.Length())
	}
	links.Each(func(i int, link *goquery.Selection) {
		if got, _ := link.Attr("data-preset"); got != "apply-stock" {
			t.Errorf("link %d data-preset = %q, want apply-stock", i+1, got)
		}
	})
	q := hrefQuery(t, doc, `tr[data-stock-rank="1"] a[data-stock-apply]`)
	for k, want := range map[string]string{
		"stem_angle": "-17.0", "stem_length": "130.0", "spacers": "0.0", "grip_x": "481.0", "grip_y": "726.0", "view": "stem",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("stock apply %s = %q, want %q", k, got, want)
		}
	}
	if q.Has("brp_x") || q.Has("brp_y") {
		t.Errorf("stock apply link contains a BRP target: %v", q)
	}
	if got := metricText(doc, "fit-cockpit-outcome"); got != "limited by stem spacers, stem length" {
		t.Errorf("continuous outcome = %q, want limited by stem spacers, stem length", got)
	}
}

func assertSelectedStockBars(t *testing.T, doc *goquery.Document, want string) {
	t.Helper()
	selected := doc.Find(`select[name="stock_bars"] option[selected]`)
	if got, _ := selected.Attr("value"); selected.Length() != 1 || got != want {
		t.Errorf("selected stock_bars = %q (%d selected options), want %q", got, selected.Length(), want)
	}
}

func TestBikeWorkbenchStockBarsDefaultKeepsSubmittedBar(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?grip_x=481&grip_y=726")
	if n := doc.Find(`select[name="stock_bars"] option`).Length(); n != 2 {
		t.Errorf("stock_bars options = %d, want 2", n)
	}
	assertSelectedStockBars(t, doc, "selected")
	if got := doc.Find("[data-stock-evaluated]").Text(); got != "450" {
		t.Errorf("evaluated = %q, want 450", got)
	}
	rows := doc.Find("tr[data-stock-rank]")
	if rows.Length() != 5 {
		t.Fatalf("stock rows = %d, want 5", rows.Length())
	}
	rows.Each(func(i int, row *goquery.Selection) {
		if got := stockCell(row, "bar"); got != "Tumbleweed Persuader 20° (800 mm)" {
			t.Errorf("row %d bar = %q, want Tumbleweed Persuader 20° (800 mm)", i+1, got)
		}
		q := hrefQuery(t, doc, fmt.Sprintf(`tr[data-stock-rank="%d"] a[data-stock-apply]`, i+1))
		if q.Get("bar") != "persuader20_800" || q.Has("stock_bars") {
			t.Errorf("row %d apply query = %v, want submitted bar and omitted stock_bars", i+1, q)
		}
	})
	if q := stockBarsCanonicalQuery(t, doc); q.Has("stock_bars") {
		t.Errorf("canonical query contains stock_bars: %v", q)
	}
}

func stockBarsCanonicalQuery(t *testing.T, doc *goquery.Document) url.Values {
	t.Helper()
	raw, ok := doc.Find("main").Attr("data-canonical-url")
	if !ok || raw == "" {
		t.Fatal("missing canonical URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

const (
	stockBarAlloy15 = "SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm)"
	stockBarAlloy25 = "SQlab 311 2.0 Alloy 16°, 25 mm rise (740 mm)"
	stockBarFLX15   = "SQlab 311 FL-X Carbon 16°, 15 mm rise (740 mm)"
	stockBarFLX30   = "SQlab 311 FL-X Carbon 16°, 30 mm rise (740 mm)"
)

type stockBarRow struct {
	bar, stem, spacers, ovalR, total string
}

func assertStockBarRows(t *testing.T, doc *goquery.Document, want []stockBarRow) {
	t.Helper()
	rows := doc.Find("tr[data-stock-rank]")
	if rows.Length() != len(want) {
		t.Fatalf("stock rows = %d, want %d", rows.Length(), len(want))
	}
	for i, row := range want {
		for cell, value := range map[string]string{
			"bar": row.bar, "stem": row.stem, "spacers": row.spacers, "oval-r": row.ovalR, "total": row.total,
		} {
			if got := stockCell(rows.Eq(i), cell); got != value {
				t.Errorf("row %d %s = %q, want %q", i+1, cell, got, value)
			}
		}
	}
}

func TestBikeWorkbenchStockFittingAcrossAllBars(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?stock_bars=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726&view=stem")
	if got := doc.Find("[data-stock-evaluated]").Text(); got != "6750" {
		t.Errorf("evaluated = %q, want 6750", got)
	}
	if got := doc.Find("[data-stock-evaluated]").Parent().Text(); !strings.Contains(got, "stock handlebar, stem, and spacer combinations") {
		t.Errorf("stock count note = %q", got)
	}
	want := []stockBarRow{
		{stockBarAlloy15, "130 mm, -17°", "0 mm", "0.60", "86.6"},
		{stockBarFLX15, "130 mm, -17°", "0 mm", "0.61", "86.6"},
		{stockBarFLX30, "130 mm, -17°", "0 mm", "0.65", "86.6"},
		{stockBarAlloy25, "130 mm, -17°", "0 mm", "0.71", "86.6"},
		{stockBarFLX15, "130 mm, -17°", "5 mm", "0.77", "86.6"},
	}
	assertStockBarRows(t, doc, want)

	const rowTwoApply = `tr[data-stock-rank="2"] a[data-stock-apply]`
	q := hrefQuery(t, doc, rowTwoApply)
	for name, value := range map[string]string{
		"bar": "sqlab311_flx_16_15", "stock_bars": "all", "stem_length": "130.0", "stem_angle": "-17.0",
		"spacers": "0.0", "bar_roll": "0.0", "rail_offset": "15.0", "brp_x": "-219.0", "grip_x": "481.0",
	} {
		if got := q.Get(name); got != value {
			t.Errorf("row 2 apply %s = %q, want %q", name, got, value)
		}
	}
	if q := hrefQuery(t, doc, "#apply-fitting"); q.Get("bar") != "persuader20_800" || q.Get("stock_bars") != "all" {
		t.Errorf("continuous apply query = %v, want submitted bar and all stock bars", q)
	}
	selected := loadBikeDoc(t, "/bike?brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726&view=stem")
	for _, metric := range []string{"fit-spacers", "fit-stem-length", "fit-cockpit-outcome"} {
		if got, want := metricText(doc, metric), metricText(selected, metric); got != want {
			t.Errorf("continuous %s = %q, want %q from selected mode", metric, got, want)
		}
	}

	t.Run("apply row two", func(t *testing.T) {
		href, _ := doc.Find(rowTwoApply).Attr("href")
		applied := loadBikeDoc(t, href)
		assertSelectedHandlebar(t, applied, "sqlab311_flx_16_15")
		assertSelectedStockBars(t, applied, "all")
		assertStockBarRows(t, applied, want)
	})
	t.Run("different submitted bar", func(t *testing.T) {
		other := loadBikeDoc(t, "/bike?stock_bars=all&bar=sqlab3ox_12_15&grip_x=481&grip_y=726")
		assertStockBarRows(t, other, want)
	})
}

func TestBikeWorkbenchStockFittingKeepsSubmittedBarRoll(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?stock_bars=all&bar_roll=5&grip_x=481&grip_y=726")
	assertStockBarRows(t, doc, []stockBarRow{
		{stockBarFLX15, "130 mm, -17°", "0 mm", "0.37", "86.6"},
		{stockBarAlloy15, "130 mm, -17°", "0 mm", "0.46", "86.6"},
		{stockBarFLX15, "130 mm, -17°", "5 mm", "0.69", "86.6"},
		{stockBarFLX30, "130 mm, -17°", "0 mm", "0.73", "86.6"},
		{stockBarAlloy25, "130 mm, -17°", "0 mm", "0.75", "86.6"},
	})
	for i := range 5 {
		q := hrefQuery(t, doc, fmt.Sprintf(`tr[data-stock-rank="%d"] a[data-stock-apply]`, i+1))
		if got := q.Get("bar_roll"); got != "5.0" {
			t.Errorf("row %d apply bar_roll = %q, want 5.0", i+1, got)
		}
	}
}

func TestBikeWorkbenchStockSweepColumn(t *testing.T) {
	for _, tc := range []struct {
		name, query, points string
	}{
		{"all bars", "stock_bars=all", "0.0"},
		{"selected bar", "bar=sqlab311_flx_12_15", "8.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := loadBikeDoc(t, "/bike?"+tc.query+"&grip_x=481&grip_y=726")
			rows := doc.Find("tr[data-stock-rank]")
			if rows.Length() != 5 {
				t.Fatalf("stock rows = %d, want 5", rows.Length())
			}
			rows.Each(func(i int, row *goquery.Selection) {
				cells := row.Find(`[data-stock-points="bar sweep"]`)
				if cells.Length() != 1 || cells.Text() != tc.points {
					t.Errorf("row %d sweep points = %q (%d cells), want %s in one cell", i+1, cells.Text(), cells.Length(), tc.points)
				}
			})
		})
	}
}

func TestBikeWorkbenchKeepsStockBarsWithoutTarget(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?stock_bars=all&view=seat")
	if n := doc.Find("[data-stock-fitting]").Length(); n != 0 {
		t.Errorf("stock fitting blocks = %d, want none", n)
	}
	assertSelectedStockBars(t, doc, "all")
	if got := stockBarsCanonicalQuery(t, doc).Get("stock_bars"); got != "all" {
		t.Errorf("canonical stock_bars = %q, want all", got)
	}
	for _, sel := range []string{"#view-bike", "#view-seat", "#view-stem", "#preset-cad-baseline-fork"} {
		if got := hrefQuery(t, doc, sel).Get("stock_bars"); got != "all" {
			t.Errorf("%s stock_bars = %q, want all", sel, got)
		}
	}
	if q := hrefQuery(t, doc, "#preset-defaults"); q.Has("stock_bars") {
		t.Errorf("Reset Defaults query contains stock_bars: %v", q)
	}
}

func TestBikeWorkbenchSubmitsStockBarsSelect(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?grip_x=481&grip_y=726")
	rec, next := submitForm(t, doc, map[string]string{"stock_bars": "all"})
	if rec.Code != http.StatusOK {
		t.Fatalf("stock_bars submission status = %d, want 200", rec.Code)
	}
	if got := next.Find("[data-stock-evaluated]").Text(); got != "6750" {
		t.Errorf("evaluated = %q, want 6750", got)
	}
	if got := stockCell(next.Find(`tr[data-stock-rank="1"]`), "bar"); got != stockBarAlloy15 {
		t.Errorf("row 1 bar = %q, want %q", got, stockBarAlloy15)
	}
}

func TestBikeWorkbenchStockBarsErrorPage(t *testing.T) {
	doc := loadBikeDocStatus(t, "/bike?stock_bars=some", http.StatusBadRequest)
	if got, _ := doc.Find(`select[name="stock_bars"]`).Attr("aria-invalid"); got != "true" {
		t.Errorf("stock_bars aria-invalid = %q, want true", got)
	}
	if got := strings.TrimSpace(doc.Find("#form-error").Text()); !strings.Contains(got, `unknown stock fitting handlebar option "some"`) {
		t.Errorf("stock_bars error message = %q", got)
	}

	doc = loadBikeDocStatus(t, "/bike?stock_bars=all&spacers=-1", http.StatusBadRequest)
	assertSelectedStockBars(t, doc, "all")
	if got := hrefQuery(t, doc, "#preset-cad-baseline-fork").Get("stock_bars"); got != "all" {
		t.Errorf("error-page CAD preset stock_bars = %q, want all", got)
	}
}

func TestBikeWorkbenchAppliesFreeStockBuild(t *testing.T) {
	x, y := gripTestTarget(t, url.Values{"stem_angle": {"0"}, "stem_length": {"70"}, "spacers": {"10"}})
	doc := loadBikeDoc(t, "/bike?grip_x="+formatFormFloat(x)+"&grip_y="+formatFormFloat(y))
	const rankOne = `tr[data-stock-rank="1"]`
	row := doc.Find(rankOne)
	for name, want := range map[string]string{"stem": "70 mm, +0°", "spacers": "10 mm", "oval-r": "0.00", "total": "100.0"} {
		if got := stockCell(row, name); got != want {
			t.Errorf("row 1 %s = %q, want %q", name, got, want)
		}
	}
	applied, ok := row.Find("a[data-stock-apply]").Attr("href")
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
	if got := stockCell(doc.Find(rankOne), "stem"); got != "70 mm, +0°" {
		t.Errorf("row 1 stem after applying = %q, want 70 mm, +0°", got)
	}
}

func TestBikeWorkbenchStockFitKeepsSaddleFitAndTargets(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726")
	q := hrefQuery(t, doc, `tr[data-stock-rank="1"] a[data-stock-apply]`)
	for k, want := range map[string]string{
		"rail_offset": "15.0", "stem_angle": "-17.0", "stem_length": "130.0", "spacers": "0.0",
		"brp_x": "-219.0", "brp_y": "713.1", "grip_x": "481.0", "grip_y": "726.0",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("stock apply %s = %q, want %q", k, got, want)
		}
	}
	if ext, err := strconv.ParseFloat(q.Get("post_ext"), 64); err != nil || math.Abs(ext-260.092) > 0.01 {
		t.Errorf("stock apply post_ext = %q, want ≈260.092", q.Get("post_ext"))
	}
	if n := doc.Find("#apply-fitting").Length(); n != 1 {
		t.Errorf("continuous apply links = %d, want 1", n)
	}
}

func TestBikeFixtureValidatesStockAndPreferences(t *testing.T) {
	fix := otsoFenrirFixture()
	fix.CockpitStock.SpacerStepMm = 0
	if err := fix.validate(); err == nil || !strings.Contains(err.Error(), "cockpit stock invalid") {
		t.Errorf("invalid stock: got %v, want cockpit stock error", err)
	}
	fix = otsoFenrirFixture()
	fix.CockpitPreferences.GripOval.SemiMinorMm = 0
	if err := fix.validate(); err == nil || !strings.Contains(err.Error(), "cockpit preferences invalid") {
		t.Errorf("invalid preferences: got %v, want cockpit preferences error", err)
	}
}

func TestFixtureHandlebarReferenceHandGrips(t *testing.T) {
	fix := otsoFenrirFixture()
	want := []struct {
		id            string
		x, y, lateral float64
	}{
		{"persuader20_800", 389.218, 755.120, 330},
		{"persuader20_840", 381.938, 755.120, 350},
		{"sqlab3ox_16_15", 388.075, 742.602, 320},
		{"sqlab3ox_16_30", 391.946, 756.658, 320},
		{"sqlab3ox_16_45", 395.817, 770.714, 320},
		{"sqlab3ox_12_15", 406.734, 742.602, 320},
		{"sqlab3ox_12_30", 409.603, 756.658, 320},
		{"sqlab3ox_12_45", 412.473, 770.714, 320},
		{"sqlab311_20_16_15", 393.810, 745.269, 300},
		{"sqlab311_20_16_25", 396.391, 754.481, 300},
		{"sqlab311_20_16_50", 402.843, 777.513, 300},
		{"sqlab311_flx_16_15", 393.810, 741.203, 300},
		{"sqlab311_flx_16_30", 397.681, 755.259, 300},
		{"sqlab311_flx_12_15", 410.985, 741.203, 300},
		{"sqlab311_flx_12_30", 413.854, 755.259, 300},
	}
	if len(fix.Handlebars) != len(want) {
		t.Fatalf("fixture has %d handlebars, want %d", len(fix.Handlebars), len(want))
	}
	for i, ref := range want {
		t.Run(ref.id, func(t *testing.T) {
			opt := fix.Handlebars[i]
			if opt.ID != ref.id {
				t.Fatalf("bar %d = %q, want %q", i, opt.ID, ref.id)
			}
			resolved, inputErr := decodeWorkbenchState(fix, url.Values{"bar": {opt.ID}})
			if inputErr != nil {
				t.Fatalf("decodeWorkbenchState: %v", inputErr)
			}
			placed, err := assembly.PlaceBike(resolved.Setup, nil)
			if err != nil {
				t.Fatalf("PlaceBike: %v", err)
			}
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
	canonical, _ := doc.Find("main").Attr("data-canonical-url")
	u, err := url.Parse(canonical)
	if err != nil {
		t.Fatalf("parse canonical URL: %v", err)
	}
	if got := u.Query().Get("bar"); got != "persuader20_800" {
		t.Errorf("canonical bar = %q, want persuader20_800", got)
	}
	if got := metricText(doc, "brp-grip-dist"); got != "556.6 mm" {
		t.Errorf("BRP to HandGrip = %q, want 556.6 mm", got)
	}
}

func TestBikeWorkbenchChosenHandlebarChangesBuild(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?bar=sqlab3ox_12_15&view=stem")
	assertSelectedHandlebar(t, doc, "sqlab3ox_12_15")
	for metric, want := range map[string]string{
		"hand-lateral": "320.0 mm", "brp-grip-dist": "573.6 mm", "sitbones-grip-rise": "+5.8 mm",
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
		`a[data-preset="cad-baseline-fork"]`,
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
	cadQuery := hrefQuery(t, errorDoc, `a[data-preset="cad-baseline-fork"]`)
	if got := cadQuery.Get("bar"); got != bar {
		t.Errorf("CAD Baseline Fork bar on error page = %q, want %q", got, bar)
	}
	badBarDoc := loadBikeDocStatus(t, "/bike?bar=unknown", http.StatusBadRequest)
	if got := badBarDoc.Find(`select[name="bar"] option[selected]`).Length(); got != 0 {
		t.Errorf("unknown bar yielded %d selected options, want 0", got)
	}
}

func TestBikeWorkbenchFitsAroundChosenHandlebar(t *testing.T) {
	const bar = "sqlab3ox_12_15"
	doc := loadBikeDoc(t, "/bike?bar="+bar+"&grip_x=481&grip_y=726&view=stem")
	for metric, want := range map[string]string{
		"fit-cockpit-outcome": "limited by stem spacers", "fit-spacers": "0.0 mm", "fit-stem-length": "124.5 mm",
	} {
		if got := metricText(doc, metric); got != want {
			t.Errorf("%s = %q, want %q", metric, got, want)
		}
	}
	if got := hrefQuery(t, doc, "#apply-fitting").Get("bar"); got != bar {
		t.Errorf("Apply fitted build bar = %q, want %q", got, bar)
	}
	const firstRow = `tr[data-stock-rank="1"]`
	for cell, want := range map[string]string{
		"stem": "120 mm, -17°", "spacers": "0 mm", "oval-r": "0.45", "total": "80.6",
	} {
		if got := stockCell(doc.Find(firstRow), cell); got != want {
			t.Errorf("row 1 %s = %q, want %q", cell, got, want)
		}
	}
	const firstApply = firstRow + " a[data-stock-apply]"
	q := hrefQuery(t, doc, firstApply)
	for field, want := range map[string]string{
		"bar": bar, "stem_length": "120.0", "stem_angle": "-17.0", "spacers": "0.0",
	} {
		if got := q.Get(field); got != want {
			t.Errorf("row 1 Apply %s = %q, want %q", field, got, want)
		}
	}
	doc.Find("a[data-stock-apply]").Each(func(i int, link *goquery.Selection) {
		href, _ := link.Attr("href")
		if u, err := url.Parse(href); err != nil || u.Query().Get("bar") != bar {
			t.Errorf("row %d Apply URL = %q, want bar=%s", i+1, href, bar)
		}
	})
	href, _ := doc.Find(firstApply).Attr("href")
	applied := loadBikeDoc(t, href)
	assertSelectedHandlebar(t, applied, bar)
	if got := stockCell(applied.Find(firstRow), "stem"); got != "120 mm, -17°" {
		t.Errorf("applied row 1 stem = %q, want 120 mm, -17°", got)
	}
}

func TestBikeFixtureValidatesHandlebars(t *testing.T) {
	cases := []struct {
		name   string
		change func(*bikeFixture)
		want   string
	}{
		{"empty default", func(f *bikeFixture) { f.DefaultHandlebarID = "" }, "missing DefaultHandlebarID"},
		{"unknown default", func(f *bikeFixture) { f.DefaultHandlebarID = "missing" }, `DefaultHandlebarID "missing" not found`},
		{"empty ID", func(f *bikeFixture) { f.Handlebars[1].ID = "" }, "handlebar option with empty ID"},
		{"duplicate ID", func(f *bikeFixture) { f.Handlebars[1].ID = f.Handlebars[0].ID }, `duplicate handlebar ID "persuader20_800"`},
		{"fixed roll", func(f *bikeFixture) { f.Handlebars[1].Setup.BarRollDeg = 5 }, `handlebar "persuader20_840" must not set BarRollDeg`},
		{"invalid placement", func(f *bikeFixture) { f.Handlebars[1].Setup.GripInsetMm = 0 }, `handlebar "persuader20_840" invalid: handlebar grip inset`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fix := otsoFenrirFixture()
			tc.change(&fix)
			if err := fix.validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("validate() = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestBikeWorkbenchSeatpostSelect(t *testing.T) {
	fix := otsoFenrirFixture()
	doc := loadBikeDoc(t, "/bike")

	options := doc.Find(`select[name="post"] option`)
	if options.Length() != len(fix.Seatposts) {
		t.Fatalf("seatpost option count = %d, want %d", options.Length(), len(fix.Seatposts))
	}
	for i, want := range fix.Seatposts {
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
	if note := doc.Find(`[data-note="post-setback"]`).Text(); !strings.Contains(note, "extension 110–300 mm · clamp 30 mm") {
		t.Errorf("post-setback note = %q, want extension 110–300 mm · clamp 30 mm", note)
	}
	canonHref, _ := doc.Find("main").Attr("data-canonical-url")
	if canonURL, err := url.Parse(canonHref); err != nil || canonURL.Query().Get("post") != "setback_0" || canonURL.Query().Has("post_setback") {
		t.Errorf("canonical URL = %q, want post=setback_0 and no post_setback", canonHref)
	}

	redshiftDoc := loadBikeDoc(t, "/bike?post=redshift_shockstop_pro")
	if got := strings.TrimSpace(redshiftDoc.Find(`[data-metric="post-setback"]`).Text()); got != "12.0 mm" {
		t.Errorf("redshift post-setback = %q, want 12.0 mm", got)
	}
	defaultFork, _ := fix.resolveForkOption("")
	bike, err := assembly.PlaceBike(assembly.BikeSetup{
		Frame:     fix.Chart.Frame,
		LowerCup:  defaultFork.DefaultLowerCup,
		Fork:      assembly.ForkSetup{Spec: defaultFork.Spec, SagPct: defaultFork.DefaultSagPct},
		Wheels:    fix.Wheels,
		Seatpost:  fixtureSeatpost(t, fix, "redshift_shockstop_pro", 263.5),
		Saddle:    fix.DefaultSaddle,
		Stem:      fix.DefaultStem,
		Handlebar: defaultHandlebarSetup(t, fix),
	}, nil)
	if err != nil {
		t.Fatalf("PlaceBike redshift: %v", err)
	}
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
	canon, _ := doc.Find("main").Attr("data-canonical-url")
	assertHasPost("canonical", canon)
	doc.Find(`a[data-diagram-view]`).Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		assertHasPost(fmt.Sprintf("diagram-view %d", i), href)
	})
	cadHref, _ := doc.Find(`a[data-preset="cad-baseline-fork"]`).Attr("href")
	assertHasPost("cad-baseline-fork", cadHref)
	applyHref, _ := doc.Find(`#apply-fitting`).Attr("href")
	assertHasPost("apply-fitting", applyHref)
	stockLinks := doc.Find(`[data-stock-apply]`)
	if stockLinks.Length() == 0 {
		t.Fatal("expected [data-stock-apply] links")
	}
	stockLinks.Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		assertHasPost(fmt.Sprintf("stock-apply %d", i), href)
	})
	defaultsHref, _ := doc.Find(`a[data-preset="defaults"]`).Attr("href")
	if u, err := url.Parse(defaultsHref); err != nil || u.Query().Get("post") != "setback_0" {
		t.Errorf("defaults href = %q, want post=setback_0", defaultsHref)
	}
}

func TestBikeWorkbenchIgnoresRetiredSetbackKey(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?post_setback=20")
	if doc.Find("#form-error").Length() != 0 {
		t.Errorf("expected no #form-error, got %d", doc.Find("#form-error").Length())
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="post-setback"]`).Text()); got != "0.0 mm" {
		t.Errorf("post-setback = %q, want 0.0 mm", got)
	}
	canon, _ := doc.Find("main").Attr("data-canonical-url")
	if u, err := url.Parse(canon); err != nil || u.Query().Has("post_setback") {
		t.Errorf("canonical URL = %q, want no post_setback", canon)
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
	doc := loadBikeDoc(t, "/bike?brp_x=-219&brp_y=713.1")
	if got := metricText(doc, "fit-saddle-outcome"); got != "limited by saddle rail offset" {
		t.Errorf("fit-saddle-outcome = %q, want limited by saddle rail offset", got)
	}
	for k, want := range map[string]string{
		"fit-setback-centered":         "54.1 mm",
		"fit-setback-centered-ext":     "264.0 mm",
		"fit-setback-centered-outcome": "reached",
		"fit-setback-reach-rail":       "+15.0 mm",
		"fit-setback-reach":            "39.6 mm",
		"fit-setback-reach-ext":        "260.1 mm",
		"fit-setback-reach-outcome":    "reached",
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
	doc25 := loadBikeDoc(t, "/bike?post=setback_25&brp_x=-219&brp_y=713.1")
	for _, k := range []string{
		"fit-setback-centered", "fit-setback-centered-ext", "fit-setback-centered-outcome",
		"fit-setback-reach-rail", "fit-setback-reach", "fit-setback-reach-ext", "fit-setback-reach-outcome",
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
	if got := metricText(doc, "fit-rail-offset"); got != "0.1 mm" {
		t.Errorf("fit-rail-offset = %q, want 0.1 mm", got)
	}
	if doc.Find(`[data-metric="fit-setback-reach"]`).Length() != 0 {
		t.Errorf("expected no fit-setback-reach when saddle fit is reached")
	}
	for k, want := range map[string]string{
		"fit-setback-centered":         "25.1 mm",
		"fit-setback-centered-ext":     "256.1 mm",
		"fit-setback-centered-outcome": "reached",
	} {
		if got := metricText(doc, k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestBikeWorkbenchSetbackNotesForwardTarget(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?post=redshift_shockstop_pro&brp_x=-150&brp_y=713.1")
	if got := metricText(doc, "fit-saddle-outcome"); got != "limited by saddle rail offset" {
		t.Errorf("fit-saddle-outcome = %q, want limited by saddle rail offset", got)
	}
	if got := metricText(doc, "fit-rail-offset"); got != "-15.0 mm" {
		t.Errorf("fit-rail-offset = %q, want -15.0 mm", got)
	}
	for k, want := range map[string]string{
		"fit-setback-reach-rail":       "-15.0 mm",
		"fit-setback-reach":            "2.0 mm",
		"fit-setback-reach-ext":        "249.8 mm",
		"fit-setback-reach-outcome":    "reached",
		"fit-setback-centered":         "0.0 mm",
		"fit-setback-centered-ext":     "245.9 mm",
		"fit-setback-centered-outcome": "limited by seatpost setback",
	} {
		if got := metricText(doc, k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	centeredSpan := doc.Find(`[data-metric="fit-setback-centered"]`).Parent().Text()
	if !strings.Contains(centeredSpan, "12.5 mm short") {
		t.Errorf("centered span = %q, want 12.5 mm short", centeredSpan)
	}
}

func TestBikeWorkbenchSetbackNotesExtensionLimit(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?brp_x=-219&brp_y=900")
	if got := metricText(doc, "fit-saddle-outcome"); got != "limited by seatpost extension, saddle rail offset" {
		t.Errorf("fit-saddle-outcome = %q, want limited by seatpost extension, saddle rail offset", got)
	}
	for k, want := range map[string]string{
		"fit-setback-centered":         "4.8 mm",
		"fit-setback-centered-ext":     "300.0 mm",
		"fit-setback-centered-outcome": "limited by seatpost extension",
		"fit-setback-reach-rail":       "+15.0 mm",
		"fit-setback-reach":            "0.0 mm",
		"fit-setback-reach-ext":        "300.0 mm",
		"fit-setback-reach-outcome":    "limited by seatpost extension, seatpost setback",
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
	if !strings.Contains(reachSpan, "140.7 mm short") {
		t.Errorf("reach span = %q, want 140.7 mm short", reachSpan)
	}
}

func TestBikeWorkbenchSetbackNotesAbsentWithoutBrpTarget(t *testing.T) {
	doc := loadBikeDoc(t, "/bike?grip_x=481&grip_y=726")
	if n := doc.Find(`[data-metric^="fit-setback"]`).Length(); n != 0 {
		t.Errorf("expected no [data-metric^=fit-setback] without BRP target, got %d", n)
	}
}
