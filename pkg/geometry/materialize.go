package geometry

import (
	"fmt"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/model"
)

// MaterializeBikeGeometry projects a FrameModel for a given size into a BikeGeometry instance.
func MaterializeBikeGeometry(frame model.FrameModel, size string) (model.BikeGeometry, error) {
	if size == "" {
		size = frame.GetDefaultSize()
	}

	geo, ok := frame.GetSizeGeometry(size)
	if !ok {
		return model.BikeGeometry{}, fmt.Errorf("size %s not defined in frame %s", size, frame.ID)
	}

	bikeID := frame.ID
	if !strings.HasSuffix(strings.ToLower(bikeID), "-"+strings.ToLower(size)) {
		bikeID = fmt.Sprintf("%s-%s", frame.ID, strings.ToLower(size))
	}

	cadA2c := frame.CadFork.A2cMm
	cadRake := frame.CadFork.RakeMm
	if cadRake <= 0 {
		return model.BikeGeometry{}, fmt.Errorf("frame %q requires positive CAD fork rake, got %v", frame.ID, cadRake)
	}

	return model.BikeGeometry{
		ID:                  bikeID,
		Manufacturer:        frame.Manufacturer,
		Model:               frame.Model,
		Size:                size,
		Year:                frame.Year,
		Category:            frame.Category,
		Notes:               frame.Notes,
		WheelSize:           frame.WheelSize,
		HeadsetCupType:      frame.HeadsetCupType,
		GeoHeadsetSpacerMm:  frame.GeoHeadsetSpacerMm,
		GeoSlopMm:           frame.GeoSlopMm,
		ChartPrecision:      InferChartPrecision(frame),
		ActualStaDeg:        frame.ActualStaDeg,
		SeatTubeOffsetXMm:   frame.SeatTubeOffsetXMm,
		CadFork:             frame.CadFork,
		ForkA2c:             cadA2c,
		ForkRake:            cadRake,
		Reach:               geo.Reach,
		Stack:               geo.Stack,
		HtaDeg:              geo.HtaDeg,
		StaDeg:              geo.StaDeg,
		Htl:                 geo.Htl,
		BbDrop:              geo.BbDrop,
		Wheelbase:           geo.Wheelbase,
		ChainstayLength:     geo.ChainstayLength,
		FrontCenter:         geo.FrontCenter,
		SeatTubeLength:      geo.SeatTubeLength,
		StandoverHeight:     geo.StandoverHeight,
		ChainstayRangeMm:    frame.ChainstayRangeMm,
		MaxSpacerMm:         frame.MaxSpacerMm,
		CanonicalStem:       frame.CanonicalStem,
		Stems:               catalog.ExpandStemSets(frame.Stems),
		ExpectedStemRangeMm: frame.ExpectedStemRangeMm,
	}, nil
}
