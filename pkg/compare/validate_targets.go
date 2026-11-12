package compare

import (
	"fmt"
	"strings"

	"bikegeo/pkg/frames"
	"bikegeo/pkg/validator"
)

// ResolveValidationTargets turns frame tokens ("frame" or "frame:size") and an
// optional size override into validation targets. No tokens means every frame
// at its default size (or at size, when given).
func ResolveValidationTargets(tokens []string, size string) ([]validator.Target, error) {
	all := frames.AllFrames()
	if len(all) == 0 {
		return nil, fmt.Errorf("no bike frames found")
	}
	var targets []validator.Target
	if len(tokens) == 0 {
		for _, f := range all {
			sz := size
			if sz == "" {
				sz = f.GetDefaultSize()
			}
			targets = append(targets, validator.Target{Frame: f, Size: sz})
		}
		return targets, nil
	}
	for _, tok := range tokens {
		f, impSz, err := ResolveFrameByToken(all, tok)
		if err != nil {
			return nil, err
		}
		sz := size
		if sz == "" {
			sz = impSz
		}
		if sz == "" {
			sz = f.GetDefaultSize()
		}
		matched := ""
		for _, s := range f.GetSizes() {
			if strings.EqualFold(s, sz) {
				matched = s
				break
			}
		}
		if matched == "" {
			return nil, fmt.Errorf("frame %q does not have size %q (available: %s)", f.ID, sz, strings.Join(f.GetSizes(), ", "))
		}
		targets = append(targets, validator.Target{Frame: *f, Size: matched})
	}
	return targets, nil
}
