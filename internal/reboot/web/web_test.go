package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

	// Verify BarClamp, leveled HTA, and traced steps match fixture chassis.HeadTubeTop + PlaceStem output.
	fix := otsoFenrirFixture()
	defaultFork, ok := findForkOption(fix.Forks, fix.DefaultForkID, fix.DefaultForkID)
	if !ok {
		t.Fatalf("missing default fork option %q", fix.DefaultForkID)
	}
	forkSetup, err := defaultFork.Spec.Setup(defaultFork.DefaultSagPct, fix.DefaultLowerCupMm)
	if err != nil {
		t.Fatalf("defaultFork.Spec.Setup: %v", err)
	}
	chassis, err := assembly.PlaceChassis(fix.Chart.Frame, forkSetup, fix.Wheels, nil)
	if err != nil {
		t.Fatalf("PlaceChassis: %v", err)
	}
	tr := pose.NewTrace()
	steererUp := pose.NewPose(chassis.HeadTubeTop.Origin(), chassis.HeadTubeTop.HeadingDeg(), tr)
	placed, err := assembly.PlaceStem(steererUp, fix.DefaultStem)
	tr.Freeze()
	if err != nil {
		t.Fatalf("PlaceStem: %v", err)
	}

	wantHTA := fmt.Sprintf("%.2f°", chassis.LeveledHeadTubeAngleDeg())
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

func TestChassisWorkbenchRendersAndRespondsToForkQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8081/chassis?fork=fox34_120&cup=12&sag=20", nil)
	rec := httptest.NewRecorder()
	NewHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /chassis status = %d, want 200", rec.Code)
	}

	doc, err := goquery.NewDocumentFromReader(rec.Body)
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	if got := strings.TrimSpace(doc.Find(`[data-metric="effective-a2c"]`).Text()); got != "506.0 mm" {
		t.Errorf("effective-a2c = %q, want 506.0 mm", got)
	}

	for _, name := range []string{"RearAxle", "RearContact", "BB", "SeatTubeTop", "HeadTubeTop", "HeadTubeBottom", "LowerCupBottom", "FrontAxle", "FrontContact"} {
		sel := fmt.Sprintf(`svg.diagram [data-landmark="%s"]`, name)
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
	placed, err := assembly.PlaceChassis(fix.Chart.Frame, forkSetup, fix.Wheels, nil)
	if err != nil {
		t.Fatalf("PlaceChassis: %v", err)
	}
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

	cadBaseline, err := assembly.PlaceCadBaseline(fix.Chart, fix.Wheels)
	if err != nil {
		t.Fatalf("PlaceCadBaseline: %v", err)
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="delta-pitch"]`).Text()); got != fmt.Sprintf("%+.2f°", cadBaseline.DeltaPitchDeg(placed)) {
		t.Errorf("delta-pitch = %q, want %+.2f°", got, cadBaseline.DeltaPitchDeg(placed))
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="residual-axle-y"]`).Text()); got != fmt.Sprintf("%+.2f mm", cadBaseline.Residuals.AxleHeightDiffMm) {
		t.Errorf("residual-axle-y = %q, want %+.2f mm", got, cadBaseline.Residuals.AxleHeightDiffMm)
	}
	if got := strings.TrimSpace(doc.Find(`[data-metric="residual-wheelbase"]`).Text()); got != fmt.Sprintf("%+.2f mm", cadBaseline.Residuals.WheelbaseDiffMm) {
		t.Errorf("residual-wheelbase = %q, want %+.2f mm", got, cadBaseline.Residuals.WheelbaseDiffMm)
	}
	if prevFork, _ := doc.Find(`input[name="prev_fork"]`).Attr("value"); prevFork != "fox34_120" {
		t.Errorf("prev_fork hidden input = %q, want fox34_120", prevFork)
	}

	for _, name := range []string{"seat-tube", "seatstay"} {
		sel := fmt.Sprintf(`svg.diagram line[data-segment="%s"][data-illustrative="true"]`, name)
		if doc.Find(sel).Length() != 1 {
			t.Errorf("expected 1 illustrative segment matching %s, got %d", sel, doc.Find(sel).Length())
		}
	}
	if _, hasIll := doc.Find(`svg.diagram line[data-segment="head-tube"]`).Attr("data-illustrative"); hasIll {
		t.Errorf("head-tube should not be marked data-illustrative")
	}

	if facePaths := doc.Find(`svg.diagram .trace-steps path[data-op="face"]`).Length(); facePaths != 2 {
		t.Errorf("expected 2 face step paths in trace-steps, got %d", facePaths)
	}

	rows := doc.Find("table tbody tr[data-step-id]")
	if rows.Length() == 0 {
		t.Fatalf("expected non-empty trace table on /chassis")
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

func TestChassisWorkbenchForkSwitchAndCadBaseline(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantCode  int
		wantA2C   string
		wantSag   string
		wantCup   string
		wantDelta string
	}{
		{
			name:     "switch suspension to rigid resets sag to 0",
			query:    "/chassis?prev_fork=fox34_120&fork=lithic_carbon_mountain&cup=12&sag=20",
			wantCode: http.StatusOK,
			wantA2C:  "495.0 mm",
			wantSag:  "0.0",
			wantCup:  "12.0",
		},
		{
			name:     "switch rigid to SID applies SID default 20% sag",
			query:    "/chassis?prev_fork=lithic_carbon_mountain&fork=sid_100&cup=12&sag=0",
			wantCode: http.StatusOK,
			wantA2C:  "486.0 mm",
			wantSag:  "20.0",
			wantCup:  "12.0",
		},
		{
			name:     "staying on SID honors explicit sag=0",
			query:    "/chassis?prev_fork=sid_100&fork=sid_100&cup=12&sag=0",
			wantCode: http.StatusOK,
			wantA2C:  "506.0 mm",
			wantSag:  "0.0",
			wantCup:  "12.0",
		},
		{
			name:     "direct rigid fork with non-zero sag returns 400",
			query:    "/chassis?fork=lithic_carbon_mountain&sag=20",
			wantCode: http.StatusBadRequest,
		},
		{
			name:      "CAD baseline route uses fixture CadFork and 0 cup",
			query:     "/chassis?baseline=cad",
			wantCode:  http.StatusOK,
			wantA2C:   "495.0 mm",
			wantSag:   "0.0",
			wantCup:   "0.0",
			wantDelta: "+0.00°",
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
			}
		})
	}
}

func TestChassisWorkbenchRejectsInvalidQuery(t *testing.T) {
	for _, target := range []string{
		"http://127.0.0.1:8081/chassis?fork=unknown",
		"http://127.0.0.1:8081/chassis?cup=-1",
		"http://127.0.0.1:8081/chassis?cup=NaN",
		"http://127.0.0.1:8081/chassis?sag=150",
		"http://127.0.0.1:8081/chassis?sag=Inf",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		NewHandler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want 400", target, rec.Code)
		}
	}
}
