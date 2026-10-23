package tui

import (
	"github.com/gdamore/tcell/v3"
)

// EventResult indicates how an event was processed.
type EventResult int

const (
	EventIgnored EventResult = iota
	EventHandled
	EventQuit
)

// Widget represents a visual component that can be rendered and receive events.
type Widget interface {
	Draw(s tcell.Screen, bounds Rect)
	HandleEvent(ev tcell.Event) EventResult
}

// ScreenView represents a full or modal view that manages layout and widgets.
type ScreenView interface {
	Init(theme Theme)
	Draw(s tcell.Screen, bounds Rect)
	HandleEvent(ev tcell.Event) EventResult
}
