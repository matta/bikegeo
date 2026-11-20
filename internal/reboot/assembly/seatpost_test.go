package assembly_test

import (
	"errors"
	"math"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

func redshiftPostSpec() assembly.SeatpostSpec {
	return assembly.SeatpostSpec{TotalLengthMm: 350, MaxInsertionMm: 250, MinInsertionMm: 100, SetbackMm: 12, RailClampLengthMm: 30}
}

func TestSeatpostInsertionBounds(t *testing.T) {
	frame := fenrirSizeLFrame() // declares the published 290 mm cap
	uncapped := fenrirSizeLFrame()
	uncapped.MaxPostInsertionMm = 0
	uncapped.SeatTubeLengthMm = 260
	uncapped.MinFrameInsertionMm = 125

	for _, tc := range []struct {
		name     string
		frame    assembly.FrameSetup
		post     assembly.SeatpostSpec
		min, max float64
	}{
		{"Fenrir with the generic post", frame, genericPostSpec(0), 100, 290},
		{"Fenrir with the Redshift", frame, redshiftPostSpec(), 100, 250},
		{"260 mm C-T, no frame cap, frame minimum 125", uncapped, genericPostSpec(0), 125, 240},
	} {
		got, err := assembly.SeatpostInsertionBounds(tc.frame, tc.post)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		want := assembly.InsertionBounds{EffectiveMinInsertionMm: tc.min, MaxAllowedInsertionMm: tc.max}
		if got != want {
			t.Errorf("%s: bounds = %+v, want %+v", tc.name, got, want)
		}
	}

	// Equal endpoints permit exactly one installation.
	pinned := frame
	pinned.MinFrameInsertionMm = 290
	got, err := assembly.SeatpostInsertionBounds(pinned, genericPostSpec(0))
	if err != nil || got.EffectiveMinInsertionMm != 290 || got.MaxAllowedInsertionMm != 290 {
		t.Errorf("pinned bounds = %+v, %v; want [290, 290]", got, err)
	}

	for _, tc := range []struct {
		name  string
		frame func(*assembly.FrameSetup)
		post  func(*assembly.SeatpostSpec)
	}{
		{"frame minimum above the cap", func(f *assembly.FrameSetup) { f.MinFrameInsertionMm = 291 }, nil},
		{"negative frame minimum", func(f *assembly.FrameSetup) { f.MinFrameInsertionMm = -1 }, nil},
		{"negative frame cap", func(f *assembly.FrameSetup) { f.MaxPostInsertionMm = -1 }, nil},
		{"NaN frame cap", func(f *assembly.FrameSetup) { f.MaxPostInsertionMm = math.NaN() }, nil},
		{"zero C-T length", func(f *assembly.FrameSetup) { f.SeatTubeLengthMm = 0 }, nil},
		{"C-T length too short for the fallback cap", func(f *assembly.FrameSetup) { f.MaxPostInsertionMm, f.SeatTubeLengthMm = 0, 110 }, nil},
		{"NaN total length", nil, func(p *assembly.SeatpostSpec) { p.TotalLengthMm = math.NaN() }},
		{"infinite maximum insertion", nil, func(p *assembly.SeatpostSpec) { p.MaxInsertionMm = math.Inf(1) }},
		{"maximum insertion equals total length", nil, func(p *assembly.SeatpostSpec) { p.MaxInsertionMm = p.TotalLengthMm }},
		{"zero minimum insertion", nil, func(p *assembly.SeatpostSpec) { p.MinInsertionMm = 0 }},
		{"minimum above maximum", nil, func(p *assembly.SeatpostSpec) { p.MinInsertionMm = p.MaxInsertionMm + 1 }},
		{"negative setback", nil, func(p *assembly.SeatpostSpec) { p.SetbackMm = -1 }},
		{"zero clamp length", nil, func(p *assembly.SeatpostSpec) { p.RailClampLengthMm = 0 }},
	} {
		f, p := frame, genericPostSpec(0)
		if tc.frame != nil {
			tc.frame(&f)
		}
		if tc.post != nil {
			tc.post(&p)
		}
		if _, err := assembly.SeatpostInsertionBounds(f, p); err == nil {
			t.Errorf("%s: want error", tc.name)
		}
	}

	var inputErr *assembly.InputError
	badSetback := genericPostSpec(-1)
	if _, err := assembly.SeatpostInsertionBounds(frame, badSetback); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputSeatpostSetback {
		t.Errorf("negative setback error = %v, want InputSeatpostSetback", err)
	}
}

func TestPlaceSeatpostInsertionEndpoints(t *testing.T) {
	top := pose.NewPose(pose.Point{}, 105)
	frame := fenrirSizeLFrame()
	post := assembly.SeatpostSetup{Spec: genericPostSpec(0)}
	bounds, err := assembly.SeatpostInsertionBounds(frame, post.Spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		insertion, exposed float64
		ok                 bool
	}{
		{100, 300, true},
		{290, 110, true},
		{99.999, 0, false},
		{290.001, 0, false},
		{math.NaN(), 0, false},
		{math.Inf(1), 0, false},
	} {
		post.InsertionMm = tc.insertion
		placed, err := assembly.PlaceSeatpost(top, post, bounds)
		if tc.ok {
			if err != nil {
				t.Errorf("insertion %v: %v", tc.insertion, err)
				continue
			}
			assertNear(t, "exposed length", top.DistanceTo(placed.ShaftTop), tc.exposed, 1e-9)
			continue
		}
		var inputErr *assembly.InputError
		if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputSeatpostInsertion {
			t.Errorf("insertion %v: error = %v, want InputSeatpostInsertion", tc.insertion, err)
		}
	}
}

func TestRailOffsetLimitUsesPostClampLength(t *testing.T) {
	setup := fenrirFittingSetup() // 60 mm rails, 30 mm clamp, level
	limit, err := setup.RailOffsetLimitMm()
	if err != nil || limit != 15 {
		t.Fatalf("limit = %v, %v; want 15", limit, err)
	}
	setup.Seatpost.Spec.RailClampLengthMm = 70
	if _, err := setup.RailOffsetLimitMm(); err == nil {
		t.Error("clamp longer than the rails: want error")
	}
}

func TestFittingProbesALegalPostEvenWhenSubmittedInsertionIsIllegal(t *testing.T) {
	setup := fenrirFittingSetup()
	setup.Seatpost.InsertionMm = 999 // not installable, but a BRP fit chooses its own insertion
	fit, err := assembly.ApplyFitting(setup, assembly.FitTarget{BRP: &fitReportBRP})
	if err != nil {
		t.Fatalf("ApplyFitting: %v", err)
	}
	if got := fit.Placement.Setup.Seatpost.InsertionMm; got < 100 || got > 290 {
		t.Errorf("fitted insertion = %v, want within [100, 290]", got)
	}
}
