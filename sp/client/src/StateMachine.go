package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
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
		fmt.Println("DFA: Received Room State. Parsing...")

		// Deserialize room state based on reconnect status
		table, err := deserializeRoomState(msg.Payload, ctx.State.Reconnected, ctx.State.PlayerCfg.NickName)
		if err != nil {
			fmt.Printf("DFA: Failed to parse room state: %v\n", err)
			ctx.NetMsgOutChan <- unet.NetMsg{Code: "STFL"}
			ctx.Popup.AddPopup("Failed to join room: invalid state", 3*time.Second)
			return &StateLobby{}
		}

		ctx.StateMutex.Lock()
		ctx.State.Table = table
		ctx.StateMutex.Unlock()

		fmt.Println("DFA: Room State parsed. Sending STOK.")
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "STOK"}
		return &StateInGame{}

	case "JNFL":
		fmt.Println("DFA: Join Failed.")
		ctx.Popup.AddPopup("Failed to join room", 3*time.Second)
		return &StateLobby{}
	}
	return nil
}

func (s *StateJoiningRoom) Exit(ctx *ProgCtx) {}

type StateInGame struct{}

func (s *StateInGame) Enter(ctx *ProgCtx) {
	ctx.StateMutex.Lock()
	ctx.State.Screen = ScreenInGame
	// Initialize table if not set by room state
	if ctx.State.Table.Players == nil {
		ctx.State.Table = PokerTable{
			CommunityCards: make([]Card, 0),
			Players:        make(map[string]PlayerData),
			MyNickname:     ctx.State.PlayerCfg.NickName,
		}
	}
	ctx.State.Table.Players[ctx.State.Table.MyNickname] = PlayerData{
		ChipCount:    ctx.State.PlayerCfg.StartingChips,
		RoundBet:     0,
		Cards:        make([]Card, 0),
		IsMyTurn:     false,
		IsReady:      false,
		ActionTaken:  "NONE",
		ActionAmount: 0,
	}
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
		if evt.Action == "RDY1" {
			data, _ := ctx.State.Table.Players[ctx.State.Table.MyNickname]
			data.IsReady = true
			ctx.State.Table.Players[ctx.State.Table.MyNickname] = data
		}

		ctx.NetMsgOutChan <- unet.NetMsg{Code: evt.Action, Payload: evt.Amount}

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
	case "PJIN":
		// Player joined, update player list
		handlePlayerJoined(ctx, msg.Payload)
		ctx.Popup.AddPopup("A player has joined", 2*time.Second)

	case "PRDY":
		nick, _ := unet.ReadString([]byte(msg.Payload))
		data, _ := ctx.State.Table.Players[nick]
		data.IsReady = true
		ctx.State.Table.Players[nick] = data

	case "GMST":
		fmt.Println("Game Started!")
		ctx.State.Table.RoundPhase = "PreFlop"
		myData, _ := ctx.State.Table.Players[ctx.State.Table.MyNickname]
		myData.Cards = make([]Card, 0)
		ctx.State.Table.Players[ctx.State.Table.MyNickname] = myData
		ctx.State.Table.CommunityCards = make([]Card, 0)
		ctx.State.Table.Pot = 0
		ctx.State.Table.RoundBet = 0
		ctx.State.Table.CurrentBet = 0
		ctx.Popup.AddPopup("Game started!", 2*time.Second)

	case "CDTP":
		// Clear previous hand
		myData, _ := ctx.State.Table.Players[ctx.State.Table.MyNickname]
		myData.Cards = make([]Card, 0)

		pBytes := []byte(msg.Payload)
		// Parse Card 1
		val1, ok1 := unet.ReadSmallInt(pBytes)
		if ok1 {
			newCard := Card{ID: val1, Symbol: TranslateCardID(val1)}
			myData.Cards = append(myData.Cards, newCard)

			// Try Parse Card 2 (offset 2 bytes for SmallInt)
			if len(pBytes) >= 4 {
				val2, ok2 := unet.ReadSmallInt(pBytes[2:])
				if ok2 {
					newCard2 := Card{ID: val2, Symbol: TranslateCardID(val2)}
					myData.Cards = append(myData.Cards, newCard2)
				}
			}
		}
		ctx.State.Table.Players[ctx.State.Table.MyNickname] = myData
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "CDOK"}

	case "CRVR":
		val, _ := strconv.Atoi(msg.Payload)
		newCard := Card{ID: val, Symbol: TranslateCardID(val)}
		ctx.State.Table.CommunityCards = append(ctx.State.Table.CommunityCards, newCard)
		ctx.Popup.AddPopup(fmt.Sprintf("Community card: %s", newCard.Symbol), 2*time.Second)

	case "PTRN":
		playerName, _ := unet.ReadString([]byte(msg.Payload))
		for name, player := range ctx.State.Table.Players {
			player.IsMyTurn = (name == playerName)
			ctx.State.Table.Players[name] = player
		}

		if playerName == ctx.State.Table.MyNickname {
			data, _ := ctx.State.Table.Players[playerName]
			ctx.Popup.AddPopup(fmt.Sprintf("Your turn! %t", data.IsMyTurn), 2*time.Second)
		}

	case "ACOK":
		fmt.Println("Action Accepted")
		ctx.Popup.AddPopup("Action accepted", 1*time.Second)

	case "ACFL":
		fmt.Println("Action Failed:", msg.Payload)
		ctx.Popup.AddPopup(fmt.Sprintf("Action failed: %s", msg.Payload), 3*time.Second)

	case "NYET":
		fmt.Println("Not your turn!")
		ctx.Popup.AddPopup("It's not your turn!", 2*time.Second)

	case "PACT":
		// Player action broadcast
		handlePlayerAction(ctx, msg.Payload)

	case "SDWN":
		// Showdown - reveal all cards
		handleShowdown(ctx, msg.Payload)
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "SDOK"}

	case "GWIN":
		// Winner announcement
		ctx.Popup.AddPopup(fmt.Sprintf("Winner: %s", msg.Payload), 5*time.Second)

	case "GMDN":
		// Game end, reset for next round
		ctx.State.Table.CommunityCards = nil
		myData, _ := ctx.State.Table.Players[ctx.State.Table.MyNickname]
		myData.Cards = nil
		ctx.State.Table.Players[ctx.State.Table.MyNickname] = myData
		ctx.State.Table.Pot = 0
		ctx.State.Table.RoundBet = 0
		ctx.State.Table.CurrentBet = 0

		// Reset player round-specific state
		for name, player := range ctx.State.Table.Players {
			player.RoundBet = 0
			player.ActionTaken = "NONE"
			player.ActionAmount = 0
			ctx.State.Table.Players[name] = player
		}

		ctx.Popup.AddPopup("Round ended. Starting new round...", 3*time.Second)
		ctx.NetMsgOutChan <- unet.NetMsg{Code: "DNOK"}
	}

	return nil
}

func (s *StateInGame) Exit(ctx *ProgCtx) {}

func validateGameAction(ctx *ProgCtx, action string, amount string) bool {
	table := &ctx.State.Table

	if action == "RDY1" || action == "GMLV" {
		return true
	}

	myData, exists := table.Players[table.MyNickname]
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

		// Check if player has enough chips
		if betAmt > myData.ChipCount {
			ctx.Popup.AddPopup(fmt.Sprintf("You only have %d chips", myData.ChipCount), 3*time.Second)
			return false
		}

		// Check minimum bet (must be at least current bet)
		if betAmt < table.CurrentBet {
			ctx.Popup.AddPopup(fmt.Sprintf("Bet must be at least %d", table.CurrentBet), 3*time.Second)
			return false
		}

	case "CALL":
		// Can only call if there's a bet
		if table.CurrentBet == 0 {
			ctx.Popup.AddPopup("There's nothing to call", 2*time.Second)
			return false
		}

		// Check if player has enough chips
		if table.CurrentBet > myData.ChipCount {
			ctx.Popup.AddPopup(fmt.Sprintf("You need %d chips to call", table.CurrentBet), 3*time.Second)
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
	// Parse PJIN payload: player nickname
	payloadBytes := []byte(payload)
	playerName, _ := unet.ReadString(payloadBytes)
	playerChips, _ := unet.ReadVarInt(payloadBytes[4+len(playerName):])

	// Add new player with default values
	ctx.State.Table.Players[playerName] = PlayerData{
		ChipCount:    int(playerChips),
		RoundBet:     0,
		Cards:        make([]Card, 0),
		IsMyTurn:     false,
		IsFolded:     false,
		IsReady:      false,
		ActionTaken:  "NONE",
		ActionAmount: 0,
	}
}

func handlePlayerAction(ctx *ProgCtx, payload string) {
	// Parse PACT payload format: "nickname:ACTION:amount"
	parts := strings.Split(payload, ":")
	if len(parts) < 3 {
		return
	}

	payloadBytes := []byte(payload)
	totalBytesRead := 0
	nickname, _ := unet.ReadString(payloadBytes[totalBytesRead:])
	action, _ := unet.ReadSmallInt(payloadBytes[len(nickname)+4:])
	amount, _ := unet.ReadVarInt(payloadBytes[len(nickname)+4+2:])

	ctx.StateMutex.Lock()
	defer ctx.StateMutex.Unlock()

	player, exists := ctx.State.Table.Players[nickname]
	if !exists {
		return
	}

	player.ActionTaken = actionIntToString(action)
	player.ActionAmount = int(amount)

	switch player.ActionTaken {
	case "BETT":
		player.RoundBet += player.ActionAmount
		ctx.State.Table.CurrentBet = player.RoundBet
		ctx.Popup.AddPopup(fmt.Sprintf("%s bet %d", nickname, player.ActionAmount), 2*time.Second)
	case "CALL":
		player.RoundBet = ctx.State.Table.CurrentBet
		callAmount := ctx.State.Table.CurrentBet - (player.RoundBet - player.ActionAmount)
		ctx.Popup.AddPopup(fmt.Sprintf("%s called %d", nickname, callAmount), 2*time.Second)
	case "FOLD":
		player.IsFolded = true
		ctx.Popup.AddPopup(fmt.Sprintf("%s folded", nickname), 2*time.Second)
	case "CHCK":
		ctx.Popup.AddPopup(fmt.Sprintf("%s checked", nickname), 2*time.Second)
	}

	ctx.State.Table.Players[nickname] = player
}

func handleShowdown(ctx *ProgCtx, payload string) {
	// Parse SDWN payload: series of cards and player info
	// For simplicity, just show a popup
	_ = payload
	ctx.Popup.AddPopup("Showdown! Revealing cards...", 3*time.Second)
}

func deserializeRoomState(payload string, isReconnect bool, myNickname string) (PokerTable, error) {
	bytes := []byte(payload)
	offset := 0
	table := PokerTable{
		CommunityCards: make([]Card, 0),
		Players:        make(map[string]PlayerData),
		MyNickname:     myNickname,
	}

	readVarInt := func() (int, error) {
		val, ok := unet.ReadVarInt(bytes[offset:])
		if !ok {
			return 0, fmt.Errorf("failed to read var int")
		}
		digits := countDigits(int(val))
		fmt.Println("Adding:", 2+digits)
		offset += 2 + digits
		return int(val), nil
	}

	pot, err := readVarInt()
	if err != nil {
		return table, errors.Join(err, fmt.Errorf("Pot Error"))
	}
	table.Pot = pot

	currentBet, err := readVarInt()
	if err != nil {
		return table, errors.Join(err, fmt.Errorf("Current Bet Error"))
	}
	table.CurrentBet = currentBet

	commCount, ok := unet.ReadSmallInt(bytes[offset:])
	if !ok {
		return table, fmt.Errorf("failed to read community card count")
	}
	offset += 2

	for range commCount {
		cardID, ok := unet.ReadSmallInt(bytes[offset:])
		if !ok {
			return table, fmt.Errorf("failed to read community card")
		}
		table.CommunityCards = append(table.CommunityCards, Card{
			ID:     cardID,
			Symbol: TranslateCardID(cardID),
		})
		offset += 2
	}

	if isReconnect {
		myChips, err := readVarInt()
		if err != nil {
			return table, errors.Join(err, fmt.Errorf("My Chips Error"))
		}

		isFolded, ok := unet.ReadSmallInt(bytes[offset:])
		if !ok {
			return table, fmt.Errorf("failed to read is folded")
		}
		offset += 2

		isReady, ok := unet.ReadSmallInt(bytes[offset:])
		if !ok {
			return table, fmt.Errorf("failed to read is ready")
		}
		offset += 2

		actionTaken, ok := unet.ReadSmallInt(bytes[offset:])
		if !ok {
			return table, fmt.Errorf("failed to read action taken")
		}
		offset += 2

		actionAmount, err := readVarInt()
		if err != nil {
			return table, errors.Join(err, fmt.Errorf("My Action Amount Error"))
		}

		table.Players[myNickname] = PlayerData{
			ChipCount:    myChips,
			IsFolded:     isFolded == 1,
			IsReady:      isReady == 1,
			ActionTaken:  actionIntToString(actionTaken),
			ActionAmount: actionAmount,
			Cards:        make([]Card, 0),
		}
	}

	playerCount, ok := unet.ReadSmallInt(bytes[offset:])
	if !ok {
		return table, fmt.Errorf("failed to read player count")
	}
	offset += 2

	for range playerCount {
		nickname, ok := unet.ReadString(bytes[offset:])
		if !ok {
			return table, fmt.Errorf("failed to read player nickname")
		}
		offset += 4 + len(nickname)

		chips, err := readVarInt()
		if err != nil {
			return table, errors.Join(err, fmt.Errorf("Chips Error"))
		}

		isFolded, ok := unet.ReadSmallInt(bytes[offset:])
		if !ok {
			return table, fmt.Errorf("failed to read player is folded")
		}
		offset += 2

		isReady, ok := unet.ReadSmallInt(bytes[offset:])
		if !ok {
			return table, fmt.Errorf("failed to read player is ready")
		}
		offset += 2

		actionTaken, ok := unet.ReadSmallInt(bytes[offset:])
		if !ok {
			return table, fmt.Errorf("failed to read player action taken")
		}
		offset += 2

		actionAmount, err := readVarInt()
		if err != nil {
			return table, errors.Join(err, fmt.Errorf("Action Amount Error"))
		}

		if _, exists := table.Players[nickname]; exists {
			continue
		}

		table.Players[nickname] = PlayerData{
			ChipCount:    chips,
			IsFolded:     isFolded == 1,
			IsReady:      isReady == 1,
			ActionTaken:  actionIntToString(actionTaken),
			ActionAmount: actionAmount,
			Cards:        make([]Card, 0),
		}
	}

	return table, nil
}

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
