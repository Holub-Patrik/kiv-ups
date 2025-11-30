package window

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type PlayerInfoComponent struct {
	bounds     rl.Rectangle
	PlayerName string
	ChipCount  int
	RoundBet   int
	IsMyTurn   bool
	cards      []RGComponent
	status     string
}

func NewPlayerInfoComponent(name string, chips, roundBet int, isMyTurn bool) *PlayerInfoComponent {
	return &PlayerInfoComponent{
		PlayerName: name,
		ChipCount:  chips,
		RoundBet:   roundBet,
		IsMyTurn:   isMyTurn,
		cards:      make([]RGComponent, 0),
		status:     "None",
	}
}

func (p *PlayerInfoComponent) AddCard(card RGComponent) {
	p.cards = append(p.cards, card)
}

func (p *PlayerInfoComponent) SetStatus(status string) {
	p.status = status
}

func (p *PlayerInfoComponent) Calculate(bounds rl.Rectangle) {
	p.bounds = bounds
	// Cards stack horizontally at bottom
	if len(p.cards) > 0 {
		padding := float32(2)
		totalCardPadding := padding * float32(len(p.cards)-1)
		cardWidth := (bounds.Width - totalCardPadding) / float32(len(p.cards))
		y := bounds.Y + bounds.Height - 40 // Fixed card height area

		for i, card := range p.cards {
			x := bounds.X + float32(i)*(cardWidth+padding)
			card.Calculate(rl.Rectangle{X: x, Y: y, Width: cardWidth, Height: 35})
		}
	}
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

	// Draw info text
	nameY := p.bounds.Y + 5
	rl.DrawText(p.PlayerName, int32(p.bounds.X+5), int32(nameY), 14, rl.White)

	chipsY := nameY + 16
	rl.DrawText(fmt.Sprintf("Chips: %d", p.ChipCount), int32(p.bounds.X+5), int32(chipsY), 12, rl.Yellow)

	if p.RoundBet > 0 {
		betY := chipsY + 14
		rl.DrawText(fmt.Sprintf("Bet: %d", p.RoundBet), int32(p.bounds.X+5), int32(betY), 12, rl.Orange)
	}

	// Draw cards
	for _, card := range p.cards {
		card.Draw(eventChannel)
	}
}

func (p *PlayerInfoComponent) GetBounds() rl.Rectangle {
	return p.bounds
}

func (p *PlayerInfoComponent) Rebuild(old RGComponent) {}
