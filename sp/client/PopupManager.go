package main

import (
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	w "poker-client/window"
)

type PopupManager struct {
	activePopups []w.PopupComponent
}

func NewPopupManager() PopupManager {
	return PopupManager{
		activePopups: make([]w.PopupComponent, 0),
	}
}

func (pm *PopupManager) AddPopup(text string, duration time.Duration) {
	newPopup := w.NewTimedPopup(text, duration)
	pm.activePopups = append(pm.activePopups, newPopup)
}

func (pm *PopupManager) Update() {
	alivePopups := make([]w.PopupComponent, 0)

	for _, popup := range pm.activePopups {
		if popup.Update() {
			alivePopups = append(alivePopups, popup)
		}
	}

	pm.activePopups = alivePopups
}

func (pm *PopupManager) Calculate(screenBounds rl.Rectangle) {
	cur_bounds := screenBounds

	for _, popup := range pm.activePopups {
		popup.Calculate(cur_bounds)

		popupBounds := popup.GetBounds()

		cur_bounds.Y += popupBounds.Height
	}
}

func (pm *PopupManager) Draw(eventChannel chan<- w.UIEvent) {
	for _, popup := range pm.activePopups {
		popup.Draw(eventChannel)
	}
}
