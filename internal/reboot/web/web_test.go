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

	// Verify BarClamp summary matches PlaceStem output.
	placed, err := assembly.PlaceStem(
		pose.NewPose(pose.Point{X: 0, Y: 0}, 180.0-68.25, nil),
		assembly.StemSetup{
			UpperCoverMm:  11.0,
			SpacersMm:     20.0,
			ClampHeightMm: 40.0,
			LengthMm:      60.0,
			AngleDeg:      -6.0,
		},
	)
	if err != nil {
		t.Fatalf("PlaceStem: %v", err)
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

	// Verify every trace table row has a matching SVG step element.
	rows := doc.Find("table tbody tr[data-step-id]")
	if rows.Length() != 6 {
		t.Fatalf("trace table rows = %d, want 6", rows.Length())
	}
	rows.Each(func(_ int, row *goquery.Selection) {
		id, ok := row.Attr("data-step-id")
		if !ok || id == "" {
			t.Errorf("table row missing data-step-id")
			return
		}
		svgMatch := doc.Find(fmt.Sprintf(`svg.diagram .trace-steps [data-step-id="%s"]`, id))
		if svgMatch.Length() != 1 {
			t.Errorf("step %s: expected 1 SVG trace element, got %d", id, svgMatch.Length())
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
