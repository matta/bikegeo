package ideal

import (
	"fmt"
	"strings"

	"bikegeo/internal/model"
)

// HasCockpitTargets reports whether the profile evaluates cockpit position attributes.
func (p *Profile) HasCockpitTargets() bool {
	if p == nil {
		return false
	}
	for _, attr := range p.Attributes {
		if attr.Ideal == nil {
			continue
		}
		switch attr.ID {
		case MetricHandlebarClampFit:
			return true
		case MetricSaddleHb, MetricSaddleHbDrop, MetricSetback,
			MetricEtt,
			MetricAett,
			MetricSpareVert,
			MetricSpareAxial,
			MetricSeatpostExtension,
			MetricSeatToRc,
			MetricChainstay,
			MetricBbDrop,
			MetricHta,
			MetricTrail,
			MetricSta,
			MetricReach,
			MetricStack,
			MetricWheelbase,
			MetricRcWbRatio,
			MetricSeatTube,
			MetricStandover,
			MetricBrpSetback,
			MetricBestStem,
			MetricStemSpacerFit,
			MetricSitBoneSetback,
			MetricIdealSta,
			MetricRailOffset,
			MetricStemLength,
			MetricStemAngle,
			MetricSpacers:
			continue
		default:
			continue
		}
	}
	return false
}

// ValidateProfile strictly validates an idealized frame profile.
// Returns an error describing the exact attribute and problem if invalid.
func ValidateProfile(p *Profile) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile name must not be empty")
	}

	if sh, ok := p.SaddleHeightMm.Get(); ok && sh <= 0 {
		return fmt.Errorf("profile saddle_height_mm must be positive, got %.1f", sh)
	}
	if p.TargetRef != "" && !p.TargetRef.IsValid() {
		return fmt.Errorf("profile target_ref must be HandGrip, StemClamp, or ReferenceBarClamp, got %q", p.TargetRef)
	}
	if p.HasCockpitTargets() && !p.TargetRef.IsValid() {
		return fmt.Errorf("profile with cockpit targets requires explicit target_ref (HandGrip, StemClamp, or ReferenceBarClamp)")
	}
	if p.TargetRef == model.TargetReferenceBarClamp {
		refBar, ok := p.ReferenceBar.Get()
		if !ok || refBar.EffectiveWidthMm() <= 0 {
			return fmt.Errorf("profile with target_ref ReferenceBarClamp requires reference_bar with positive width")
		}
	}

	if len(p.Attributes) == 0 {
		return fmt.Errorf("profile %q must define at least one attribute", p.Name)
	}

	seenIDs := make(map[MetricID]bool)
	hasScored := false

	for idx, attr := range p.Attributes {
		if attr.ID == "" {
			return fmt.Errorf("profile %q attribute #%d is missing id", p.Name, idx+1)
		}
		if seenIDs[attr.ID] {
			return fmt.Errorf("profile %q contains duplicate attribute id %q", p.Name, attr.ID)
		}
		seenIDs[attr.ID] = true

		if _, ok := LookupAttribute(attr.ID); !ok {
			return fmt.Errorf("profile %q attribute %q is not a recognized attribute id", p.Name, attr.ID)
		}

		if attr.LimitPoints < 0 {
			return fmt.Errorf("profile %q attribute %q limit_points must be non-negative, got %.2f", p.Name, attr.ID, attr.LimitPoints)
		}
		if attr.LimitPoints > 0 && attr.Cost != nil {
			return fmt.Errorf("profile %q attribute %q must price with either cost or limit_points, not both", p.Name, attr.ID)
		}
		if attr.LimitPoints > 0 {
			if attr.Ideal == nil {
				return fmt.Errorf("profile %q scored attribute %q must define an ideal (free) range", p.Name, attr.ID)
			}
			if _, ok := hardwareLimitAttributes[attr.ID]; !ok {
				return fmt.Errorf("profile %q attribute %q has no hardware limit for limit_points", p.Name, attr.ID)
			}
			hasScored = true
		}
		if c := attr.Cost; c != nil {
			if c.Below < 0 || c.Above < 0 {
				return fmt.Errorf("profile %q attribute %q cost must be non-negative, got below %.2f / above %.2f", p.Name, attr.ID, c.Below, c.Above)
			}
			if c.Below == 0 && c.Above == 0 {
				return fmt.Errorf("profile %q attribute %q cost must price at least one side", p.Name, attr.ID)
			}
			if attr.Ideal == nil {
				return fmt.Errorf("profile %q scored attribute %q must define an ideal (free) range", p.Name, attr.ID)
			}
			hasScored = true
		}

		if attr.Ideal != nil {
			idealMin, hasIdealMin := attr.Ideal.Min.Get()
			idealMax, hasIdealMax := attr.Ideal.Max.Get()
			if !hasIdealMin && !hasIdealMax {
				return fmt.Errorf("profile %q attribute %q ideal range must define min, max, or both", p.Name, attr.ID)
			}
			if hasIdealMin && hasIdealMax && idealMin > idealMax {
				return fmt.Errorf("profile %q attribute %q ideal min (%.1f) exceeds max (%.1f)", p.Name, attr.ID, idealMin, idealMax)
			}
		}
	}

	if !hasScored {
		return fmt.Errorf("profile %q must define at least one scored (costed) attribute", p.Name)
	}

	return nil
}
