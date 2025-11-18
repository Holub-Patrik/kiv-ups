package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type BoundsBox struct {
	bounds       rl.Rectangle
	width_ratio  float32
	height_ratio float32
	child        RGComponent
}

func NewBoundsBox(w_ratio float32, h_ratio float32, child RGComponent) *BoundsBox {
	return &BoundsBox{
		width_ratio:  w_ratio,
		height_ratio: h_ratio,
		child:        child,
	}
}

func (b *BoundsBox) SetChild(child RGComponent) {
	b.child = child
}

func (b *BoundsBox) GetBounds() rl.Rectangle {
	return b.bounds
}

func (b *BoundsBox) Calculate(bounds rl.Rectangle) {
	b.bounds = bounds

	innerWidth := b.bounds.Width * b.width_ratio
	innerHeight := b.bounds.Height * b.height_ratio

	innerX := (b.bounds.Width - innerWidth) / 2
	innerY := (b.bounds.Height - innerHeight) / 2

	childBounds := rl.Rectangle{
		X:      b.bounds.X + innerX,
		Y:      b.bounds.Y + innerY,
		Width:  innerWidth,
		Height: innerHeight,
	}

	b.child.Calculate(childBounds)
}

func (b *BoundsBox) Draw(eventChannel chan<- UIEvent) {
	b.child.Draw(eventChannel)
}
