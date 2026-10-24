package validator

import (
	"fmt"
	"math"

	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

// SizeResidual captures the CAD geometry residuals and consistency metrics for a single frame size.
type SizeResidual struct {
	Size                 string                 `json:"size"`
	HtaDeg               float64                `json:"hta_deg"`
	Stack                float64                `json:"stack"`
	Reach                float64                `json:"reach"`
	Htl                  float64                `json:"htl"`
	BbDrop               float64                `json:"bb_drop"`
	ForkRake             float64                `json:"fork_rake"`
	ChainstayLength      float64                `json:"chainstay_length,omitempty"`
	ImpliedStructuralA2c float64                `json:"implied_structural_a2c"`
	DeclaredSpacer       float64                `json:"declared_spacer"`
	ImpliedForkA2c       float64                `json:"implied_fork_a2c"`
	StatedStaticForkA2c  float64                `json:"stated_static_fork_a2c"`
	ExpectedForkA2c      float64                `json:"expected_fork_a2c"`
	DeltaA2c             float64                `json:"delta_a2c"` // Expected - Calculated
	DeltaVsBest          float64                `json:"delta_vs_best"`
	FcPub                float64                `json:"fc_pub,omitempty"`
	FcCalc               float64                `json:"fc_calc,omitempty"`
	DeltaFc              *float64               `json:"delta_fc,omitempty"` // Calc - Pub
	WbPub                float64                `json:"wb_pub,omitempty"`
	WbCalc               float64                `json:"wb_calc,omitempty"`
	DeltaWb              *float64               `json:"delta_wb,omitempty"` // Calc - Pub
	ValidationReport     model.ValidationReport `json:"validation_report"`
}

// SizeResiduals calculates the geometric residuals for every size in the specified configuration of a FrameModel.
// If configID is empty, the baseline configuration is used.
func SizeResiduals(frame model.FrameModel, configID string) ([]SizeResidual, error) {
	sizes := frame.GetSizes()
	if len(sizes) == 0 {
		return nil, fmt.Errorf("frame %s has no sizes defined", frame.ID)
	}

	targetCfg := frame.GetConfiguration(configID)
	if targetCfg == nil {
		targetCfg = frame.GetBaselineConfiguration()
	}
	if targetCfg == nil {
		return nil, fmt.Errorf("frame %s has no configuration matching %q", frame.ID, configID)
	}

	var results []SizeResidual

	for _, sz := range sizes {
		bike, err := geometry.MaterializeBikeGeometry(frame, sz, targetCfg.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to materialize size %s: %w", sz, err)
		}

		valReport := ValidateBikeGeometry(bike)

		geoSpacer := bike.GetGeoHeadsetSpacerMm()
		structuralA2c, calculatedForkA2c := geometry.CalculateGeometricA2c(
			bike.Stack,
			bike.BbDrop,
			bike.Htl,
			bike.HtaDeg,
			bike.ForkRake,
			geoSpacer,
		)

		statedStaticA2c := bike.ForkA2c
		if statedStaticA2c <= 0 && bike.ForkTravelMm > 0 {
			statedStaticA2c = bike.ForkTravelMm + 412.0
		}

		sagCompression := 0.0
		if bike.SagPercentagePublished > 0 && bike.ForkTravelMm > 0 {
			sagCompression = bike.ForkTravelMm * (bike.SagPercentagePublished / 100.0)
		}
		expectedForkA2c := geometry.Round(statedStaticA2c-sagCompression, 1)

		deltaA2c := 0.0
		if expectedForkA2c > 0 {
			deltaA2c = geometry.Round(expectedForkA2c-calculatedForkA2c, 1)
		}

		var deltaFc *float64
		fcCalc := valReport.FrontCenterCalc
		if bike.FrontCenter > 0 {
			d := geometry.Round(fcCalc-bike.FrontCenter, 1)
			deltaFc = &d
		}

		var deltaWb *float64
		wbCalc := 0.0
		if bike.Wheelbase > 0 && bike.ChainstayLength > 0 {
			calculatedFcHoriz := geometry.CalculateFrontCenter(
				bike.Reach,
				bike.Htl,
				bike.HtaDeg,
				bike.ForkRake,
				structuralA2c,
			)
			csHorizontal := math.Sqrt(math.Max(0, bike.ChainstayLength*bike.ChainstayLength-bike.BbDrop*bike.BbDrop))
			wbCalc = geometry.Round(csHorizontal+calculatedFcHoriz, 1)
			d := geometry.Round(wbCalc-bike.Wheelbase, 1)
			deltaWb = &d
		}

		res := SizeResidual{
			Size:                 sz,
			HtaDeg:               bike.HtaDeg,
			Stack:                bike.Stack,
			Reach:                bike.Reach,
			Htl:                  bike.Htl,
			BbDrop:               bike.BbDrop,
			ForkRake:             bike.ForkRake,
			ChainstayLength:      bike.ChainstayLength,
			ImpliedStructuralA2c: geometry.Round(structuralA2c, 2),
			DeclaredSpacer:       geoSpacer,
			ImpliedForkA2c:       geometry.Round(calculatedForkA2c, 1),
			StatedStaticForkA2c:  statedStaticA2c,
			ExpectedForkA2c:      expectedForkA2c,
			DeltaA2c:             deltaA2c,
			FcPub:                bike.FrontCenter,
			FcCalc:               fcCalc,
			DeltaFc:              deltaFc,
			WbPub:                bike.Wheelbase,
			WbCalc:               wbCalc,
			DeltaWb:              deltaWb,
			ValidationReport:     valReport,
		}

		results = append(results, res)
	}

	return results, nil
}
