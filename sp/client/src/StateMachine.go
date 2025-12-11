package main

import (
	"errors"
	"fmt"
	"strconv"
	"time"

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
	HandleNetwork(ctx *ProgCtx, msg unet.NetEvent) LogicState
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
		ctx.NetHandler.SendCommand(unet.NetConnect{Host: evt.Host, Port: evt.Port})
	}

	return nil
}

func (s *StateMainMenu) HandleNetwork(ctx *ProgCtx, msg unet.NetEvent) LogicState {
	switch msg.(type) {
	case unet.NetConnecting:
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenConnecting
		ctx.StateMutex.Unlock()
		return &StateConnecting{false}
	}

	return nil
}

func (s *StateMainMenu) Exit(ctx *ProgCtx) {}

type StateConnecting struct {
	reconnecting bool
}

func (s *StateConnecting) Enter(ctx *ProgCtx) {
	fmt.Println("DFA: Entered Connecting State")
	nickPayload, ok := unet.WriteString(ctx.State.Nickname)
	if !ok {
		ctx.NetHandler.SendCommand(unet.NetDisconnect{})
		ctx.Popup.AddPopup("Failed parsing, catastrophe has happened", time.Second*5)
	}
	ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "CONN", Payload: nickPayload})
}

func (s *StateConnecting) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	switch input.(type) {
	case EvtAcceptReconnect:
		ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "RCON"})
		return &StateJoiningRoom{}

	case EvtDeclineReconnect:
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenConnecting
		ctx.StateMutex.Unlock()
		return &StateSendingInfo{}

	case EvtCancelConnect:
		ctx.NetHandler.SendCommand(unet.NetDisconnect{})
		return &StateMainMenu{}
	}
	return nil
}

func (s *StateConnecting) HandleNetwork(ctx *ProgCtx, msg unet.NetEvent) LogicState {
	switch evt := msg.(type) {
	case unet.NetMessage:
		switch evt.Msg.Code {
		case "PNOK":
			fmt.Println("DFA: Nick accepted.")
			return &StateSendingInfo{}

		case "FULL":
			fmt.Println("Server full")
			ctx.Popup.AddPopup("Server full", time.Second*3)
			ctx.NetHandler.SendCommand(unet.NetDisconnect{})
			return &StateMainMenu{}

		case "FAIL":
			fmt.Println("DFA: Connection Failed (FAIL).")
			ctx.NetHandler.SendCommand(unet.NetDisconnect{})
			return &StateMainMenu{}

		case "RCON":
			if !s.reconnecting {
				ctx.StateMutex.Lock()
				ctx.State.Screen = ScreenReconnecting
				ctx.StateMutex.Unlock()
			} else {
				ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "RCON"})
				ctx.State.Reconnected = true
				return &StateJoiningRoom{}
			}
		}

	case unet.NetConnected:
		return &StateConnecting{false}

	case unet.NetReconnected:
		return &StateConnecting{true}

	case unet.NetDisconnected:
		ctx.Popup.AddPopup("Connection couldn't be established", time.Second*2)
		return &StateMainMenu{}
	}

	return nil
}

func (s *StateConnecting) Exit(ctx *ProgCtx) {}

type StateSendingInfo struct{}

func (s *StateSendingInfo) Enter(ctx *ProgCtx) {
	data, _ := ctx.State.Table.Players[ctx.State.Nickname]
	chipStr, ok := unet.WriteVarInt(data.ChipCount)

	if !ok {
		ctx.NetHandler.SendCommand(unet.NetDisconnect{})
		ctx.Popup.AddPopup("Failed parsing, catastrophe has happened", time.Second*5)
		return
	}

	ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "PINF", Payload: chipStr})
}

func (s *StateSendingInfo) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	return nil
}

func (s *StateSendingInfo) HandleNetwork(ctx *ProgCtx, msg unet.NetEvent) LogicState {
	switch evt := msg.(type) {
	case unet.NetMessage:
		switch evt.Msg.Code {
		case "PIOK":
			fmt.Println("DFA: Info Accepted")
			return &StateRequestingRooms{}

		case "FAIL":
			fmt.Println("DFA: Player Info Rejected.")
			ctx.NetHandler.SendCommand(unet.NetDisconnect{})
			return &StateMainMenu{}
		}

	case unet.NetReconnected:
		return &StateConnecting{false}

	case unet.NetDisconnected:
		ctx.Popup.AddPopup("Connection lost", time.Second*3)
		return &StateMainMenu{}
	}

	return nil
}

func (s *StateSendingInfo) Exit(ctx *ProgCtx) {}

type StateRequestingRooms struct{}

func (s *StateRequestingRooms) Enter(ctx *ProgCtx) {
	fmt.Println("DFA: Requesting Rooms...")
	ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "RMRQ"})

	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenWaitingForRooms
	ctx.State.Rooms = make(map[int]Room)
	ctx.StateMutex.Unlock()
}

func (s *StateRequestingRooms) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	return nil
}

func (s *StateRequestingRooms) HandleNetwork(ctx *ProgCtx, msg unet.NetEvent) LogicState {
	switch evt := msg.(type) {
	case unet.NetMessage:
		switch evt.Msg.Code {
		case "ROOM":
			err := handleRoomData(ctx, evt.Msg)
			if err == nil {
				ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "RMOK"})
			} else {
				ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "RMFL"})
			}

		case "DONE":
			ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "DNOK"})
			return &StateLobby{}
		}

	case unet.NetReconnected:
		return &StateConnecting{false}

	case unet.NetDisconnected:
		ctx.Popup.AddPopup("Server stopped responding", time.Second*5)
		return &StateMainMenu{}
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
		ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "JOIN", Payload: payload})
		return &StateJoiningRoom{}

	case EvtBackToMain:
		ctx.NetHandler.SendCommand(unet.NetDisconnect{})
		return &StateMainMenu{}

	case EvtRefreshRooms:
		return &StateRequestingRooms{}
	}

	return nil
}

func (s *StateLobby) HandleNetwork(ctx *ProgCtx, msg unet.NetEvent) LogicState {
	switch msg.(type) {
	case unet.NetReconnected:
		return &StateConnecting{false}

	case unet.NetDisconnected:
		ctx.Popup.AddPopup("Server connection failed", time.Second*5)
		return &StateMainMenu{}
	}
	return nil
}

func (s *StateLobby) Exit(ctx *ProgCtx) {}

type StateJoiningRoom struct{}

func (s *StateJoiningRoom) Enter(ctx *ProgCtx) {}

func (s *StateJoiningRoom) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	return nil
}

func (s *StateJoiningRoom) HandleNetwork(ctx *ProgCtx, msg unet.NetEvent) LogicState {
	switch evt := msg.(type) {
	case unet.NetMessage:
		switch evt.Msg.Code {
		case "JNOK":
			fmt.Println("DFA: Join OK. Waiting for Room State...")
			return nil

		case "RMST":
			fmt.Println("DFA: Received Room State. Parsing...")

			ctx.StateMutex.Lock()
			err := deserializeRoomState(ctx, evt.Msg.Payload)
			ctx.StateMutex.Unlock()

			if err != nil {
				fmt.Printf("DFA: Failed to parse room state: %v\n", err)
				ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "STFL"})
				ctx.Popup.AddPopup("Failed to join room: invalid state", time.Second*3)
				return &StateLobby{}
			}

			fmt.Println("DFA: Room State parsed. Sending STOK.")
			ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "STOK"})
			return &StateInGame{}

		case "JNFL":
			fmt.Println("DFA: Join Failed.")
			ctx.Popup.AddPopup("Failed to join room", time.Second*3)
			return &StateLobby{}
		}

	case unet.NetReconnected:
		ctx.Popup.AddPopup("Failed to join room", time.Second*3)
		return &StateConnecting{true}

	case unet.NetReconnecting:
		ctx.Popup.AddPopup("Server stopped responding, attempting reconnect", time.Second*3)

	case unet.NetDisconnected:
		ctx.Popup.AddPopup("Server connection failed", time.Second*3)
		return &StateMainMenu{}
	}

	return nil
}

func (s *StateJoiningRoom) Exit(ctx *ProgCtx) {}

type GameAction any
type BetAction struct{ amount int }
type CallAction struct{ amount int }
type CheckAction struct{}
type FoldAction struct{}
type ReadyAction struct{}

type StateInGame struct {
	last_action GameAction
}

func (s *StateInGame) Enter(ctx *ProgCtx) {
	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenInGame
	ctx.StateMutex.Unlock()
	ctx.UI.SetDirty()
}

func (s *StateInGame) HandleInput(ctx *ProgCtx, input UserInputEvent) LogicState {
	switch evt := input.(type) {
	case EvtGameAction:
		fmt.Println("DFA: Sending Game Action ->", evt.Action, evt.Amount)

		// Validate action before sending
		if !validateGameAction(ctx, evt.Action, evt.Amount) {
			return nil
		}

		ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: evt.Action, Payload: evt.Amount})

		switch evt.Action {
		case "BETT":
			intAmount, _ := unet.ReadVarInt([]byte(evt.Amount))
			s.last_action = BetAction{int(intAmount)}
		case "CALL":
			myData, _ := ctx.State.Table.Players[ctx.State.Nickname]
			callAmount := min(myData.ChipCount, ctx.State.Table.HighBet)
			s.last_action = CallAction{callAmount}
		case "RDY1":
			s.last_action = ReadyAction{}
		case "CHCK":
			s.last_action = CheckAction{}
		case "FOLD":
			s.last_action = FoldAction{}
		case "GMLV":
			return &StateLobby{}
		}

	case EvtBackToMain:
		fmt.Println("DFA: Leaving Game")
		ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "GMLV"})
		return &StateLobby{}
	}
	return nil
}

func (s *StateInGame) HandleNetwork(ctx *ProgCtx, msg unet.NetEvent) LogicState {
	ctx.StateMutex.Lock()
	defer ctx.StateMutex.Unlock()
	defer ctx.UI.SetDirty()

	switch evt := msg.(type) {
	case unet.NetMessage:
		switch evt.Msg.Code {
		case "PJIN":
			handlePlayerJoined(ctx, evt.Msg.Payload)
			ctx.Popup.AddPopup("A player has joined", 2*time.Second)

		case "PRDY":
			nick, _ := unet.ReadString([]byte(evt.Msg.Payload))
			data, _ := ctx.State.Table.Players[nick]
			data.IsReady = true
			ctx.State.Table.Players[nick] = data

		case "GMST":
			fmt.Println("Game Started!")
			ctx.State.Table.RoundPhase = "PreFlop"
			myData, _ := ctx.State.Table.Players[ctx.State.Nickname]
			myData.Cards = make([]Card, 0)
			ctx.State.Table.Players[ctx.State.Nickname] = myData
			ctx.State.Table.CommunityCards = make([]Card, 0)
			ctx.State.Table.Pot = 0
			ctx.State.Table.HighBet = 0
			ctx.Popup.AddPopup("Game started!", 2*time.Second)

		case "CDTP":
			myData, _ := ctx.State.Table.Players[ctx.State.Nickname]
			myData.Cards = make([]Card, 0)

			parseTypes := []unet.ParseTypes{
				unet.SmallInt,
				unet.SmallInt,
			}

			results, _, err := unet.ParseMessage(evt.Msg.Payload, parseTypes)
			if err != nil {
				ctx.Popup.AddPopup("Error during parsing, disconnecting", time.Second*3)
				ctx.NetHandler.SendCommand(unet.NetDisconnect{})
				return &StateMainMenu{}
			}

			c1 := results[0].(int)
			c2 := results[1].(int)
			myData.Cards = append(myData.Cards,
				Card{ID: c1, Symbol: TranslateCardID(c1)},
				Card{ID: c2, Symbol: TranslateCardID(c2)},
			)

			ctx.State.Table.Players[ctx.State.Nickname] = myData
			ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "CDOK"})

		case "GMRD":
			fmt.Println("Handling Game Round")
			ctx.State.Table.HighBet = 0
			for name, player := range ctx.State.Table.Players {
				player.TotalBet += player.RoundBet
				player.RoundBet = 0
				player.ActionTaken = "NONE"
				player.ActionAmount = 0
				ctx.State.Table.Players[name] = player
			}

		case "CRVR":
			val, _ := strconv.Atoi(evt.Msg.Payload)
			newCard := Card{ID: val, Symbol: TranslateCardID(val)}
			ctx.State.Table.CommunityCards = append(ctx.State.Table.CommunityCards, newCard)
			ctx.Popup.AddPopup(fmt.Sprintf("Community card: %s", newCard.Symbol), 2*time.Second)

		case "PTRN":
			playerName, _ := unet.ReadString([]byte(evt.Msg.Payload))

			for name, data := range ctx.State.Table.Players {
				data.IsMyTurn = (name == playerName)
				ctx.State.Table.Players[name] = data
			}

			if playerName == ctx.State.Nickname {
				data, _ := ctx.State.Table.Players[playerName]
				ctx.Popup.AddPopup(fmt.Sprintf("Your turn! [ %t | %t ]", data.IsMyTurn, data.IsFolded), time.Second*5)
			}

		case "TOUT":
			playerName, _ := unet.ReadString([]byte(evt.Msg.Payload))
			if playerName == ctx.State.Nickname {
				ctx.Popup.AddPopup("You timed out", time.Second*2)
			} else {
				ctx.Popup.AddPopup(fmt.Sprintf("%s Timed Out", playerName), time.Second*2)
			}

			data, _ := ctx.State.Table.Players[playerName]
			data.IsMyTurn = false
			data.IsFolded = true
			ctx.State.Table.Players[playerName] = data

		case "ACOK":
			switch act := s.last_action.(type) {
			case BetAction:
				ctx.State.Table.HighBet = act.amount
				ctx.State.Table.Pot += act.amount

				data, _ := ctx.State.Table.Players[ctx.State.Nickname]
				data.ChipCount -= act.amount
				ctx.State.Table.Players[ctx.State.Nickname] = data

			case CallAction:
				ctx.State.Table.Pot += act.amount

				data, _ := ctx.State.Table.Players[ctx.State.Nickname]
				data.ChipCount -= act.amount
				ctx.State.Table.Players[ctx.State.Nickname] = data

			case ReadyAction:
				data, _ := ctx.State.Table.Players[ctx.State.Nickname]
				data.IsReady = true
				ctx.State.Table.Players[ctx.State.Nickname] = data

			case FoldAction:
				data, _ := ctx.State.Table.Players[ctx.State.Nickname]
				data.IsFolded = true
				ctx.State.Table.Players[ctx.State.Nickname] = data
			}

			fmt.Println("Action Accepted")
			ctx.Popup.AddPopup("Action accepted", 1*time.Second)

		case "ACFL":
			fmt.Println("Action Failed:", evt.Msg.Payload)
			ctx.Popup.AddPopup(fmt.Sprintf("Action failed: %s", evt.Msg.Payload), 3*time.Second)

		case "NYET":
			fmt.Println("Not your turn!")
			ctx.Popup.AddPopup("It's not your turn!", 2*time.Second)

		case "PACT":
			handlePlayerAction(ctx, evt.Msg.Payload)

		case "SDWN":
			handleShowdown(ctx, evt.Msg.Payload)

		case "GLOS":
			ctx.Popup.AddPopup("Everyone lost. Casino Won.", time.Second*3)

		case "GWIN":
			parseTypes := []unet.ParseTypes{unet.String, unet.VarInt}
			res, _, err := unet.ParseMessage(evt.Msg.Payload, parseTypes)
			if err != nil {
				fmt.Println("Server sent malformed win state :(")
			} else {
				winner := res[0].(string)
				winnerAmount := res[1].(int)

				data, _ := ctx.State.Table.Players[winner]
				data.ChipCount += winnerAmount
				ctx.State.Table.Players[winner] = data

				ctx.Popup.AddPopup(fmt.Sprintf("Player: %s won %d chips", winner, winnerAmount), 5*time.Second)
			}

		case "GMDN":
			ctx.State.Table.CommunityCards = nil
			ctx.State.Showdown = false

			ctx.State.Table.RoundPhase = ""
			ctx.State.Table.Pot = 0
			ctx.State.Table.HighBet = 0

			// Reset player round-specific state
			for name, player := range ctx.State.Table.Players {
				pCards := make([]Card, 0)
				if name != ctx.State.Nickname {
					pCards = append(pCards, Card{Hidden: true})
					pCards = append(pCards, Card{Hidden: true})
				}

				player.IsReady = false
				player.IsMyTurn = false
				player.IsFolded = false

				player.Cards = pCards
				player.RoundBet = 0
				player.TotalBet = 0
				player.ActionAmount = 0
				player.ActionTaken = "NONE"
				ctx.State.Table.Players[name] = player
			}

			ctx.Popup.AddPopup("Round ended. Starting new round...", time.Second*3)
			ctx.NetHandler.SendNetMsg(unet.NetMsg{Code: "DNOK"})
		}

	case unet.NetReconnecting:
		ctx.Popup.AddPopup("Server stopped responding, attempting reconnect.", time.Second*3)

	case unet.NetReconnected:
		fmt.Println("StateInGame: Reconnected passing true to state connecting to bypass question")
		return &StateConnecting{true}

	case unet.NetDisconnected:
		ctx.Popup.AddPopup("Server stopped responding.", time.Second*3)
		return &StateMainMenu{}
	}

	return nil
}

func (s *StateInGame) Exit(ctx *ProgCtx) {
	myData, _ := ctx.State.Table.Players[ctx.State.Nickname]
	ctx.State.Table.Players = nil
	ctx.State.Table.CommunityCards = nil
	ctx.State.Table.HighBet = 0
	ctx.State.Table.Pot = 0
	ctx.State.Table.Players = make(map[string]PlayerData)
	ctx.State.Table.Players[ctx.State.Nickname] = myData
}

func validateGameAction(ctx *ProgCtx, action string, amount string) bool {
	table := &ctx.State.Table

	if action == "RDY1" || action == "GMLV" {
		return true
	}

	if action == "SDOK" {
		return ctx.State.Showdown
	}

	myData, exists := table.Players[ctx.State.Nickname]
	if !exists {
		ctx.Popup.AddPopup("Error: Player data not found", 3*time.Second)
		return false
	}

	if !myData.IsMyTurn {
		ctx.Popup.AddPopup("It's not your turn!", 2*time.Second)
		return false
	}

	switch action {
	case "BETT":
		// Validate bet amount
		betAmt, err := strconv.Atoi(amount)
		if err != nil || betAmt <= 0 {
			ctx.Popup.AddPopup("Invalid bet amount", 2*time.Second)
			return false
		}

		if betAmt > myData.ChipCount {
			ctx.Popup.AddPopup(fmt.Sprintf("You only have %d chips", myData.ChipCount), 3*time.Second)
			return false
		}

	case "CALL":
		if table.HighBet == 0 {
			ctx.Popup.AddPopup("There's nothing to call", 2*time.Second)
			return false
		}

	case "FOLD", "CHCK":
		// These are always valid on your turn
		return true

	default:
		ctx.Popup.AddPopup("Unknown action", 2*time.Second)
		return false
	}

	return true
}

func handlePlayerJoined(ctx *ProgCtx, payload string) {
	types := []unet.ParseTypes{
		unet.String,
		unet.VarInt,
		unet.SmallInt,
		unet.SmallInt,
		unet.SmallInt,
		unet.SmallInt,
		unet.VarInt,
		unet.VarInt,
		unet.VarInt,
	}

	// we are parsing entire message, thus we ignore consumedBytes
	parseResults, _, err := unet.ParseMessage(payload, types)
	if err != nil {
		fmt.Println("Error occured: ", err.Error())
		return
	}

	pNick := parseResults[0].(string)
	pChips := parseResults[1].(int)
	pIsFolded := parseResults[2].(int)
	pIsReady := parseResults[3].(int)
	pIsMyTurn := parseResults[4].(int)
	pActionTaken := parseResults[5].(int)
	pActionAmount := parseResults[6].(int)
	pRoundBet := parseResults[7].(int)
	pTotalBet := parseResults[8].(int)

	pCards := make([]Card, 0)
	pCards = append(pCards, Card{Hidden: true})
	pCards = append(pCards, Card{Hidden: true})

	ctx.State.Table.Players[pNick] = PlayerData{
		ChipCount:    pChips,
		RoundBet:     pRoundBet,
		TotalBet:     pTotalBet,
		Cards:        pCards,
		IsMyTurn:     pIsMyTurn == 1,
		IsFolded:     pIsFolded == 1,
		IsReady:      pIsReady == 1,
		ActionTaken:  actionIntToString(pActionTaken),
		ActionAmount: pActionAmount,
	}
}

func handlePlayerAction(ctx *ProgCtx, payload string) {
	parseTypes := []unet.ParseTypes{
		unet.String,
		unet.SmallInt,
		unet.VarInt,
	}
	res, _, err := unet.ParseMessage(payload, parseTypes)
	if err != nil {
		// here a state desync will happen, maybe it would be better to disconnect at this point for resync
		return
	}

	pNick := res[0].(string)
	pActionTaken := res[1].(int)
	pActionAmount := res[2].(int)
	fmt.Println("Parsed Action: ", pNick, pActionTaken, pActionAmount)

	player, exists := ctx.State.Table.Players[pNick]
	if !exists {
		return
	}

	player.ActionTaken = actionIntToString(pActionTaken)
	player.ActionAmount = pActionAmount

	switch player.ActionTaken {
	case "BETT":
		player.RoundBet += player.ActionAmount
		player.ChipCount -= player.ActionAmount
		ctx.State.Table.HighBet = player.RoundBet
		ctx.State.Table.Pot += player.RoundBet
		ctx.Popup.AddPopup(fmt.Sprintf("%s bet %d", pNick, player.ActionAmount), 2*time.Second)
	case "CALL":
		player.RoundBet = pActionAmount
		ctx.State.Table.Pot += pActionAmount
		player.ChipCount -= pActionAmount
		ctx.Popup.AddPopup(fmt.Sprintf("%s called %d", pNick, pActionAmount), 2*time.Second)
	case "FOLD":
		player.IsFolded = true
		ctx.Popup.AddPopup(fmt.Sprintf("%s folded", pNick), 2*time.Second)
	case "CHCK":
		ctx.Popup.AddPopup(fmt.Sprintf("%s checked", pNick), 2*time.Second)
	case "LEFT":
		ctx.Popup.AddPopup(fmt.Sprintf("%s left", pNick), 2*time.Second)
		delete(ctx.State.Table.Players, pNick)
		return
	}

	ctx.State.Table.Players[pNick] = player
}

func handleShowdown(ctx *ProgCtx, payload string) {
	pCount, _ := unet.ReadSmallInt([]byte(payload))
	parseTypes := []unet.ParseTypes{
		unet.String,
		unet.SmallInt,
		unet.SmallInt,
	}

	nextPayload := string([]byte(payload[2:]))
	for range pCount {
		res, consumed, err := unet.ParseMessage(nextPayload, parseTypes)
		if err != nil {
			continue
		}

		nextPayload = string([]byte(nextPayload[consumed:]))

		pNick := res[0].(string)
		pC1 := res[1].(int)
		pC2 := res[2].(int)

		pData, exists := ctx.State.Table.Players[pNick]
		if !exists {
			continue
		}
		pData.Cards[0] = Card{Hidden: false, ID: pC1, Symbol: TranslateCardID(pC1)}
		pData.Cards[1] = Card{Hidden: false, ID: pC2, Symbol: TranslateCardID(pC2)}
	}

	ctx.State.Showdown = true
	ctx.Popup.AddPopup("Showdown! Revealing cards...", 3*time.Second)
}

func deserializeRoomState(ctx *ProgCtx, payload string) error {
	if ctx.State.Table.Players == nil {
		ctx.State.Table.Players = make(map[string]PlayerData)
	}

	ctx.State.Table.CommunityCards = make([]Card, 0)

	readTypes := []unet.ParseTypes{
		unet.VarInt,
		unet.VarInt,
		unet.SmallInt,
		unet.SmallInt,
		unet.SmallInt,
		unet.SmallInt,
	}

	res, consumed, err := unet.ParseMessage(payload, readTypes)
	if err != nil {
		return errors.Join(err, fmt.Errorf("Error occured during 1st phase"))
	}

	nextPayload := string([]byte(payload[consumed:]))

	ctx.State.Table.Pot = res[0].(int)
	ctx.State.Table.HighBet = res[1].(int)
	cardsDealt := res[2].(int) == 1
	card1 := res[3].(int)
	card2 := res[4].(int)
	commCount := res[5].(int)

	readCardTypes := make([]unet.ParseTypes, commCount+1)
	for i := range readCardTypes {
		readCardTypes[i] = unet.SmallInt
	}

	res, consumed, err = unet.ParseMessage(nextPayload, readCardTypes)
	if err != nil {
		return errors.Join(err, fmt.Errorf("Error occured during reading CommunityCards and PlayerCount"))
	}

	nextPayload = string([]byte(nextPayload[consumed:]))

	for index := range commCount {
		cardID := res[index].(int)
		ctx.State.Table.CommunityCards = append(ctx.State.Table.CommunityCards, Card{
			ID:     cardID,
			Symbol: TranslateCardID(cardID),
		})
	}

	playerCount := res[commCount].(int)

	readPlayerTypes := []unet.ParseTypes{
		unet.String,
		unet.VarInt,
		unet.SmallInt,
		unet.SmallInt,
		unet.SmallInt,
		unet.SmallInt,
		unet.VarInt,
		unet.VarInt,
		unet.VarInt,
	}

	for range playerCount {
		res, consumed, err := unet.ParseMessage(nextPayload, readPlayerTypes)
		if err != nil {
			return errors.Join(err, fmt.Errorf("Error occured during reading player"))
		}

		nextPayload = string([]byte(nextPayload[consumed:]))

		pNick := res[0].(string)
		pChips := res[1].(int)
		pIsFolded := res[2].(int)
		pIsReady := res[3].(int)
		pIsMyTurn := res[4].(int)
		pActionTaken := res[5].(int)
		pActionAmount := res[6].(int)
		pRoundBet := res[7].(int)
		pTotalBet := res[8].(int)

		pCards := make([]Card, 0)
		pCards = append(pCards, Card{Hidden: true})
		pCards = append(pCards, Card{Hidden: true})

		pData := PlayerData{
			RoundBet:     pRoundBet,
			TotalBet:     pTotalBet,
			ChipCount:    pChips,
			IsFolded:     pIsFolded == 1,
			IsReady:      pIsReady == 1,
			IsMyTurn:     pIsMyTurn == 1,
			ActionTaken:  actionIntToString(pActionTaken),
			ActionAmount: pActionAmount,
			Cards:        pCards,
		}

		if pNick == ctx.State.Nickname && cardsDealt {
			pData.Cards[0] = Card{ID: card1, Symbol: TranslateCardID(card1), Hidden: false}
			pData.Cards[1] = Card{ID: card2, Symbol: TranslateCardID(card2), Hidden: false}
		}

		ctx.State.Table.Players[pNick] = pData
	}

	fmt.Println(ctx.State.Table.Players)

	return nil
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
