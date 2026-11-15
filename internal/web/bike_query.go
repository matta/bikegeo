package web

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"bikegeo/internal/catalog"
	"bikegeo/internal/frames"
	"bikegeo/internal/model"
)

func parseBikeQuery(v url.Values, allFrames []model.FrameModel) (bikeParams, []string) {
	var errs []string
	var p bikeParams

	frameID := strings.TrimSpace(v.Get("frame"))
	prevFrame := strings.TrimSpace(v.Get("prev_frame"))
	switchedFrame := prevFrame != "" && frameID != "" && !strings.EqualFold(prevFrame, frameID)

	if frameID == "" {
		frameID = "otso-fenrir-stainless"
	}
	fm, err := frames.GetFrameByID(frameID)
	if err != nil {
		errs = append(errs, err.Error())
		p.frameID = frameID
	} else {
		p.frameModel = *fm
		p.frameID = fm.ID
		sizes := fm.GetSizes()
		reqSize := strings.TrimSpace(v.Get("size"))
		switch {
		case reqSize == "":
			p.size = defaultSizeForFrame(sizes)
		case hasExactFold(sizes, reqSize) != "":
			p.size = hasExactFold(sizes, reqSize)
		case switchedFrame:
			p.size = defaultSizeForFrame(sizes)
		default:
			errs = append(errs, fmt.Sprintf("size %q not found on frame %q", reqSize, fm.ID))
			p.size = reqSize
		}

		choices := forkChoices(*fm)
		reqFork := strings.TrimSpace(v.Get("fork"))
		switch {
		case reqFork == "":
			if hasExactFold(choices, "rigid") != "" {
				p.fork = "rigid"
			} else if len(choices) > 0 {
				p.fork = choices[0]
			}
		case switchedFrame && hasExactFold(choices, reqFork) == "":
			if hasExactFold(choices, "rigid") != "" {
				p.fork = "rigid"
			} else if len(choices) > 0 {
				p.fork = choices[0]
			}
		default:
			p.fork = reqFork
		}
	}

	if rawSag := strings.TrimSpace(v.Get("sag")); rawSag != "" {
		clean := strings.TrimSuffix(rawSag, "%")
		sagVal, err := parseFiniteFloat("sag", clean)
		if err != nil {
			errs = append(errs, err.Error())
		} else if sagVal < 0 || sagVal >= 100 {
			errs = append(errs, fmt.Sprintf("sag must be in [0, 100), got %.1f", sagVal))
		} else if !switchedFrame || p.fork != "rigid" || sagVal == 0 {
			p.sag = strconv.FormatFloat(sagVal, 'f', -1, 64)
		}
	}

	saddleID := strings.TrimSpace(v.Get("saddle"))
	if saddleID == "" {
		p.saddle = catalog.DefaultSaddle()
	} else if sd, ok := catalog.GetSaddle(saddleID); ok {
		p.saddle = sd
	} else {
		errs = append(errs, fmt.Sprintf("unknown saddle %q", saddleID))
	}

	barID := strings.TrimSpace(v.Get("handlebar"))
	if barID == "" {
		p.handlebar = catalog.DefaultHandlebar()
	} else if hb, ok := catalog.GetHandlebar(barID); ok {
		p.handlebar = hb
	} else {
		errs = append(errs, fmt.Sprintf("unknown handlebar %q", barID))
	}

	tr := strings.TrimSpace(v.Get("target_ref"))
	if tr == "" {
		p.targetRef = model.TargetHandGrip
	} else {
		ref := model.TargetRef(tr)
		if !ref.IsValid() {
			errs = append(errs, fmt.Sprintf("unknown target_ref %q (use hand_grip, stem_clamp, or reference_bar_clamp)", tr))
		} else {
			p.targetRef = ref
		}
	}

	p.saddleH = model.DefaultIdmatchSaddleHeightMm
	if raw := strings.TrimSpace(v.Get("saddle_h")); raw != "" {
		val, err := parseFiniteFloat("saddle_h", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else if val <= 0 {
			errs = append(errs, fmt.Sprintf("saddle_h must be positive, got %.1f", val))
		} else {
			p.saddleH = val
		}
	}

	if raw := strings.TrimSpace(v.Get("stem_len")); raw != "" {
		val, err := parseFiniteFloat("stem_len", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else if val <= 0 {
			errs = append(errs, fmt.Sprintf("stem_len must be positive, got %.1f", val))
		} else {
			p.hasStemLen = true
			p.stemLen = val
		}
	}

	if raw := strings.TrimSpace(v.Get("stem_ang")); raw != "" {
		val, err := parseFiniteFloat("stem_ang", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else if math.Abs(val) >= 90 {
			errs = append(errs, fmt.Sprintf("stem_ang must satisfy |ang| < 90°, got %.1f", val))
		} else {
			p.hasStemAng = true
			p.stemAng = val
		}
	}

	if raw := strings.TrimSpace(v.Get("spacer")); raw != "" {
		val, err := parseFiniteFloat("spacer", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else if val < 0 {
			errs = append(errs, fmt.Sprintf("spacer must be non-negative, got %.1f", val))
		} else {
			p.hasSpacer = true
			p.spacer = val
		}
	}

	if raw := strings.TrimSpace(v.Get("bar_roll")); raw != "" {
		val, err := parseFiniteFloat("bar_roll", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			p.barRoll = val
		}
	}

	p.postSB = model.DefaultSeatpostSetbackMm
	if raw := strings.TrimSpace(v.Get("post_sb")); raw != "" {
		val, err := parseFiniteFloat("post_sb", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else if val < 0 {
			errs = append(errs, fmt.Sprintf("post_sb must be non-negative, got %.1f", val))
		} else {
			p.postSB = val
		}
	}

	if raw := strings.TrimSpace(v.Get("rail_off")); raw != "" {
		val, err := parseFiniteFloat("rail_off", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			p.hasRailOff = true
			p.railOff = val
		}
	}

	if raw := strings.TrimSpace(v.Get("target_x")); raw != "" {
		val, err := parseFiniteFloat("target_x", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			p.hasTargetX = true
			p.targetX = val
		}
	}

	if raw := strings.TrimSpace(v.Get("target_y")); raw != "" {
		val, err := parseFiniteFloat("target_y", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			p.hasTargetY = true
			p.targetY = val
		}
	}

	return p, errs
}

func parseFiniteFloat(name, raw string) (float64, error) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%s: %q is not a finite number", name, raw)
	}
	return f, nil
}

func defaultSizeForFrame(sizes []string) string {
	for _, s := range sizes {
		if strings.EqualFold(s, "L") {
			return s
		}
	}
	if len(sizes) > 0 {
		return sizes[0]
	}
	return ""
}

func hasExactFold(vals []string, want string) string {
	for _, v := range vals {
		if strings.EqualFold(v, want) {
			return v
		}
	}
	return ""
}

func formatFloatInput(v float64) string {
	if math.Abs(v) < 1e-9 {
		return "0"
	}
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
}
