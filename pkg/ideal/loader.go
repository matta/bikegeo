package ideal

import (
	"fmt"
	"sort"
	"strings"
)

// DefaultProfile returns the default ideal frame profile.
func DefaultProfile() *Profile {
	p := &Profile{
		Name:           "default",
		Description:    "Approximation of pre-profile hard-coded scoring",
		SaddleHeightMm: Float(780),
		Attributes: []AttributeSpec{
			{
				ID:         "setback",
				Weight:     10,
				Ideal:      Exact(230),
				Acceptable: Between(220, 240),
				EdgeScore:  Float(80),
				Falloff:    FalloffBoth(10, 10),
				Floor:      Float(-200),
			},
			{
				ID:         "aett",
				Weight:     10,
				Ideal:      Between(625, 636),
				Acceptable: Between(604.5, 648),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(20.5, 12),
				Floor:      Float(-200),
			},
			{
				ID:         "sparevert",
				Weight:     5,
				Ideal:      Between(70, 100),
				Acceptable: Between(62.5, 117.5),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(7.5, 17.5),
				Floor:      Float(-100),
			},
			{
				ID:         "seatpost_extension",
				Weight:     4,
				Ideal:      Between(260, 320),
				Acceptable: Between(220, 350),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(25, 25),
				Floor:      Float(-100),
			},
			{
				ID:         "frontwt",
				Weight:     4,
				Ideal:      Between(35.8, 38.2),
				Acceptable: Between(34.8, 39.2),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(2.0, 2.0),
				Floor:      Float(-100),
			},
			{
				ID:         "bbdrop",
				Weight:     3,
				Ideal:      Exact(70),
				Acceptable: Between(60, 80),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(10, 10),
				Floor:      Float(-100),
			},
			{
				ID:         "trail",
				Weight:     2,
				Ideal:      Between(98, 106),
				Acceptable: Between(92, 109),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(10, 10),
				Floor:      Float(-100),
			},
			DisplayOnly("hta"),
			DisplayOnly("seat_to_rc"),
			DisplayOnly("reach"),
			DisplayOnly("stack"),
			DisplayOnly("sta"),
			DisplayOnly("chainstay"),
			DisplayOnly("saddle_hb"),
		},
	}
	_, _ = NormalizeAndValidate(p)
	return p
}

// IdmatchProfile returns the idmatch biomechanical fit profile (BRP -> HB).
func IdmatchProfile() *Profile {
	p := &Profile{
		Name:           "idmatch",
		Description:    "Biomechanical fit profile based on idmatch Bikelab report (BRP -> HB)",
		SaddleHeightMm: Float(746),
		Attributes: []AttributeSpec{
			{
				ID:         "brp_setback",
				Weight:     10,
				Ideal:      Exact(219),
				Acceptable: Between(214, 224),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(20, 20),
				Floor:      Float(-200),
			},
			DisplayOnlyWithTarget("saddle_hb", Between(695, 705), Between(685, 715), Float(75), FalloffBoth(15, 15)),
			DisplayOnlyWithTarget("saddle_hb_drop", Between(8, 18), Between(0, 26), Float(75), FalloffBoth(10, 10)),
			DisplayOnly("best_stem"),
			{
				ID:         "handlebar_clamp_fit",
				Weight:     15,
				Ideal:      AtLeast(100),
				Acceptable: AtLeast(75),
				EdgeScore:  Float(75),
				Falloff:    FalloffBelow(75),
				Floor:      Float(-200),
			},
			{
				ID:         "seatpost_extension",
				Weight:     4,
				Ideal:      Between(250, 310),
				Acceptable: Between(210, 340),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(25, 25),
				Floor:      Float(-100),
			},
			{
				ID:         "frontwt",
				Weight:     4,
				Ideal:      Between(35.8, 38.2),
				Acceptable: Between(34.8, 39.2),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(2.0, 2.0),
				Floor:      Float(-100),
			},
			{
				ID:         "bbdrop",
				Weight:     3,
				Ideal:      Exact(70),
				Acceptable: Between(60, 80),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(10, 10),
				Floor:      Float(-100),
			},
			{
				ID:         "trail",
				Weight:     2,
				Ideal:      Between(98, 106),
				Acceptable: Between(92, 109),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(10, 10),
				Floor:      Float(-100),
			},
			DisplayOnly("hta"),
			DisplayOnly("reach"),
			DisplayOnly("stack"),
			DisplayOnly("sta"),
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
	profiles := []*Profile{
		DefaultProfile(),
		IdmatchProfile(),
	}
	sort.SliceStable(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles
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
			if attr.Floor == nil {
				fl := -200.0
				attr.Floor = &fl
			}
		}
	}

	if err := ValidateProfile(p); err != nil {
		return nil, err
	}

	return p, nil
}

// GetProfile looks up a compiled ideal profile by name (defaults to "default" when empty).
func GetProfile(name string) (*Profile, error) {
	cleanName := strings.TrimSuffix(strings.TrimSpace(name), ".json")
	if cleanName == "" {
		cleanName = "default"
	}

	for _, p := range AllProfiles() {
		if strings.EqualFold(p.Name, cleanName) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("ideal profile %q not found", cleanName)
}

// LoadProfile is a compatibility wrapper for GetProfile.
func LoadProfile(_ string, name string) (*Profile, error) {
	return GetProfile(name)
}

// ListProfiles returns summaries of all compiled ideal profiles.
func ListProfiles(_ ...string) ([]ProfileSummary, error) {
	var summaries []ProfileSummary
	for _, p := range AllProfiles() {
		summaries = append(summaries, summarizeProfile(p))
	}
	return summaries, nil
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
