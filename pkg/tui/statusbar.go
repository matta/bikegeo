package tui

import (
	"github.com/gdamore/tcell/v3"
)

// KeyHint represents a keyboard shortcut and its short label.
type KeyHint struct {
	Key  string
	Desc string
}

// StatusBar renders key shortcuts and status/message indicators at the bottom of the screen.
type StatusBar struct {
	Hints       []KeyHint
	Message     string
	IsError     bool
	IsDirty     bool
	DirtyPrompt string
	Theme       Theme
}

// NewStatusBar creates an initialized StatusBar.
func NewStatusBar(hints []KeyHint) *StatusBar {
	return &StatusBar{
		Hints:       hints,
		DirtyPrompt: "[MODIFIED *]",
		Theme:       DefaultTheme(),
	}
}

// Draw renders the status bar in the bottom row of bounds.
func (sb *StatusBar) Draw(s tcell.Screen, bounds Rect) {
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}

	y := bounds.Y + bounds.H - 1
	FillRect(s, Rect{X: bounds.X, Y: y, W: bounds.W, H: 1}, sb.Theme.Footer)

	curX := bounds.X + 1

	// Draw dirty flag if modified
	if sb.IsDirty {
		dirtyStyle := sb.Theme.Warning
		DrawText(s, curX, y, sb.DirtyPrompt+" ", dirtyStyle)
		curX += len([]rune(sb.DirtyPrompt)) + 1
	}

	// Reserved space for status/message on right
	msgW := 0
	if sb.Message != "" {
		msgW = len([]rune(sb.Message)) + 2
	}
	maxHintX := bounds.X + bounds.W - msgW - 1

	// Draw key hints that fit
	for _, hint := range sb.Hints {
		hintLen := len([]rune(hint.Key)) + len([]rune(hint.Desc)) + 2
		if curX+hintLen >= maxHintX {
			break
		}
		DrawText(s, curX, y, hint.Key, sb.Theme.Focus)
		curX += len([]rune(hint.Key))

		desc := " " + hint.Desc + " "
		DrawText(s, curX, y, desc, sb.Theme.Footer)
		curX += len([]rune(desc))
	}

	// Draw status / error message
	if sb.Message != "" {
		msgStyle := sb.Theme.Success
		if sb.IsError {
			msgStyle = sb.Theme.Error
		}
		posX := bounds.X + bounds.W - len([]rune(sb.Message)) - 1
		if posX < curX {
			posX = curX
		}
		avail := bounds.X + bounds.W - posX
		DrawTextTruncated(s, posX, y, avail, sb.Message, msgStyle)
	}
}

// HandleEvent for StatusBar (non-interactive display).
func (sb *StatusBar) HandleEvent(ev tcell.Event) EventResult {
	return EventIgnored
}
