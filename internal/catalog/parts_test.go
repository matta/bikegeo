package catalog_test

import (
	"slices"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/catalog"
	"github.com/matta/bikegeo/internal/pose"
)

func forkByID(id string) (catalog.Fork, bool) {
	for _, f := range catalog.Forks() {
		if f.ID == id {
			return f, true
		}
	}
	return catalog.Fork{}, false
}

func rigidClassValidFor(c catalog.RigidClass, travel assembly.ForkTravel) bool {
	switch travel {
	case assembly.TravelRigid:
		switch c {
		case catalog.RigidClass440, catalog.RigidClass470, catalog.RigidClass485, catalog.RigidClass495:
			return true
		case catalog.RigidClassNone:
			return false
		default:
			return false
		}
	case assembly.Travel100, assembly.Travel120, assembly.Travel130:
		return c == catalog.RigidClassNone
	default:
		return false
	}
}

// TestForks checks the 16 fork models, their variants, IDs, and physical steerers.
func TestForks(t *testing.T) {
	forks := catalog.Forks()
	if len(forks) != 16 {
		t.Fatalf("got %d forks, want 16", len(forks))
	}
	for _, id := range []string{"rigid_483", "rigid_485", "rigid_490", "rigid_500", "rigid_510"} {
		if _, ok := forkByID(id); ok {
			t.Errorf("forkByID(%q) ok = true, want false", id)
		}
	}
	seen := map[string]bool{}
	seenVariantIDs := map[string]bool{}
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
		if !rigidClassValidFor(f.RigidClass, f.Travel) {
			t.Errorf("fork %q has rigid class %q for travel %v", f.ID, f.RigidClass, f.Travel)
		}
		if !f.Travel.Valid() {
			t.Errorf("fork %q has unsupported travel %v", f.ID, f.Travel)
		}
		if len(f.Variants) == 0 {
			t.Errorf("fork %q has no variants", f.ID)
		}
		for i, v := range f.Variants {
			if v.StaticA2cMm <= 0 || v.RakeMm <= 0 {
				t.Errorf("fork %q variant %d has non-positive dims %+v", f.ID, i, v)
			}
			if i > 0 {
				prev := f.Variants[i-1]
				if v.StaticA2cMm < prev.StaticA2cMm || (v.StaticA2cMm == prev.StaticA2cMm && v.RakeMm <= prev.RakeMm) {
					t.Errorf("fork %q variants not strictly ascending: %+v then %+v", f.ID, prev, v)
				}
			}
			vid := f.VariantID(v)
			if seenVariantIDs[vid] {
				t.Errorf("duplicate variant ID %q", vid)
			}
			seenVariantIDs[vid] = true
		}
		if got, ok := forkByID(f.ID); !ok || got.ID != f.ID {
			t.Errorf("forkByID(%q) = %q, %v", f.ID, got.ID, ok)
		}
		switch f.Steerer {
		case assembly.PhysicalSteererStraight118:
			straight = append(straight, f.ID)
		case assembly.PhysicalSteererTapered:
		default:
			t.Errorf("fork %q has unknown steerer %q", f.ID, f.Steerer)
		}
	}
	want := []string{"sunliner_440", "surly_ecr_468", "surly_krampus_483", "sklar_steel_485"}
	if !slices.Equal(straight, want) {
		t.Errorf("straight-steerer forks = %v, want %v", straight, want)
	}
	if _, ok := forkByID("missing"); ok {
		t.Errorf("forkByID(missing) ok = true, want false")
	}
}

// TestHandlebars checks the 15 catalog handlebars construct valid Handlebar parts.
func TestHandlebars(t *testing.T) {
	bars := catalog.Handlebars()
	if len(bars) != 15 {
		t.Fatalf("got %d handlebars, want 15", len(bars))
	}
	if bars[0].ID != "persuader20_800" {
		t.Errorf("first handlebar = %q, want persuader20_800", bars[0].ID)
	}
	seen := map[string]bool{}
	barClamp := assembly.BarClampPort{
		ClampInParent: pose.OriginPose(assembly.DynamicWorld).Anchor(assembly.StemLocal),
	}
	for _, bar := range bars {
		if seen[bar.ID] {
			t.Errorf("duplicate handlebar ID %q", bar.ID)
		}
		seen[bar.ID] = true
		if bar.Part.GripInBar().CoordinateSystem() != assembly.HandlebarLocal {
			t.Errorf("handlebar %q GripInBar system = %s", bar.ID, bar.Part.GripInBar().CoordinateSystem())
		}
		if _, err := assembly.MountHandlebar(barClamp, bar.Part, 0); err != nil {
			t.Errorf("handlebar %q does not mount: %v", bar.ID, err)
		}
	}
}

// TestSeatposts checks the four seatposts (three RigidPost and one LinkagePost).
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
		if p.SetbackMm() != setback {
			t.Errorf("seatpost %q setback = %v mm, want %v", p.ID, p.SetbackMm(), setback)
		}
		want := [3]float64{400, 365, 100}
		if p.ID == "redshift_shockstop_pro" {
			want = [3]float64{350, 250, 100}
			if _, ok := p.Part.(assembly.LinkagePost); !ok {
				t.Errorf("redshift_shockstop_pro type = %T, want assembly.LinkagePost", p.Part)
			}
		} else if _, ok := p.Part.(assembly.RigidPost); !ok {
			t.Errorf("%s type = %T, want assembly.RigidPost", p.ID, p.Part)
		}
		limits := p.Part.InsertionLimits()
		if got := [3]float64{p.TotalLengthMm(), limits.Max(), limits.Min()}; got != want {
			t.Errorf("seatpost %q length/max/min = %v, want %v", p.ID, got, want)
		}
	}
}

// TestDefaultSaddleAndTargets checks the SQlab 610 and the default idmatch targets.
func TestDefaultSaddleAndTargets(t *testing.T) {
	saddle := catalog.DefaultSaddle()
	if saddle.ID != "sqlab_610" || saddle.Name != "SQlab 610 Ergolux Active 2.1" {
		t.Errorf("saddle = %q %q", saddle.ID, saddle.Name)
	}
	want := assembly.SaddleDimensions{
		RailCenterToBrpMm:      32,
		RailCenterToSitBonesMm: -48,
		RailToBrpHeightMm:      45,
		SitBonesElevationMm:    10,
		RailLengthMm:           60,
	}
	if saddle.Part.Dimensions() != want {
		t.Errorf("saddle dims = %+v, want %+v", saddle.Part.Dimensions(), want)
	}
	if got := catalog.DefaultBrpTarget(); got != pose.NewPoint(assembly.DynamicWorld, -219, 713.1) {
		t.Errorf("DefaultBrpTarget() = %v", got)
	}
	if got := catalog.DefaultGripTarget(); got != pose.NewPoint(assembly.DynamicWorld, 481, 726) {
		t.Errorf("DefaultGripTarget() = %v", got)
	}
}

// TestCatalogDefaultsAndLookups checks the default part IDs, ID lookups, and
// default stem dimensions.
func TestCatalogDefaultsAndLookups(t *testing.T) {
	bar, ok := catalog.HandlebarByID(catalog.DefaultHandlebarID)
	if !ok || bar.ID != "persuader20_800" {
		t.Errorf("HandlebarByID(%q) = %q, %v; want persuader20_800, true", catalog.DefaultHandlebarID, bar.ID, ok)
	}
	if _, ok := catalog.HandlebarByID("missing"); ok {
		t.Errorf("HandlebarByID(missing) ok = true, want false")
	}

	post, ok := catalog.SeatpostByID(catalog.DefaultSeatpostID)
	if !ok || post.ID != "redshift_shockstop_pro" {
		t.Errorf("SeatpostByID(%q) = %q, %v; want redshift_shockstop_pro, true", catalog.DefaultSeatpostID, post.ID, ok)
	}
	if _, ok := catalog.SeatpostByID("missing"); ok {
		t.Errorf("SeatpostByID(missing) ok = true, want false")
	}

	if catalog.SpacerStepMm != 5.0 {
		t.Errorf("SpacerStepMm = %v, want 5", catalog.SpacerStepMm)
	}

	stem, err := assembly.NewStem(catalog.DefaultStemClampHeightMm, catalog.DefaultStemLengthMm, catalog.DefaultStemAngleDeg)
	if err != nil {
		t.Fatalf("NewStem(defaults): %v", err)
	}
	steerer, err := assembly.NewSteererPort(
		pose.NewPose(pose.Origin(assembly.FrameLocal), 111.75).Anchor(assembly.SteererLocal),
		assembly.DefaultUpperStackMm,
		assembly.DefaultMaxSpacersMm,
	)
	if err != nil {
		t.Fatalf("NewSteererPort: %v", err)
	}
	if _, err := assembly.MountStem(steerer, catalog.DefaultSpacersMm, stem); err != nil {
		t.Errorf("MountStem(default stem): %v", err)
	}
	if stem.ClampHeightMm() != 40 || stem.LengthMm() != 60 || stem.AngleDeg() != -6 {
		t.Errorf("default stem = (%v, %v, %v), want (40, 60, -6)", stem.ClampHeightMm(), stem.LengthMm(), stem.AngleDeg())
	}
}

// TestTypicalLowerCup checks the Cane Creek 40-Series stacks.
func TestTypicalLowerCup(t *testing.T) {
	want := map[assembly.LowerCupStandard]float64{
		assembly.LowerCupEC34: 12, assembly.LowerCupEC44: 12,
		assembly.LowerCupZS44: 4, assembly.LowerCupZS56: 4,
		assembly.LowerCupIS42: 1, assembly.LowerCupIS52: 1,
	}
	for std, stack := range want {
		got, ok := catalog.TypicalLowerCup(std)
		if !ok || got.Standard != std || got.StackMm != stack {
			t.Errorf("TypicalLowerCup(%s) = %+v, %v; want stack %v", std, got, ok, stack)
		}
	}
	for _, std := range []assembly.LowerCupStandard{assembly.LowerCupBareHeadTube, "invalid"} {
		if got, ok := catalog.TypicalLowerCup(std); ok || got != (assembly.LowerCupSetup{}) {
			t.Errorf("TypicalLowerCup(%q) = %+v, %v; want zero, false", std, got, ok)
		}
	}
}

// TestLowerCupFor checks per-fork cup resolution and steerer rejection.
func TestLowerCupFor(t *testing.T) {
	cup := func(std assembly.LowerCupStandard, stack float64) assembly.LowerCupSetup {
		return assembly.LowerCupSetup{Standard: std, StackMm: stack}
	}
	straight, tapered := assembly.PhysicalSteererStraight118, assembly.PhysicalSteererTapered
	cases := []struct {
		name       string
		bore       assembly.LowerBore
		defaultStd assembly.LowerCupStandard
		steerer    assembly.PhysicalSteerer
		want       assembly.LowerCupSetup
		ok         bool
	}{
		{"34 straight", assembly.Bore34Pressed, assembly.LowerCupEC34, straight, cup(assembly.LowerCupEC34, 12), true},
		{"34 tapered", assembly.Bore34Pressed, assembly.LowerCupEC34, tapered, assembly.LowerCupSetup{}, false},
		{"42 straight", assembly.Bore42Integrated, assembly.LowerCupIS42, straight, cup(assembly.LowerCupIS42, 1), true},
		{"42 tapered", assembly.Bore42Integrated, assembly.LowerCupIS42, tapered, assembly.LowerCupSetup{}, false},
		{"44 straight keeps ZS44", assembly.Bore44Pressed, assembly.LowerCupZS44, straight, cup(assembly.LowerCupZS44, 4), true},
		{"44 straight EC44 default", assembly.Bore44Pressed, assembly.LowerCupEC44, straight, cup(assembly.LowerCupEC44, 12), true},
		{"44 tapered from ZS44 default", assembly.Bore44Pressed, assembly.LowerCupZS44, tapered, cup(assembly.LowerCupEC44, 12), true},
		{"44 tapered", assembly.Bore44Pressed, assembly.LowerCupEC44, tapered, cup(assembly.LowerCupEC44, 12), true},
		{"56 straight", assembly.Bore56Pressed, assembly.LowerCupZS56, straight, cup(assembly.LowerCupZS56, 4), true},
		{"56 tapered", assembly.Bore56Pressed, assembly.LowerCupZS56, tapered, cup(assembly.LowerCupZS56, 4), true},
		{"52 straight", assembly.Bore52Integrated, assembly.LowerCupIS52, straight, cup(assembly.LowerCupIS52, 1), true},
		{"52 tapered", assembly.Bore52Integrated, assembly.LowerCupIS52, tapered, cup(assembly.LowerCupIS52, 1), true},
		{"unknown bore", "nope", assembly.LowerCupEC44, straight, assembly.LowerCupSetup{}, false},
		{"unknown steerer", assembly.Bore44Pressed, assembly.LowerCupEC44, "nope", assembly.LowerCupSetup{}, false},
	}
	for _, tc := range cases {
		got, ok := catalog.LowerCupFor(tc.bore, tc.defaultStd, tc.steerer)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: LowerCupFor = %+v, %v; want %+v, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

// TestRigidClassValidFor checks the class and travel pairing.
func TestRigidClassValidFor(t *testing.T) {
	for _, c := range []catalog.RigidClass{catalog.RigidClass440, catalog.RigidClass470, catalog.RigidClass485, catalog.RigidClass495} {
		if !rigidClassValidFor(c, assembly.TravelRigid) {
			t.Errorf("rigidClassValidFor(%q, rigid) = false, want true", c)
		}
	}
	for _, tr := range []assembly.ForkTravel{assembly.Travel100, assembly.Travel120, assembly.Travel130} {
		if !rigidClassValidFor(catalog.RigidClassNone, tr) {
			t.Errorf("rigidClassValidFor(none, %v) = false, want true", tr)
		}
	}
	for _, tc := range []struct {
		c  catalog.RigidClass
		tr assembly.ForkTravel
	}{
		{catalog.RigidClassNone, assembly.TravelRigid},
		{catalog.RigidClass485, assembly.Travel100},
		{catalog.RigidClass("510"), assembly.TravelRigid},
		{catalog.RigidClassNone, assembly.ForkTravel(-1)},
	} {
		if rigidClassValidFor(tc.c, tc.tr) {
			t.Errorf("rigidClassValidFor(%q, %v) = true, want false", tc.c, tc.tr)
		}
	}
}
