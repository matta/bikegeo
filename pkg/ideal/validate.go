package ideal

import (
	"fmt"
	"strings"
)

// ValidateProfile strictly validates an idealized frame profile.
// Returns an error describing the exact attribute and problem if invalid.
func ValidateProfile(p *Profile) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile name must not be empty")
	}

	if p.SaddleHeightMm != nil && *p.SaddleHeightMm <= 0 {
		return fmt.Errorf("profile saddle_height_mm must be positive, got %.1f", *p.SaddleHeightMm)
	}

	if len(p.Attributes) == 0 {
		return fmt.Errorf("profile %q must define at least one attribute", p.Name)
	}

	seenIDs := make(map[string]bool)
	hasWeighted := false

	for idx, attr := range p.Attributes {
		id := strings.ToLower(strings.TrimSpace(attr.ID))
		if id == "" {
			return fmt.Errorf("profile %q attribute #%d is missing id", p.Name, idx+1)
		}
		if seenIDs[id] {
			return fmt.Errorf("profile %q contains duplicate attribute id %q", p.Name, attr.ID)
		}
		seenIDs[id] = true

		if _, ok := LookupAttribute(id); !ok {
			return fmt.Errorf("profile %q attribute %q is not a recognized attribute id", p.Name, attr.ID)
		}

		if attr.Weight < 0 {
			return fmt.Errorf("profile %q attribute %q weight must be non-negative, got %.1f", p.Name, attr.ID, attr.Weight)
		}

		if attr.Weight > 0 {
			hasWeighted = true
			if attr.Ideal == nil {
				return fmt.Errorf("profile %q weighted attribute %q must define an ideal range", p.Name, attr.ID)
			}
		}

		if attr.Ideal != nil {
			if attr.Ideal.Min == nil && attr.Ideal.Max == nil {
				return fmt.Errorf("profile %q attribute %q ideal range must define min, max, or both", p.Name, attr.ID)
			}
			if attr.Ideal.Min != nil && attr.Ideal.Max != nil && *attr.Ideal.Min > *attr.Ideal.Max {
				return fmt.Errorf("profile %q attribute %q ideal min (%.1f) exceeds max (%.1f)", p.Name, attr.ID, *attr.Ideal.Min, *attr.Ideal.Max)
			}

			// Validate Acceptable range if present
			if attr.Acceptable != nil {
				if attr.Acceptable.Min == nil && attr.Acceptable.Max == nil {
					return fmt.Errorf("profile %q attribute %q acceptable range must define min, max, or both", p.Name, attr.ID)
				}
				if attr.Acceptable.Min != nil && attr.Acceptable.Max != nil && *attr.Acceptable.Min > *attr.Acceptable.Max {
					return fmt.Errorf("profile %q attribute %q acceptable min (%.1f) exceeds max (%.1f)", p.Name, attr.ID, *attr.Acceptable.Min, *attr.Acceptable.Max)
				}

				// One-sided consistency
				if attr.Ideal.Min != nil && attr.Acceptable.Min == nil {
					return fmt.Errorf("profile %q attribute %q ideal has min bound but acceptable is unbounded below", p.Name, attr.ID)
				}
				if attr.Ideal.Min == nil && attr.Acceptable.Min != nil {
					return fmt.Errorf("profile %q attribute %q ideal is unbounded below but acceptable has min bound", p.Name, attr.ID)
				}
				if attr.Ideal.Max != nil && attr.Acceptable.Max == nil {
					return fmt.Errorf("profile %q attribute %q ideal has max bound but acceptable is unbounded above", p.Name, attr.ID)
				}
				if attr.Ideal.Max == nil && attr.Acceptable.Max != nil {
					return fmt.Errorf("profile %q attribute %q ideal is unbounded above but acceptable has max bound", p.Name, attr.ID)
				}

				// Subset check
				if attr.Ideal.Min != nil && attr.Acceptable.Min != nil && *attr.Acceptable.Min > *attr.Ideal.Min {
					return fmt.Errorf("profile %q attribute %q acceptable min (%.1f) must be <= ideal min (%.1f)", p.Name, attr.ID, *attr.Acceptable.Min, *attr.Ideal.Min)
				}
				if attr.Ideal.Max != nil && attr.Acceptable.Max != nil && *attr.Acceptable.Max < *attr.Ideal.Max {
					return fmt.Errorf("profile %q attribute %q acceptable max (%.1f) must be >= ideal max (%.1f)", p.Name, attr.ID, *attr.Acceptable.Max, *attr.Ideal.Max)
				}
			}

			// Check if acceptable is wider than ideal
			isWider := false
			if attr.Acceptable != nil {
				if attr.Ideal.Min != nil && attr.Acceptable.Min != nil && *attr.Acceptable.Min < *attr.Ideal.Min {
					isWider = true
				}
				if attr.Ideal.Max != nil && attr.Acceptable.Max != nil && *attr.Acceptable.Max > *attr.Ideal.Max {
					isWider = true
				}
			}

			if isWider {
				if attr.EdgeScore == nil {
					return fmt.Errorf("profile %q attribute %q acceptable range is wider than ideal, but edge_score is not defined", p.Name, attr.ID)
				}
				if *attr.EdgeScore <= 0 || *attr.EdgeScore > 100 {
					return fmt.Errorf("profile %q attribute %q edge_score (%.1f) must be in (0, 100]", p.Name, attr.ID, *attr.EdgeScore)
				}
			} else if attr.EdgeScore != nil {
				if *attr.EdgeScore <= 0 || *attr.EdgeScore > 100 {
					return fmt.Errorf("profile %q attribute %q edge_score (%.1f) must be in (0, 100]", p.Name, attr.ID, *attr.EdgeScore)
				}
			}

			// Falloff validation for each bounded side
			if attr.Ideal.Min != nil {
				if attr.Falloff == nil || attr.Falloff.Below == nil || *attr.Falloff.Below <= 0 {
					return fmt.Errorf("profile %q attribute %q must define positive falloff.below", p.Name, attr.ID)
				}
			}
			if attr.Ideal.Max != nil {
				if attr.Falloff == nil || attr.Falloff.Above == nil || *attr.Falloff.Above <= 0 {
					return fmt.Errorf("profile %q attribute %q must define positive falloff.above", p.Name, attr.ID)
				}
			}

			// Floor validation
			if attr.Floor != nil && *attr.Floor >= 0 {
				return fmt.Errorf("profile %q attribute %q floor (%.1f) must be negative", p.Name, attr.ID, *attr.Floor)
			}
		}
	}

	if !hasWeighted {
		return fmt.Errorf("profile %q must define at least one weighted attribute", p.Name)
	}

	return nil
}
