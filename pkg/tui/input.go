package tui

import (
	"strings"

	"github.com/gdamore/tcell/v3"
)

// TextInput is a single-line editable text input widget.
type TextInput struct {
	Text        string
	CursorPos   int
	Placeholder string
	Focused     bool
	Theme       Theme

	OnChange func(newText string)
	OnEnter  func(text string)
}

// NewTextInput creates an empty TextInput.
func NewTextInput(initialText string) *TextInput {
	runes := []rune(initialText)
	return &TextInput{
		Text:      initialText,
		CursorPos: len(runes),
		Theme:     DefaultTheme(),
	}
}

// SetText sets the text and places cursor at the end.
func (ti *TextInput) SetText(s string) {
	ti.Text = s
	ti.CursorPos = len([]rune(s))
}

// Draw renders the text input within bounds.
func (ti *TextInput) Draw(s tcell.Screen, bounds Rect) {
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}

	st := ti.Theme.Input
	if ti.Focused {
		st = ti.Theme.InputFocus
	}

	FillRect(s, bounds, st)

	runes := []rune(ti.Text)
	displayStr := ti.Text
	if len(runes) == 0 && !ti.Focused && ti.Placeholder != "" {
		DrawTextTruncated(s, bounds.X, bounds.Y, bounds.W, ti.Placeholder, ti.Theme.Muted)
		return
	}

	// Calculate scroll offset if text exceeds width
	scrollOffset := 0
	if ti.CursorPos >= bounds.W {
		scrollOffset = ti.CursorPos - bounds.W + 1
	}

	if scrollOffset < len(runes) {
		displayStr = string(runes[scrollOffset:])
	} else {
		displayStr = ""
	}

	DrawTextTruncated(s, bounds.X, bounds.Y, bounds.W, displayStr, st)

	if ti.Focused {
		curX := bounds.X + (ti.CursorPos - scrollOffset)
		if curX >= bounds.X && curX < bounds.X+bounds.W {
			s.ShowCursor(curX, bounds.Y)
		}
	}
}

// HandleEvent handles typing and cursor movement.
func (ti *TextInput) HandleEvent(ev tcell.Event) EventResult {
	if !ti.Focused {
		return EventIgnored
	}

	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventIgnored
	}

	runes := []rune(ti.Text)

	switch kev.Key() {
	case tcell.KeyLeft:
		if ti.CursorPos > 0 {
			ti.CursorPos--
			return EventHandled
		}
	case tcell.KeyRight:
		if ti.CursorPos < len(runes) {
			ti.CursorPos++
			return EventHandled
		}
	case tcell.KeyHome:
		ti.CursorPos = 0
		return EventHandled
	case tcell.KeyEnd:
		ti.CursorPos = len(runes)
		return EventHandled
	case tcell.KeyBackspace:
		if ti.CursorPos > 0 {
			ti.Text = string(runes[:ti.CursorPos-1]) + string(runes[ti.CursorPos:])
			ti.CursorPos--
			if ti.OnChange != nil {
				ti.OnChange(ti.Text)
			}
			return EventHandled
		}
	case tcell.KeyDelete:
		if ti.CursorPos < len(runes) {
			ti.Text = string(runes[:ti.CursorPos]) + string(runes[ti.CursorPos+1:])
			if ti.OnChange != nil {
				ti.OnChange(ti.Text)
			}
			return EventHandled
		}
	case tcell.KeyEnter:
		if ti.OnEnter != nil {
			ti.OnEnter(ti.Text)
			return EventHandled
		}
	case tcell.KeyRune:
		s := kev.Str()
		if s != "" {
			// Don't insert control characters
			if strings.ContainsAny(s, "\r\n\t") {
				return EventIgnored
			}
			ins := []rune(s)
			ti.Text = string(runes[:ti.CursorPos]) + s + string(runes[ti.CursorPos:])
			ti.CursorPos += len(ins)
			if ti.OnChange != nil {
				ti.OnChange(ti.Text)
			}
			return EventHandled
		}
	}

	return EventIgnored
}
