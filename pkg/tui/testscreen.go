package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

// NewMockScreen creates an initialized tcell.Screen paired with a vt.MockTerm terminal emulator.
func NewMockScreen(t *testing.T, width, height int) (vt.MockTerm, tcell.Screen) {
	t.Helper()
	mt := vt.NewMockTerm(vt.MockOptSize{X: vt.Col(width), Y: vt.Row(height)})
	scr, err := tcell.NewTerminfoScreenFromTty(mt)
	if err != nil {
		t.Fatalf("failed to create screen from mock terminal: %v", err)
	}
	if err = scr.Init(); err != nil {
		t.Fatalf("failed to initialize screen: %v", err)
	}
	return mt, scr
}

// ScreenRowText extracts the rendered text across a given horizontal row of the screen.
func ScreenRowText(s tcell.Screen, y int) string {
	w, h := s.Size()
	if y < 0 || y >= h {
		return ""
	}
	var sb strings.Builder
	for x := 0; x < w; x++ {
		str, _, width := s.Get(x, y)
		if str == "" {
			str = " "
		}
		sb.WriteString(str)
		if width > 1 {
			x += width - 1
		}
	}
	return sb.String()
}

// ScreenDump returns a slice of strings, one per row of the screen.
func ScreenDump(s tcell.Screen) []string {
	_, h := s.Size()
	rows := make([]string, h)
	for y := 0; y < h; y++ {
		rows[y] = ScreenRowText(s, y)
	}
	return rows
}

// ScreenContains checks if a substring appears anywhere on the screen.
func ScreenContains(s tcell.Screen, substr string) bool {
	_, h := s.Size()
	for y := 0; y < h; y++ {
		if strings.Contains(ScreenRowText(s, y), substr) {
			return true
		}
	}
	return false
}
