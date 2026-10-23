package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestListViewNavigation(t *testing.T) {
	_, scr := NewMockScreen(t, 40, 10)
	defer scr.Fini()

	lv := NewListView()
	lv.SetItems([]ListItem{
		{ID: "1", Text: "Item One"},
		{ID: "2", Text: "Item Two"},
		{ID: "3", Text: "Item Three"},
	})
	lv.Focused = true

	// Initial render
	lv.Draw(scr, Rect{X: 0, Y: 0, W: 40, H: 10})
	scr.Show()

	if !ScreenContains(scr, "Item One") {
		t.Fatalf("expected Screen to contain 'Item One'")
	}
	sel, ok := lv.SelectedItem()
	if !ok || sel.ID != "1" {
		t.Fatalf("expected selection to be Item One, got %+v", sel)
	}

	// Press Down
	downEv := tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone)
	if res := lv.HandleEvent(downEv); res != EventHandled {
		t.Fatalf("expected EventHandled on KeyDown")
	}
	sel, _ = lv.SelectedItem()
	if sel.ID != "2" {
		t.Fatalf("expected selection to be Item Two, got %+v", sel)
	}

	// Press 'k' (Up)
	upEv := tcell.NewEventKey(tcell.KeyRune, "k", tcell.ModNone)
	lv.HandleEvent(upEv)
	sel, _ = lv.SelectedItem()
	if sel.ID != "1" {
		t.Fatalf("expected selection to be Item One after Up, got %+v", sel)
	}
}

func TestTextInputEditing(t *testing.T) {
	_, scr := NewMockScreen(t, 40, 5)
	defer scr.Fini()

	input := NewTextInput("bike")
	input.Focused = true

	input.Draw(scr, Rect{X: 2, Y: 1, W: 20, H: 1})
	scr.Show()
	if !ScreenContains(scr, "bike") {
		t.Fatalf("expected Screen to contain 'bike'")
	}

	// Type "geo"
	for _, r := range "geo" {
		ev := tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone)
		input.HandleEvent(ev)
	}

	if input.Text != "bikegeo" {
		t.Fatalf("expected text 'bikegeo', got %q", input.Text)
	}

	// Backspace
	bsEv := tcell.NewEventKey(tcell.KeyBackspace, "", tcell.ModNone)
	input.HandleEvent(bsEv)
	if input.Text != "bikege" {
		t.Fatalf("expected text 'bikege', got %q", input.Text)
	}

	// Left arrow + Insert 'X'
	leftEv := tcell.NewEventKey(tcell.KeyLeft, "", tcell.ModNone)
	input.HandleEvent(leftEv)
	insEv := tcell.NewEventKey(tcell.KeyRune, "X", tcell.ModNone)
	input.HandleEvent(insEv)

	if input.Text != "bikegXe" {
		t.Fatalf("expected text 'bikegXe', got %q", input.Text)
	}
}

func TestFormAndCyclePicker(t *testing.T) {
	_, scr := NewMockScreen(t, 60, 15)
	defer scr.Fini()

	picker := NewCyclePicker([]string{"opt1", "opt2", "opt3"}, []string{"Option 1", "Option 2", "Option 3"})
	input := NewTextInput("value1")

	form := NewForm()
	form.AddField(FormField{ID: "picker", Label: "Choice", Widget: picker})
	form.AddField(FormField{ID: "val", Label: "Value", Widget: input})

	form.Draw(scr, Rect{X: 1, Y: 1, W: 50, H: 10})
	scr.Show()

	if !ScreenContains(scr, "Option 1") {
		t.Fatalf("expected Screen to contain 'Option 1'")
	}

	// Cycle picker with Right arrow
	rightEv := tcell.NewEventKey(tcell.KeyRight, "", tcell.ModNone)
	form.HandleEvent(rightEv)
	if picker.SelectedValue() != "opt2" {
		t.Fatalf("expected picker to cycle to 'opt2', got %q", picker.SelectedValue())
	}

	// Tab to next field
	tabEv := tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone)
	form.HandleEvent(tabEv)
	if form.FocusedIndex != 1 {
		t.Fatalf("expected focused field index 1, got %d", form.FocusedIndex)
	}
	if !input.Focused {
		t.Fatalf("expected input to be focused")
	}
}

func TestModalDialogButtons(t *testing.T) {
	_, scr := NewMockScreen(t, 60, 20)
	defer scr.Fini()

	clicked := false
	modal := NewModalDialog("Save Changes?", 40, 10)
	modal.Message = "Do you want to save before closing?"
	modal.Buttons = []ModalButton{
		{Label: "Save", Action: func() { clicked = true }},
		{Label: "Cancel", Action: func() {}},
	}
	modal.FocusButton = 0

	modal.Draw(scr, Rect{X: 0, Y: 0, W: 60, H: 20})
	scr.Show()

	if !ScreenContains(scr, "Save Changes?") {
		t.Fatalf("expected modal title on screen")
	}
	if !ScreenContains(scr, "Save") || !ScreenContains(scr, "Cancel") {
		t.Fatalf("expected buttons on screen")
	}

	enterEv := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	modal.HandleEvent(enterEv)

	if !clicked {
		t.Fatalf("expected Save button action to be invoked")
	}
}

func TestStatusBarRendering(t *testing.T) {
	_, scr := NewMockScreen(t, 80, 5)
	defer scr.Fini()

	sb := NewStatusBar([]KeyHint{
		{Key: "[s]", Desc: "Save"},
		{Key: "[q]", Desc: "Quit"},
	})
	sb.IsDirty = true
	sb.Message = "Validated successfully"

	sb.Draw(scr, Rect{X: 0, Y: 0, W: 80, H: 5})
	scr.Show()

	line := ScreenRowText(scr, 4)
	if !strings.Contains(line, "[MODIFIED *]") {
		t.Fatalf("expected dirty flag in status bar, got line: %q", line)
	}
	if !strings.Contains(line, "Save") || !strings.Contains(line, "Quit") {
		t.Fatalf("expected key hints in status bar, got line: %q", line)
	}
	if !strings.Contains(line, "Validated successfully") {
		t.Fatalf("expected status message in status bar, got line: %q", line)
	}
}
