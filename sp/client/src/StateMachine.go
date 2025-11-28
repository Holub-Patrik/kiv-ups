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
	case "PNOK":
		fmt.Println("DFA: Nick accepted.")
		return &StateSendingInfo{}

	case "FAIL":
		fmt.Println("DFA: Connection Failed (FAIL).")
		ctx.NetHandler.Disconnect()
		return &StateMainMenu{}

	case "RCON":
		return &StateReconnecting{}
	}
	return nil
}

func (s *StateConnecting) Exit(ctx *ProgCtx) {}

type StateReconnecting struct{}

func (s *StateReconnecting) Enter(ctx *ProgCtx) {
	fmt.Println("DFA: Reconnecting...")
	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenReconnecting
	ctx.StateMutex.Unlock()
}

func (s *StateReconnecting) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	switch input.(type) {
	case EvtAcceptReconnect:
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "RCON"}
		ctx.State.Reconnected = true
		return &StateJoiningRoom{}

	case EvtDeclineReconnect:
		fmt.Println("User declined reconnect, sending PlayerInfo as if RCON wasn't offered")
		ctx.State.Reconnected = false
		return &StateSendingInfo{}
	}
	return nil
}

func (s *StateReconnecting) HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState {
	return nil
}

func (s *StateReconnecting) Exit(ctx *ProgCtx) {}

type StateSendingInfo struct{}

func (s *StateSendingInfo) Enter(ctx *ProgCtx) {
	chipStr, _ := unet.WriteVarInt(ctx.State.PlayerCfg.StartingChips)

	ctx.NetMsgOutChan <- unet.NetMsg{Code: "PINF", Payload: chipStr}
	fmt.Println("DFA: Sending Player Info...")
}

func (s *StateSendingInfo) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	return nil
}

func (s *StateSendingInfo) HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState {
	switch msg.Code {
	case "PIOK":
		// Info Accepted. Now we request rooms.
		// Client: PKRNRMRQ
		fmt.Println("DFA: Info Accepted. Requesting Rooms...")
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "RMRQ", Payload: ""}
		return &StateRequestingRooms{}

	case "FAIL":
		fmt.Println("DFA: Player Info Rejected.")
		ctx.NetHandler.Disconnect()
		return &StateMainMenu{}
	}
	return nil
}

func (s *StateSendingInfo) Exit(ctx *ProgCtx) {}

type StateRequestingRooms struct{}

func (s *StateRequestingRooms) Enter(ctx *ProgCtx) {
	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenWaitingForRooms
	// Clear existing rooms on refresh
	ctx.State.Rooms = make(map[int]Room)
	ctx.StateMutex.Unlock()
}

func (s *StateRequestingRooms) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	return nil
}

func (s *StateRequestingRooms) HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState {
	switch msg.Code {
	case "ROOM":
		err := handleRoomData(ctx, msg)
		if err == nil {
			ctx.NetMsgOutChan <- unet.NetMsg{Code: "RMOK"}
		} else {
			ctx.NetMsgOutChan <- unet.NetMsg{Code: "RMFL"}
		}
		return nil

	case "DONE":
		return &StateLobby{}
	}
	return nil
}

func (s *StateRequestingRooms) Exit(ctx *ProgCtx) {}

type StateLobby struct{}

func (s *StateLobby) Enter(ctx *ProgCtx) {
	fmt.Println("DFA: Entered Lobby")
	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenRoomSelect
	ctx.StateMutex.Unlock()
	ctx.UI.SetDirty()
}

func (s *StateLobby) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	switch evt := input.(type) {
	case EvtRoomJoin:
		idInt, _ := strconv.Atoi(evt.RoomID)
		payload := fmt.Sprintf("%04d", idInt)

		fmt.Printf("DFA: Joining Room %s with payload %s\n", evt.RoomID, payload)
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "JOIN", Payload: payload}
		return &StateJoiningRoom{}

	case EvtBackToMain:
		ctx.NetHandler.Disconnect()
		return &StateMainMenu{}
	}
	return nil
}

func (s *StateLobby) HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState {
	if msg.Code == "RMUP" {
		// PKRPRMUP[RoomUpdate]
		// For now we might just ignore partial updates or try to parse
		// Sending OK just to keep protocol happy if strictly required,
		// but spec says UPOK | UPFL

		ctx.NetMsgOutChan <- unet.NetMsg{Code: "UPOK"}
	}
	return nil
}

func (s *StateLobby) Exit(ctx *ProgCtx) {}

type StateJoiningRoom struct{}

func (s *StateJoiningRoom) Enter(ctx *ProgCtx) {}

func (s *StateJoiningRoom) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	return nil
}

func (s *StateJoiningRoom) HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState {
	switch msg.Code {
	case "JNOK":
		fmt.Println("DFA: Join OK. Waiting for Room State...")
		return nil

	case "RMST":
		// if reconnecting, the information we are given is different
		if ctx.State.Reconnected {
		}
		// PKRPRMST[RoomState]
		// Parse room state (players, etc)
		// For now we assume it parses correctly
		fmt.Println("DFA: Received Room State. Sending STOK.")
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "STOK"}
		return &StateInGame{}

	case "JNFL":
		fmt.Println("DFA: Join Failed.")
		return &StateLobby{}
	}
	return nil
}

func (s *StateJoiningRoom) Exit(ctx *ProgCtx) {}

type StateInGame struct{}

func (s *StateInGame) Enter(ctx *ProgCtx) {
	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenInGame
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
		fmt.Println("DFA: Sending Game Action ->", evt.Action, evt.Amount)

		switch evt.Action {
		case "RDY1":
			ctx.NetMsgOutChan <- unet.NetMsg{Code: "RDY1"}
		case "GMLV":
			ctx.NetMsgOutChan <- unet.NetMsg{Code: "GMLV"}
			return &StateLobby{}
		default:
			ctx.NetMsgOutChan <- unet.NetMsg{Code: evt.Action, Payload: evt.Amount}
		}

	case EvtBackToMain:
		fmt.Println("DFA: Leaving Game")
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "GMLV"}
		return &StateLobby{}
	}
	return nil
}

func (s *StateInGame) HandleNetwork(ctx *ProgCtx, msg unet.NetMsg) LogicState {
	ctx.StateMutex.Lock()
	defer ctx.StateMutex.Unlock()
	defer ctx.UI.SetDirty()

	switch msg.Code {
	case "GMST":
		fmt.Println("Game Started!")
		ctx.State.Table.MyHand = make([]Card, 0)
		ctx.State.Table.CommunityCards = make([]Card, 0)
		ctx.State.Table.Pot = 0

	case "CDTP":
		pBytes := []byte(msg.Payload)
		// Parse Card 1
		val1, ok1 := unet.ReadSmallInt(pBytes)
		if ok1 {
			newCard := Card{ID: val1, Symbol: TranslateCardID(val1)}
			ctx.State.Table.MyHand = append(ctx.State.Table.MyHand, newCard)

			// Try Parse Card 2 (offset 2 bytes for SmallInt)
			if len(pBytes) >= 4 {
				val2, ok2 := unet.ReadSmallInt(pBytes[2:])
				if ok2 {
					newCard2 := Card{ID: val2, Symbol: TranslateCardID(val2)}
					ctx.State.Table.MyHand = append(ctx.State.Table.MyHand, newCard2)
				}
			}
		}

		ctx.NetMsgOutChan <- unet.NetMsg{Code: "CDOK"}

	case "CRVR":
		val, _ := strconv.Atoi(msg.Payload) // Or ReadSmallInt
		newCard := Card{ID: val, Symbol: TranslateCardID(val)}
		ctx.State.Table.CommunityCards = append(ctx.State.Table.CommunityCards, newCard)

	case "PTRN": // Turn
		// Broadcast(PKRPPTRN[PlayerID])
		// Highlight player.

	case "ACOK":
		fmt.Println("Action Accepted")

	case "ACFL":
		fmt.Println("Action Failed/Invalid")

	case "BETT", "CALL", "FOLD", "CHCK":
		// Broadcasts of other players' actions

	case "SDWN":
		// Add show down logic parsing
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "SDOK"}

	case "GMDN":
		ctx.State.Table.CommunityCards = nil
		ctx.State.Table.MyHand = nil
		ctx.State.Table.Pot = 0
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "DNOK"}
	}

	return nil
}

func (s *StateInGame) Exit(ctx *ProgCtx) {}

func startConnection(ctx *ProgCtx, host, port string) {
	ctx.State.IsConnecting = true

	go func() {
		success := ctx.NetHandler.Connect(host, port)
		if !success {
			ctx.UserInputChan <- EvtCancelConnect{}
			return
		}

		ctx.NetMsgInChan = ctx.NetHandler.MsgIn()
		ctx.NetMsgOutChan = ctx.NetHandler.MsgOut()

		nickPayload, _ := unet.WriteString(ctx.State.PlayerCfg.NickName)

		ctx.NetMsgOutChan <- unet.NetMsg{Code: "CONN", Payload: nickPayload}
	}()
}

func handleRoomData(ctx *ProgCtx, msg unet.NetMsg) error {
	room := deserializeRoom(msg.Payload)
	if room.ID == -1 {
		return fmt.Errorf("invalid room")
	}

	fmt.Printf("GameThread: Received Room: ID=%d, Name=%s\n", room.ID, room.Name)
	ctx.StateMutex.Lock()
	if ctx.State.Rooms == nil {
		ctx.State.Rooms = make(map[int]Room)
	}
	ctx.State.Rooms[room.ID] = room
	ctx.StateMutex.Unlock()
	return nil
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

	// 4 bytes for len + len(name)
	nameLen, _ := unet.ReadBigInt(byte_payload[4:]) // Re-read len to calculate offset
	offset += 4 + nameLen

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
