package tui

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

type dummyView struct {
	title string
	draws int
}

func (d *dummyView) Init(theme Theme) {}
func (d *dummyView) Draw(s tcell.Screen, bounds Rect) {
	d.draws++
	DrawText(s, 2, 2, d.title, tcell.StyleDefault)
}
func (d *dummyView) HandleEvent(ev tcell.Event) EventResult {
	if kev, ok := ev.(*tcell.EventKey); ok {
		if kev.Key() == tcell.KeyEscape || kev.Key() == tcell.KeyRune && kev.Str() == "q" {
			return EventQuit
		}
	}
	return EventIgnored
}

func TestAppLifecycleWithMockScreen(t *testing.T) {
	mt, scr := NewMockScreen(t, 80, 24)
	defer scr.Fini()

	app := NewApp(scr)
	view := &dummyView{title: "Test Ideal Editor"}
	app.PushView(view)

	if !ScreenContains(scr, "Test Ideal Editor") {
		t.Fatalf("expected screen to contain %q, but got dump:\n%s", "Test Ideal Editor", ScreenDump(scr))
	}

	w, h := mt.Pos().X, mt.Pos().Y
	_ = w
	_ = h
}
