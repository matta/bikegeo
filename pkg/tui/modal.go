package tui

import (
	"strings"

	"github.com/gdamore/tcell/v3"
)

// ModalButton represents an action button inside a modal.
type ModalButton struct {
	Label  string
	Action func()
}

// ModalDialog displays a centered modal window over background views.
type ModalDialog struct {
	Title        string
	Width        int
	Height       int
	Content      Widget
	Message      string
	Buttons      []ModalButton
	FocusButton  int
	Theme        Theme
	StatusMsg    string
	StatusIsWarn bool

	OnClose func()
}

// NewModalDialog creates an initialized modal dialog.
func NewModalDialog(title string, width, height int) *ModalDialog {
	return &ModalDialog{
		Title:  title,
		Width:  width,
		Height: height,
		Theme:  DefaultTheme(),
	}
}

// Init sets the dialog theme.
func (m *ModalDialog) Init(theme Theme) {
	m.Theme = theme
}

// Draw renders the modal dialog centered in the provided bounds.
func (m *ModalDialog) Draw(s tcell.Screen, bounds Rect) {
	w := m.Width
	if w <= 0 || w > bounds.W-4 {
		w = bounds.W - 4
	}
	h := m.Height
	if h <= 0 || h > bounds.H-2 {
		h = bounds.H - 2
	}

	x := bounds.X + (bounds.W-w)/2
	y := bounds.Y + (bounds.H-h)/2
	boxRect := Rect{X: x, Y: y, W: w, H: h}

	// Clear dialog background
	FillRect(s, boxRect, m.Theme.Default)
	DrawBox(s, boxRect, m.Title, m.Theme.Border)

	// Interior content rect
	contentRect := Rect{
		X: x + 2,
		Y: y + 2,
		W: w - 4,
		H: h - 4,
	}

	// Render Message if present
	if m.Message != "" {
		lines := strings.Split(m.Message, "\n")
		for i, line := range lines {
			if y+2+i >= y+h-2 {
				break
			}
			DrawTextTruncated(s, x+2, y+2+i, w-4, line, m.Theme.Default)
		}
	}

	// Render custom content widget if present
	if m.Content != nil {
		m.Content.Draw(s, contentRect)
	}

	// Render status/validation message at bottom if present
	if m.StatusMsg != "" {
		st := m.Theme.Success
		if m.StatusIsWarn {
			st = m.Theme.Error
		}
		DrawTextTruncated(s, x+2, y+h-3, w-4, m.StatusMsg, st)
	}

	// Render action buttons on the bottom line
	if len(m.Buttons) > 0 {
		btnY := y + h - 2
		btnX := x + w - 2
		// Draw buttons right-to-left
		for i := len(m.Buttons) - 1; i >= 0; i-- {
			btn := m.Buttons[i]
			label := " " + btn.Label + " "
			btnW := len([]rune(label))
			btnX -= btnW + 1

			st := m.Theme.Button
			if i == m.FocusButton {
				st = m.Theme.ButtonFocus
			}
			DrawText(s, btnX, btnY, label, st)
		}
	}
}

// HandleEvent manages button focus and content delegation.
func (m *ModalDialog) HandleEvent(ev tcell.Event) EventResult {
	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventIgnored
	}

	if kev.Key() == tcell.KeyEscape {
		if m.OnClose != nil {
			m.OnClose()
			return EventHandled
		}
	}

	// Allow switching focus between content and buttons or between buttons
	numButtons := len(m.Buttons)
	if numButtons > 0 {
		switch kev.Key() {
		case tcell.KeyLeft:
			if m.FocusButton > 0 {
				m.FocusButton--
				return EventHandled
			}
		case tcell.KeyRight:
			if m.FocusButton < numButtons-1 {
				m.FocusButton++
				return EventHandled
			}
		case tcell.KeyEnter:
			if m.FocusButton >= 0 && m.FocusButton < numButtons {
				if m.Buttons[m.FocusButton].Action != nil {
					m.Buttons[m.FocusButton].Action()
					return EventHandled
				}
			}
		}
	}

	// Delegate to content widget
	if m.Content != nil {
		return m.Content.HandleEvent(ev)
	}

	return EventIgnored
}
