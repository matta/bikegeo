package query_test

import (
	"errors"
	"math"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
	"bikegeo/internal/reboot/pose"
	"bikegeo/internal/reboot/query"
)

func mustParseBike(t *testing.T, raw string) query.Bike {
	t.Helper()
	b, err := query.ParseBike(mustParseQuery(t, raw))
	if err != nil {
		t.Fatalf("ParseBike(%q): %v", raw, err)
	}
	return b
}

func near(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.9f, want %.9f (tol %g)", name, got, want, tol)
	}
}

func cupString(c assembly.LowerCupSetup) string {
	return string(c.Standard) + " " + query.FormatFloat(c.StackMm)
}

func TestParseBikeDefaults(t *testing.T) {
	for name, q := range map[string]url.Values{"nil": nil, "empty": {}} {
		b, err := query.ParseBike(q)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		s := b.Setup
		if b.Frame.ID != "otso-fenrir-stainless" || b.Size.Name != "L" || b.ForkID != "lithic_carbon_mountain-495x44" ||
			b.Bar.ID != "persuader20_800" || b.Post.ID != "redshift_shockstop_pro" || b.View != query.ViewBike {
			t.Errorf("%s: unexpected IDs %q %q %q %q %q view %q", name, b.Frame.ID, b.Size.Name, b.ForkID, b.Bar.ID, b.Post.ID, b.View)
		}
		if s.LowerCup.Standard != assembly.LowerCupEC44 || s.LowerCup.StackMm != 12 ||
			s.Fork.Spec.StaticA2cMm != 495 || s.Fork.SagPct != 0 {
			t.Errorf("%s: cup/fork = %+v / %+v", name, s.LowerCup, s.Fork)
		}
		if s.Seatpost.InsertionMm != 175.0 || s.Stem.SpacersMm != 20 || s.Stem.LengthMm != 60 || s.Stem.AngleDeg != -6 {
			t.Errorf("%s: post/stem = %v / %+v", name, s.Seatpost.InsertionMm, s.Stem)
		}
		if b.BrpTarget == nil || *b.BrpTarget != (pose.Point{X: -219, Y: 713.1}) ||
			b.GripTarget == nil || *b.GripTarget != (pose.Point{X: 481, Y: 726}) {
			t.Errorf("%s: targets = %v / %v", name, b.BrpTarget, b.GripTarget)
		}
		if b.AllBarOptions || b.AllForkOptions {
			t.Errorf("%s: options should default to selected", name)
		}
	}
	if !reflect.DeepEqual(query.DefaultBike(), mustParseBike(t, "")) {
		t.Error("DefaultBike differs from ParseBike(nil)")
	}
}

func TestParseBikeAllFramesAndSlidingDropouts(t *testing.T) {
	b := mustParseBike(t, "frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20")
	if b.Frame.ID != "surly-krampus" || b.Size.Name != "XL" || b.ForkID != "fox34_120-534x44" ||
		b.Setup.Fork.SagPct != 20 || cupString(b.Setup.LowerCup) != "EC44 12" || b.Setup.Seatpost.InsertionMm != 175.0 {
		t.Errorf("krampus = %s %s %s sag %v cup %s ins %v", b.Frame.ID, b.Size.Name, b.ForkID, b.Setup.Fork.SagPct, cupString(b.Setup.LowerCup), b.Setup.Seatpost.InsertionMm)
	}

	b = mustParseBike(t, "frame=binary-havok&size=L")
	if b.ForkID != "rigid_485-485x45" || b.Setup.Frame.ChainstayMm != 460 || cupString(b.Setup.LowerCup) != "EC44 12" || b.Setup.Seatpost.InsertionMm != 175.0 {
		t.Errorf("havok physical = %s chainstay %v cup %s ins %v", b.ForkID, b.Setup.Frame.ChainstayMm, cupString(b.Setup.LowerCup), b.Setup.Seatpost.InsertionMm)
	}

	b = mustParseBike(t, "frame=binary-havok&size=L&fork=basis&cup=99&fork_a2c=999&sag=50")
	if b.Setup.Frame.ChainstayMm != 445 || cupString(b.Setup.LowerCup) != "EC44 10" || b.Setup.Fork.Spec.StaticA2cMm != 485 ||
		b.Setup.Fork.SagPct != 0 || b.Setup.Fork.Spec.Steerer != assembly.SteererBasis {
		t.Errorf("havok basis = chainstay %v cup %s fork %+v", b.Setup.Frame.ChainstayMm, cupString(b.Setup.LowerCup), b.Setup.Fork)
	}

	b = mustParseBike(t, "frame=salsa-fargo&size=XS&post=setback_0")
	bounds, err := assembly.SeatpostInsertionBounds(b.Setup.Frame, b.Post.Spec)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "fargo min", bounds.EffectiveMinInsertionMm, 100, 1e-9)
	near(t, "fargo max", bounds.MaxAllowedInsertionMm, 335.6, 1e-9)
	near(t, "fargo insertion", b.Setup.Seatpost.InsertionMm, 217.8, 1e-9)

	for _, f := range catalog.Frames() {
		for _, sz := range f.Sizes {
			if _, err := query.ParseBike(url.Values{"frame": {f.ID}, "size": {sz.Name}}); err != nil {
				t.Errorf("frame %s size %s: %v", f.ID, sz.Name, err)
			}
		}
	}
}

func TestParseBikeSwitchesQ109(t *testing.T) {
	// D8: esker-hayduke now carries lithic_carbon_mountain, so switching from Fenrir's default
	// preserves lithic_carbon_mountain-495x44, while switching from sid_100-506x44 falls back to pike_120.
	b := mustParseBike(t, "prev_frame=otso-fenrir-stainless&frame=esker-hayduke&size=L&fork=lithic_carbon_mountain-495x44&fork_a2c=495&cup=12&sag=0&stem_length=80&bar=sqlab3ox_16_30")
	if b.Size.Name != "S3" || b.ForkID != "lithic_carbon_mountain-495x44" || b.Setup.Fork.Spec.StaticA2cMm != 495 ||
		cupString(b.Setup.LowerCup) != "EC44 12" || b.Setup.Fork.SagPct != 0 || b.Setup.Stem.LengthMm != 80 || b.Bar.ID != "sqlab3ox_16_30" {
		t.Errorf("esker switch (lithic) = %s %s a2c %v cup %s sag %v stem %v bar %s", b.Size.Name, b.ForkID, b.Setup.Fork.Spec.StaticA2cMm, cupString(b.Setup.LowerCup), b.Setup.Fork.SagPct, b.Setup.Stem.LengthMm, b.Bar.ID)
	}
	b = mustParseBike(t, "prev_frame=otso-fenrir-stainless&frame=esker-hayduke&size=L&fork=sid_100-506x44&fork_a2c=506&cup=12&sag=20&stem_length=80&bar=sqlab3ox_16_30")
	if b.Size.Name != "S3" || b.ForkID != "pike_120" || b.Setup.Fork.Spec.StaticA2cMm != 531 ||
		cupString(b.Setup.LowerCup) != "EC44 12" || b.Setup.Fork.SagPct != 25 || b.Setup.Stem.LengthMm != 80 || b.Bar.ID != "sqlab3ox_16_30" {
		t.Errorf("esker switch (sid->pike) = %s %s a2c %v cup %s sag %v stem %v bar %s", b.Size.Name, b.ForkID, b.Setup.Fork.Spec.StaticA2cMm, cupString(b.Setup.LowerCup), b.Setup.Fork.SagPct, b.Setup.Stem.LengthMm, b.Bar.ID)
	}

	b = mustParseBike(t, "prev_frame=otso-fenrir-stainless&frame=surly-krampus&size=L&fork=lithic_carbon_mountain-495x44&fork_a2c=495&cup=12&sag=0")
	if b.Size.Name != "L" || b.ForkID != "surly_krampus_483" || b.Setup.Fork.Spec.StaticA2cMm != 483 ||
		cupString(b.Setup.LowerCup) != "EC44 12" || b.Setup.Fork.SagPct != 0 {
		t.Errorf("krampus switch = %s %s a2c %v cup %s sag %v", b.Size.Name, b.ForkID, b.Setup.Fork.Spec.StaticA2cMm, cupString(b.Setup.LowerCup), b.Setup.Fork.SagPct)
	}

	b = mustParseBike(t, "prev_frame=otso-fenrir-stainless&frame=salsa-cutthroat&size=L&fork=sid_100-506x44&fork_a2c=506&cup=12&sag=25")
	if b.Size.Name != "56cm" || b.ForkID != "sid_100-506x44" || cupString(b.Setup.LowerCup) != "IS52 0" ||
		b.Setup.Fork.Spec.StaticA2cMm != 506 || b.Setup.Fork.Spec.RakeMm != 44 || b.Setup.Fork.SagPct != 20 {
		t.Errorf("cutthroat switch = %s %s cup %s a2c %v rake %v sag %v", b.Size.Name, b.ForkID, cupString(b.Setup.LowerCup), b.Setup.Fork.Spec.StaticA2cMm, b.Setup.Fork.Spec.RakeMm, b.Setup.Fork.SagPct)
	}

	b = mustParseBike(t, "prev_frame=otso-fenrir-stainless&frame=binary-havok&fork=basis&fork_a2c=495&cup=0&sag=0")
	if b.ForkID != query.BasisForkID || b.Setup.Fork.Spec.StaticA2cMm != 485 || cupString(b.Setup.LowerCup) != "EC44 10" || b.Setup.Frame.ChainstayMm != 445 {
		t.Errorf("havok basis switch = %s a2c %v cup %s chainstay %v", b.ForkID, b.Setup.Fork.Spec.StaticA2cMm, cupString(b.Setup.LowerCup), b.Setup.Frame.ChainstayMm)
	}

	b = mustParseBike(t, "prev_post=setback_0&post=redshift_shockstop_pro&post_insert=275")
	if b.Setup.Seatpost.InsertionMm != 250 {
		t.Errorf("post switch insertion = %v, want 250", b.Setup.Seatpost.InsertionMm)
	}
	b = mustParseBike(t, "post=redshift_shockstop_pro&post_insert=275")
	if b.Setup.Seatpost.InsertionMm != 275 {
		t.Errorf("direct insertion = %v, want 275", b.Setup.Seatpost.InsertionMm)
	}
	b = mustParseBike(t, "frame=otso-fenrir-stainless&prev_size=XL&size=L&post=setback_0&post_insert=350")
	if b.Setup.Seatpost.InsertionMm != 290 {
		t.Errorf("size switch insertion = %v, want 290", b.Setup.Seatpost.InsertionMm)
	}

	b = mustParseBike(t, "prev_fork=basis&fork=fox34_120-534x44&fork_a2c=495&cup=0&sag=0")
	if b.Setup.Fork.Spec.StaticA2cMm != 534 || cupString(b.Setup.LowerCup) != "EC44 12" || b.Setup.Fork.SagPct != 25 {
		t.Errorf("basis->fox = a2c %v cup %s sag %v", b.Setup.Fork.Spec.StaticA2cMm, cupString(b.Setup.LowerCup), b.Setup.Fork.SagPct)
	}
	// Leaving the basis restores the physical cup even when the cup
	// standard is shared (binary-havok: basis EC44 10 mm, physical EC44 12 mm).
	b = mustParseBike(t, "frame=binary-havok&size=L&prev_fork=basis&fork=rigid_485-485x45&fork_a2c=485&cup=10&sag=0")
	if cupString(b.Setup.LowerCup) != "EC44 12" {
		t.Errorf("basis->physical cup = %s, want EC44 12", cupString(b.Setup.LowerCup))
	}
	b = mustParseBike(t, "prev_fork=lithic_carbon_mountain-495x44&fork=fox34_120-534x44&fork_a2c=495&cup=14&sag=0")
	if b.Setup.Fork.Spec.StaticA2cMm != 534 || cupString(b.Setup.LowerCup) != "EC44 14" || b.Setup.Fork.SagPct != 25 {
		t.Errorf("fork switch = a2c %v cup %s sag %v", b.Setup.Fork.Spec.StaticA2cMm, cupString(b.Setup.LowerCup), b.Setup.Fork.SagPct)
	}
}

func TestParseBikeTargetsAndURLRoundTrip(t *testing.T) {
	for _, raw := range []string{
		"",
		"brp=off",
		"grip=off",
		"brp=off&grip=off",
		"brp_x=&brp_y=",
		"brp=off&brp_x=-200&brp_y=710",
		"bar_options=all",
		"fork_options=all",
		"fork=basis",
		"post_insert=144.10725197204226&cup=12.5000000001",
		"view=seat&stem_angle=-0&rail_pitch=-0",
	} {
		b := mustParseBike(t, raw)
		u := b.URL()
		if strings.Contains(u, "prev_") {
			t.Errorf("%q: URL %q contains prev_*", raw, u)
		}
		if b.BrpTarget == nil && !strings.Contains(u, "brp=off") {
			t.Errorf("%q: cleared BRP missing brp=off in %q", raw, u)
		}
		if b.GripTarget == nil && !strings.Contains(u, "grip=off") {
			t.Errorf("%q: cleared grip missing grip=off in %q", raw, u)
		}
		if strings.Contains(u, "-0&") || strings.HasSuffix(u, "=-0") {
			t.Errorf("%q: URL %q has negative zero", raw, u)
		}
		parsed, err := url.Parse(u)
		if err != nil {
			t.Fatal(err)
		}
		round, err := query.ParseBike(parsed.Query())
		if err != nil {
			t.Fatalf("%q: round trip %q: %v", raw, u, err)
		}
		if !reflect.DeepEqual(b, round) {
			t.Errorf("%q: round trip mismatch\n got %+v\nwant %+v", raw, round, b)
		}
	}
	if query.FormatFloat(math.Copysign(0, -1)) != "0" {
		t.Error("FormatFloat(-0) != \"0\"")
	}
}

func TestParseBikeValidationErrors(t *testing.T) {
	for raw, field := range map[string]string{
		"fram=surly-krampus": "fram", "bar_option=all": "bar_option", "stock_bars=all": "stock_bars",
		"stock_forks=all": "stock_forks", "post_ext=200": "post_ext", "post_setback=15": "post_setback", "z=1&a=2": "a",
		"frame=otso-fenrir-stainless&frame=surly-krampus": "frame", "stem_length=60&stem_length=70": "stem_length",
		"frame=nope": "frame", "frame=esker-hayduke&size=L": "size", "frame=surly-ecr&fork=sid_100": "fork",
		"bar=nope": "bar", "post=nope": "post",
		"fork_a2c=abc": "fork_a2c", "fork_a2c=NaN": "fork_a2c", "cup=inf": "cup", "sag=nope": "sag",
		"spacers=x": "spacers", "stem_length=x": "stem_length", "stem_angle=x": "stem_angle", "bar_roll=x": "bar_roll",
		"post_insert=x": "post_insert", "rail_offset=x": "rail_offset", "rail_pitch=x": "rail_pitch",
		"brp_x=-219": "brp_y", "brp_y=713.1": "brp_x", "brp_x=-219&brp_y=": "brp_y", "brp=on": "brp",
		"grip_x=481": "grip_y", "grip=yes": "grip", "brp_x=NaN&brp_y=713.1": "brp_x",
		"bar_options=some": "bar_options", "fork_options=some": "fork_options", "view=top": "view",
	} {
		_, err := query.ParseBike(mustParseQuery(t, raw))
		var ie *query.InputError
		if !errors.As(err, &ie) {
			t.Errorf("%q: err = %v, want *InputError", raw, err)
			continue
		}
		if ie.Field != field {
			t.Errorf("%q: Field = %q, want %q", raw, ie.Field, field)
		}
	}
}

func TestBikeEvaluate(t *testing.T) {
	eval := func(raw string) query.BikeResult {
		t.Helper()
		r, err := mustParseBike(t, raw).Evaluate(query.RankedBuildLimit)
		if err != nil {
			t.Fatalf("Evaluate(%q): %v", raw, err)
		}
		return r
	}

	r := eval("")
	fs := r.Placement.Frameset
	near(t, "reach", fs.LeveledReachMm(), 419.815874, 1e-5)
	near(t, "stack", fs.LeveledStackMm(), 647.381365, 1e-5)
	near(t, "ett", fs.EttMm(), 599.074926, 1e-5)
	near(t, "wheelbase", fs.WheelbaseMm(), 1152.913834, 1e-5)
	brp := r.Placement.Saddle.BRP.Origin()
	near(t, "brp x", brp.X, -157.024907, 1e-5)
	near(t, "brp y", brp.Y, 682.682197, 1e-5)
	if r.Score == nil {
		t.Fatal("default Score nil")
	}
	if r.Score.Tier != assembly.FitsNeither {
		t.Errorf("tier = %v", r.Score.Tier)
	}
	near(t, "score", r.Score.Total, 53.639215, 1e-5)
	near(t, "gripR", r.Score.GripR, 9.736876, 1e-5)
	near(t, "brp miss x", r.Score.BrpMissXMm, -61.975093, 1e-5)
	near(t, "brp miss y", r.Score.BrpMissYMm, 30.417803, 1e-5)
	if r.Fitting == nil {
		t.Fatal("Fitting nil")
	}
	fsu := r.Fitting.Placement.Setup
	near(t, "fit insertion", fsu.Seatpost.InsertionMm, 143.437621, 1e-5)
	if fsu.Saddle.RailOffsetMm != 15 || fsu.Stem.SpacersMm != 0 || fsu.Stem.LengthMm != 130 {
		t.Errorf("fit setup = rail %v spacers %v stem %v", fsu.Saddle.RailOffsetMm, fsu.Stem.SpacersMm, fsu.Stem.LengthMm)
	}
	if r.SetbackCentered == nil || r.SetbackReach == nil {
		t.Fatal("setback fittings nil")
	}
	near(t, "centered setback", r.SetbackCentered.Placement.Setup.Seatpost.Spec.SetbackMm, 63.610422, 1e-5)
	near(t, "centered insertion", r.SetbackCentered.Placement.Setup.Seatpost.InsertionMm, 129.146763, 1e-5)
	near(t, "reach setback", r.SetbackReach.Placement.Setup.Seatpost.Spec.SetbackMm, 49.154381, 1e-5)
	near(t, "reach insertion", r.SetbackReach.Placement.Setup.Seatpost.InsertionMm, 133.149622, 1e-5)
	if r.Builds == nil || r.Builds.Evaluated != 749 || len(r.Builds.Builds) != 5 {
		t.Fatalf("builds = %+v", r.Builds)
	}
	top := r.Builds.Builds[0]
	st := top.Placement.Setup.Stem
	if top.Score.Tier != assembly.FitsNeither || top.HandlebarID != "persuader20_800" || top.ForkID != "lithic_carbon_mountain-495x44" ||
		st.LengthMm != 120 || st.AngleDeg != -30 || st.SpacersMm != 0 {
		t.Errorf("top build = %v %s %s stem %+v", top.Score.Tier, top.HandlebarID, top.ForkID, st)
	}
	near(t, "top total", top.Score.Total, 54.271845, 1e-5)
	near(t, "top gripR", top.Score.GripR, 1.897639, 1e-5)

	r = eval("stem_length=65&stem_angle=-6")
	if r.Score != nil || r.Fitting == nil || r.Builds == nil {
		t.Errorf("off-catalog stem: score %v fitting %v builds %v", r.Score, r.Fitting, r.Builds)
	}

	r = eval("brp=off")
	if r.Score == nil || r.Score.Tier != assembly.FitsNeither || r.Score.HasBrp || r.SetbackCentered != nil || r.SetbackReach != nil {
		t.Fatalf("brp off = %+v", r)
	}
	near(t, "brp off total", r.Score.Total, 70.913554, 1e-5)
	near(t, "brp off top total", r.Builds.Builds[0].Score.Total, 54.512454, 1e-5)

	r = eval("grip=off")
	if r.Score != nil || r.Builds != nil || r.Fitting == nil || r.SetbackCentered == nil {
		t.Errorf("grip off = %+v", r)
	}

	r = eval("brp=off&grip=off")
	if r.Score != nil || r.Fitting != nil || r.SetbackCentered != nil || r.SetbackReach != nil || r.Builds != nil {
		t.Errorf("both off = %+v", r)
	}

	r = eval("frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20&post=redshift_shockstop_pro&post_insert=163.13014276565696&rail_offset=15&rail_pitch=0&bar=persuader20_800&bar_roll=0&stem_length=110&stem_angle=-7&spacers=0")
	if r.Score == nil || r.Score.Tier != assembly.FitsAll {
		t.Fatalf("krampus score = %+v", r.Score)
	}
	near(t, "krampus total", r.Score.Total, 84.690417237, 1e-6)
	near(t, "krampus gripR", r.Score.GripR, 0.725092414, 1e-6)
	near(t, "krampus miss x", r.Score.BrpMissXMm, -1.764808833, 1e-6)
	near(t, "krampus miss y", r.Score.BrpMissYMm, 0, 1e-6)
	near(t, "krampus top", r.Builds.Builds[0].Score.Total, r.Score.Total, 1e-9)
	if r.Builds.Builds[0].Score.Tier != assembly.FitsAll {
		t.Errorf("krampus top tier = %v", r.Builds.Builds[0].Score.Tier)
	}

	r = eval("fork_options=all")
	if r.Builds.Evaluated != 8988 {
		t.Errorf("all forks evaluated = %d, want 8988", r.Builds.Evaluated)
	}
	top = r.Builds.Builds[0]
	if top.ForkID != "sid_100-506x51" || top.Placement.Setup.Fork.SagPct != 20 {
		t.Errorf("all forks top = %s sag %v", top.ForkID, top.Placement.Setup.Fork.SagPct)
	}
	near(t, "all forks total", top.Score.Total, 60.642370, 1e-5)
	near(t, "all forks gripR", top.Score.GripR, 1.446100, 1e-4)

	_, err := mustParseBike(t, "post=redshift_shockstop_pro&post_insert=275").Evaluate(query.RankedBuildLimit)
	var ie *assembly.InputError
	if !errors.As(err, &ie) || ie.Field != assembly.InputSeatpostInsertion {
		t.Errorf("out-of-bounds Evaluate err = %v, want InputError seatpost insertion", err)
	}
}

func TestBasisFrameset(t *testing.T) {
	fs, err := query.DefaultBike().BasisFrameset()
	if err != nil {
		t.Fatalf("BasisFrameset: %v", err)
	}
	near(t, "default basis pitch", float64(fs.PitchDeg()), -0.010692172, 1e-9)
}
