package ideals

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v3"

	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
	"bikegeo/pkg/tui"
)

// modalTwoPaneLayout arranges form controls on the left and live curve analysis on the right.
type modalTwoPaneLayout struct {
	Form      *tui.Form
	Inspector *CurveInspector
	Theme     tui.Theme
}

func (l *modalTwoPaneLayout) Draw(s tcell.Screen, bounds tui.Rect) {
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}

	leftW := 38
	if bounds.W < 80 {
		leftW = 34
	}
	if leftW > bounds.W/2 {
		leftW = bounds.W / 2
	}

	leftRect := tui.Rect{X: bounds.X, Y: bounds.Y, W: leftW, H: bounds.H}
	l.Form.Draw(s, leftRect)

	divX := bounds.X + leftW + 1
	if divX < bounds.X+bounds.W-8 {
		for y := bounds.Y; y < bounds.Y+bounds.H; y++ {
			tui.DrawText(s, divX, y, "│", l.Theme.Border)
		}

		rightX := divX + 2
		rightW := bounds.X + bounds.W - rightX
		if rightW > 0 {
			rightRect := tui.Rect{X: rightX, Y: bounds.Y, W: rightW, H: bounds.H}
			l.Inspector.Draw(s, rightRect)
		}
	}
}

func (l *modalTwoPaneLayout) HandleEvent(ev tcell.Event) tui.EventResult {
	if l.Form != nil {
		return l.Form.HandleEvent(ev)
	}
	return tui.EventIgnored
}

// AttributeModal allows editing all fields of a single AttributeSpec.
type AttributeModal struct {
	Spec      ideal.AttributeSpec
	IsNew     bool
	Theme     tui.Theme
	Modal     *tui.ModalDialog
	Form      *tui.Form
	Inspector *CurveInspector
	Layout    *modalTwoPaneLayout

	idPicker      *tui.CyclePicker
	weightInput   *tui.TextInput
	idealMinInput *tui.TextInput
	idealMaxInput *tui.TextInput
	accMinInput   *tui.TextInput
	accMaxInput   *tui.TextInput
	edgeInput     *tui.TextInput
	fallBelow     *tui.TextInput
	fallAbove     *tui.TextInput
	floorInput    *tui.TextInput
	probeInput    *tui.TextInput

	allKnown []ideal.AttributeMeta

	OnSave   func(updated ideal.AttributeSpec)
	OnCancel func()
}

// SetBikeRows configures fleet and comparison datasets for live curve evaluation.
func (am *AttributeModal) SetBikeRows(fleet, compare []model.CanonicalComparisonRow) {
	if am.Inspector != nil {
		am.Inspector.SetBikeRows(fleet, compare)
	}
}

// NewAttributeModal creates an attribute editor modal.
func NewAttributeModal(spec ideal.AttributeSpec, isNew bool, theme tui.Theme) *AttributeModal {
	known := ideal.AllAttributes()
	var optIDs []string
	var optLabels []string
	for _, a := range known {
		optIDs = append(optIDs, a.ID)
		optLabels = append(optLabels, fmt.Sprintf("%s (%s [%s])", a.Label, a.ID, a.Unit))
	}

	am := &AttributeModal{
		Spec:     spec,
		IsNew:    isNew,
		Theme:    theme,
		allKnown: known,
	}

	title := "Edit Attribute: " + spec.ID
	if isNew {
		title = "Add Attribute"
	}

	am.Modal = tui.NewModalDialog(title, 104, 26)
	am.Modal.Theme = theme

	// Build form controls
	am.idPicker = tui.NewCyclePicker(optIDs, optLabels)
	if spec.ID != "" {
		am.idPicker.SetSelected(spec.ID)
	}

	meta, _ := ideal.LookupAttribute(am.idPicker.SelectedValue())
	unit := meta.Unit
	if unit == "" {
		unit = "mm"
	}

	am.weightInput = tui.NewTextInput(fmt.Sprintf("%.0f", spec.Weight))
	if spec.Weight == 0 {
		am.weightInput.SetText("0")
	}

	var idealMin, idealMax string
	if spec.Ideal != nil {
		if spec.Ideal.Min != nil {
			idealMin = fmtFloat(*spec.Ideal.Min)
		}
		if spec.Ideal.Max != nil {
			idealMax = fmtFloat(*spec.Ideal.Max)
		}
	}
	am.idealMinInput = tui.NewTextInput(idealMin)
	am.idealMaxInput = tui.NewTextInput(idealMax)

	var accMin, accMax string
	if spec.Acceptable != nil {
		if spec.Acceptable.Min != nil {
			accMin = fmtFloat(*spec.Acceptable.Min)
		}
		if spec.Acceptable.Max != nil {
			accMax = fmtFloat(*spec.Acceptable.Max)
		}
	}
	am.accMinInput = tui.NewTextInput(accMin)
	am.accMaxInput = tui.NewTextInput(accMax)

	var edgeStr string
	if spec.EdgeScore != nil {
		edgeStr = fmtFloat(*spec.EdgeScore)
	}
	am.edgeInput = tui.NewTextInput(edgeStr)

	var fbStr, faStr string
	if spec.Falloff != nil {
		if spec.Falloff.Below != nil {
			fbStr = fmtFloat(*spec.Falloff.Below)
		}
		if spec.Falloff.Above != nil {
			faStr = fmtFloat(*spec.Falloff.Above)
		}
	}
	am.fallBelow = tui.NewTextInput(fbStr)
	am.fallAbove = tui.NewTextInput(faStr)

	var flStr string
	if spec.Floor != nil {
		flStr = fmtFloat(*spec.Floor)
	}
	am.floorInput = tui.NewTextInput(flStr)
	am.probeInput = tui.NewTextInput("")

	form := tui.NewForm()
	form.Theme = theme

	form.AddField(tui.FormField{ID: "id", Label: "Attribute", Widget: am.idPicker})
	form.AddField(tui.FormField{ID: "weight", Label: "Weight", Hint: "(0 = display only)", Widget: am.weightInput})
	form.AddField(tui.FormField{ID: "ideal_min", Label: "Ideal Min", Hint: unit, Widget: am.idealMinInput})
	form.AddField(tui.FormField{ID: "ideal_max", Label: "Ideal Max", Hint: unit, Widget: am.idealMaxInput})
	form.AddField(tui.FormField{ID: "acc_min", Label: "Acceptable Min", Hint: unit, Widget: am.accMinInput})
	form.AddField(tui.FormField{ID: "acc_max", Label: "Acceptable Max", Hint: unit, Widget: am.accMaxInput})
	form.AddField(tui.FormField{ID: "edge", Label: "Edge Score", Hint: "(0..100]", Widget: am.edgeInput})
	form.AddField(tui.FormField{ID: "fall_below", Label: "Falloff Below", Hint: unit + " (>0)", Widget: am.fallBelow})
	form.AddField(tui.FormField{ID: "fall_above", Label: "Falloff Above", Hint: unit + " (>0)", Widget: am.fallAbove})
	form.AddField(tui.FormField{ID: "floor", Label: "Score Floor", Hint: "(<0, e.g. -100)", Widget: am.floorInput})
	form.AddField(tui.FormField{ID: "probe", Label: "Probe Value", Hint: unit + " (test)", Widget: am.probeInput})

	am.Form = form
	am.Inspector = NewCurveInspector(spec, meta, theme)
	am.Layout = &modalTwoPaneLayout{
		Form:      form,
		Inspector: am.Inspector,
		Theme:     theme,
	}
	am.Modal.Content = am.Layout

	// Update unit hints on ID change
	am.idPicker.OnChange = func(idx int, val string) {
		m, _ := ideal.LookupAttribute(val)
		u := m.Unit
		if u == "" {
			u = "mm"
		}
		for i := range form.Fields {
			switch form.Fields[i].ID {
			case "ideal_min", "ideal_max", "acc_min", "acc_max", "fall_below", "fall_above":
				form.Fields[i].Hint = u
			case "probe":
				form.Fields[i].Hint = u + " (test)"
			}
		}
		am.validateLive()
	}

	changeHook := func(string) {
		am.validateLive()
	}
	am.weightInput.OnChange = changeHook
	am.idealMinInput.OnChange = changeHook
	am.idealMaxInput.OnChange = changeHook
	am.accMinInput.OnChange = changeHook
	am.accMaxInput.OnChange = changeHook
	am.edgeInput.OnChange = changeHook
	am.fallBelow.OnChange = changeHook
	am.fallAbove.OnChange = changeHook
	am.floorInput.OnChange = changeHook
	am.probeInput.OnChange = changeHook

	am.Modal.Buttons = []tui.ModalButton{
		{
			Label: "Save",
			Action: func() {
				if attr, err := am.BuildSpec(); err == nil {
					if am.OnSave != nil {
						am.OnSave(attr)
					}
				} else {
					am.Modal.StatusMsg = err.Error()
					am.Modal.StatusIsWarn = true
				}
			},
		},
		{
			Label: "Cancel",
			Action: func() {
				if am.OnCancel != nil {
					am.OnCancel()
				}
			},
		},
		{
			Label: "Toggle Fleet (F2)",
			Action: func() {
				if am.Inspector != nil {
					am.Inspector.ToggleDataset()
				}
			},
		},
	}
	am.Modal.OnClose = func() {
		if am.OnCancel != nil {
			am.OnCancel()
		}
	}

	am.validateLive()
	return am
}

func fmtFloat(v float64) string {
	if v == float64(int(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

func parseOptFloat(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number %q", s)
	}
	return &v, nil
}

// BuildSpec constructs and validates the AttributeSpec from the modal inputs.
func (am *AttributeModal) BuildSpec() (ideal.AttributeSpec, error) {
	id := am.idPicker.SelectedValue()
	if id == "" {
		return ideal.AttributeSpec{}, fmt.Errorf("attribute ID cannot be empty")
	}

	weightVal, err := strconv.ParseFloat(strings.TrimSpace(am.weightInput.Text), 64)
	if err != nil {
		weightVal = 0
	}
	if weightVal < 0 {
		return ideal.AttributeSpec{}, fmt.Errorf("weight must be non-negative")
	}

	idealMin, err := parseOptFloat(am.idealMinInput.Text)
	if err != nil {
		return ideal.AttributeSpec{}, fmt.Errorf("ideal min: %w", err)
	}
	idealMax, err := parseOptFloat(am.idealMaxInput.Text)
	if err != nil {
		return ideal.AttributeSpec{}, fmt.Errorf("ideal max: %w", err)
	}

	accMin, err := parseOptFloat(am.accMinInput.Text)
	if err != nil {
		return ideal.AttributeSpec{}, fmt.Errorf("acceptable min: %w", err)
	}
	accMax, err := parseOptFloat(am.accMaxInput.Text)
	if err != nil {
		return ideal.AttributeSpec{}, fmt.Errorf("acceptable max: %w", err)
	}

	edge, err := parseOptFloat(am.edgeInput.Text)
	if err != nil {
		return ideal.AttributeSpec{}, fmt.Errorf("edge score: %w", err)
	}

	fallBelow, err := parseOptFloat(am.fallBelow.Text)
	if err != nil {
		return ideal.AttributeSpec{}, fmt.Errorf("falloff below: %w", err)
	}
	fallAbove, err := parseOptFloat(am.fallAbove.Text)
	if err != nil {
		return ideal.AttributeSpec{}, fmt.Errorf("falloff above: %w", err)
	}

	floor, err := parseOptFloat(am.floorInput.Text)
	if err != nil {
		return ideal.AttributeSpec{}, fmt.Errorf("score floor: %w", err)
	}

	spec := ideal.AttributeSpec{
		ID:     id,
		Weight: weightVal,
	}

	if idealMin != nil || idealMax != nil {
		spec.Ideal = &ideal.Range{Min: idealMin, Max: idealMax}
	}
	if accMin != nil || accMax != nil {
		spec.Acceptable = &ideal.Range{Min: accMin, Max: accMax}
	}
	spec.EdgeScore = edge
	if fallBelow != nil || fallAbove != nil {
		spec.Falloff = &ideal.Falloff{Below: fallBelow, Above: fallAbove}
	}
	spec.Floor = floor

	// Validate spec against domain rules
	testProf := &ideal.Profile{
		Name:       "validation_probe",
		Attributes: []ideal.AttributeSpec{spec},
	}
	if spec.Weight == 0 {
		// Provide dummy weighted attribute to satisfy profile level constraint
		testProf.Attributes = append(testProf.Attributes, ideal.AttributeSpec{
			ID:      "aett",
			Weight:  1,
			Ideal:   &ideal.Range{Min: ptrFloat(600), Max: ptrFloat(600)},
			Falloff: &ideal.Falloff{Below: ptrFloat(10), Above: ptrFloat(10)},
		})
	}
	if err := ideal.ValidateProfile(testProf); err != nil {
		return spec, err
	}

	return spec, nil
}

func (am *AttributeModal) validateLive() {
	spec, err := am.BuildSpec()
	meta, _ := ideal.LookupAttribute(am.idPicker.SelectedValue())

	if am.probeInput != nil {
		if val, err := parseOptFloat(am.probeInput.Text); err == nil {
			am.Inspector.SetProbeVal(val)
		} else {
			am.Inspector.SetProbeVal(nil)
		}
	}

	if am.Inspector != nil {
		am.Inspector.SetSpec(spec, meta)
	}

	if err != nil {
		am.Modal.StatusMsg = "⚠ " + err.Error()
		am.Modal.StatusIsWarn = true
	} else {
		am.Modal.StatusMsg = "✓ Valid attribute parameters"
		am.Modal.StatusIsWarn = false
	}
}

// Init view interface.
func (am *AttributeModal) Init(theme tui.Theme) {
	am.Theme = theme
	am.Modal.Theme = theme
	am.Form.Theme = theme
	if am.Inspector != nil {
		am.Inspector.Theme = theme
	}
	if am.Layout != nil {
		am.Layout.Theme = theme
	}
}

// Draw renders modal overlay.
func (am *AttributeModal) Draw(s tcell.Screen, bounds tui.Rect) {
	am.Modal.Draw(s, bounds)
}

// HandleEvent dispatches events to modal.
func (am *AttributeModal) HandleEvent(ev tcell.Event) tui.EventResult {
	if kev, ok := ev.(*tcell.EventKey); ok {
		if kev.Key() == tcell.KeyEscape {
			if am.OnCancel != nil {
				am.OnCancel()
				return tui.EventHandled
			}
		}
		if kev.Key() == tcell.KeyCtrlS {
			if attr, err := am.BuildSpec(); err == nil {
				if am.OnSave != nil {
					am.OnSave(attr)
					return tui.EventHandled
				}
			}
		}
		if kev.Key() == tcell.KeyF2 || kev.Key() == tcell.KeyCtrlF {
			if am.Inspector != nil {
				am.Inspector.ToggleDataset()
				return tui.EventHandled
			}
		}
	}
	return am.Modal.HandleEvent(ev)
}
