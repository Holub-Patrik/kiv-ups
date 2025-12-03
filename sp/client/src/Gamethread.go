package main

import (
	"fmt"
	"time"

	unet "poker-client/ups_net"
)

func gameThread(ctx *ProgCtx) {
	fmt.Println("GameThread started (DFA Version).")

	// Initial State
	var currentState LogicState = &StateMainMenu{}
	currentState.Enter(ctx)

	for !ctx.ShouldClose {
		var nextState LogicState = nil

		select {
		case input := <-ctx.UserInputChan:
			// Special case for Quit
			if _, ok := input.(EvtQuit); ok {
				ctx.ShouldClose = true
				break
			}
			nextState = currentState.HandleInput(ctx, input)

		case netEvt := <-ctx.EventChan:
			nextState = currentState.HandleNetwork(ctx, netEvt)

		default:
			time.Sleep(time.Millisecond * 10)
		}

		// Handle Transition
		if nextState != nil {
			currentState.Exit(ctx)
			currentState = nextState
			currentState.Enter(ctx)

			// Force UI to redraw on state change
			ctx.UI.SetDirty()
		}
	}

	fmt.Println("meThread shutting down.")
	ctx.NetHandler.SendCommand(unet.NetShutdown{})
	ctx.DoneChan <- true
}
