package ideals

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v3"

	"bikegeo/pkg/ideal"
	"bikegeo/pkg/tui"
)

// FocusSection indicates whether header metadata or attribute list is focused.
type FocusSection int

const (
	FocusHeader FocusSection = iota
	FocusAttributes
)

// ProfileEditScreen is the primary screen for viewing and editing an ideal profile.
type ProfileEditScreen struct {
	App       *tui.App
	Session   *Session
	Theme     tui.Theme
	StatusBar *tui.StatusBar

	// Header inputs
	nameInput   *tui.TextInput
	descInput   *tui.TextInput
	saddleInput *tui.TextInput
	headerFocus int // 0: name, 1: desc, 2: saddle

	// Section focus
	Section FocusSection

	// Attribute list
	selectedAttrIdx int
	scrollOffset    int

	statusMsg   string
	statusIsErr bool

	OnBack func()
}

// NewProfileEditScreen creates an initialized ProfileEditScreen.
func NewProfileEditScreen(app *tui.App, session *Session) *ProfileEditScreen {
	p := session.Working
	shStr := ""
	if p.SaddleHeightMm != nil {
		shStr = fmt.Sprintf("%.0f", *p.SaddleHeightMm)
	}

	screen := &ProfileEditScreen{
		App:             app,
		Session:         session,
		Theme:           tui.DefaultTheme(),
		nameInput:       tui.NewTextInput(p.Name),
		descInput:       tui.NewTextInput(p.Description),
		saddleInput:     tui.NewTextInput(shStr),
		headerFocus:     0,
		Section:         FocusAttributes,
		selectedAttrIdx: 0,
	}

	screen.nameInput.OnChange = func(s string) {
		session.Working.Name = strings.TrimSpace(s)
		screen.updateValidationStatus()
	}
	screen.descInput.OnChange = func(s string) {
		session.Working.Description = s
	}
	screen.saddleInput.OnChange = func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			session.Working.SaddleHeightMm = nil
		} else if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 {
			session.Working.SaddleHeightMm = &v
		}
		screen.updateValidationStatus()
	}

	screen.StatusBar = tui.NewStatusBar([]tui.KeyHint{
		{Key: "[Enter]", Desc: "Edit"},
		{Key: "[a]", Desc: "Add"},
		{Key: "[d]", Desc: "Del"},
		{Key: "[K/J]", Desc: "Move"},
		{Key: "[Tab]", Desc: "Section"},
		{Key: "[s]", Desc: "Save"},
		{Key: "[^A]", Desc: "SaveAs"},
		{Key: "[q]", Desc: "Exit"},
	})

	screen.updateValidationStatus()
	return screen
}

// Init sets theme.
func (pe *ProfileEditScreen) Init(theme tui.Theme) {
	pe.Theme = theme
	pe.StatusBar.Theme = theme
	pe.nameInput.Theme = theme
	pe.descInput.Theme = theme
	pe.saddleInput.Theme = theme
}

func (pe *ProfileEditScreen) updateValidationStatus() {
	if err := pe.Session.Validate(); err != nil {
		pe.statusMsg = "⚠ " + err.Error()
		pe.statusIsErr = true
	} else {
		pe.statusMsg = "✓ Valid Profile"
		pe.statusIsErr = false
	}
}

// Draw renders header, attribute table, and status bar.
const (
	colAttrW   = 26
	colWeightW = 8
	colIdealW  = 16
	colAccW    = 16
	colEdgeW   = 6
	colFallW   = 14
	colFloorW  = 7
)

func padRune(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return s + strings.Repeat(" ", width-len(runes))
}

// Draw renders header, attribute table, and status bar.
func (pe *ProfileEditScreen) Draw(s tcell.Screen, bounds tui.Rect) {
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}

	// 0. Completely clear background to prevent bleed-through
	tui.FillRect(s, bounds, pe.Theme.Default)

	// 1. Title bar (row 0)
	title := fmt.Sprintf(" IDEAL PROFILE: %s ", pe.Session.Working.Name)
	if pe.Session.IsDirty() {
		title += "[MODIFIED *] "
	}
	tui.FillRect(s, tui.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: 1}, pe.Theme.Selected)
	tui.DrawText(s, bounds.X+1, bounds.Y, title, pe.Theme.Selected)

	// 2. Header Metadata Box (rows 1..5)
	headerBox := tui.Rect{X: bounds.X, Y: bounds.Y + 1, W: bounds.W, H: 5}
	tui.FillRect(s, headerBox, pe.Theme.Default)
	tui.DrawBox(s, headerBox, "Profile Metadata", pe.Theme.Border)

	// Metadata fields
	labelStyle := pe.Theme.Label
	if pe.Section == FocusHeader {
		labelStyle = pe.Theme.Focus
	}

	tui.DrawText(s, bounds.X+2, bounds.Y+2, "Name:", labelStyle)
	pe.nameInput.Focused = (pe.Section == FocusHeader && pe.headerFocus == 0)
	pe.nameInput.Draw(s, tui.Rect{X: bounds.X+8, Y: bounds.Y+2, W: 20, H: 1})

	tui.DrawText(s, bounds.X+30, bounds.Y+2, "Saddle Ht (mm):", labelStyle)
	pe.saddleInput.Focused = (pe.Section == FocusHeader && pe.headerFocus == 1)
	pe.saddleInput.Draw(s, tui.Rect{X: bounds.X+46, Y: bounds.Y+2, W: 10, H: 1})

	tui.DrawText(s, bounds.X+2, bounds.Y+3, "Description:", labelStyle)
	pe.descInput.Focused = (pe.Section == FocusHeader && pe.headerFocus == 2)
	descW := bounds.W - 18
	if descW < 10 {
		descW = 10
	}
	pe.descInput.Draw(s, tui.Rect{X: bounds.X+15, Y: bounds.Y+3, W: descW, H: 1})

	// 3. Attribute Table (rows 6 .. bounds.H - 2)
	tableY := bounds.Y + 6
	tableH := bounds.H - 7
	if tableH < 4 {
		tableH = 4
	}

	tableBox := tui.Rect{X: bounds.X, Y: tableY, W: bounds.W, H: tableH}
	tui.FillRect(s, tableBox, pe.Theme.Default)
	tui.DrawBox(s, tableBox, fmt.Sprintf("Attributes (%d)", len(pe.Session.Working.Attributes)), pe.Theme.Border)

	// Column header
	colY := tableY + 1
	headerLine := formatTableHeader(bounds.W - 4)
	tui.DrawText(s, bounds.X+2, colY, headerLine, pe.Theme.Header)
	tui.DrawText(s, bounds.X+2, colY+1, strings.Repeat("─", bounds.W-4), pe.Theme.Border)

	// Rows viewport
	rowStartY := colY + 2
	viewportH := tableH - 3
	attrs := pe.Session.Working.Attributes

	// Adjust scroll offset
	if pe.selectedAttrIdx < pe.scrollOffset {
		pe.scrollOffset = pe.selectedAttrIdx
	}
	if pe.selectedAttrIdx >= pe.scrollOffset+viewportH {
		pe.scrollOffset = pe.selectedAttrIdx - viewportH + 1
	}
	if pe.scrollOffset < 0 {
		pe.scrollOffset = 0
	}

	for i := 0; i < viewportH; i++ {
		idx := pe.scrollOffset + i
		y := rowStartY + i
		if y >= tableY+tableH-1 {
			break
		}

		if idx >= len(attrs) {
			tui.FillRect(s, tui.Rect{X: bounds.X + 2, Y: y, W: bounds.W - 4, H: 1}, pe.Theme.Default)
			continue
		}

		attr := attrs[idx]
		isSelected := (idx == pe.selectedAttrIdx && pe.Section == FocusAttributes)
		st := pe.Theme.Default
		prefix := "  "
		if isSelected {
			st = pe.Theme.Selected
			prefix = "▶ "
		}

		rowText := prefix + formatAttributeRow(attr, bounds.W-6)
		tui.FillRect(s, tui.Rect{X: bounds.X + 2, Y: y, W: bounds.W - 4, H: 1}, st)
		tui.DrawTextTruncated(s, bounds.X+2, y, bounds.W-4, rowText, st)
	}

	// 4. Status Bar (bottom row)
	pe.StatusBar.IsDirty = pe.Session.IsDirty()
	pe.StatusBar.Message = pe.statusMsg
	pe.StatusBar.IsError = pe.statusIsErr
	pe.StatusBar.Draw(s, bounds)
}

func formatTableHeader(w int) string {
	parts := []string{
		padRune("Attribute", colAttrW),
		padRune("Weight", colWeightW),
		padRune("Ideal Range", colIdealW),
		padRune("Acceptable", colAccW),
		padRune("Edge", colEdgeW),
		padRune("Falloff", colFallW),
		padRune("Floor", colFloorW),
	}
	return "  " + strings.Join(parts, " ")
}

func formatAttributeRow(attr ideal.AttributeSpec, w int) string {
	meta, _ := ideal.LookupAttribute(attr.ID)
	label := meta.Label
	if label == "" {
		label = attr.ID
	}
	unit := meta.Unit
	dec := meta.Decimals

	weightStr := "display"
	if attr.Weight > 0 {
		weightStr = fmt.Sprintf("%.0f", attr.Weight)
	}

	idealStr := ideal.FormatRange(attr.Ideal, unit, dec)
	if idealStr == "" {
		idealStr = "—"
	}

	accStr := ideal.FormatRange(attr.Acceptable, unit, dec)
	if accStr == "" {
		accStr = "—"
	}

	edgeStr := "—"
	if attr.EdgeScore != nil {
		edgeStr = fmt.Sprintf("%.0f", *attr.EdgeScore)
	}

	fallStr := "—"
	if attr.Falloff != nil {
		var parts []string
		if attr.Falloff.Below != nil {
			parts = append(parts, fmt.Sprintf("-%.0f", *attr.Falloff.Below))
		} else {
			parts = append(parts, "—")
		}
		if attr.Falloff.Above != nil {
			parts = append(parts, fmt.Sprintf("+%.0f", *attr.Falloff.Above))
		} else {
			parts = append(parts, "—")
		}
		fallStr = strings.Join(parts, "/")
	}

	floorStr := "—"
	if attr.Floor != nil {
		floorStr = fmt.Sprintf("%.0f", *attr.Floor)
	}

	attrCol := fmt.Sprintf("%s (%s)", label, attr.ID)
	parts := []string{
		padRune(attrCol, colAttrW),
		padRune(weightStr, colWeightW),
		padRune(idealStr, colIdealW),
		padRune(accStr, colAccW),
		padRune(edgeStr, colEdgeW),
		padRune(fallStr, colFallW),
		padRune(floorStr, colFloorW),
	}
	return strings.Join(parts, " ")
}

// HandleEvent dispatches keyboard input across sections, shortcuts, and modals.
func (pe *ProfileEditScreen) HandleEvent(ev tcell.Event) tui.EventResult {
	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return tui.EventIgnored
	}

	// Global Hotkeys
	switch kev.Key() {
	case tcell.KeyCtrlS:
		pe.saveProfile()
		return tui.EventHandled
	case tcell.KeyCtrlA:
		pe.promptSaveAs()
		return tui.EventHandled
	case tcell.KeyTab:
		// Switch between Header and Attributes section
		if pe.Section == FocusHeader {
			pe.headerFocus = (pe.headerFocus + 1) % 3
			if pe.headerFocus == 0 {
				pe.Section = FocusAttributes
			}
		} else {
			pe.Section = FocusHeader
			pe.headerFocus = 0
		}
		return tui.EventHandled
	case tcell.KeyBacktab:
		if pe.Section == FocusHeader {
			if pe.headerFocus > 0 {
				pe.headerFocus--
			} else {
				pe.Section = FocusAttributes
			}
		} else {
			pe.Section = FocusHeader
			pe.headerFocus = 2
		}
		return tui.EventHandled
	}

	// Header navigation
	if pe.Section == FocusHeader {
		if kev.Key() == tcell.KeyDown {
			pe.Section = FocusAttributes
			return tui.EventHandled
		}
		if kev.Key() == tcell.KeyEscape {
			pe.Section = FocusAttributes
			return tui.EventHandled
		}

		var curInput *tui.TextInput
		switch pe.headerFocus {
		case 0:
			curInput = pe.nameInput
		case 1:
			curInput = pe.saddleInput
		case 2:
			curInput = pe.descInput
		}
		if curInput != nil {
			return curInput.HandleEvent(ev)
		}
		return tui.EventIgnored
	}

	// Attribute list navigation
	n := len(pe.Session.Working.Attributes)
	switch kev.Key() {
	case tcell.KeyUp:
		if pe.selectedAttrIdx > 0 {
			pe.selectedAttrIdx--
		} else {
			pe.Section = FocusHeader
			pe.headerFocus = 0
		}
		return tui.EventHandled
	case tcell.KeyDown:
		if pe.selectedAttrIdx < n-1 {
			pe.selectedAttrIdx++
		}
		return tui.EventHandled
	case tcell.KeyEnter:
		if n > 0 && pe.selectedAttrIdx >= 0 && pe.selectedAttrIdx < n {
			pe.openEditAttributeModal(pe.selectedAttrIdx)
			return tui.EventHandled
		}
	case tcell.KeyEscape:
		pe.confirmExit()
		if pe.App.CurrentView() == nil {
			return tui.EventQuit
		}
		return tui.EventHandled
	case tcell.KeyRune:
		switch kev.Str() {
		case "k":
			if pe.selectedAttrIdx > 0 {
				pe.selectedAttrIdx--
				return tui.EventHandled
			}
		case "j":
			if pe.selectedAttrIdx < n-1 {
				pe.selectedAttrIdx++
				return tui.EventHandled
			}
		case "K": // Move attribute Up
			if pe.selectedAttrIdx > 0 {
				pe.Session.MoveAttribute(pe.selectedAttrIdx, pe.selectedAttrIdx-1)
				pe.selectedAttrIdx--
				pe.updateValidationStatus()
				return tui.EventHandled
			}
		case "J": // Move attribute Down
			if pe.selectedAttrIdx < n-1 {
				pe.Session.MoveAttribute(pe.selectedAttrIdx, pe.selectedAttrIdx+1)
				pe.selectedAttrIdx++
				pe.updateValidationStatus()
				return tui.EventHandled
			}
		case "a", "n":
			pe.openAddAttributeModal()
			return tui.EventHandled
		case "d":
			if n > 0 && pe.selectedAttrIdx >= 0 && pe.selectedAttrIdx < n {
				pe.Session.RemoveAttribute(pe.selectedAttrIdx)
				if pe.selectedAttrIdx >= len(pe.Session.Working.Attributes) {
					pe.selectedAttrIdx = len(pe.Session.Working.Attributes) - 1
				}
				if pe.selectedAttrIdx < 0 {
					pe.selectedAttrIdx = 0
				}
				pe.updateValidationStatus()
				return tui.EventHandled
			}
		case "s":
			pe.saveProfile()
			return tui.EventHandled
		case "q":
			pe.confirmExit()
			if pe.App.CurrentView() == nil {
				return tui.EventQuit
			}
			return tui.EventHandled
		}
	}

	return tui.EventIgnored
}

func (pe *ProfileEditScreen) openEditAttributeModal(idx int) {
	attr := pe.Session.Working.Attributes[idx]
	modal := NewAttributeModal(attr, false, pe.Theme)
	modal.OnSave = func(updated ideal.AttributeSpec) {
		pe.Session.Working.Attributes[idx] = updated
		pe.updateValidationStatus()
		pe.App.PopView()
	}
	modal.OnCancel = func() {
		pe.App.PopView()
	}
	pe.App.PushView(modal)
}

func (pe *ProfileEditScreen) openAddAttributeModal() {
	newSpec := ideal.AttributeSpec{
		ID:     "bbdrop",
		Weight: 5,
		Ideal:  &ideal.Range{Min: ptrFloat(70), Max: ptrFloat(70)},
		Acceptable: &ideal.Range{Min: ptrFloat(60), Max: ptrFloat(80)},
		EdgeScore:  ptrFloat(75),
		Falloff:    &ideal.Falloff{Below: ptrFloat(10), Above: ptrFloat(10)},
	}
	modal := NewAttributeModal(newSpec, true, pe.Theme)
	modal.OnSave = func(updated ideal.AttributeSpec) {
		pe.Session.AddAttribute(updated)
		pe.selectedAttrIdx = len(pe.Session.Working.Attributes) - 1
		pe.updateValidationStatus()
		pe.App.PopView()
	}
	modal.OnCancel = func() {
		pe.App.PopView()
	}
	pe.App.PushView(modal)
}

func (pe *ProfileEditScreen) saveProfile() {
	if err := pe.Session.Save(); err != nil {
		errModal := tui.NewModalDialog("Cannot Save Profile", 50, 8)
		errModal.Message = err.Error()
		errModal.Buttons = []tui.ModalButton{
			{Label: "OK", Action: func() { pe.App.PopView() }},
		}
		pe.App.PushView(errModal)
		return
	}
	pe.statusMsg = fmt.Sprintf("✓ Saved to %s.json", pe.Session.Working.Name)
	pe.statusIsErr = false
}

func (pe *ProfileEditScreen) promptSaveAs() {
	saveAsModal := tui.NewModalDialog("Save Profile As", 50, 10)
	nameInput := tui.NewTextInput(pe.Session.Working.Name + "_copy")
	nameInput.Focused = true

	form := tui.NewForm()
	form.AddField(tui.FormField{ID: "name", Label: "New Profile Name", Widget: nameInput})
	saveAsModal.Content = form

	saveAsModal.Buttons = []tui.ModalButton{
		{
			Label: "Save",
			Action: func() {
				newName := strings.TrimSpace(nameInput.Text)
				if err := pe.Session.SaveAs(newName); err != nil {
					saveAsModal.StatusMsg = err.Error()
					saveAsModal.StatusIsWarn = true
					return
				}
				pe.App.PopView()
				pe.statusMsg = fmt.Sprintf("✓ Saved as %s.json", newName)
				pe.statusIsErr = false
			},
		},
		{
			Label: "Cancel",
			Action: func() {
				pe.App.PopView()
			},
		},
	}
	pe.App.PushView(saveAsModal)
}

func (pe *ProfileEditScreen) confirmExit() {
	if !pe.Session.IsDirty() {
		if pe.OnBack != nil {
			pe.OnBack()
		} else {
			pe.App.PopView()
		}
		return
	}

	exitModal := tui.NewModalDialog("Unsaved Changes", 52, 10)
	exitModal.Message = "You have unsaved changes.\nDo you want to save before leaving?"
	exitModal.Buttons = []tui.ModalButton{
		{
			Label: "Save & Exit",
			Action: func() {
				if err := pe.Session.Save(); err == nil {
					pe.App.PopView() // pop modal
					if pe.OnBack != nil {
						pe.OnBack()
					} else {
						pe.App.PopView()
					}
				} else {
					exitModal.StatusMsg = err.Error()
					exitModal.StatusIsWarn = true
				}
			},
		},
		{
			Label: "Discard",
			Action: func() {
				pe.App.PopView() // pop modal
				if pe.OnBack != nil {
					pe.OnBack()
				} else {
					pe.App.PopView()
				}
			},
		},
		{
			Label: "Cancel",
			Action: func() {
				pe.App.PopView() // cancel modal
			},
		},
	}
	pe.App.PushView(exitModal)
}
