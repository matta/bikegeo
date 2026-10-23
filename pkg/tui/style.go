package tui

import (
	"strings"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
)

// Theme defines standard styles for the TUI.
type Theme struct {
	Default     tcell.Style
	Header      tcell.Style
	Footer      tcell.Style
	Selected    tcell.Style
	Focus       tcell.Style
	Border      tcell.Style
	Title       tcell.Style
	Label       tcell.Style
	Input       tcell.Style
	InputFocus  tcell.Style
	Button      tcell.Style
	ButtonFocus tcell.Style
	Success     tcell.Style
	Warning     tcell.Style
	Error       tcell.Style
	Muted       tcell.Style
}

// DefaultTheme returns the standard color palette.
func DefaultTheme() Theme {
	def := tcell.StyleDefault.Background(color.Reset).Foreground(color.Reset)
	return Theme{
		Default:     def,
		Header:      def.Bold(true).Foreground(color.Teal),
		Footer:      def.Foreground(color.Silver),
		Selected:    tcell.StyleDefault.Background(color.Navy).Foreground(color.White).Bold(true),
		Focus:       def.Foreground(color.Aqua).Bold(true),
		Border:      def.Foreground(color.GetColor("steelblue")),
		Title:       def.Bold(true).Foreground(color.Aqua),
		Label:       def.Foreground(color.Silver),
		Input:       tcell.StyleDefault.Background(color.GetColor("darkslategray")).Foreground(color.White),
		InputFocus:  tcell.StyleDefault.Background(color.GetColor("midnightblue")).Foreground(color.White).Bold(true),
		Button:      tcell.StyleDefault.Background(color.Gray).Foreground(color.White),
		ButtonFocus: tcell.StyleDefault.Background(color.GetColor("dodgerblue")).Foreground(color.White).Bold(true),
		Success:     def.Foreground(color.Lime).Bold(true),
		Warning:     def.Foreground(color.Yellow).Bold(true),
		Error:       def.Foreground(color.Red).Bold(true),
		Muted:       def.Foreground(color.Gray),
	}
}

// Rect defines a rectangular screen region.
type Rect struct {
	X int
	Y int
	W int
	H int
}

// Contains checks if point is within rect.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// DrawText writes text within bounds starting at (x, y) with a given style.
func DrawText(s tcell.Screen, x, y int, text string, style tcell.Style) {
	s.PutStrStyled(x, y, text, style)
}

// DrawTextTruncated writes text clipped to maxLen.
func DrawTextTruncated(s tcell.Screen, x, y int, maxLen int, text string, style tcell.Style) {
	if maxLen <= 0 {
		return
	}
	runes := []rune(text)
	if len(runes) > maxLen {
		if maxLen > 1 {
			text = string(runes[:maxLen-1]) + "…"
		} else {
			text = string(runes[:maxLen])
		}
	}
	s.PutStrStyled(x, y, text, style)
}

// DrawBox draws a border box around the rectangle.
func DrawBox(s tcell.Screen, r Rect, title string, style tcell.Style) {
	if r.W < 2 || r.H < 2 {
		return
	}

	// Corners
	s.Put(r.X, r.Y, "┌", style)
	s.Put(r.X+r.W-1, r.Y, "┐", style)
	s.Put(r.X, r.Y+r.H-1, "└", style)
	s.Put(r.X+r.W-1, r.Y+r.H-1, "┘", style)

	// Horizontal borders
	for x := r.X + 1; x < r.X+r.W-1; x++ {
		s.Put(x, r.Y, "─", style)
		s.Put(x, r.Y+r.H-1, "─", style)
	}

	// Vertical borders
	for y := r.Y + 1; y < r.Y+r.H-1; y++ {
		s.Put(r.X, y, "│", style)
		s.Put(r.X+r.W-1, y, "│", style)
	}

	// Title
	if title != "" && r.W > 4 {
		titleText := " " + title + " "
		maxTitleW := r.W - 4
		if len(titleText) > maxTitleW {
			titleText = " " + title[:maxTitleW-3] + "… "
		}
		s.PutStrStyled(r.X+2, r.Y, titleText, style.Bold(true))
	}
}

// FillRect clears a rectangular region with empty spaces and a style.
func FillRect(s tcell.Screen, r Rect, st tcell.Style) {
	for y := r.Y; y < r.Y+r.H; y++ {
		line := strings.Repeat(" ", r.W)
		s.PutStrStyled(r.X, y, line, st)
	}
}
