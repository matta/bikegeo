package catalog_test

import (
	"slices"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
	"bikegeo/internal/reboot/pose"
)

// TestForks checks the 22 fork models and the Q72 steerers.
func TestForks(t *testing.T) {
	forks := catalog.Forks()
	if len(forks) != 22 {
		t.Fatalf("got %d forks, want 22", len(forks))
	}
	seen := map[string]bool{}
	var straight []string
	for _, f := range forks {
		if f.ID == "" {
			t.Errorf("fork %q has an empty ID", f.Name)
		}
		if seen[f.ID] {
			t.Errorf("duplicate fork ID %q", f.ID)
		}
		seen[f.ID] = true
		if f.Name == "" {
			t.Errorf("fork %q has an empty name", f.ID)
		}
		if f.TravelMm < 0 {
			t.Errorf("fork %q travel %v mm < 0", f.ID, f.TravelMm)
		}
		switch f.Steerer {
		case assembly.SteererStraight118:
			straight = append(straight, f.ID)
		case assembly.SteererTapered:
		case assembly.SteererCAD:
			t.Errorf("fork %q uses the CAD sentinel steerer", f.ID)
		default:
			t.Errorf("fork %q has unknown steerer %q", f.ID, f.Steerer)
		}
	}
	want := []string{"sunliner_440", "surly_ecr_468", "surly_krampus_483", "sklar_steel_485"}
	if !slices.Equal(straight, want) {
		t.Errorf("straight-steerer forks = %v, want %v (Q72)", straight, want)
	}
}

// TestHandlebars checks the 15 handlebars place and leave the roll to the workbench.
func TestHandlebars(t *testing.T) {
	bars := catalog.Handlebars()
	if len(bars) != 15 {
		t.Fatalf("got %d handlebars, want 15", len(bars))
	}
	if bars[0].ID != "persuader20_800" {
		t.Errorf("first handlebar = %q, want persuader20_800", bars[0].ID)
	}
	seen := map[string]bool{}
	for _, bar := range bars {
		if seen[bar.ID] {
			t.Errorf("duplicate handlebar ID %q", bar.ID)
		}
		seen[bar.ID] = true
		if bar.Setup.BarRollDeg != 0 {
			t.Errorf("handlebar %q BarRollDeg = %v, want 0", bar.ID, bar.Setup.BarRollDeg)
		}
		if _, err := assembly.PlaceHandlebar(pose.NewPose(pose.Point{}, 0), bar.Setup); err != nil {
			t.Errorf("handlebar %q does not place: %v", bar.ID, err)
		}
	}
}

// TestSeatposts checks the four seatposts (Q28, Q75c).
func TestSeatposts(t *testing.T) {
	posts := catalog.Seatposts()
	var ids []string
	for _, p := range posts {
		ids = append(ids, p.ID)
	}
	if want := []string{"setback_0", "setback_15", "setback_25", "redshift_shockstop_pro"}; !slices.Equal(ids, want) {
		t.Fatalf("seatpost IDs = %v, want %v", ids, want)
	}
	for i, setback := range []float64{0, 15, 25, 12} {
		p := posts[i]
		if err := p.Spec.Validate(); err != nil {
			t.Errorf("seatpost %q invalid: %v", p.ID, err)
		}
		if p.Spec.SetbackMm != setback {
			t.Errorf("seatpost %q setback = %v mm, want %v", p.ID, p.Spec.SetbackMm, setback)
		}
		want := [3]float64{400, 365, 100}
		if p.ID == "redshift_shockstop_pro" {
			want = [3]float64{350, 250, 100}
		}
		if got := [3]float64{p.Spec.TotalLengthMm, p.Spec.MaxInsertionMm, p.Spec.MinInsertionMm}; got != want {
			t.Errorf("seatpost %q length/max/min = %v, want %v", p.ID, got, want)
		}
	}
}

// TestDefaultSaddleAndTargets checks the SQlab 610, the idmatch targets, and the sag steps.
func TestDefaultSaddleAndTargets(t *testing.T) {
	saddle := catalog.DefaultSaddle()
	if saddle.ID != "sqlab_610" || saddle.Name != "SQlab 610 Ergolux Active 2.1" {
		t.Errorf("saddle = %q %q", saddle.ID, saddle.Name)
	}
	want := assembly.SaddleSetup{
		RailCenterToBrpMm:      32,
		RailCenterToSitBonesMm: -48,
		RailToBrpHeightMm:      45,
		SitBonesElevationMm:    10,
		RailLengthMm:           60,
		RailPitchDeg:           0,
		RailOffsetMm:           0,
	}
	if saddle.Setup != want {
		t.Errorf("saddle setup = %+v, want %+v", saddle.Setup, want)
	}
	if got := catalog.DefaultBrpTarget(); got != (pose.Point{X: -219, Y: 713.1}) {
		t.Errorf("DefaultBrpTarget() = %v", got)
	}
	if got := catalog.DefaultGripTarget(); got != (pose.Point{X: 481, Y: 726}) {
		t.Errorf("DefaultGripTarget() = %v", got)
	}
	sags := catalog.SuspensionSagPcts()
	if !slices.Equal(sags, []float64{15, 20, 25}) {
		t.Fatalf("SuspensionSagPcts() = %v, want [15 20 25]", sags)
	}
	sags[0] = 99
	if got := catalog.SuspensionSagPcts(); got[0] != 15 {
		t.Errorf("mutating a returned SuspensionSagPcts slice changed the next call: %v", got)
	}
}
