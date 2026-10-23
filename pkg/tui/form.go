package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
)

// CyclePicker is a form control allowing cycling through a list of choices using Left/Right or Space.
type CyclePicker struct {
	Options       []string
	Labels        []string
	SelectedIndex int
	Focused       bool
	Theme         Theme

	OnChange func(index int, val string)
}

// NewCyclePicker creates an initialized CyclePicker.
func NewCyclePicker(options []string, labels []string) *CyclePicker {
	if len(labels) == 0 {
		labels = options
	}
	return &CyclePicker{
		Options: options,
		Labels:  labels,
		Theme:   DefaultTheme(),
	}
}

// SetSelected sets the selected option by value.
func (cp *CyclePicker) SetSelected(val string) {
	for i, opt := range cp.Options {
		if opt == val {
			cp.SelectedIndex = i
			return
		}
	}
}

// SelectedValue returns the currently active option value.
func (cp *CyclePicker) SelectedValue() string {
	if cp.SelectedIndex >= 0 && cp.SelectedIndex < len(cp.Options) {
		return cp.Options[cp.SelectedIndex]
	}
	return ""
}

// Draw renders the cycle picker.
func (cp *CyclePicker) Draw(s tcell.Screen, bounds Rect) {
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}

	st := cp.Theme.Input
	if cp.Focused {
		st = cp.Theme.InputFocus
	}

	FillRect(s, bounds, st)

	label := ""
	if cp.SelectedIndex >= 0 && cp.SelectedIndex < len(cp.Labels) {
		label = cp.Labels[cp.SelectedIndex]
	}

	text := fmt.Sprintf("◀ %s ▶", label)
	DrawTextTruncated(s, bounds.X, bounds.Y, bounds.W, text, st)
}

// HandleEvent cycles choices with Left, Right, Space.
func (cp *CyclePicker) HandleEvent(ev tcell.Event) EventResult {
	if !cp.Focused {
		return EventIgnored
	}

	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventIgnored
	}

	n := len(cp.Options)
	if n == 0 {
		return EventIgnored
	}

	prev := cp.SelectedIndex

	switch kev.Key() {
	case tcell.KeyLeft:
		cp.SelectedIndex = (cp.SelectedIndex - 1 + n) % n
	case tcell.KeyRight:
		cp.SelectedIndex = (cp.SelectedIndex + 1) % n
	case tcell.KeyRune:
		if kev.Str() == " " {
			cp.SelectedIndex = (cp.SelectedIndex + 1) % n
		} else {
			return EventIgnored
		}
	default:
		return EventIgnored
	}

	if cp.SelectedIndex != prev && cp.OnChange != nil {
		cp.OnChange(cp.SelectedIndex, cp.SelectedValue())
	}

	return EventHandled
}

// FormField represents a single labeled field inside a Form.
type FormField struct {
	ID       string
	Label    string
	Hint     string
	Widget   Widget
	IsButton bool
}

// Form manages multiple labeled input fields with Tab/Shift+Tab navigation.
type Form struct {
	Fields       []FormField
	FocusedIndex int
	Theme        Theme

	OnSubmit func()
	OnCancel func()
}

// NewForm creates an initialized Form.
func NewForm() *Form {
	return &Form{
		Theme: DefaultTheme(),
	}
}

// AddField appends a field to the form.
func (f *Form) AddField(field FormField) {
	f.Fields = append(f.Fields, field)
	f.updateFocus()
}

// FocusField sets focus to a field by ID.
func (f *Form) FocusField(id string) {
	for i, field := range f.Fields {
		if field.ID == id {
			f.FocusedIndex = i
			f.updateFocus()
			return
		}
	}
}

func (f *Form) updateFocus() {
	for i, fld := range f.Fields {
		focused := i == f.FocusedIndex
		switch w := fld.Widget.(type) {
		case *TextInput:
			w.Focused = focused
		case *CyclePicker:
			w.Focused = focused
		}
	}
}

// Draw renders the form fields vertically within bounds.
func (f *Form) Draw(s tcell.Screen, bounds Rect) {
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}

	labelW := 18
	for _, fld := range f.Fields {
		if len([]rune(fld.Label)) > labelW-2 {
			labelW = len([]rune(fld.Label)) + 2
		}
	}
	if labelW > bounds.W/2 {
		labelW = bounds.W / 2
	}

	for i, fld := range f.Fields {
		y := bounds.Y + i
		if y >= bounds.Y+bounds.H {
			break
		}

		isFocused := i == f.FocusedIndex
		lblStyle := f.Theme.Label
		if isFocused {
			lblStyle = f.Theme.Focus
		}

		if fld.IsButton {
			btnBounds := Rect{X: bounds.X, Y: y, W: bounds.W, H: 1}
			btnStyle := f.Theme.Button
			if isFocused {
				btnStyle = f.Theme.ButtonFocus
			}
			FillRect(s, btnBounds, btnStyle)
			DrawText(s, bounds.X+2, y, fld.Label, btnStyle)
			continue
		}

		// Draw label
		DrawTextTruncated(s, bounds.X, y, labelW, fld.Label, lblStyle)

		// Draw widget
		widgetX := bounds.X + labelW
		widgetW := bounds.W - labelW
		hintW := 0
		if fld.Hint != "" {
			hintW = len([]rune(fld.Hint)) + 1
			if hintW > widgetW/2 {
				hintW = widgetW / 2
			}
			widgetW -= hintW
		}

		if widgetW > 0 {
			fld.Widget.Draw(s, Rect{X: widgetX, Y: y, W: widgetW, H: 1})
		}

		if hintW > 0 {
			DrawText(s, widgetX+widgetW+1, y, fld.Hint, f.Theme.Muted)
		}
	}
}

// HandleEvent handles Tab/Shift+Tab and delegates to focused field.
func (f *Form) HandleEvent(ev tcell.Event) EventResult {
	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventIgnored
	}

	n := len(f.Fields)
	if n == 0 {
		return EventIgnored
	}

	switch kev.Key() {
	case tcell.KeyTab:
		f.FocusedIndex = (f.FocusedIndex + 1) % n
		f.updateFocus()
		return EventHandled
	case tcell.KeyBacktab:
		f.FocusedIndex = (f.FocusedIndex - 1 + n) % n
		f.updateFocus()
		return EventHandled
	case tcell.KeyDown:
		// If focused widget is not consuming down arrow
		f.FocusedIndex = (f.FocusedIndex + 1) % n
		f.updateFocus()
		return EventHandled
	case tcell.KeyUp:
		// If focused widget is not consuming up arrow
		f.FocusedIndex = (f.FocusedIndex - 1 + n) % n
		f.updateFocus()
		return EventHandled
	case tcell.KeyEnter:
		// If on a button
		if f.FocusedIndex >= 0 && f.FocusedIndex < n && f.Fields[f.FocusedIndex].IsButton {
			if f.OnSubmit != nil {
				f.OnSubmit()
				return EventHandled
			}
		}
	case tcell.KeyEscape:
		if f.OnCancel != nil {
			f.OnCancel()
			return EventHandled
		}
	}

	// Delegate to the focused widget
	if f.FocusedIndex >= 0 && f.FocusedIndex < n {
		res := f.Fields[f.FocusedIndex].Widget.HandleEvent(ev)
		if res == EventHandled {
			return EventHandled
		}
	}

	return EventIgnored
}
