package window

import (
	rg "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type TextBoxComponent struct {
	bounds   rl.Rectangle
	ID       string
	Text     *string // Pointer to the model string
	maxChars int
	editMode bool
}

func NewTextBoxComponent(id string, text *string, maxChars int) *TextBoxComponent {
	return &TextBoxComponent{
		ID:       id,
		Text:     text,
		maxChars: maxChars,
	}
}

func (t *TextBoxComponent) Calculate(bounds rl.Rectangle) {
	t.bounds = bounds
}

func (t *TextBoxComponent) Draw(eventChannel chan<- UIEvent) {
	if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
		t.editMode = rl.CheckCollisionPointRec(rl.GetMousePosition(), t.bounds)
	}

	changed := rg.TextBox(t.bounds, t.Text, t.maxChars, t.editMode)

	if changed {
		eventChannel <- UIEvent{SourceID: t.ID, Type: EventValueChange}
	}
}

func (t *TextBoxComponent) GetBounds() rl.Rectangle {
	return t.bounds
}

func (t *TextBoxComponent) Rebuild(old RGComponent) {
	if old == nil {
		return
	}

	if oldTBC, ok := old.(*TextBoxComponent); ok {
		t.editMode = oldTBC.editMode
	}
}
