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
				ID:         "seat_to_rc",
				Weight:     4,
				Ideal:      AtLeast(235),
				Acceptable: AtLeast(220),
				EdgeScore:  Float(70),
				Falloff:    FalloffBelow(20),
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
				ID:         "hta",
				Weight:     2,
				Ideal:      Between(68, 69),
				Acceptable: Between(67.5, 69.5),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(0.5, 0.5),
				Floor:      Float(-200),
			},
			{
				ID:         "frontwt",
				Weight:     1,
				Ideal:      AtLeast(38),
				Acceptable: AtLeast(38),
				Falloff:    FalloffBelow(5),
				Floor:      Float(-100),
			},
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
			{
				ID:         "saddle_hb",
				Weight:     10,
				Ideal:      Between(695, 705),
				Acceptable: Between(685, 715),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(15, 15),
				Floor:      Float(-200),
			},
			{
				ID:         "saddle_hb_drop",
				Weight:     5,
				Ideal:      Between(8, 18),
				Acceptable: Between(0, 26),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(10, 10),
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
				ID:         "hta",
				Weight:     2,
				Ideal:      Between(68, 69),
				Acceptable: Between(67.5, 69.5),
				EdgeScore:  Float(75),
				Falloff:    FalloffBoth(0.5, 0.5),
				Floor:      Float(-200),
			},
			{
				ID:         "frontwt",
				Weight:     1,
				Ideal:      AtLeast(38),
				Acceptable: AtLeast(38),
				Falloff:    FalloffBelow(5),
				Floor:      Float(-100),
			},
			DisplayOnly("reach"),
			DisplayOnly("stack"),
			DisplayOnly("sta"),
			DisplayOnly("chainstay"),
			DisplayOnly("setback"),
			DisplayOnly("aett"),
			DisplayOnly("sparevert"),
			DisplayOnly("seat_to_rc"),
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
