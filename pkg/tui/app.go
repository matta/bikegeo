package tui

import (
	"sync"

	"github.com/gdamore/tcell/v3"
)

// App manages the application lifecycle, screen stack, and TUI event loop.
type App struct {
	Screen tcell.Screen
	Theme  Theme

	mu          sync.Mutex
	screenStack []ScreenView
	running     bool
	quitChan    chan struct{}
}

// NewApp creates a new TUI App with the specified tcell.Screen.
func NewApp(s tcell.Screen) *App {
	return &App{
		Screen:   s,
		Theme:    DefaultTheme(),
		quitChan: make(chan struct{}),
	}
}

// PushView adds a view to the top of the stack and initializes it.
func (a *App) PushView(v ScreenView) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v.Init(a.Theme)
	a.screenStack = append(a.screenStack, v)
	a.redraw()
}

// PopView removes the top view from the stack.
func (a *App) PopView() ScreenView {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := len(a.screenStack)
	if n == 0 {
		return nil
	}
	top := a.screenStack[n-1]
	a.screenStack = a.screenStack[:n-1]
	if len(a.screenStack) > 0 {
		a.redraw()
	}
	return top
}

// CurrentView returns the active view at the top of the stack.
func (a *App) CurrentView() ScreenView {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := len(a.screenStack)
	if n == 0 {
		return nil
	}
	return a.screenStack[n-1]
}

// Redraw requests a full repaint.
func (a *App) Redraw() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.redraw()
}

func (a *App) redraw() {
	if a.Screen == nil {
		return
	}
	w, h := a.Screen.Size()
	bounds := Rect{X: 0, Y: 0, W: w, H: h}
	a.Screen.Clear()

	// Render all views from bottom to top (enabling modal overlays)
	for _, v := range a.screenStack {
		v.Draw(a.Screen, bounds)
	}

	a.Screen.Show()
}

// Quit terminates the app event loop.
func (a *App) Quit() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		a.running = false
		close(a.quitChan)
	}
}

// Run starts the main event loop and blocks until Quit is called or top view exits.
func (a *App) Run() error {
	a.mu.Lock()
	a.running = true
	a.mu.Unlock()

	a.Redraw()

	eventQ := a.Screen.EventQ()

	for {
		select {
		case <-a.quitChan:
			return nil
		case ev, ok := <-eventQ:
			if !ok {
				return nil
			}

			switch tev := ev.(type) {
			case *tcell.EventResize:
				a.Screen.Sync()
				a.Redraw()
				continue
			case *tcell.EventKey:
				if tev.Key() == tcell.KeyCtrlC {
					a.Quit()
					return nil
				}
			}

			cur := a.CurrentView()
			if cur == nil {
				a.Quit()
				return nil
			}

			res := cur.HandleEvent(ev)
			if res == EventQuit {
				a.PopView()
			}
			if a.CurrentView() == nil {
				a.Quit()
				return nil
			}

			a.Redraw()
		}
	}
}
