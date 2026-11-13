package validator

import (
	"fmt"
	"math"

	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

// SizeResidual holds the millimeter discrepancies ("residuals") between
// manufacturer-published geometry values for a single frame size and the values
// reconstructed from the frame's kinematic triangle equations and declared CAD
// fork + headset spacer setup:
//   - DeltaA2c (ExpectedForkA2c - ImpliedForkA2c): declared CAD fork A2C minus the
//     fork A2C back-solved from Stack, BBDrop, HTL, HTA, ForkRake, and DeclaredSpacer.
//   - DeltaFc (FcCalc - FcPub): 3D front-center reconstructed at the declared
//     structural A2C minus the published FrontCenter.
//   - DeltaWb (WbCalc - WbPub): wheelbase reconstructed at the declared structural
//     A2C minus the published Wheelbase.
//   - DeltaVsBest: populated by pkg/fit as ImpliedStructuralA2c minus the
//     min-max midpoint structural A2C across all sizes of the frame.
type SizeResidual struct {
	Size                 string
	HtaDeg               float64
	Stack                float64
	Reach                float64
	Htl                  float64
	BbDrop               float64
	ForkRake             float64
	ChainstayLength      float64
	ImpliedStructuralA2c float64
	DeclaredSpacer       float64
	ImpliedForkA2c       float64
	StatedStaticForkA2c  float64
	ExpectedForkA2c      float64
	DeltaA2c             float64 // Expected - Calculated
	DeltaVsBest          float64
	FcPub                float64
	FcCalc               float64
	DeltaFc              *float64 // Calc - Pub
	WbPub                float64
	WbCalc               float64
	DeltaWb              *float64 // Calc - Pub
	ValidationReport     model.ValidationReport
}

// SizeResiduals reconstructs implied fork A2C, front-center, and wheelbase
// from each size's published chart dimensions and returns their signed
// discrepancies against the frame's declared CAD fork and published numbers.
func SizeResiduals(frame model.FrameModel) ([]SizeResidual, error) {
	sizes := frame.GetSizes()
	if len(sizes) == 0 {
		return nil, fmt.Errorf("frame %s has no sizes defined", frame.ID)
	}

	var results []SizeResidual

	for _, sz := range sizes {
		bike, err := geometry.MaterializeBikeGeometry(frame, sz)
		if err != nil {
			return nil, fmt.Errorf("failed to materialize size %s: %w", sz, err)
		}

		valReport := ValidateBikeGeometry(bike)

		geoSpacer := bike.GetGeoHeadsetSpacerMm()
		structuralA2c, err := geometry.CalculateGeometricA2c(
			bike.Stack,
			bike.BbDrop,
			bike.Htl,
			bike.HtaDeg,
			bike.ForkRake,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate geometric A2C for size %s: %w", sz, err)
		}
		calculatedForkA2c := structuralA2c - geoSpacer

		expectedForkA2c := bike.GetCadA2c()

		deltaA2c := 0.0
		if expectedForkA2c > 0 {
			deltaA2c = geometry.Round(expectedForkA2c-calculatedForkA2c, 1)
		}

		// Evaluate FC and WB at the declared CAD structural A2C (fork A2C + CAD spacer)
		// so residuals reflect full frame+fork discrepancy rather than masking Stack error.
		evalStructuralA2c := structuralA2c
		if expectedForkA2c > 0 {
			evalStructuralA2c = expectedForkA2c + geoSpacer
		}

		var deltaFc *float64
		fcCalc := geometry.Round(
			geometry.CalculateFrontCenter3D(
				bike.Reach,
				bike.Htl,
				bike.HtaDeg,
				bike.ForkRake,
				evalStructuralA2c,
				bike.BbDrop,
			),
			1,
		)
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
				evalStructuralA2c,
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
			StatedStaticForkA2c:  expectedForkA2c,
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
