package geometry_test

import (
	"math"
	"testing"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

func seatAxisTestBike() model.BikeGeometry {
	return model.BikeGeometry{
		ID: "test-m", Manufacturer: "Test", Model: "Bike", Size: "M",
		Reach: 398.0, Stack: 656.0, HtaDeg: 69.0, StaDeg: 74.0,
		Htl: 150.0, BbDrop: 70.0, ForkRake: 44.0, ForkA2c: 521.0,
		HeadsetCupType: model.CupEC44, GeoHeadsetSpacerMm: 12.0, Wheelbase: 1150.0,
	}
}

func TestCalculateEttMatchesIndustryDefinition(t *testing.T) {
	reach, stack, hta, sta := 398.0, 656.0, 69.0, 74.0
	want := reach + stack/math.Tan(sta*math.Pi/180.0)
	got := geometry.CalculateEtt(reach, stack, model.Deg(hta), model.Deg(sta))
	if math.Abs(got-want) > 0.01 {
		t.Errorf("CalculateEtt = %.3f, want reach + stack/tan(STA) = %.3f", got, want)
	}
}

func TestSeatAxisMetricsIndependentOfSaddle(t *testing.T) {
	bike := seatAxisTestBike()
	profile := catalog.DefaultRiderProfile()
	var first *model.BikeFitAnalysis
	brps := map[float64]bool{}
	for _, s := range catalog.CommonSaddleCatalog {
		fit, err := geometry.AnalyzeBikeFitWithSaddle(bike, model.Opt[float64]{}, model.Opt[float64]{}, profile, s)
		if err != nil {
			t.Fatalf("%s: %v", s.ID, err)
		}
		brps[fit.DeadCenterBRP.BrpSetbackMm] = true
		if first == nil {
			f := fit
			first = &f
			continue
		}
		if fit.Aett != first.Aett || fit.Ett != first.Ett || fit.ActualSetback != first.ActualSetback ||
			fit.SpareVerticalStack != first.SpareVerticalStack || fit.SeatToRearCenter != first.SeatToRearCenter {
			t.Errorf("%s: seat-axis metrics changed with saddle: AETT %.1f/%.1f ETT %.1f/%.1f setback %.1f/%.1f",
				s.ID, fit.Aett, first.Aett, fit.Ett, first.Ett, fit.ActualSetback, first.ActualSetback)
		}
	}
	if len(brps) < 2 {
		t.Errorf("expected BRP setback to differ between saddles, got %v", brps)
	}
}
