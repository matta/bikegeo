package tui

import (
	"github.com/gdamore/tcell/v3"
)

// ListItem represents an entry in a ListView.
type ListItem struct {
	ID        string
	Text      string
	Secondary string
	Valid     bool
}

// ListView renders a selectable list of items with scrolling.
type ListView struct {
	Items         []ListItem
	SelectedIndex int
	ScrollOffset  int
	Focused       bool
	Theme         Theme

	OnSelect func(index int, item ListItem)
	OnChange func(index int, item ListItem)
}

// NewListView creates an initialized ListView.
func NewListView() *ListView {
	return &ListView{
		Theme: DefaultTheme(),
	}
}

// SetItems updates the list items and bounds checks the selection.
func (lv *ListView) SetItems(items []ListItem) {
	lv.Items = items
	if lv.SelectedIndex >= len(items) {
		lv.SelectedIndex = len(items) - 1
	}
	if lv.SelectedIndex < 0 && len(items) > 0 {
		lv.SelectedIndex = 0
	}
	lv.adjustScroll(10) // default viewport height
}

// SelectedItem returns the currently selected item, if any.
func (lv *ListView) SelectedItem() (ListItem, bool) {
	if lv.SelectedIndex >= 0 && lv.SelectedIndex < len(lv.Items) {
		return lv.Items[lv.SelectedIndex], true
	}
	return ListItem{}, false
}

func (lv *ListView) adjustScroll(viewportH int) {
	if viewportH <= 0 {
		return
	}
	if lv.SelectedIndex < lv.ScrollOffset {
		lv.ScrollOffset = lv.SelectedIndex
	}
	if lv.SelectedIndex >= lv.ScrollOffset+viewportH {
		lv.ScrollOffset = lv.SelectedIndex - viewportH + 1
	}
	if lv.ScrollOffset < 0 {
		lv.ScrollOffset = 0
	}
}

// Draw renders the list within the specified rectangle.
func (lv *ListView) Draw(s tcell.Screen, bounds Rect) {
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}

	lv.adjustScroll(bounds.H)

	for i := 0; i < bounds.H; i++ {
		itemIdx := lv.ScrollOffset + i
		y := bounds.Y + i

		if itemIdx >= len(lv.Items) {
			DrawText(s, bounds.X, y, "", lv.Theme.Default)
			continue
		}

		item := lv.Items[itemIdx]
		isSelected := itemIdx == lv.SelectedIndex

		st := lv.Theme.Default
		prefix := "  "
		if isSelected {
			if lv.Focused {
				st = lv.Theme.Selected
			} else {
				st = lv.Theme.Button
			}
			prefix = "▶ "
		}

		line := prefix + item.Text
		if item.Secondary != "" {
			// Pad line or format secondary
			padLen := bounds.W - len([]rune(line)) - len([]rune(item.Secondary)) - 1
			if padLen > 0 {
				line = line + string(make([]rune, padLen)) + " " + item.Secondary
			}
		}

		FillRect(s, Rect{X: bounds.X, Y: y, W: bounds.W, H: 1}, st)
		DrawTextTruncated(s, bounds.X, y, bounds.W, line, st)
	}
}

// HandleEvent handles key navigation.
func (lv *ListView) HandleEvent(ev tcell.Event) EventResult {
	kev, ok := ev.(*tcell.EventKey)
	if !ok {
		return EventIgnored
	}

	total := len(lv.Items)
	if total == 0 {
		return EventIgnored
	}

	prev := lv.SelectedIndex

	switch kev.Key() {
	case tcell.KeyUp:
		if lv.SelectedIndex > 0 {
			lv.SelectedIndex--
		}
	case tcell.KeyDown:
		if lv.SelectedIndex < total-1 {
			lv.SelectedIndex++
		}
	case tcell.KeyPgUp:
		lv.SelectedIndex -= 10
		if lv.SelectedIndex < 0 {
			lv.SelectedIndex = 0
		}
	case tcell.KeyPgDn:
		lv.SelectedIndex += 10
		if lv.SelectedIndex >= total {
			lv.SelectedIndex = total - 1
		}
	case tcell.KeyHome:
		lv.SelectedIndex = 0
	case tcell.KeyEnd:
		lv.SelectedIndex = total - 1
	case tcell.KeyEnter:
		if lv.OnSelect != nil && lv.SelectedIndex >= 0 && lv.SelectedIndex < total {
			lv.OnSelect(lv.SelectedIndex, lv.Items[lv.SelectedIndex])
			return EventHandled
		}
	case tcell.KeyRune:
		switch kev.Str() {
		case "k":
			if lv.SelectedIndex > 0 {
				lv.SelectedIndex--
			}
		case "j":
			if lv.SelectedIndex < total-1 {
				lv.SelectedIndex++
			}
		case "g":
			lv.SelectedIndex = 0
		case "G":
			lv.SelectedIndex = total - 1
		default:
			return EventIgnored
		}
	default:
		return EventIgnored
	}

	if lv.SelectedIndex != prev && lv.OnChange != nil {
		lv.OnChange(lv.SelectedIndex, lv.Items[lv.SelectedIndex])
	}

	return EventHandled
}
