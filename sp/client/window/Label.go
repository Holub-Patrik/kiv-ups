package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type LabelComponent struct {
	bounds   rl.Rectangle
	Text     string
	FontSize int32
	Color    rl.Color
}

func NewLabelComponent(text string, fontSize int32, color rl.Color) *LabelComponent {
	return &LabelComponent{
		Text:     text,
		FontSize: fontSize,
		Color:    color,
	}
}

func (l *LabelComponent) Calculate(bounds rl.Rectangle) {
	l.bounds = bounds
}

func (l *LabelComponent) Draw(eventChannel chan<- UIEvent) {
	// Simple centered text for now
	textWidth := rl.MeasureText(l.Text, l.FontSize)
	x := l.bounds.X + (l.bounds.Width/2 - float32(textWidth)/2)
	y := l.bounds.Y + (l.bounds.Height/2 - float32(l.FontSize)/2)
	rl.DrawText(l.Text, int32(x), int32(y), l.FontSize, l.Color)
}

func (l *LabelComponent) GetBounds() rl.Rectangle {
	return l.bounds
}

func (l *LabelComponent) Rebuild(old RGComponent) {
	/* noop since no children and no persistent state */
}
