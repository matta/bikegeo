package assembly_test

import (
	"errors"
	"math"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
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
	top := pose.NewPose(pose.Origin(assembly.DynamicWorld), 105)
	frame := fenrirSizeLFrame()
	post := assembly.SeatpostSetup{Spec: genericPostSpec(0)}
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
		placed, err := assembly.PlaceSeatpost(top, frame, post)
		if tc.ok {
			if err != nil {
				t.Errorf("insertion %v: %v", tc.insertion, err)
				continue
			}
			assertNear(t, "exposed length", top.Origin().DistanceTo(placed.ShaftTop.Origin()), tc.exposed, 1e-9)
			continue
		}
		var inputErr *assembly.InputError
		if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputSeatpostInsertion {
			t.Errorf("insertion %v: error = %v, want InputSeatpostInsertion", tc.insertion, err)
		}
	}
}

// PlaceSeatpost derives its insertion limits from the frame, so the same
// post and insertion can fit one frame and not another.
func TestPlaceSeatpostUsesFrameInsertionLimits(t *testing.T) {
	top := pose.NewPose(pose.Origin(assembly.DynamicWorld), 105)
	uncapped := fenrirSizeLFrame()
	uncapped.MaxPostInsertionMm = 0 // 490 − 20 = 470 mm cap, so the post's 365 mm binds
	withMinimum := fenrirSizeLFrame()
	withMinimum.MinFrameInsertionMm = 125
	noSeatTube := fenrirSizeLFrame()
	noSeatTube.SeatTubeLengthMm = 0
	for _, tc := range []struct {
		name      string
		frame     assembly.FrameSetup
		insertion float64
		ok        bool
		field     assembly.InputField
	}{
		{"300 mm within the post's 365 mm on an uncapped frame", uncapped, 300, true, ""},
		{"365 mm at the post maximum on an uncapped frame", uncapped, 365, true, ""},
		{"365.001 mm past the post maximum", uncapped, 365.001, false, assembly.InputSeatpostInsertion},
		{"300 mm past the Fenrir's 290 mm cap", fenrirSizeLFrame(), 300, false, assembly.InputSeatpostInsertion},
		{"110 mm below a 125 mm frame minimum", withMinimum, 110, false, assembly.InputSeatpostInsertion},
		{"125 mm at the frame minimum", withMinimum, 125, true, ""},
		{"invalid frame seat tube", noSeatTube, 200, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			post := assembly.SeatpostSetup{Spec: genericPostSpec(0), InsertionMm: tc.insertion}
			_, err := assembly.PlaceSeatpost(top, tc.frame, post)
			if tc.ok {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("want error")
			}
			var inputErr *assembly.InputError
			if tc.field != "" && (!errors.As(err, &inputErr) || inputErr.Field != tc.field) {
				t.Errorf("error = %v, want %s", err, tc.field)
			}
		})
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

// Isolated Seatpost tests below construct no bicycle frame or ambient world.

func TestRigidPostIsolated(t *testing.T) {
	post, err := assembly.NewRigidPost(400, 100, 365, 15, 30)
	if err != nil {
		t.Fatalf("NewRigidPost: %v", err)
	}
	if post.TotalLengthMm() != 400 || post.SetbackMm() != 15 || post.RailClampLengthMm() != 30 {
		t.Errorf("unexpected rigid post dimensions: total=%v setback=%v clamp=%v",
			post.TotalLengthMm(), post.SetbackMm(), post.RailClampLengthMm())
	}
	limits := post.InsertionLimits()
	if limits.Min() != 100 || limits.Max() != 365 ||
		limits.MinSource() != "seatpost min insertion" || limits.MaxSource() != "seatpost max insertion" {
		t.Errorf("unexpected limits: %+v", limits)
	}

	for _, state := range []assembly.LoadState{assembly.Static, assembly.Dynamic} {
		clamp := post.RailClamp(state)
		if clamp.ClampLengthMm != 30 {
			t.Errorf("%s clamp length = %v, want 30", state, clamp.ClampLengthMm)
		}
		if clamp.CradleInParent.Src() != assembly.SeatpostCradle || clamp.CradleInParent.Dst() != assembly.SeatpostLocal {
			t.Errorf("%s transform systems = %s -> %s", state, clamp.CradleInParent.Src(), clamp.CradleInParent.Dst())
		}
		origin := clamp.CradleInParent.OriginInDst()
		assertNear(t, string(state)+" X", origin.X(), 400, 1e-12)
		assertNear(t, string(state)+" Y", origin.Y(), 15, 1e-12)
		assertNear(t, string(state)+" heading", float64(clamp.CradleInParent.HeadingInDst().Deg()), 0, 1e-12)
	}

	var inputErr *assembly.InputError
	if _, err := assembly.NewRigidPost(400, 100, 365, -1, 30); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputSeatpostSetback {
		t.Errorf("negative setback error = %v, want InputSeatpostSetback", err)
	}
	for _, tc := range []struct {
		name                           string
		total, minI, maxI, setb, clamp float64
	}{
		{"NaN total", math.NaN(), 100, 365, 0, 30},
		{"Inf max", 400, 100, math.Inf(1), 0, 30},
		{"zero min", 400, 0, 365, 0, 30},
		{"min > max", 400, 366, 365, 0, 30},
		{"max == total", 400, 100, 400, 0, 30},
		{"NaN setback", 400, 100, 365, math.NaN(), 30},
		{"zero clamp", 400, 100, 365, 0, 0},
	} {
		if _, err := assembly.NewRigidPost(tc.total, tc.minI, tc.maxI, tc.setb, tc.clamp); err == nil {
			t.Errorf("%s: want error", tc.name)
		}
	}

	var zero assembly.RigidPost
	assertPanics(t, "zero RigidPost RailClamp", func() { _ = zero.RailClamp(assembly.Static) })
	assertPanics(t, "RigidPost invalid LoadState", func() { _ = post.RailClamp(assembly.LoadState("")) })
}

func TestLinkagePostStaticVsDynamic(t *testing.T) {
	sag := pose.NewVector(assembly.SeatpostLocal, -6, 5)
	post, err := assembly.NewLinkagePost(350, 100, 250, 7, 30, sag)
	if err != nil {
		t.Fatalf("NewLinkagePost: %v", err)
	}
	if post.TotalLengthMm() != 350 || post.StaticSetbackMm() != 7 || post.RailClampLengthMm() != 30 {
		t.Errorf("unexpected linkage post dimensions")
	}
	if post.SagStep().X() != -6 || post.SagStep().Y() != 5 {
		t.Errorf("SagStep = (%v, %v), want (-6, 5)", post.SagStep().X(), post.SagStep().Y())
	}

	staticClamp := post.RailClamp(assembly.Static)
	dynamicClamp := post.RailClamp(assembly.Dynamic)

	staticOrigin := staticClamp.CradleInParent.OriginInDst()
	dynamicOrigin := dynamicClamp.CradleInParent.OriginInDst()
	assertNear(t, "static X", staticOrigin.X(), 350, 1e-12)
	assertNear(t, "static Y", staticOrigin.Y(), 7, 1e-12)
	assertNear(t, "static heading", float64(staticClamp.CradleInParent.HeadingInDst().Deg()), 0, 1e-12)

	assertNear(t, "dynamic X", dynamicOrigin.X(), 344, 1e-12)
	assertNear(t, "dynamic Y", dynamicOrigin.Y(), 12, 1e-12)
	assertNear(t, "dynamic heading", float64(dynamicClamp.CradleInParent.HeadingInDst().Deg()), 0, 1e-12)

	delta := dynamicOrigin.Sub(staticOrigin)
	assertNear(t, "delta X", delta.X(), -6, 1e-12)
	assertNear(t, "delta Y", delta.Y(), 5, 1e-12)

	// Non-finite or past-shaft-bottom sag is rejected.
	if _, err := assembly.NewLinkagePost(350, 100, 250, 7, 30, pose.NewVector(assembly.SeatpostLocal, math.NaN(), 5)); err == nil {
		t.Error("NaN sagStep: want error")
	}
	if _, err := assembly.NewLinkagePost(350, 100, 250, 7, 30, pose.NewVector(assembly.SeatpostLocal, -350, 5)); err == nil {
		t.Error("sagStep reaching shaft bottom: want error")
	}

	// Wrong CoordinateSystem or zero Vector panics deterministically.
	assertPanics(t, "wrong CoordinateSystem sagStep", func() {
		_, _ = assembly.NewLinkagePost(350, 100, 250, 7, 30, pose.NewVector(assembly.FrameLocal, -6, 5))
	})
	assertPanics(t, "zero sagStep", func() {
		_, _ = assembly.NewLinkagePost(350, 100, 250, 7, 30, pose.Vector{})
	})
	assertPanics(t, "LinkagePost invalid LoadState", func() {
		_ = post.RailClamp(assembly.LoadState("bad"))
	})
}

func TestMateSeatpostAndClampedSeatpostIsolated(t *testing.T) {
	collarInFrame := pose.NewPose(pose.NewPoint(assembly.FrameLocal, -120, 470), 105).Anchor(assembly.SeatTubeCollar)
	frameRange, err := assembly.NewBoundedRange(125, 290, "frame min insertion", "frame max insertion")
	if err != nil {
		t.Fatalf("frameRange: %v", err)
	}
	socket := assembly.SeatTubeSocket{
		CollarInFrame:    collarInFrame,
		AllowedInsertion: frameRange,
	}

	// LinkagePost [100, 250] intersected with frame [125, 290] => [125, 250].
	sag := pose.NewVector(assembly.SeatpostLocal, -6, 5)
	linkage, err := assembly.NewLinkagePost(350, 100, 250, 7, 30, sag)
	if err != nil {
		t.Fatalf("NewLinkagePost: %v", err)
	}
	dof, err := assembly.MateSeatpost(socket, linkage)
	if err != nil {
		t.Fatalf("MateSeatpost: %v", err)
	}
	allowed := dof.AllowedInsertion()
	if allowed.Min() != 125 || allowed.Max() != 250 ||
		allowed.MinSource() != "frame min insertion" || allowed.MaxSource() != "seatpost max insertion" {
		t.Errorf("allowed = %+v", allowed)
	}

	// Incompatible socket and post fail immediately at MateSeatpost time.
	shortRange, _ := assembly.NewBoundedRange(0, 90, "frame min insertion", "frame seat tube clearance")
	shortSocket := assembly.SeatTubeSocket{CollarInFrame: collarInFrame, AllowedInsertion: shortRange}
	if _, err := assembly.MateSeatpost(shortSocket, linkage); err == nil {
		t.Error("incompatible socket/post should fail at MateSeatpost time")
	}

	// Nil post or uninitialized socket range returns error; wrong CollarInFrame panics.
	if _, err := assembly.MateSeatpost(socket, nil); err == nil {
		t.Error("nil post: want error")
	}
	if _, err := assembly.MateSeatpost(assembly.SeatTubeSocket{CollarInFrame: collarInFrame}, linkage); err == nil {
		t.Error("uninitialized AllowedInsertion: want error")
	}
	assertPanics(t, "wrong CollarInFrame Src", func() {
		bad := pose.NewPose(pose.NewPoint(assembly.FrameLocal, 0, 0), 105).Anchor(assembly.SteererLocal)
		_, _ = assembly.MateSeatpost(assembly.SeatTubeSocket{CollarInFrame: bad, AllowedInsertion: frameRange}, linkage)
	})
	assertPanics(t, "zero CollarInFrame", func() {
		_, _ = assembly.MateSeatpost(assembly.SeatTubeSocket{AllowedInsertion: frameRange}, linkage)
	})

	// Clamp validates insertion bounds and produces BoundStatus margins.
	var inputErr *assembly.InputError
	if _, err := dof.Clamp(124.9); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputSeatpostInsertion {
		t.Errorf("below min Clamp = %v, want InputSeatpostInsertion", err)
	}
	if _, err := dof.Clamp(250.1); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputSeatpostInsertion {
		t.Errorf("above max Clamp = %v, want InputSeatpostInsertion", err)
	}

	clamped, err := dof.Clamp(150)
	if err != nil {
		t.Fatalf("Clamp(150): %v", err)
	}
	ins := clamped.Insertion()
	if ins.Field != assembly.InputSeatpostInsertion || ins.Value != 150 ||
		ins.Min != 125 || ins.Max != 250 ||
		ins.MinSource != "frame min insertion" || ins.MaxSource != "seatpost max insertion" {
		t.Errorf("Insertion BoundStatus = %+v", ins)
	}
	assertNear(t, "MarginMm", ins.MarginMm(), 25, 1e-12)
	if !ins.WithinMargin(30) || ins.WithinMargin(20) {
		t.Error("WithinMargin check failed for 25 mm margin")
	}
	assertNear(t, "ExposedMm", clamped.ExposedMm(), 200, 1e-12)

	// Shaft point X = 150 mm maps onto the collar origin (-120, 470) in FrameLocal.
	atCollar := clamped.PostInFrame().ApplyPoint(pose.NewPoint(assembly.SeatpostLocal, 150, 0))
	assertNear(t, "collar X", atCollar.X(), -120, 1e-12)
	assertNear(t, "collar Y", atCollar.Y(), 470, 1e-12)

	// RailClampInFrame shifts by sagStep rotated into FrameLocal between Static and Dynamic.
	staticFrame := clamped.RailClampInFrame(assembly.Static)
	dynamicFrame := clamped.RailClampInFrame(assembly.Dynamic)
	if staticFrame.CradleInParent.Src() != assembly.SeatpostCradle || staticFrame.CradleInParent.Dst() != assembly.FrameLocal {
		t.Errorf("RailClampInFrame systems = %s -> %s", staticFrame.CradleInParent.Src(), staticFrame.CradleInParent.Dst())
	}
	wantSagInFrame := collarInFrame.ApplyVector(pose.NewVector(assembly.SeatTubeCollar, -6, 5))
	gotSagInFrame := dynamicFrame.CradleInParent.OriginInDst().Sub(staticFrame.CradleInParent.OriginInDst())
	assertNear(t, "frame sag X", gotSagInFrame.X(), wantSagInFrame.X(), 1e-12)
	assertNear(t, "frame sag Y", gotSagInFrame.Y(), wantSagInFrame.Y(), 1e-12)
	assertNear(t, "static heading in frame", float64(staticFrame.CradleInParent.HeadingInDst().Deg()), 105, 1e-12)
	assertNear(t, "dynamic heading in frame", float64(dynamicFrame.CradleInParent.HeadingInDst().Deg()), 105, 1e-12)

	// PlaceInWorld works in any isolated coordinate system without DynamicWorld.
	benchWorld := pose.NewCoordinateSystem("test:bench_world")
	frameToBench := pose.NewTransform(assembly.FrameLocal, benchWorld, pose.Origin(benchWorld), -2.5)
	staticPlaced := clamped.PlaceInWorld(frameToBench, assembly.Static)
	dynamicPlaced := clamped.PlaceInWorld(frameToBench, assembly.Dynamic)

	assertNear(t, "exposed distance", staticPlaced.SeatTubeTop.Origin().DistanceTo(staticPlaced.ShaftTop.Origin()), 200, 1e-12)
	assertNear(t, "shaft top invariant across states X", dynamicPlaced.ShaftTop.Origin().X(), staticPlaced.ShaftTop.Origin().X(), 1e-12)
	assertNear(t, "shaft top invariant across states Y", dynamicPlaced.ShaftTop.Origin().Y(), staticPlaced.ShaftTop.Origin().Y(), 1e-12)
	if staticPlaced.CradleInWorld.Src() != assembly.SeatpostCradle || staticPlaced.CradleInWorld.Dst() != benchWorld {
		t.Errorf("CradleInWorld systems = %s -> %s", staticPlaced.CradleInWorld.Src(), staticPlaced.CradleInWorld.Dst())
	}
	assertNear(t, "cradle matches RailPivot X", dynamicPlaced.CradleInWorld.OriginInDst().X(), dynamicPlaced.RailPivot.Origin().X(), 1e-12)
	assertNear(t, "cradle matches RailPivot Y", dynamicPlaced.CradleInWorld.OriginInDst().Y(), dynamicPlaced.RailPivot.Origin().Y(), 1e-12)
	wantSagInBench := frameToBench.ApplyVector(wantSagInFrame)
	gotSagInBench := dynamicPlaced.RailPivot.Origin().Sub(staticPlaced.RailPivot.Origin())
	assertNear(t, "bench sag X", gotSagInBench.X(), wantSagInBench.X(), 1e-12)
	assertNear(t, "bench sag Y", gotSagInBench.Y(), wantSagInBench.Y(), 1e-12)

	assertPanics(t, "PlaceInWorld wrong frameToWorld Src", func() {
		wrong := pose.NewTransform(assembly.SteererLocal, benchWorld, pose.Origin(benchWorld), 0)
		_ = clamped.PlaceInWorld(wrong, assembly.Static)
	})
}
