package main

import (
	"fmt"
	"time"

	unet "poker-client/ups_net"
)

func gameThread(ctx *ProgCtx) {
	fmt.Println("GameThread started.")
	// ticker := time.NewTicker(1 * time.Second)
	// defer ticker.Stop()

	for !ctx.ShouldClose {
		select {
		case input := <-ctx.UserInputChan:
			handleUserInput(ctx, input)

		case msg, ok := <-ctx.NetMsgInChan:
			if !ok {
				// Channel closed, network handler shut down
				fmt.Println("NetMsgInChan closed. Shutting down GameThread.")
				ctx.StateMutex.Lock()
				ctx.State.Screen = ScreenMainMenu
				ctx.Popup.AddPopup("Connection lost.", time.Second*3)
				ctx.StateMutex.Unlock()
				ctx.ShouldClose = true
				break
			}
			fmt.Println("GamneThread: Got message")

			handleNetworkMessage(ctx, msg)

			// case <-ticker.C:
			// 	if ctx.State.Screen == ScreenInGame {
			// 		ctx.NetMsgOutChan <- unet.NetMsg{Code: "PING"}
			// 	}
		default:
			time.Sleep(time.Millisecond * 20)
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

		go func(host, port string) {
			success := ctx.NetHandler.Connect(host, port)
			fmt.Println("GameThread: Connect result:", success)

			if !success {
				ctx.StateMutex.Lock()
				ctx.State.Screen = ScreenServerSelect
				// here I need to implement the notification system
				ctx.Popup.AddPopup("Failed to connect to "+host+":"+port, time.Second*3)
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

	ctx.StateMutex.RLock()
	currentScreen := ctx.State.Screen
	ctx.StateMutex.RUnlock()

	/* if currentScreen == ScreenConnecting && msg.Code != "00OK" {
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenMainMenu
		ctx.Popup.AddPopup("Unexpected Response during connection", time.Second*3)
		ctx.State.IsConnecting = false
		ctx.StateMutex.Unlock()
	} */

	switch msg.Code {
	case "00OK":
		if currentScreen == ScreenConnecting {
			fmt.Println("GameThread: Connection OK, requesting rooms...")
			ctx.NetMsgOutChan <- unet.NetMsg{Code: "RMRQ", Payload: ""}
		}

	case "ROOM":
		room := deserializeRoom(msg.Payload)
		fmt.Printf("GameThread: Received Room: ID=%s, Name=%s\n", room.ID, room.Name)
		ctx.StateMutex.Lock()
		if ctx.State.Rooms == nil {
			ctx.State.Rooms = make(map[int]Room)
		}
		ctx.State.Rooms[room.ID] = room
		ctx.StateMutex.Unlock()

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
		room := deserializeRoom(msg.Payload)
		fmt.Printf("GameThread: Received Room Update: ID=%s\n", room.ID)
		ctx.StateMutex.Lock()
		// Just replace the existing room data
		if ctx.State.Rooms != nil {
			ctx.State.Rooms[room.ID] = room
		}
		ctx.StateMutex.Unlock()
		// No ACK needed for a broadcast update

	case "ERR": // Example error code
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenMainMenu
		ctx.Popup.AddPopup(msg.Payload, time.Second*3)
		ctx.State.IsConnecting = false
		ctx.StateMutex.Unlock()
	}
}

func deserializeRoom(payload string) Room {
	err_room := Room{ID: -1, Name: "Invalid Room Data"}
	valid_room := Room{}

	offset := 0
	byte_payload := []byte(payload[offset:])

	id, ok := unet.ReadBigInt(byte_payload)

	if !ok {
		return err_room
	} else {
		valid_room.ID = id
	}

	offset += 4
	name, ok := unet.ReadString(byte_payload[offset:])

	if !ok {
		return err_room
	} else {
		valid_room.Name = name
	}

	offset += 4 + len(name)
	curr_players, ok := unet.ReadSmallInt(byte_payload[offset:])

	if !ok {
		return err_room
	} else {
		valid_room.CurrentPlayers = curr_players
	}

	offset += 2
	max_players, ok := unet.ReadSmallInt(byte_payload[offset:])

	if !ok {
		return err_room
	} else {
		valid_room.MaxPlayers = max_players
	}

	return valid_room
}
