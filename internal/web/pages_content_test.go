package web

import (
	"net/http"
	"strings"
	"testing"
)

// Content checks carried over from the retired CLI subcommand tests: each
// page must show the same headline facts the terminal output used to.
func TestPages_ContentCarriedOverFromCLI(t *testing.T) {
	cases := []struct {
		url  string
		want []string
	}{
		{"/help", []string{"idmatch", "Cockpit target: 700 mm from the BRP, +3 mm above the sit bones", "HandGrip", "40–100 mm", "60–110 mm"}},
		{"/saddles?id=berthoud-aspin-open", []string{"Berthoud Aspin Open"}},
		{"/saddles?id=selle-italia-slr-boost-superflow-l3", []string{"Selle Italia SLR Boost Superflow"}},
		{"/saddle-swap", []string{"70mm BRP Point (X, Y)", "BB-to-SIT Distance (geometric proxy)", "691.9 mm"}},
		{"/saddle-swap?from=berthoud-aspin-open&to=sqlab-610-ergolux", []string{"691.9 mm", "700.0 mm"}},
		{"/saddle-swap?saddle-height=750&sit-bone-setback=290", []string{"750.0 mm", "-290.0"}},
		{"/saddle-swap?from=sqlab-610-ergolux&to=selle-italia-slr-boost-superflow-l3", []string{"Selle Italia SLR"}},
		{"/saddle-swap?frame=otso-fenrir-stainless:L", []string{"Fenrir"}},
		{"/handlebars", []string{"persuader20_800", "sqlab3OX_16_30"}},
		{"/handlebars?id=persuader20_800&compare=sqlab3OX_16_30", []string{
			"Handlebar Fit Translation: Tumbleweed Persuader 20° (800mm) -> SQlab 3OX 16° (30mm Rise)",
			"Delta (Bar 2 - Bar 1)", "+3.1 mm", "+1.5 mm",
			"-3.1 mm Reach and -1.5 mm Stack",
		}},
		{"/frames?q=havok", []string{"binary-havok"}},
	}
	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			rec, doc := do(t, c.url)
			mustStatus(t, rec, http.StatusOK)
			got := doc.Text()
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q", w)
				}
			}
		})
	}
}

func TestSaddleDefaultSupportTarget(t *testing.T) {
	for _, path := range []string{"/saddles", "/saddle-swap"} {
		t.Run(path, func(t *testing.T) {
			rec, doc := do(t, path)
			mustStatus(t, rec, http.StatusOK)
			body := doc.Text()
			if got := doc.Find(`input[name="sit-bone-setback"]`).AttrOr("placeholder", ""); got != "behind BB, default 299" {
				t.Error("default support-target hint must reflect the corrected reference")
			}
			if path == "/saddles" && !strings.Contains(body, "Target SIT Setback: 299mm") {
				t.Error("saddle calculations must use the corrected default support target")
			}
			if path == "/saddle-swap" && !strings.Contains(body, "-299.0") {
				t.Error("saddle swap must preserve the corrected reference support point")
			}
		})
	}
}
