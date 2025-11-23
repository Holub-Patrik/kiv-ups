package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type PanelComponent struct {
	bounds rl.Rectangle
	Color  rl.Color
	child  RGComponent
}

func NewPanelComponent(color rl.Color, given_child RGComponent) *PanelComponent {
	return &PanelComponent{Color: color, child: given_child}
}

func (p *PanelComponent) Calculate(bounds rl.Rectangle) {
	p.bounds = bounds
	p.child.Calculate(bounds)
}

func (p *PanelComponent) Draw(eventChannel chan<- UIEvent) {
	rl.DrawRectangleRec(p.bounds, p.Color)
	rl.DrawRectangleLinesEx(p.bounds, 1, rl.White) // Debug border

	p.child.Draw(eventChannel)
}

func (p *PanelComponent) GetBounds() rl.Rectangle {
	return p.bounds
}

func (p *PanelComponent) SetChild(child RGComponent) {
	p.child = child
}

func (p *PanelComponent) Rebuild(old RGComponent) {
	if old == nil {
		return
	}

	if oldPC, ok := old.(*PanelComponent); ok {
		p.child.Rebuild(oldPC.child)
	}
}
