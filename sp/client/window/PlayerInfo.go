package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type PlayerInfoComponent struct {
	bounds   rl.Rectangle
	IsMyTurn bool
	desc     *VStack
	cards    *HStack
}

func NewPlayerInfoComponent(isMyTurn bool) *PlayerInfoComponent {
	return &PlayerInfoComponent{
		IsMyTurn: isMyTurn,
		cards:    NewHStack(2),
		desc:     NewVStack(2),
	}
}

func (p *PlayerInfoComponent) AddCard(card RGComponent) {
	p.cards.AddChild(card)
}

func (p *PlayerInfoComponent) AddDesc(status RGComponent) {
	p.desc.AddChild(status)
}

func (p *PlayerInfoComponent) Calculate(bounds rl.Rectangle) {
	p.bounds = bounds
	descBounds := rl.Rectangle{
		X:      bounds.X,
		Y:      bounds.Y,
		Height: bounds.Height / 2,
		Width:  bounds.Width,
	}

	cardBounds := rl.Rectangle{
		X:      bounds.X,
		Y:      bounds.Y + descBounds.Height,
		Height: bounds.Height / 2,
		Width:  bounds.Width,
	}

	p.desc.Calculate(descBounds)
	p.cards.Calculate(cardBounds)
}

func (p *PlayerInfoComponent) Draw(eventChannel chan<- UIEvent) {
	// Turn indicator: red border + tint
	if p.IsMyTurn {
		rl.DrawRectangleRec(p.bounds, rl.NewColor(120, 30, 30, 180))
		rl.DrawRectangleLinesEx(p.bounds, 3, rl.Red)
	} else {
		rl.DrawRectangleRec(p.bounds, rl.NewColor(40, 40, 40, 180))
		rl.DrawRectangleLinesEx(p.bounds, 1, rl.Gray)
	}

	p.cards.Draw(eventChannel)
	p.desc.Draw(eventChannel)
}

func (p *PlayerInfoComponent) GetBounds() rl.Rectangle {
	return p.bounds
}

func (p *PlayerInfoComponent) Rebuild(old RGComponent) {}
