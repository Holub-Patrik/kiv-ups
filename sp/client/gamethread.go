package main

import (
	"fmt"
	unet "poker-client/ups_net" // Using alias 'unet' from your main.go
	"strings"
)

// gameThread is the main logic loop of the application.
// It should be run as a goroutine.
func gameThread(ctx *ProgCtx) {
	fmt.Println("GameThread started.")
	// This ticker is for any game-loop logic, e.g., pings
	// ticker := time.NewTicker(1 * time.Second)
	// defer ticker.Stop()

	// This is the Game Thread's main loop
	for !ctx.ShouldClose {
		select {
		// --- Handle User Input ---
		case input := <-ctx.UserInputChan:
			handleUserInput(ctx, input)

		// --- Handle Network Messages ---
		case msg, ok := <-ctx.NetMsgInChan:
			if !ok {
				// Channel closed, network handler shut down
				fmt.Println("NetMsgInChan closed. Shutting down GameThread.")
				ctx.StateMutex.Lock()
				ctx.State.Screen = ScreenMainMenu
				// TODO: Add error notification
				ctx.State.ErrorMessage = "Connection lost."
				ctx.StateMutex.Unlock()
				ctx.ShouldClose = true
				break
			}

			handleNetworkMessage(ctx, msg)

			// --- Handle other periodic tasks ---
			// case <-ticker.C:
			// 	// e.g., send a PING message
			// 	if ctx.State.Screen == ScreenInGame {
			// 		ctx.NetMsgOutChan <- unet.NetMsg{Code: "PING"}
			// 	}
		}
	}

	fmt.Println("GameThread shutting down.")
	ctx.DoneChan <- true // Signal main() that we are done
}

// handleUserInput processes events from the Render Thread
func handleUserInput(ctx *ProgCtx, input UserInputEvent) {
	switch evt := input.(type) {
	case EvtConnectClicked:
		fmt.Println("GameThread: Received EvtConnectClicked")

		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenConnecting
		ctx.State.IsConnecting = true
		ctx.StateMutex.Unlock()

		// Start connection in a separate goroutine so we don't block this loop
		go func(host, port string) {
			success := ctx.NetHandler.Connect(host, port)
			fmt.Println("GameThread: Connect result:", success)

			if !success {
				ctx.StateMutex.Lock()
				ctx.State.Screen = ScreenServerSelect
				// here I need to implement the notification system
				ctx.State.ErrorMessage = "Failed to connect to " + host + ":" + port
				ctx.State.IsConnecting = false
				ctx.StateMutex.Unlock()
				return
			}

			ctx.NetMsgInChan = ctx.NetHandler.MsgIn()
			ctx.NetMsgOutChan = ctx.NetHandler.MsgOut()

			go ctx.NetHandler.Run()

			fmt.Println("GameThread: Sending CONN message")
			ctx.NetMsgOutChan <- unet.NetMsg{Code: "CONN", Payload: ""}
		}(evt.Host, evt.Port)

	case EvtCancelConnectClicked:
		fmt.Println("GameThread: Cancelling connection...")
		ctx.NetHandler.Close()
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenMainMenu
		ctx.State.IsConnecting = false
		ctx.StateMutex.Unlock()

	case EvtQuitClicked:
		ctx.ShouldClose = true
	}
}

// handleNetworkMessage processes messages from the Network Thread
func handleNetworkMessage(ctx *ProgCtx, msg unet.NetMsg) {
	fmt.Printf("GameThread: Received NetMsg Code: %s\n", msg.Code)

	// Get current screen to provide context
	ctx.StateMutex.RLock()
	currentScreen := ctx.State.Screen
	ctx.StateMutex.RUnlock()

	switch msg.Code {
	// --- Scenario: Connect ---
	case "00OK":
		if currentScreen == ScreenConnecting {
			// This is the OK for our "CONN" message.
			// Now, request the room list.
			fmt.Println("GameThread: Connection OK, requesting rooms...")
			ctx.NetMsgOutChan <- unet.NetMsg{Code: "RMRQ", Payload: ""}
			// We are still in the "Connecting" screen, waiting for rooms.
		}
		// This is also the ACK for "ROOM". We don't need to do anything.

		// This is also the ACK for "DONE". We don't need to do anything.

	case "ROOM":
		// Server is sending us a room.
		room := deserializeRoom(msg.Payload) // You need to implement this!
		fmt.Printf("GameThread: Received Room: ID=%s, Name=%s\n", room.ID, room.Name)
		ctx.StateMutex.Lock()
		if ctx.State.Rooms == nil {
			ctx.State.Rooms = make(map[string]Room)
		}
		ctx.State.Rooms[room.ID] = room
		ctx.StateMutex.Unlock()

		// Send ACK
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "00OK", Payload: ""}

	case "DONE":
		// Server is done sending rooms.
		fmt.Println("GameThread: Received DONE (end of room list)")
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenRoomSelect
		ctx.State.IsConnecting = false
		ctx.StateMutex.Unlock()

		// Send ACK
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "00OK", Payload: ""}

	case "RMUP":
		// Server is sending a room update.
		room := deserializeRoom(msg.Payload) // You need to implement this!
		fmt.Printf("GameThread: Received Room Update: ID=%s\n", room.ID)
		ctx.StateMutex.Lock()
		// Just replace the existing room data
		if ctx.State.Rooms != nil {
			ctx.State.Rooms[room.ID] = room
		}
		ctx.StateMutex.Unlock()
		// No ACK needed for a broadcast update (usually)

	case "ERR": // Example error code
		ctx.StateMutex.Lock()
		// TODO: Add Notification
		ctx.State.Screen = ScreenMainMenu
		// ctx.State.ErrorMessage = msg.Payload // Show server error to user
		ctx.State.IsConnecting = false
		ctx.StateMutex.Unlock()
	}
}

// deserializeRoom is a placeholder. You must implement this
// to parse your [RoomData] payload.
func deserializeRoom(payload string) Room {
	// Example: Payload is "ID001;Poker Room 1;4;8"
	parts := strings.Split(payload, ";")
	if len(parts) >= 4 {
		// Add error handling for Atoi, etc.
		return Room{
			ID:             parts[0],
			Name:           parts[1],
			CurrentPlayers: 4, //_ = parts[2]
			MaxPlayers:     8, //_ = parts[3]
		}
	}
	// Return an empty/error room
	return Room{ID: "ERR", Name: "Invalid Room Data"}
}
