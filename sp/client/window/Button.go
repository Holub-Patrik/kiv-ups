package window

import (
	rg "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type ButtonComponent struct {
	bounds     rl.Rectangle
	ID         string
	Text       string
	max_width  float32
	max_height float32
}

func NewButtonComponent(id string, text string, max_w float32, max_h float32) *ButtonComponent {
	return &ButtonComponent{ID: id, Text: text, max_width: max_w, max_height: max_h}
}

func (b *ButtonComponent) Calculate(bounds rl.Rectangle) {
	temp_bounds := bounds

	if bounds.Width > b.max_width {
		temp_bounds.Width = b.max_width
	}

	if bounds.Height > b.max_height {
		temp_bounds.Height = b.max_height
	}

	b.bounds = temp_bounds
}

func (b *ButtonComponent) Draw(eventChannel chan<- UIEvent) {
	if rg.Button(b.bounds, b.Text) {
		eventChannel <- UIEvent{SourceID: b.ID, Type: EventClick}
	}
}

func (b *ButtonComponent) GetBounds() rl.Rectangle {
	return b.bounds
}

func (b *ButtonComponent) Rebuild(old RGComponent) {
	/* noop since this is true leaf node*/
}
