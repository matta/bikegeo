package ideal

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/model"
)

// IdmatchProfile returns the idmatch biomechanical fit profile (BRP -> HandGrip).
func IdmatchProfile() *Profile {
	refSaddle := model.ReferenceIdmatchSaddle()
	refBrp := refSaddle.BrpPointAtHeightAndSetback(model.DefaultIdmatchSaddleHeightMm, model.DefaultIdmatchBrpSetbackMm)
	refSitBones := refSaddle.SitBonePointFromBrp(refBrp)
	p := &Profile{
		Name:                   "idmatch",
		Description:            "Biomechanical fit profile based on idmatch Bikelab report (BRP -> HandGrip)",
		SaddleHeightMm:         model.Some(model.DefaultIdmatchSaddleHeightMm),
		TargetSitBoneSetbackMm: model.Some(-refSitBones.X),
		ReferenceSitBones:      refSitBones,
		ReferenceBrp:           refBrp,
		ReferenceSaddle:        refSaddle,
		TargetRef:              model.TargetHandGrip,
		Attributes: []AttributeSpec{
			{
				ID:         "brp_setback",
				Weight:     10,
				Ideal:      Exact(model.DefaultIdmatchBrpSetbackMm),
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
			DisplayOnly("rc_wb_ratio"),
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
	if p.ReferenceSaddle.ID == "" {
		p.ReferenceSaddle = model.ReferenceIdmatchSaddle()
	}
	refSH := p.SaddleHeightMm.Or(model.DefaultIdmatchSaddleHeightMm)
	refBrpSB := model.DefaultIdmatchBrpSetbackMm
	var dropMin, dropMax model.Opt[float64]
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
			lowID := strings.ToLower(attr.ID)
			if lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback" {
				if vMin, ok := attr.Ideal.Min.Get(); ok {
					if vMax, ok := attr.Ideal.Max.Get(); ok {
						refBrpSB = (vMin + vMax) / 2.0
					} else {
						refBrpSB = vMin
					}
				} else if vMax, ok := attr.Ideal.Max.Get(); ok {
					refBrpSB = vMax
				}
			} else if lowID == "saddle_hb_drop" || lowID == "hb_drop" || lowID == "saddlehbdrop" {
				dropMin = attr.Ideal.Min
				dropMax = attr.Ideal.Max
			}
		}
	}
	if p.ReferenceBrp == (geom2d.Point2{}) {
		p.ReferenceBrp = p.ReferenceSaddle.BrpPointAtHeightAndSetback(refSH, refBrpSB)
	}
	if p.ReferenceSitBones == (geom2d.Point2{}) {
		p.ReferenceSitBones = p.ReferenceSaddle.SitBonePointFromBrp(p.ReferenceBrp)
	}
	if !p.TargetSitBoneSetbackMm.IsSet() {
		p.TargetSitBoneSetbackMm = model.Some(-p.ReferenceSitBones.X)
	}
	_ = dropMin
	_ = dropMax

	if err := ValidateProfile(p); err != nil {
		return nil, err
	}

	return p, nil
}

// ForSaddle returns a copy of p with brp_setback and saddle_hb attribute ranges
// projected from p.ReferenceSitBones onto saddle's BRP point.
func (p *Profile) ForSaddle(saddle model.SaddleSpec) *Profile {
	if p == nil || saddle.ID == "" {
		return p
	}
	ref := p.ReferenceSaddle
	if ref.ID == "" {
		ref = model.ReferenceIdmatchSaddle()
	}
	if saddle.ID == ref.ID {
		return p
	}
	refBrp := p.ReferenceBrp
	if refBrp == (geom2d.Point2{}) {
		refBrp = ref.BrpPointAtHeightAndSetback(p.SaddleHeightMm.Or(model.DefaultIdmatchSaddleHeightMm), model.DefaultIdmatchBrpSetbackMm)
	}
	sitBones := p.ReferenceSitBones
	if sitBones == (geom2d.Point2{}) {
		sitBones = ref.SitBonePointFromBrp(refBrp)
	}
	targetBrp := saddle.BrpPointFromSitBones(sitBones)

	refRise := 3.0
	for _, attr := range p.Attributes {
		lowID := strings.ToLower(attr.ID)
		if (lowID == "saddle_hb_drop" || lowID == "hb_drop" || lowID == "saddlehbdrop") && attr.Ideal != nil {
			if dMin, ok := attr.Ideal.Min.Get(); ok {
				if dMax, ok := attr.Ideal.Max.Get(); ok {
					refRise = (dMin + dMax) / 2.0
				} else {
					refRise = dMin
				}
			}
		}
	}

	mapRange := func(r *Range, fn func(float64) float64) *Range {
		if r == nil {
			return nil
		}
		return &Range{
			Min: r.Min.Map(fn),
			Max: r.Max.Map(fn),
		}
	}

	copied := *p
	copied.Attributes = make([]AttributeSpec, len(p.Attributes))
	for i, attr := range p.Attributes {
		lowID := strings.ToLower(attr.ID)
		switch lowID {
		case "brp_setback", "brp", "brpsetback":
			fn := func(v float64) float64 {
				return saddle.BrpSetbackFromSitBones(ref.SitBoneSetbackFromBrp(v))
			}
			attr.Ideal = mapRange(attr.Ideal, fn)
			attr.Acceptable = mapRange(attr.Acceptable, fn)
		case "saddle_hb", "saddlehb":
			fn := func(v float64) float64 {
				if v <= 0 {
					return 0
				}
				hbY := sitBones.Y + refRise
				dyRef := hbY - refBrp.Y
				dxRef := math.Sqrt(math.Max(0, v*v-dyRef*dyRef))
				hb := geom2d.Point2{X: refBrp.X + dxRef, Y: hbY}
				return math.Round(hb.DistanceTo(targetBrp)*10) / 10
			}
			attr.Ideal = mapRange(attr.Ideal, fn)
			attr.Acceptable = mapRange(attr.Acceptable, fn)
		}
		copied.Attributes[i] = attr
	}
	return &copied
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
		TargetRef:      p.TargetRef,
		AttributeCount: len(p.Attributes),
		ScoredCount:    scoredCount,
	}
}
