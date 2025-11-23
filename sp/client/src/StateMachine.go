package main

import (
	"fmt"
	"strconv"

	unet "poker-client/ups_net"
)

func TranslateCardID(id int) string {
	if id < 0 || id > 51 {
		return "??"
	}

	ranks := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "Jack", "Queen", "King", "Ace"}
	suits := []string{"Hearts", "Diamonds", "Clubs", "Spades"}

	suitIdx := id / 13
	rankIdx := id % 13

	if suitIdx >= len(suits) {
		return "??"
	}

	return ranks[rankIdx] + " of " + suits[suitIdx]
}

type LogicState interface {
	Enter(ctx *ProgCtx)
	HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState
	HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState
	Exit(ctx *ProgCtx)
}

type StateMainMenu struct{}

func (s *StateMainMenu) Enter(ctx *ProgCtx) {
	fmt.Println("DFA: Entered Menu State")

	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenMainMenu
	ctx.StateMutex.Unlock()
}

func (s *StateMainMenu) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	switch evt := input.(type) {
	case EvtConnect:
		fmt.Println("Connection to:", evt.Host+":"+evt.Port)
		startConnection(ctx, evt.Host, evt.Port)
		return &StateConnecting{}
	}
	return nil
}

func (s *StateMainMenu) HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState {
	return nil
}

func (s *StateMainMenu) Exit(ctx *ProgCtx) {}

type StateConnecting struct{}

func (s *StateConnecting) Enter(ctx *ProgCtx) {
	fmt.Println("DFA: Entered Connecting State")

	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenConnecting
	ctx.StateMutex.Unlock()
}

func (s *StateConnecting) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	switch input.(type) {
	case EvtCancelConnect:
		ctx.NetHandler.Disconnect()
		return &StateMainMenu{}
	}
	return nil
}

func (s *StateConnecting) HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState {
	switch msg.Code {
	case "00OK":
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "RMRQ", Payload: ""}
		return nil
	case "ROOM", "DONE":
		if msg.Code == "DONE" {
			ctx.StateMutex.Lock()
			ctx.State.Screen = ScreenRoomSelect
			ctx.StateMutex.Unlock()
			return &StateLobby{}
		}
		handleRoomData(ctx, msg)
	}
	return nil
}

func (s *StateConnecting) Exit(ctx *ProgCtx) {}

type StateLobby struct{}

func (s *StateLobby) Enter(ctx *ProgCtx) {
	fmt.Println("DFA: Entered Lobby")

	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenRoomSelect
	ctx.StateMutex.Unlock()
}

func (s *StateLobby) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	switch evt := input.(type) {
	case EvtRoomJoin:
		idInt, _ := strconv.Atoi(evt.RoomID)
		payload := fmt.Sprintf("%04d", idInt)

		fmt.Printf("DFA: Joining Room %s with payload %s\n", evt.RoomID, payload)
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "JOIN", Payload: payload}

		// TODO:
		// here this can fail, we need to wait for an ok from the server
		// which isn't currently being sent but it has to be implemented
		return &StateInGame{}

	case EvtBackToMain:
		ctx.NetHandler.Disconnect()
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenMainMenu
		ctx.StateMutex.Unlock()
		return &StateMainMenu{}
	}
	return nil
}

func (s *StateLobby) HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState {
	if msg.Code == "RMUP" || msg.Code == "ROOM" {
		handleRoomData(ctx, msg)
	}
	// TODO:
	// implement msg.Code == "MVTR" (moved to room) to trigger the DFA state change
	return nil
}

func (s *StateLobby) Exit(ctx *ProgCtx) {}

type StateInGame struct{}

func (s *StateInGame) Enter(ctx *ProgCtx) {
	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenInGame
	// Reset table data
	ctx.State.Table = PokerTable{
		MyHand:         make([]Card, 0),
		CommunityCards: make([]Card, 0),
		Players:        make(map[int]PlayerData),
	}
	ctx.StateMutex.Unlock()
	ctx.UI.SetDirty()
}

func (s *StateInGame) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	switch evt := input.(type) {
	case EvtGameAction:
		// Translate UI clicks to Net Messages
		fmt.Println("DFA: Sending Game Action ->", evt.Action, evt.Amount)
		ctx.NetMsgOutChan <- unet.NetMsg{Code: evt.Action, Payload: evt.Amount}
		if evt.Action == "GMLV" {
			fmt.Println("DFA: Returning to state lobby (GMLV)")
			return &StateLobby{}
		}
	case EvtBackToMain:
		// Leave room logic
		fmt.Println("DFA: Returning to state lobby (EvtBackToMain)")
		return &StateLobby{} // Simplified
	}
	return nil
}

func (s *StateInGame) HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState {
	ctx.StateMutex.Lock()
	defer ctx.StateMutex.Unlock()
	defer ctx.UI.SetDirty() // Almost any net message here changes UI

	switch msg.Code {
	case "GMST":
		fmt.Println("Game Started!")
		ctx.State.Table.MyHand = make([]Card, 0)
		ctx.State.Table.CommunityCards = make([]Card, 0)
		ctx.State.Table.Pot = 0

	case "CDTP":
		val, _ := strconv.Atoi(msg.Payload)
		newCard := Card{ID: val, Symbol: TranslateCardID(val)}
		ctx.State.Table.MyHand = append(ctx.State.Table.MyHand, newCard)
		fmt.Printf("Got Card: %s\n", newCard.Symbol)

	case "CRVR":
		val, _ := strconv.Atoi(msg.Payload)
		newCard := Card{ID: val, Symbol: TranslateCardID(val)}
		ctx.State.Table.CommunityCards = append(ctx.State.Table.CommunityCards, newCard)

	case "TURN": // It's someone's turn
		// Payload "00", "01" etc (Player Index)
		// Check if it's us? We don't know our own index yet in this simplified client
		// But we can show who's thinking.

	case "BETT", "CALL", "FOLD":
		// Update pot, player status, etc.
		// Payload format based on server: "00" (PlayerID) or "000100" (PlayerID + Amount)

	// we are being moved back into the lobby
	case "GMDN":
		ctx.State.Table.CommunityCards = nil
		ctx.State.Table.MyHand = nil
		ctx.State.Table.Pot = 0
	}

	return nil
}

func (s *StateInGame) Exit(ctx *ProgCtx) {
}

func startConnection(ctx *ProgCtx, host, port string) {
	ctx.State.IsConnecting = true

	go func() {
		success := ctx.NetHandler.Connect(host, port)
		if !success {
			ctx.UserInputChan <- EvtCancelConnect{} // Simple retry trigger
			return
		}

		ctx.NetMsgInChan = ctx.NetHandler.MsgIn()
		ctx.NetMsgOutChan = ctx.NetHandler.MsgOut()
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "CONN", Payload: ""}
	}()
}

func handleRoomData(ctx *ProgCtx, msg unet.NetMsg) {
	// existing deserialize logic...
	// We need to expose deserializeRoom or copy it here.
	// For brevity, assuming we access the one in gamethread or move it to a shared utility.

	room := deserializeRoom(msg.Payload)
	if room.ID != -1 {
		// valid room, send ok and proceed
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "00OK"}
	} else {
		// invalid room, send fail and do not process further
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "FAIL"}
		return
	}

	fmt.Printf("GameThread: Received Room: ID=%d, Name=%s\n", room.ID, room.Name)
	ctx.StateMutex.Lock()
	if ctx.State.Rooms == nil {
		ctx.State.Rooms = make(map[int]Room)
	}
	ctx.State.Rooms[room.ID] = room
	ctx.StateMutex.Unlock()
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
