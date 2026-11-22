package catalog_test

import (
	"slices"
	"strings"
	"testing"

	legacyframes "bikegeo/internal/frames"
	"bikegeo/internal/geometry"
	"bikegeo/internal/model"
	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
	"bikegeo/internal/reboot/pose"
)

// legacyHeadset maps a legacy HeadsetCupType to its bore and physical lower
// cup (Q66; stack is legacy CupStackMm).
func legacyHeadset(t *testing.T, c model.HeadsetCupType) (assembly.LowerBore, assembly.LowerCupSetup) {
	t.Helper()
	var bore assembly.LowerBore
	var std assembly.LowerCupStandard
	switch c {
	case model.CupEC34:
		bore, std = assembly.Bore34Pressed, assembly.LowerCupEC34
	case model.CupEC44:
		bore, std = assembly.Bore44Pressed, assembly.LowerCupEC44
	case model.CupZS56:
		bore, std = assembly.Bore56Pressed, assembly.LowerCupZS56
	case model.CupIS52:
		bore, std = assembly.Bore52Integrated, assembly.LowerCupIS52
	case model.CupZS44, model.CupIS42:
		t.Fatalf("no legacy frame uses %s", c)
	default:
		t.Fatalf("unknown legacy cup %q", c)
	}
	return bore, assembly.LowerCupSetup{Standard: std, StackMm: c.CupStackMm()}
}

func legacyFramesByID() map[string]model.FrameModel {
	out := map[string]model.FrameModel{}
	for _, f := range legacyframes.AllFrames() {
		out[f.ID] = f
	}
	return out
}

// missingLegacyFrames returns, in legacy order, the IDs of legacy frames that
// frames lacks (Q119). A catalog-only frame is not an error.
func missingLegacyFrames(legacy []model.FrameModel, frames []catalog.Frame) []string {
	have := map[string]bool{}
	for _, f := range frames {
		have[f.ID] = true
	}
	var missing []string
	for _, l := range legacy {
		if !have[l.ID] {
			missing = append(missing, l.ID)
		}
	}
	return missing
}

// TestMissingLegacyFrames checks that missingLegacyFrames catches dropped legacy
// frames while allowing catalog-only frames (Q119).
func TestMissingLegacyFrames(t *testing.T) {
	legacy := legacyframes.AllFrames()
	base := catalog.Frames()

	dropped := slices.DeleteFunc(slices.Clone(base), func(f catalog.Frame) bool {
		return f.ID == "otso-fenrir-stainless"
	})
	withExtra := append(slices.Clone(base), catalog.Frame{ID: "scratch-new-frame"})

	for _, tc := range []struct {
		name   string
		frames []catalog.Frame
		want   []string
	}{
		{"identical", base, nil},
		{"catalog-only frame", withExtra, nil},
		{"dropped frame", dropped, []string{"otso-fenrir-stainless"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := missingLegacyFrames(legacy, tc.frames)
			if !slices.Equal(got, tc.want) {
				t.Errorf("missingLegacyFrames = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestFramesMatchLegacy pins every legacy frame value in the catalog, skipping
// catalog-only frames (Q119).
func TestFramesMatchLegacy(t *testing.T) {
	if missing := missingLegacyFrames(legacyframes.AllFrames(), catalog.Frames()); len(missing) > 0 {
		t.Fatalf("legacy frames missing from the catalog: %v", missing)
	}
	modelOverride := map[string]string{"otso-fenrir-stainless": "Fenrir Stainless"}
	allowanceSizes := map[string]bool{"sklar-pbj/L": true, "esker-hayduke/S4": true}
	legacy := legacyFramesByID()
	for _, f := range catalog.Frames() {
		t.Run(f.ID, func(t *testing.T) {
			l, ok := legacy[f.ID]
			if !ok {
				t.Skipf("catalog-only frame (Q119)")
			}
			wantModel := l.Model
			if m, ok := modelOverride[f.ID]; ok {
				wantModel = m
			}
			if f.Manufacturer != l.Manufacturer || f.Model != wantModel || f.Year != l.Year || f.DefaultSize != l.DefaultSize {
				t.Errorf("identity = %q %q %d default %q, want %q %q %d default %q",
					f.Manufacturer, f.Model, f.Year, f.DefaultSize, l.Manufacturer, wantModel, l.Year, l.DefaultSize)
			}
			var names []string
			for _, s := range f.Sizes {
				names = append(names, s.Name)
			}
			if !slices.Equal(names, l.Sizes) {
				t.Errorf("sizes = %v, want %v", names, l.Sizes)
			}
			wantMaxCs := 0.0
			if len(l.ChainstayRangeMm) == 2 {
				wantMaxCs = l.ChainstayRangeMm[1]
			}
			if f.MaxChainstayMm != wantMaxCs {
				t.Errorf("MaxChainstayMm = %v, want %v", f.MaxChainstayMm, wantMaxCs)
			}
			bore, cup := legacyHeadset(t, l.HeadsetCupType)
			basisCup := assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube}
			if l.GeoHeadsetSpacerMm != 0 {
				basisCup = assembly.LowerCupSetup{Standard: cup.Standard, StackMm: l.GeoHeadsetSpacerMm}
			}
			basisFork := assembly.ForkSpec{
				StaticA2cMm: l.CadFork.A2cMm,
				RakeMm:      l.CadFork.RakeMm,
				Steerer:     assembly.SteererBasis,
			}
			if f.ID == "ritchey-ultra" {
				basisFork.StaticA2cMm = 508.5 // Q71
			}
			if f.BasisLowerCup != basisCup {
				t.Errorf("BasisLowerCup = %+v, want %+v", f.BasisLowerCup, basisCup)
			}
			if f.BasisFork != basisFork {
				t.Errorf("BasisFork = %+v, want %+v", f.BasisFork, basisFork)
			}
			collar := 0.0
			if strings.HasPrefix(f.ID, "seven-") {
				collar = 35 // Q75d
			}
			for _, s := range f.Sizes {
				g, ok := l.Geometries[s.Name]
				if !ok {
					t.Errorf("legacy has no geometry for %q", s.Name)
					continue
				}
				fr := s.ChartFrame
				for _, col := range []struct {
					name      string
					got, want float64
				}{
					{"Reach", fr.ReachMm, g.Reach},
					{"Stack", fr.StackMm, g.Stack},
					{"HTA", float64(fr.HeadTubeAngleDeg), float64(g.HtaDeg)},
					{"STA", float64(fr.SeatTubeAngleDeg), float64(g.StaDeg)},
					{"HTL", fr.HeadTubeLengthMm, g.Htl},
					{"BB drop", fr.BbDropMm, g.BbDrop},
					{"Chainstay", fr.ChainstayMm, g.ChainstayLength},
					{"Wheelbase", s.PublishedWheelbaseMm, g.Wheelbase},
					{"Seat tube", fr.SeatTubeLengthMm, g.SeatTubeLength + collar},
				} {
					if col.got != col.want {
						t.Errorf("%s %s = %v, legacy %v", s.Name, col.name, col.got, col.want)
					}
				}
				if fr.LowerBore != bore || fr.DefaultLowerCup.Standard != cup.Standard {
					t.Errorf("%s headset = %s %s, want %s %s (stacks follow Q136)", s.Name, fr.LowerBore, fr.DefaultLowerCup.Standard, bore, cup.Standard)
				}
				wantInsertion := 0.0
				if f.ID == "otso-fenrir-stainless" && s.Name == "L" {
					wantInsertion = 290 // Q95
				}
				if fr.MaxPostInsertionMm != wantInsertion || fr.MinFrameInsertionMm != 0 {
					t.Errorf("%s insertion max/min = %v/%v, want %v/0", s.Name, fr.MaxPostInsertionMm, fr.MinFrameInsertionMm, wantInsertion)
				}
				if has := s.Allowance != (catalog.ChartAllowance{}); has != allowanceSizes[f.ID+"/"+s.Name] {
					t.Errorf("%s allowance = %+v", s.Name, s.Allowance)
				}
			}
		})
	}
}

// TestPrecisionMatchesLegacyInference pins each ported frame's precision to
// legacy's inference, skipping catalog-only frames (Q80, Q119).
func TestPrecisionMatchesLegacyInference(t *testing.T) {
	legacy := legacyFramesByID()
	for _, f := range catalog.Frames() {
		l, ok := legacy[f.ID]
		if !ok {
			continue
		}
		p := geometry.InferChartPrecision(l)
		want := catalog.ChartPrecision{
			ReachMm:          p.ReachMm,
			StackMm:          p.StackMm,
			HeadTubeAngleDeg: pose.Deg(p.HtaDeg),
			HeadTubeLengthMm: p.HtlMm,
			BbDropMm:         p.BbDropMm,
			ChainstayMm:      p.ChainstayMm,
			WheelbaseMm:      p.WheelbaseMm,
		}
		if f.ChartPrecision != want {
			t.Errorf("%s precision = %+v, legacy inference %+v", f.ID, f.ChartPrecision, want)
		}
	}
}
