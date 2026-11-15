package ideal

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
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
		CockpitTargetDistMm:    700,
		CockpitTargetRiseMm:    3,
		Attributes: []AttributeSpec{
			{
				ID:    "brp_setback",
				Cost:  CostEach(1.2),
				Ideal: Exact(model.DefaultIdmatchBrpSetbackMm),
			},
			{
				ID:          "rail_offset",
				LimitPoints: 8,
				Ideal:       Between(-5, 5),
			},
			DisplayOnly("saddle_hb"),
			DisplayOnly("saddle_hb_drop"),
			DisplayOnly("best_stem"),
			DisplayOnly("stem_spacer_fit"),
			{
				// Oval Fits is priced on the selected setup's normalized radius R,
				// not on the displayed value: free inside the oval (R <= 1), then
				// Cost.Above points per unit of R beyond the edge.
				ID:    "handlebar_clamp_fit",
				Cost:  &Cost{Above: OvalCostPerR},
				Ideal: AtLeast(1),
			},
			{
				ID:          "stem_length",
				LimitPoints: 8,
				Ideal:       Between(70, 80),
			},
			{
				ID:          "stem_angle",
				LimitPoints: 1,
				Ideal:       Exact(0),
			},
			{
				ID:          "spacers",
				LimitPoints: 8,
				Ideal:       Between(0, 20),
			},
			{
				ID:    "seatpost_extension",
				Cost:  CostBoth(0.18, 0.21),
				Ideal: Between(250, 310),
			},
			{
				ID:    "rc_wb_ratio",
				Cost:  CostEach(4),
				Ideal: Between(37, 39),
			},
			{
				ID:    "bbdrop",
				Cost:  CostEach(0.44),
				Ideal: Between(65, 75),
			},
			{
				ID:    "trail",
				Cost:  CostEach(0.41),
				Ideal: Between(98, 106),
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

// NormalizeAndValidate fills a Profile's reference saddle, BRP, sit-bone and cockpit
// target defaults and validates it.
func NormalizeAndValidate(p *Profile) (*Profile, error) {
	if p.ReferenceSaddle.ID == "" {
		p.ReferenceSaddle = model.ReferenceIdmatchSaddle()
	}
	refSH := p.SaddleHeightMm.Or(model.DefaultIdmatchSaddleHeightMm)
	refBrpSB := model.DefaultIdmatchBrpSetbackMm
	for i := range p.Attributes {
		attr := &p.Attributes[i]
		if attr.ID != MetricBrpSetback || attr.Ideal == nil {
			continue
		}
		if vMin, ok := attr.Ideal.Min.Get(); ok {
			if vMax, ok := attr.Ideal.Max.Get(); ok {
				refBrpSB = (vMin + vMax) / 2.0
			} else {
				refBrpSB = vMin
			}
		} else if vMax, ok := attr.Ideal.Max.Get(); ok {
			refBrpSB = vMax
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

	if err := ValidateProfile(p); err != nil {
		return nil, err
	}

	return p, nil
}

// ForSaddle returns a copy of p with the brp_setback free value carried from
// p.ReferenceSitBones onto saddle's BRP: the sit-bone setback is the invariant, so
// a saddle whose BRP sits elsewhere relative to its sit bones gets a shifted free
// value. The cockpit target for another saddle comes from CockpitTarget.
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

	fn := func(v float64) float64 {
		return saddle.BrpSetbackFromSitBones(ref.SitBoneSetbackFromBrp(v))
	}
	copied := *p
	copied.Attributes = make([]AttributeSpec, len(p.Attributes))
	for i, attr := range p.Attributes {
		if attr.ID == MetricBrpSetback && attr.Ideal != nil {
			attr.Ideal = &Range{Min: attr.Ideal.Min.Map(fn), Max: attr.Ideal.Max.Map(fn)}
		}
		copied.Attributes[i] = attr
	}
	return &copied
}

// CockpitTarget returns the cockpit target for saddle: the distance from the
// saddle's BRP to the hands, and their rise above the sit bones. For the reference
// saddle these are CockpitTargetDistMm and CockpitTargetRiseMm. For another saddle
// the hands keep their position relative to the sit bones, so the distance is
// re-measured from that saddle's BRP (rounded to 0.1 mm); the rise is unchanged.
// ok is false when the profile has no cockpit target.
func (p *Profile) CockpitTarget(saddle model.SaddleSpec) (distMm, riseMm float64, ok bool) {
	if p == nil || p.CockpitTargetDistMm <= 0 {
		return 0, 0, false
	}
	distMm, riseMm = p.CockpitTargetDistMm, p.CockpitTargetRiseMm
	ref := p.ReferenceSaddle
	if ref.ID == "" {
		ref = model.ReferenceIdmatchSaddle()
	}
	if saddle.ID == "" || saddle.ID == ref.ID {
		return distMm, riseMm, true
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

	handY := sitBones.Y + riseMm
	dyRef := handY - refBrp.Y
	dxRef := math.Sqrt(math.Max(0, distMm*distMm-dyRef*dyRef))
	hand := geom2d.Point2{X: refBrp.X + dxRef, Y: handY}
	return math.Round(hand.DistanceTo(targetBrp)*10) / 10, riseMm, true
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
