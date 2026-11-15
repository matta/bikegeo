package web

import (
	"fmt"
	"math"
	"net/http"
	"net/url"

	"bikegeo/internal/frames"
	"bikegeo/internal/model"
)

// NudgeLink is one zero-JS GET hyperlink that shifts a single parameter on /bike.
type NudgeLink struct {
	Label string
	Href  string
	Delta string
}

// NudgeControl groups the current value and incremental GET links for one parameter.
type NudgeControl struct {
	Param string
	Label string
	Value string
	Unit  string
	Links []NudgeLink
}

type bikeData struct {
	base
	Banner         []string
	Ready          bool
	FrameID        string
	Size           string
	Fork           string
	Sag            string
	SaddleID       string
	HandlebarID    string
	TargetRef      string
	TargetRefLabel string
	SaddleH        string
	StemLen        string
	StemAng        string
	Spacer         string
	BarRoll        string
	PostSB         string
	RailOff        string
	TargetX        string
	TargetY        string
	Frames         []Option
	Sizes          []Option
	Forks          []Option
	Sags           []Option
	Saddles        []Option
	Handlebars     []Option
	TargetRefs     []Option
	Nudges         []NudgeControl
	SVG            bikeSVGView
	CockpitSVG     bikeSVGView
	SaddleSVG      bikeSVGView
	SummaryTable   TableView
	LedgerTable    TableView
	Permalink      string
}

type bikeParams struct {
	frameModel model.FrameModel
	frameID    string
	size       string
	fork       string
	sag        string
	saddle     model.SaddleSpec
	handlebar  model.HandlebarSpec
	targetRef  model.TargetRef
	saddleH    float64
	hasStemLen bool
	stemLen    float64
	hasStemAng bool
	stemAng    float64
	hasSpacer  bool
	spacer     float64
	barRoll    float64
	postSB     float64
	hasRailOff bool
	railOff    float64
	hasTargetX bool
	targetX    float64
	hasTargetY bool
	targetY    float64
}

func (s *server) bike(w http.ResponseWriter, r *http.Request) {
	allFrames := frames.AllFrames()
	p, errs := parseBikeQuery(r.URL.Query(), allFrames)
	d, status := buildBikeWorkbench(allFrames, p, errs)
	s.render(w, status, "bike.html", d)
}

func buildBikeNudges(baseVals url.Values, stemLen, stemAng, spacer, barRoll, saddleH, railOff, halfRailWin float64) []NudgeControl {
	specs := []struct {
		param    string
		label    string
		val      float64
		unit     string
		min, max float64
		deltas   []float64
	}{
		{"stem_len", "Stem len", stemLen, "mm", 10, 250, []float64{-10, -5, 5, 10}},
		{"stem_ang", "Stem ang", stemAng, "°", -85, 85, []float64{-10, -5, 5, 10}},
		{"spacer", "Spacers", spacer, "mm", 0, 200, []float64{-10, -5, 5, 10}},
		{"bar_roll", "Bar roll", barRoll, "°", -90, 90, []float64{-10, -5, 5, 10}},
		{"saddle_h", "Saddle H", saddleH, "mm", 100, 1200, []float64{-10, -5, 5, 10}},
		{"rail_off", "Rail offset", railOff, "mm", -halfRailWin, halfRailWin, []float64{-10, -5, 5, 10}},
	}
	out := make([]NudgeControl, 0, len(specs))
	for _, sp := range specs {
		ctrl := NudgeControl{
			Param: sp.param,
			Label: sp.label,
			Value: formatFloatInput(sp.val),
			Unit:  sp.unit,
		}
		for _, delta := range sp.deltas {
			next := math.Max(sp.min, math.Min(sp.max, sp.val+delta))
			q := cloneValues(baseVals)
			q.Set(sp.param, formatFloatInput(next))
			sign := ""
			if delta > 0 {
				sign = "+"
			}
			dLabel := fmt.Sprintf("%s%g", sign, delta)
			ctrl.Links = append(ctrl.Links, NudgeLink{
				Label: dLabel,
				Delta: dLabel,
				Href:  "/bike?" + q.Encode(),
			})
		}
		out = append(out, ctrl)
	}
	return out
}

func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vs := range v {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}
