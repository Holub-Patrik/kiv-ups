package main

import (
	"fmt"
	"time"
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

		case msg, ok := <-ctx.NetMsgInChan:
			if !ok {
				// Network closed
				// this causes infinite loop
				fmt.Println("GameThread: NetMsgInChan closed.")
				ctx.NetMsgInChan = nil
				ctx.NetMsgInChan = nil

				if !ctx.ShouldClose {
					ctx.Popup.AddPopup("Connection Lost", time.Second*3)
					nextState = &StateMainMenu{} // Fallback to safe state
				}
			} else {
				// Global handlers (like PING) could go here

				// State specific handlers
				nextState = currentState.HandleNetwork(ctx, msg)
			}

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

	fmt.Println("GameThread shutting down.")
	if ctx.NetHandler.MsgOut() != nil {
		ctx.NetHandler.Disconnect()
	}
	ctx.DoneChan <- true
}
