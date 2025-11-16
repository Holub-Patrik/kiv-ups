package window

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

type GameScreen struct {
	// GameScreen is a VStack.
	*VStack

	playerBar *HStack
	actionBar *HStack
}

func NewGameScreen(padding float32) *GameScreen {
	mainLayout := NewVStack(padding)

	playerBar := NewHStack(padding)
	actionBar := NewHStack(padding)

	return &GameScreen{
		VStack:    mainLayout,
		playerBar: playerBar,
		actionBar: actionBar,
	}
}

func (gs *GameScreen) Calculate(bounds rl.Rectangle) {
	gs.bounds = bounds

	padding := gs.padding
	const actionBarRatio float32 = 0.2

	actionBarHeight := (bounds.Height - padding*3) * actionBarRatio
	playerBarHeight := (bounds.Height - padding*3) * (1.0 - actionBarRatio)

	playerBounds := rl.Rectangle{
		X:      bounds.X + padding,
		Y:      bounds.Y + padding,
		Width:  bounds.Width - padding*2,
		Height: playerBarHeight,
	}

	actionBounds := rl.Rectangle{
		X:      bounds.X + padding,
		Y:      bounds.Y + padding + playerBarHeight + padding,
		Width:  bounds.Width - padding*2,
		Height: actionBarHeight,
	}

	gs.playerBar.Calculate(playerBounds)
	gs.actionBar.Calculate(actionBounds)
}

func (gs *GameScreen) Draw(eventChannel chan<- UIEvent) {
	gs.playerBar.Draw(eventChannel)
	gs.actionBar.Draw(eventChannel)
}

func (gs *GameScreen) AddPlayerCard(card RGComponent) {
	gs.playerBar.AddChild(card)
}

func (gs *GameScreen) AddActionButton(button RGComponent) {
	gs.actionBar.AddChild(button)
}
