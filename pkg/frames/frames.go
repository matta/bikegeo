package frames

import (
	"fmt"
	"sort"
	"strings"

	"bikegeo/pkg/model"
)

var registry = []model.FrameModel{
	BinaryHavok,
	BinaryKinetik,
	EskerHayduke,
	FairlightHolt2,
	MootsMountaineer,
	OtsoFenrirStainless,
	RevelElJefe,
	RitcheyUltra,
	SalsaCutthroat,
	SalsaFargo,
	SevenExpedition926d,
	SevenExpedition926f,
	SklarPBJ,
	SourPastaParty,
	SurlyECR,
	SurlyKrampus,
	TumbleweedSunliner,
	TurnerVennTI,
}

// AllFrames returns a copy of all compiled bike frame models sorted by Manufacturer and Model.
func AllFrames() []model.FrameModel {
	out := make([]model.FrameModel, len(registry))
	for i := range registry {
		out[i] = registry[i].Clone()
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Manufacturer != out[j].Manufacturer {
			return out[i].Manufacturer < out[j].Manufacturer
		}
		return out[i].Model < out[j].Model
	})
	return out
}

// GetFrameByID looks up a compiled bike frame model by ID (case-insensitive).
func GetFrameByID(id string) (*model.FrameModel, error) {
	for i := range registry {
		if strings.EqualFold(registry[i].ID, id) {
			cloned := registry[i].Clone()
			return &cloned, nil
		}
	}
	return nil, fmt.Errorf("frame with ID %q not found", id)
}
