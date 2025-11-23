package window

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type PotDisplayComponent struct {
	bounds   rl.Rectangle
	Pot      int
	RoundBet int
}

func NewPotDisplayComponent(pot, roundBet int) *PotDisplayComponent {
	return &PotDisplayComponent{Pot: pot, RoundBet: roundBet}
}

func (p *PotDisplayComponent) Calculate(bounds rl.Rectangle) { p.bounds = bounds }

func (p *PotDisplayComponent) Draw(eventChannel chan<- UIEvent) {
	rl.DrawRectangleRec(p.bounds, rl.NewColor(0, 60, 0, 200))
	rl.DrawRectangleLinesEx(p.bounds, 2, rl.Gold)

	potText := fmt.Sprintf("Pot: %d", p.Pot)
	roundText := fmt.Sprintf("Round: %d", p.RoundBet)

	// Center vertically
	totalH := float32(24 + 16 + 5)
	y := p.bounds.Y + (p.bounds.Height-totalH)/2

	potW := rl.MeasureText(potText, 24)
	rl.DrawText(potText, int32(p.bounds.X+(p.bounds.Width-float32(potW))/2), int32(y), 24, rl.Gold)

	roundW := rl.MeasureText(roundText, 16)
	rl.DrawText(roundText, int32(p.bounds.X+(p.bounds.Width-float32(roundW))/2), int32(y+29), 16, rl.White)
}

func (p *PotDisplayComponent) GetBounds() rl.Rectangle { return p.bounds }
