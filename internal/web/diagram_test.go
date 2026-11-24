package web

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"slices"
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
	ovalPose := pose.NewPose[assembly.FrameAxis](*def.GripTarget, pose.Heading[assembly.DynamicWorld](oval.MajorAxisHeadingDeg))
	for _, along := range []float64{-oval.SemiMajorMm, oval.SemiMajorMm} {
		for _, across := range []float64{-oval.SemiMinorMm, oval.SemiMinorMm} {
			c := toSVG(ovalPose.Translate(pose.Vec[assembly.FrameAxis]{X: along, Y: across}).Origin())
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
			for _, c := range brpTargetFramingPts(pose.Point[assembly.DynamicWorld]{X: -650, Y: 1200}) {
				if !box.contains(c) {
					t.Errorf("%s extreme viewBox %+v does not contain BRP corner %+v", view, box, c)
				}
			}
		}
		if view == "bike" || view == "stem" {
			for _, c := range gripTargetFramingPts(pose.Point[assembly.DynamicWorld]{X: 950, Y: 1150}) {
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

func TestDiagramLabelCurationAndDOMInvariants(t *testing.T) {
	allLandmarks := []string{
		"RearAxle", "RearContact", "BB", "SeatTubeTop",
		"RailCradle", "RailCenter", "BRP", "SitBones",
		"HeadTubeTop", "CoverTop", "SpacerTop", "SteererClamp",
		"BarClamp", "HandGrip", "HeadTubeBottom", "LowerCupBottom",
		"FrontAxle", "FrontContact",
	}

	cases := []struct {
		path          string
		wantLandmarks []string
		wantTargets   []string
	}{
		{
			path: "/bike",
			wantLandmarks: []string{
				"RearAxle", "RearContact", "BB", "SeatTubeTop",
				"BRP", "SitBones", "HeadTubeTop", "HandGrip",
				"FrontAxle", "FrontContact",
			},
			wantTargets: []string{"Target BRP", "Target Grip"},
		},
		{
			path:          "/bike?view=seat",
			wantLandmarks: []string{"RailCradle", "RailCenter", "BRP", "SitBones"},
			wantTargets:   []string{"Target BRP"},
		},
		{
			path:          "/bike?view=stem",
			wantLandmarks: []string{"HeadTubeTop", "CoverTop", "SpacerTop", "SteererClamp", "BarClamp", "HandGrip"},
			wantTargets:   []string{"Target Grip"},
		},
		{
			path:          "/bike?view=seat&brp_x=-219&brp_y=450",
			wantLandmarks: []string{"SeatTubeTop", "RailCradle", "RailCenter", "BRP", "SitBones"},
			wantTargets:   []string{"Target BRP"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			doc := loadBikeDoc(t, tc.path)
			// Every one of the 18 landmark dots + <title> tooltip stays in DOM.
			if n := doc.Find("svg.diagram .landmarks [data-landmark]").Length(); n != len(allLandmarks) {
				t.Fatalf("landmark count = %d, want %d", n, len(allLandmarks))
			}
			var gotLandmarks []string
			for _, name := range allLandmarks {
				g := doc.Find(fmt.Sprintf(`svg.diagram .landmarks [data-landmark=%q]`, name))
				if g.Length() != 1 {
					t.Fatalf("missing [data-landmark=%q]", name)
				}
				if g.Find("circle").Length() != 1 {
					t.Errorf("%s missing <circle>", name)
				}
				if title := strings.TrimSpace(g.Find("title").Text()); title != name {
					t.Errorf("%s <title> = %q, want %q", name, title, name)
				}
				if txt := g.Find("text"); txt.Length() == 1 {
					gotLandmarks = append(gotLandmarks, strings.TrimSpace(txt.Text()))
				}
			}
			if !slices.Equal(gotLandmarks, tc.wantLandmarks) {
				t.Errorf("visible landmark labels = %v, want %v", gotLandmarks, tc.wantLandmarks)
			}

			var gotTargets []string
			doc.Find("svg.diagram .target-labels text").Each(func(_ int, el *goquery.Selection) {
				gotTargets = append(gotTargets, strings.TrimSpace(el.Text()))
			})
			if !slices.Equal(gotTargets, tc.wantTargets) {
				t.Errorf("visible target labels = %v, want %v", gotTargets, tc.wantTargets)
			}

			doc.Find("svg.diagram text").Each(func(_ int, el *goquery.Selection) {
				if fs, ok := el.Attr("font-size"); ok {
					t.Errorf("text %q has font-size=%q; must use CSS rem", el.Text(), fs)
				}
			})
		})
	}

	// In view=bike, detail dots render smaller (r="3") and subdued.
	bikeDoc := loadBikeDoc(t, "/bike")
	for _, detail := range []string{"RailCradle", "RailCenter", "CoverTop", "SpacerTop", "SteererClamp", "BarClamp", "HeadTubeBottom", "LowerCupBottom"} {
		g := bikeDoc.Find(fmt.Sprintf(`svg.diagram .landmarks [data-landmark=%q]`, detail))
		if d, _ := g.Attr("data-landmark-detail"); d != "true" {
			t.Errorf("bike view %s data-landmark-detail = %q, want true", detail, d)
		}
		if r, _ := g.Find("circle").Attr("r"); r != "3" {
			t.Errorf("bike view detail %s circle r = %q, want 3", detail, r)
		}
	}
}

func TestDiagramZeroLabelCollisionsAndViewBoxContainment(t *testing.T) {
	// Build a reachable fitted URL so BRP == Target BRP and HandGrip == Target Grip.
	targetSetup := mustPlace(t, mustBikeValues(t, map[string][]string{
		"post_insert": {"200"},
		"rail_offset": {"5"},
		"stem_length": {"80"},
		"spacers":     {"20"},
	}))
	wantBrp := targetSetup.Saddle.BRP.Origin()
	wantGrip := targetSetup.Handlebar.HandGrip.Origin()
	fittedURL := fmt.Sprintf("/bike?post_insert=200&rail_offset=5&stem_length=80&spacers=20&brp_x=%s&brp_y=%s&grip_x=%s&grip_y=%s",
		formatFormFloat(wantBrp.X), formatFormFloat(wantBrp.Y),
		formatFormFloat(wantGrip.X), formatFormFloat(wantGrip.Y),
	)

	builds := []struct {
		name string
		base string
	}{
		{"default", "/bike"},
		{"cleared_targets", "/bike?brp=off&grip=off"},
		{"fitted_coincident", fittedURL},
		{"spacers_zero", "/bike?spacers=0"},
		{"rail_offset_zero", "/bike?rail_offset=0"},
		{"basis_fork", "/bike?fork=basis"},
		{"swept_bar_short_stem", "/bike?bar=persuader20_840&stem_length=35&spacers=0"},
		{"xs_frame", "/bike?frame=otso-fenrir-stainless&size=XS"},
		{"xl_frame", "/bike?frame=otso-fenrir-stainless&size=XL"},
	}

	for _, b := range builds {
		for _, view := range []string{"bike", "seat", "stem"} {
			t.Run(b.name+"/"+view, func(t *testing.T) {
				sep := "&"
				if !strings.Contains(b.base, "?") {
					sep = "?"
				}
				doc := loadBikeDoc(t, b.base+sep+"view="+view)
				vb := parseViewBoxAttr(t, doc)
				upp := unitsPerPx(t, doc)
				viewW := vb.W / upp
				viewH := vb.H / upp

				// Pure Web 1.0 checks.
				if doc.Find("script").Length() != 0 {
					t.Errorf("unexpected <script> element")
				}
				doc.Find("[style]").Each(func(_ int, el *goquery.Selection) {
					if !el.Is("svg.diagram") {
						t.Errorf("unexpected inline style outside svg.diagram")
					}
				})

				type placedLabel struct {
					name   string
					box    screenBox
					ptPxX  float64
					ptPxY  float64
					leader *goquery.Selection
				}
				var placed []placedLabel

				recordLabel := func(name string, ptSVG svgPoint, textSel, leaderSel *goquery.Selection) {
					lx := parseFloatAttr(t, textSel, "x")
					ly := parseFloatAttr(t, textSel, "y")
					anchor, _ := textSel.Attr("text-anchor")
					ax := (lx - vb.X) / upp
					ay := (ly - vb.Y) / upp
					box := boxForLabelAnchor(ax, ay, labelBoxWidthPx(name), labelBoxHeightPx, anchor)

					// Every visible label box must stay inside viewBox.
					if box.minX < -0.5 || box.maxX > viewW+0.5 || box.minY < -0.5 || box.maxY > viewH+0.5 {
						t.Errorf("label %q box %+v escapes viewBox [0..%.1f, 0..%.1f]", name, box, viewW, viewH)
					}

					ptPxX := (ptSVG.X - vb.X) / upp
					ptPxY := (ptSVG.Y - vb.Y) / upp
					nx, ny := box.nearestPoint(ptPxX, ptPxY)
					dist := pose.Vec[assembly.DynamicWorld]{X: nx - ptPxX, Y: ny - ptPxY}.Norm()
					if dist > labelLeaderMinDistPx {
						if leaderSel.Length() != 1 {
							t.Errorf("displaced label %q (dist=%.1f px) missing .label-leader", name, dist)
						} else {
							x1 := parseFloatAttr(t, leaderSel, "x1")
							y1 := parseFloatAttr(t, leaderSel, "y1")
							if math.Abs(x1-ptSVG.X) > 0.05 || math.Abs(y1-ptSVG.Y) > 0.05 {
								t.Errorf("leader %q start (%.2f, %.2f) != point (%.2f, %.2f)", name, x1, y1, ptSVG.X, ptSVG.Y)
							}
						}
					} else if leaderSel.Length() != 0 {
						t.Errorf("undisplaced label %q (dist=%.1f px) emitted unexpected .label-leader", name, dist)
					}

					placed = append(placed, placedLabel{name: name, box: box, ptPxX: ptPxX, ptPxY: ptPxY, leader: leaderSel})
				}

				doc.Find("svg.diagram .landmarks [data-landmark]").Each(func(_ int, g *goquery.Selection) {
					txt := g.Find("text")
					if txt.Length() == 0 {
						return
					}
					name, _ := g.Attr("data-landmark")
					ptSVG := landmarkCentre(t, doc, name)
					recordLabel(name, ptSVG, txt, g.Find("line.label-leader"))
				})

				doc.Find("svg.diagram .target-labels [data-target-label-group]").Each(func(_ int, g *goquery.Selection) {
					txt := g.Find("text")
					if txt.Length() == 0 {
						return
					}
					kind, _ := g.Attr("data-target-label-group")
					marker := doc.Find(fmt.Sprintf(`[data-target-marker=%q]`, kind))
					tr, _ := marker.Attr("transform")
					var tx, ty float64
					if _, err := fmt.Sscanf(tr, "translate(%g %g)", &tx, &ty); err != nil {
						t.Fatalf("parse target transform %q: %v", tr, err)
					}
					recordLabel(strings.TrimSpace(txt.Text()), svgPoint{X: tx, Y: ty}, txt, g.Find("line.label-leader"))
				})

				if len(placed) == 0 {
					t.Fatalf("expected at least 1 visible label")
				}
				for i := 0; i < len(placed); i++ {
					for j := i + 1; j < len(placed); j++ {
						if placed[i].box.intersects(placed[j].box, 0) {
							t.Errorf("labels %q %+v and %q %+v intersect", placed[i].name, placed[i].box, placed[j].name, placed[j].box)
						}
					}
				}
			})
		}
	}
}
