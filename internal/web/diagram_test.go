package web

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"bikegeo/internal/assembly"
	"bikegeo/internal/pose"
	"bikegeo/internal/query"
)

func parseFloatAttr(t *testing.T, sel *goquery.Selection, attr string) float64 {
	t.Helper()
	raw, ok := sel.Attr(attr)
	if !ok {
		t.Fatalf("element missing attribute %q", attr)
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		t.Fatalf("attribute %s=%q: %v", attr, raw, err)
	}
	return v
}

func TestDiagramTargetsPassFailZonesAndMissVectors(t *testing.T) {
	// Default /bike has targets active and both out of gate (fail).
	for _, view := range []string{"bike", "seat", "stem"} {
		t.Run("default_"+view, func(t *testing.T) {
			doc := loadBikeDoc(t, "/bike?view="+view)
			if doc.Find("svg.diagram .fit-targets").Length() != 1 {
				t.Fatalf("expected 1 .fit-targets layer")
			}
			if doc.Find(".diagram-legend").Length() != 1 {
				t.Fatalf("expected 1 .diagram-legend when targets active")
			}
			if doc.Find(".diagram-legend svg[aria-hidden='true']").Length() != 4 {
				t.Errorf("expected 4 aria-hidden decorative legend swatches, got %d", doc.Find(".diagram-legend svg[aria-hidden='true']").Length())
			}

			brpGroup := doc.Find(`[data-target="brp"]`)
			if brpGroup.Length() != 1 {
				t.Fatalf("expected 1 [data-target='brp'], got %d", brpGroup.Length())
			}
			if status, _ := brpGroup.Attr("data-target-status"); status != "fail" {
				t.Errorf("default brp data-target-status = %q, want fail", status)
			}
			brpZone := brpGroup.Find(`rect[data-target-zone="brp"]`)
			if brpZone.Length() != 1 {
				t.Fatalf("expected rect[data-target-zone='brp']")
			}
			if w, _ := brpZone.Attr("width"); w != "20.00" {
				t.Errorf("brp zone width = %q, want 20.00", w)
			}
			if h, _ := brpZone.Attr("height"); h != "4.00" {
				t.Errorf("brp zone height = %q, want 4.00", h)
			}
			if brpGroup.Find(`line[data-target-miss="brp"]`).Length() != 1 {
				t.Errorf("expected miss line for default brp target")
			}
			brpMarker := brpGroup.Find(`[data-target-marker="brp"]`)
			if brpMarker.Length() != 1 {
				t.Fatalf("expected [data-target-marker='brp']")
			}
			if title := strings.TrimSpace(brpMarker.Find("title").Text()); !strings.Contains(title, "Target BRP") || !strings.Contains(title, "fail") {
				t.Errorf("brp marker title = %q, want Target BRP and fail", title)
			}
			if _, hasDash := brpMarker.Find("circle").Attr("stroke-dasharray"); !hasDash {
				t.Errorf("failing brp marker ring should have stroke-dasharray")
			}

			gripGroup := doc.Find(`[data-target="grip"]`)
			if gripGroup.Length() != 1 {
				t.Fatalf("expected 1 [data-target='grip'], got %d", gripGroup.Length())
			}
			if status, _ := gripGroup.Attr("data-target-status"); status != "fail" {
				t.Errorf("default grip data-target-status = %q, want fail", status)
			}
			gripZone := gripGroup.Find(`ellipse[data-target-zone="grip"]`)
			if gripZone.Length() != 1 {
				t.Fatalf("expected ellipse[data-target-zone='grip']")
			}
			if rx, _ := gripZone.Attr("rx"); rx != "20.00" {
				t.Errorf("grip zone rx = %q, want 20.00", rx)
			}
			if ry, _ := gripZone.Attr("ry"); ry != "10.00" {
				t.Errorf("grip zone ry = %q, want 10.00", ry)
			}
			if tr, _ := gripZone.Attr("transform"); !strings.Contains(tr, "rotate(-60.00)") {
				t.Errorf("grip zone transform = %q, want rotate(-60.00)", tr)
			}
			if gripGroup.Find(`line[data-target-miss="grip"]`).Length() != 1 {
				t.Errorf("expected miss line for default grip target")
			}
			gripMarker := gripGroup.Find(`[data-target-marker="grip"]`)
			if gripMarker.Length() != 1 {
				t.Fatalf("expected [data-target-marker='grip']")
			}
			if title := strings.TrimSpace(gripMarker.Find("title").Text()); !strings.Contains(title, "Target HandGrip") || !strings.Contains(title, "fail") {
				t.Errorf("grip marker title = %q, want Target HandGrip and fail", title)
			}
		})
	}

	// Following #apply-fitting on a build with reachable targets reaches both gates -> pass status and omitted miss lines.
	targetSetup := mustPlace(t, mustBikeValues(t, map[string][]string{
		"post_insert": {"200"},
		"rail_offset": {"5"},
		"stem_length": {"80"},
		"spacers":     {"20"},
	}))
	wantBrp := targetSetup.Saddle.BRP.Origin()
	wantGrip := targetSetup.Handlebar.HandGrip.Origin()
	reachablePath := fmt.Sprintf("/bike?brp_x=%s&brp_y=%s&grip_x=%s&grip_y=%s",
		formatFormFloat(wantBrp.X), formatFormFloat(wantBrp.Y),
		formatFormFloat(wantGrip.X), formatFormFloat(wantGrip.Y),
	)
	unfittedDoc := loadBikeDoc(t, reachablePath)
	applyHref, ok := unfittedDoc.Find("#apply-fitting").Attr("href")
	if !ok {
		t.Fatalf("missing #apply-fitting href")
	}
	fittedDoc := loadBikeDoc(t, applyHref)
	for _, kind := range []string{"brp", "grip"} {
		grp := fittedDoc.Find(fmt.Sprintf(`[data-target=%q]`, kind))
		if grp.Length() != 1 {
			t.Fatalf("fitted %s missing", kind)
		}
		if status, _ := grp.Attr("data-target-status"); status != "pass" {
			t.Errorf("fitted %s data-target-status = %q, want pass", kind, status)
		}
		if _, hasDash := grp.Find(fmt.Sprintf(`[data-target-marker=%q] circle`, kind)).Attr("stroke-dasharray"); hasDash {
			t.Errorf("passing %s marker ring should be solid (no stroke-dasharray)", kind)
		}
		// Distinct dash pattern on zone compared to default fail state.
		passDash, _ := grp.Find(fmt.Sprintf(`[data-target-zone=%q]`, kind)).Attr("stroke-dasharray")
		failDash, _ := unfittedDoc.Find(fmt.Sprintf(`[data-target-zone=%q]`, kind)).Attr("stroke-dasharray")
		if passDash == failDash {
			t.Errorf("%s pass and fail zone stroke-dasharray are identical (%q)", kind, passDash)
		}
		if fittedDoc.Find(fmt.Sprintf(`[data-target-miss=%q]`, kind)).Length() != 0 {
			t.Errorf("fitted reachable %s should omit coincident miss line", kind)
		}
	}
}

func TestDiagramPartialAndClearedTargets(t *testing.T) {
	onlyGrip := loadBikeDoc(t, "/bike?brp=off")
	if onlyGrip.Find(".fit-targets").Length() != 1 {
		t.Fatalf("brp=off should still render .fit-targets for grip")
	}
	if onlyGrip.Find(`[data-target="brp"]`).Length() != 0 {
		t.Errorf("brp=off must omit [data-target='brp']")
	}
	if onlyGrip.Find(`[data-target="grip"]`).Length() != 1 {
		t.Errorf("brp=off must keep [data-target='grip']")
	}
	if onlyGrip.Find(".diagram-legend").Length() != 1 {
		t.Errorf("brp=off should keep .diagram-legend")
	}

	onlyBrp := loadBikeDoc(t, "/bike?grip=off")
	if onlyBrp.Find(`[data-target="brp"]`).Length() != 1 {
		t.Errorf("grip=off must keep [data-target='brp']")
	}
	if onlyBrp.Find(`[data-target="grip"]`).Length() != 0 {
		t.Errorf("grip=off must omit [data-target='grip']")
	}

	none := loadBikeDoc(t, "/bike?brp=off&grip=off")
	if none.Find(".fit-targets").Length() != 0 {
		t.Errorf("brp=off&grip=off must omit .fit-targets")
	}
	if none.Find(".diagram-legend").Length() != 0 {
		t.Errorf("brp=off&grip=off must omit .diagram-legend")
	}
}

func TestDiagramTargetAwareFraming(t *testing.T) {
	def := query.DefaultBike()
	oval := assembly.DefaultGripOval()

	// Seat view contains Target BRP and all 4 gate-box corners.
	seatDoc := loadBikeDoc(t, "/bike?view=seat")
	seatBox := parseViewBoxAttr(t, seatDoc)
	for _, c := range brpTargetFramingPts(*def.BrpTarget) {
		if !seatBox.contains(c) {
			t.Errorf("seat viewBox %+v does not contain BRP gate corner %+v", seatBox, c)
		}
	}

	// Stem view contains Target Grip and all 4 oriented corners of its fit oval.
	stemDoc := loadBikeDoc(t, "/bike?view=stem")
	stemBox := parseViewBoxAttr(t, stemDoc)
	ovalPose := pose.NewPose(*def.GripTarget, oval.MajorAxisHeadingDeg)
	for _, along := range []float64{-oval.SemiMajorMm, oval.SemiMajorMm} {
		for _, across := range []float64{-oval.SemiMinorMm, oval.SemiMinorMm} {
			c := toSVG(ovalPose.Translate(along, across).Origin())
			if !stemBox.contains(c) {
				t.Errorf("stem viewBox %+v does not contain Grip oval corner %+v", stemBox, c)
			}
		}
	}

	// Extreme off-bike target expands wholeBox and stays bounded in close-ups without inversion.
	for _, view := range []string{"bike", "seat", "stem"} {
		path := fmt.Sprintf("/bike?view=%s&brp_x=-650&brp_y=1200&grip_x=950&grip_y=1150", view)
		doc := loadBikeDoc(t, path)
		box := parseViewBoxAttr(t, doc)
		if !(box.W > 0 && box.H > 0) || math.IsNaN(box.X) || math.IsNaN(box.Y) {
			t.Fatalf("%s extreme viewBox invalid: %+v", view, box)
		}
		if view == "bike" || view == "seat" {
			for _, c := range brpTargetFramingPts(pose.Point{X: -650, Y: 1200}) {
				if !box.contains(c) {
					t.Errorf("%s extreme viewBox %+v does not contain BRP corner %+v", view, box, c)
				}
			}
		}
		if view == "bike" || view == "stem" {
			for _, c := range gripTargetFramingPts(pose.Point{X: 950, Y: 1150}) {
				if !box.contains(c) {
					t.Errorf("%s extreme viewBox %+v does not contain Grip corner %+v", view, box, c)
				}
			}
		}
	}

	// Verify miss line endpoints match placed landmark to target marker center.
	bikeDoc := loadBikeDoc(t, "/bike")
	brpPlaced := landmarkCentre(t, bikeDoc, "BRP")
	brpMiss := bikeDoc.Find(`line[data-target-miss="brp"]`)
	if x1 := parseFloatAttr(t, brpMiss, "x1"); math.Abs(x1-brpPlaced.X) > 0.02 {
		t.Errorf("brp miss x1 = %v, want %v", x1, brpPlaced.X)
	}
	if y1 := parseFloatAttr(t, brpMiss, "y1"); math.Abs(y1-brpPlaced.Y) > 0.02 {
		t.Errorf("brp miss y1 = %v, want %v", y1, brpPlaced.Y)
	}
	wantBrp := toSVG(*def.BrpTarget)
	if x2 := parseFloatAttr(t, brpMiss, "x2"); math.Abs(x2-wantBrp.X) > 0.02 {
		t.Errorf("brp miss x2 = %v, want %v", x2, wantBrp.X)
	}
	if y2 := parseFloatAttr(t, brpMiss, "y2"); math.Abs(y2-wantBrp.Y) > 0.02 {
		t.Errorf("brp miss y2 = %v, want %v", y2, wantBrp.Y)
	}
}

func TestWebPackageUsesNoRawTrig(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	fset := token.NewFileSet()
	banned := map[string]bool{"Sin": true, "Cos": true, "Tan": true, "Atan": true, "Atan2": true, "Asin": true, "Acos": true, "Sincos": true}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Clean(name), nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if ok && ident.Name == "math" && banned[sel.Sel.Name] {
				t.Errorf("%s:%d calls raw trig math.%s (use internal/pose)", name, fset.Position(sel.Pos()).Line, sel.Sel.Name)
			}
			return true
		})
	}
}
