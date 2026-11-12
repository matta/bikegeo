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
		{"/ideals", []string{"idmatch"}},
		{"/ideals?name=idmatch", []string{"Ideal Profile: idmatch", "BRP Setback (brp_setback)"}},
		{"/saddles?id=berthoud-aspin-open", []string{"Berthoud Aspin Open"}},
		{"/saddles?id=selle-italia-slr-boost-superflow-l3", []string{"Selle Italia SLR Boost Superflow"}},
		{"/saddle-swap", []string{"70mm BRP Point (X, Y)", "Fit Dimension / Checkpoint"}},
		{"/saddle-swap?saddle-height=750&sit-bone-setback=290", []string{"750.0 mm", "-290.0"}},
		{"/saddle-swap?from=sqlab-610-ergolux&to=selle-italia-slr-boost-superflow-l3", []string{"Selle Italia SLR"}},
		{"/saddle-swap?frame=otso-fenrir-stainless:L", []string{"Fenrir"}},
		{"/handlebars", []string{"persuader20_800", "sqlab3OX_16_30"}},
		{"/handlebars?id=persuader20_800&compare=sqlab3OX_16_30", []string{
			"Handlebar Fit Translation: Tumbleweed Persuader 20° (800mm) -> SQlab 3OX 16° (30mm Rise)",
			"Delta (Bar 2 - Bar 1)", "+3.1 mm", "+1.5 mm",
			"-3.1 mm Reach and -1.5 mm Stack",
		}},
		{"/forks", []string{"sid_110", "RockShox SID 110mm", "521/44"}},
		{"/stems", []string{"mtb", "gravel"}},
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
