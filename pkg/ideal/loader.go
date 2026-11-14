package ideal

import (
	"fmt"
	"strings"

	"bikegeo/pkg/model"
)

// IdmatchProfile returns the idmatch biomechanical fit profile (BRP -> HB).
func IdmatchProfile() *Profile {
	p := &Profile{
		Name:           "idmatch",
		Description:    "Biomechanical fit profile based on idmatch Bikelab report (BRP -> HB)",
		SaddleHeightMm: model.Some(746.0),
		Attributes: []AttributeSpec{
			{
				ID:         "brp_setback",
				Weight:     10,
				Ideal:      Exact(219),
				Acceptable: Between(214, 224),
				EdgeScore:  model.Some(75.0),
				Falloff:    FalloffBoth(20, 20),
				Floor:      model.Some(-200.0),
			},
			DisplayOnlyWithTarget("saddle_hb", Between(695, 705), Between(685, 715), model.Some(75.0), FalloffBoth(15, 15)),
			DisplayOnlyWithTarget("saddle_hb_drop", Between(-2, 8), Between(-10, 16), model.Some(75.0), FalloffBoth(10, 10)),
			DisplayOnly("best_stem"),
			{
				ID:         "handlebar_clamp_fit",
				Weight:     15,
				Ideal:      AtLeast(100),
				Acceptable: AtLeast(75),
				EdgeScore:  model.Some(75.0),
				Falloff:    FalloffBelow(75),
				Floor:      model.Some(-200.0),
			},
			{
				ID:         "seatpost_extension",
				Weight:     4,
				Ideal:      Between(250, 310),
				Acceptable: Between(210, 340),
				EdgeScore:  model.Some(75.0),
				Falloff:    FalloffBoth(25, 25),
				Floor:      model.Some(-100.0),
			},
			{
				ID:         "frontwt",
				Weight:     4,
				Ideal:      Between(35.8, 38.2),
				Acceptable: Between(34.8, 39.2),
				EdgeScore:  model.Some(75.0),
				Falloff:    FalloffBoth(2.0, 2.0),
				Floor:      model.Some(-100.0),
			},
			{
				ID:         "bbdrop",
				Weight:     3,
				Ideal:      Exact(70),
				Acceptable: Between(60, 80),
				EdgeScore:  model.Some(75.0),
				Falloff:    FalloffBoth(10, 10),
				Floor:      model.Some(-100.0),
			},
			{
				ID:         "trail",
				Weight:     2,
				Ideal:      Between(98, 106),
				Acceptable: Between(92, 109),
				EdgeScore:  model.Some(75.0),
				Falloff:    FalloffBoth(10, 10),
				Floor:      model.Some(-100.0),
			},
			DisplayOnly("hta"),
			DisplayOnly("reach"),
			DisplayOnly("stack"),
			DisplayOnly("sta"),
			DisplayOnly("ett"),
			DisplayOnly("aett"),
			DisplayOnly("chainstay"),
			DisplayOnly("seat_to_rc"),
			DisplayOnly("sparevert"),
		},
	}
	_, _ = NormalizeAndValidate(p)
	return p
}

// AllProfiles returns all compiled ideal profiles in alphabetical order.
func AllProfiles() []*Profile {
	return []*Profile{
		IdmatchProfile(),
	}
}

// NormalizeAndValidate populates default Acceptable and Floor fields on a Profile and validates it.
func NormalizeAndValidate(p *Profile) (*Profile, error) {
	for i := range p.Attributes {
		attr := &p.Attributes[i]
		if attr.Ideal != nil {
			if attr.Acceptable == nil {
				attr.Acceptable = &Range{
					Min: attr.Ideal.Min,
					Max: attr.Ideal.Max,
				}
			}
			if !attr.Floor.IsSet() {
				attr.Floor = model.Some(-200.0)
			}
		}
	}

	if err := ValidateProfile(p); err != nil {
		return nil, err
	}

	return p, nil
}

// GetProfile looks up a compiled ideal profile by name (defaults to "idmatch" when empty).
func GetProfile(name string) (*Profile, error) {
	cleanName := strings.TrimSuffix(strings.TrimSpace(name), ".json")
	if cleanName == "" || strings.EqualFold(cleanName, "default") {
		cleanName = "idmatch"
	}

	for _, p := range AllProfiles() {
		if strings.EqualFold(p.Name, cleanName) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("ideal profile %q not found", cleanName)
}

// ListProfiles returns summaries of all compiled ideal profiles.
func ListProfiles() []ProfileSummary {
	var summaries []ProfileSummary
	for _, p := range AllProfiles() {
		summaries = append(summaries, summarizeProfile(p))
	}
	return summaries
}

func summarizeProfile(p *Profile) ProfileSummary {
	scoredCount := 0
	for _, attr := range p.Attributes {
		if attr.Weight > 0 && attr.Ideal != nil {
			scoredCount++
		}
	}
	return ProfileSummary{
		Name:           p.Name,
		Description:    p.Description,
		SaddleHeightMm: p.SaddleHeightMm,
		AttributeCount: len(p.Attributes),
		ScoredCount:    scoredCount,
	}
}
