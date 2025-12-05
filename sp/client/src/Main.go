package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	unet "poker-client/ups_net"
	w "poker-client/window"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func initProgCtx() *ProgCtx {
	ctx := ProgCtx{}
	seededSource := rand.NewSource(time.Now().UnixNano())
	r := rand.New(seededSource)

	ctx.UserInputChan = make(chan UserInputEvent, 10) // Buffered
	ctx.DoneChan = make(chan bool)

	ctx.State.Screen = ScreenMainMenu
	ctx.State.Rooms = make(map[int]Room)
	ctx.State.Table.Players = make(map[string]PlayerData)
	ctx.StateMutex = sync.RWMutex{}
	ctx.State.ServerIP = "127.0.0.1"
	ctx.State.ServerPort = "8080"
	ctx.State.Nickname = "Client" + fmt.Sprintf("%d", r.Intn(100))
	ctx.State.ChipsStr = fmt.Sprintf("%d", rand.Intn(1_000_000_000))
	ctx.State.BetAmount = ""

	ctx.NetHandler = unet.NetHandler{}
	ctx.NetHandler.Init()
	go ctx.NetHandler.Run()

	ctx.EventChan = ctx.NetHandler.EventChan()

	ctx.Popup = NewPopupManager()

	buildUI(&ctx)

	return &ctx
}

func handleUIEvent(ctx *ProgCtx, event w.UIEvent) {
	switch event.SourceID {
	case "MainMenu_ConnectBtn":
		if len(ctx.State.Nickname) <= 0 || len(ctx.State.Nickname) > 9999 {
			ctx.Popup.AddPopup("Please enter a nick within the length limits", time.Second*3)
			return
		}

		if len(ctx.State.ChipsStr) <= 0 || len(ctx.State.ChipsStr) > 100 {
			ctx.Popup.AddPopup("Please enter chip value within the length limits", time.Second*3)
			return
		}

		amount, err := strconv.Atoi(strings.TrimSpace(ctx.State.ChipsStr))
		if err != nil {
			ctx.Popup.AddPopup("Please enter a numeric chip value", time.Second*3)
			return
		}

		_, ok := unet.WriteVarInt(amount)
		if !ok {
			ctx.Popup.AddPopup("Invalid value, please enter a different one", time.Second*3)
			return
		}

		ctx.State.Table.Players[ctx.State.Nickname] = PlayerData{
			ChipCount: amount,
		}

		ctx.UserInputChan <- EvtConnect{Host: ctx.State.ServerIP, Port: ctx.State.ServerPort}

	case "MainMenu_CloseBtn":
		ctx.UserInputChan <- EvtQuit{}

	case "Server_ConfirmBtn":
		// Get the IP/Port safely from the state
		ctx.StateMutex.RLock()
		host := ctx.State.ServerIP
		port := ctx.State.ServerPort
		ctx.StateMutex.RUnlock()
		fmt.Println(ctx.State.ServerIP, ctx.State.ServerPort)
		fmt.Println("ConfirmBtn: ", host, port)
		ctx.UserInputChan <- EvtConnect{Host: host, Port: port}

	case "Connecting_CancelBtn":
		ctx.UserInputChan <- EvtCancelConnect{}

	case "Reconnect_Accept":
		ctx.UserInputChan <- EvtAcceptReconnect{}

	case "Reconnect_Decline":
		ctx.UserInputChan <- EvtDeclineReconnect{}

	case "RoomSelect_BackBtn":
		ctx.UserInputChan <- EvtBackToMain{}

	case "Game_Bet":
		ctx.StateMutex.RLock()
		betStr := strings.TrimSpace(ctx.State.BetAmount)
		myNickname := ctx.State.Nickname
		table := ctx.State.Table
		ctx.StateMutex.RUnlock()

		if betStr == "" {
			ctx.Popup.AddPopup("Enter bet amount", time.Second*2)
			return
		}

		amount, err := strconv.Atoi(betStr)
		if err != nil || amount <= 0 {
			ctx.Popup.AddPopup("Invalid amount (use numbers only)", time.Second*2)
			return
		}

		myData, exists := table.Players[myNickname]
		if !exists {
			ctx.Popup.AddPopup("Error: Player data not found", time.Second*3)
			return
		}

		if amount > myData.ChipCount {
			ctx.Popup.AddPopup(fmt.Sprintf("You only have %d chips", myData.ChipCount), time.Second*3)
			return
		}

		netStr, ok := unet.WriteVarInt(amount)
		if !ok {
			ctx.Popup.AddPopup("Bet amount is invalid", time.Second*2)
			return
		}

		ctx.StateMutex.Lock()
		ctx.State.BetAmount = ""
		ctx.StateMutex.Unlock()

		ctx.UserInputChan <- EvtGameAction{Action: "BETT", Amount: netStr}

	case "Game_Ready":
		ctx.UserInputChan <- EvtGameAction{Action: "RDY1"}

	case "Game_Check":
		ctx.UserInputChan <- EvtGameAction{Action: "CHCK"}

	case "Game_Leave":
		ctx.UserInputChan <- EvtGameAction{Action: "GMLV"}

	case "Game_Fold":
		ctx.UserInputChan <- EvtGameAction{Action: "FOLD"}

	case "Game_Call":
		ctx.UserInputChan <- EvtGameAction{Action: "CALL"}

	case "Game_ShowOK":
		ctx.UserInputChan <- EvtGameAction{Action: "SDOK"}

	default:
		after, found := strings.CutPrefix(event.SourceID, "join_")
		if found {
			ctx.UserInputChan <- EvtRoomJoin{RoomID: after}
		}
	}
}

func main() {
	test_msg := make([]byte, 0)
	msg_1 := []byte("PKRNGMST\n")
	msg_2 := []byte("PKRPCDTP00043322\n")
	for _, cur_byte := range msg_1 {
		test_msg = append(test_msg, cur_byte)
	}
	// test_msg = append(test_msg, 0)
	for _, cur_byte := range msg_2 {
		test_msg = append(test_msg, cur_byte)
	}
	parser := unet.Parser{}
	results := parser.ParseBytes(test_msg)
	parser.ResetParser()
	results = parser.ParseBytes(test_msg[results.BytesParsed:])

	if results.Error {
		fmt.Println("Fuck OFF")
		return
	}

	rl.SetConfigFlags(rl.FlagWindowResizable)

	const (
		screenWidth  int32 = 1600
		screenHeight int32 = 1000
	)

	rl.InitWindow(screenWidth, screenHeight, "Poker Client")
	defer rl.CloseWindow()

	rl.SetTargetFPS(60)

	ctx := initProgCtx()

	// Start the "Game Thread"
	go gameThread(ctx)

	elementsToDraw := make([]UIElement, 0)

	// meant for calculation/recalculation
	screenBounds := rl.Rectangle{
		X: 0, Y: 0,
		Width:  float32(screenWidth),
		Height: float32(screenHeight),
	}

	for !rl.WindowShouldClose() && !ctx.ShouldClose {
		tmpScreenHeight := int32(rl.GetScreenHeight())
		tmpScreenWidth := int32(rl.GetScreenWidth())

		if tmpScreenHeight != screenHeight {
			screenBounds.Height = float32(tmpScreenHeight)
			ctx.UI.SetDirty()
		}

		if tmpScreenWidth != screenWidth {
			screenBounds.Width = float32(tmpScreenWidth)
			ctx.UI.SetDirty()
		}

		// Get the current screen safely
		ctx.StateMutex.RLock()
		currentScreen := ctx.State.Screen
		ctx.StateMutex.RUnlock()

		switch currentScreen {
		case ScreenMainMenu:
			elementsToDraw = append(elementsToDraw, ctx.UI.MainMenu)

		case ScreenConnecting, ScreenWaitingForRooms: // Reuse connecting screen for waiting
			elementsToDraw = append(elementsToDraw, ctx.UI.MainMenu, ctx.UI.Connecting)

		case ScreenReconnecting:
			elementsToDraw = append(elementsToDraw, ctx.UI.Reconnecting)

		case ScreenRoomSelect:
			roomSelect := buildRoomSelectUI(ctx)
			roomSelect.component.Rebuild(ctx.UI.RoomSelect.component)
			ctx.UI.RoomSelect = roomSelect

			elementsToDraw = append(elementsToDraw, ctx.UI.RoomSelect)

		case ScreenInGame:
			gameScreen := buildGameScreen(ctx)
			gameScreen.component.Rebuild(ctx.UI.Game.component)
			ctx.UI.Game = gameScreen

			elementsToDraw = append(elementsToDraw, ctx.UI.Game)
		}

		// calculate popups everytime
		ctx.Popup.Calculate(screenBounds)

		rl.BeginDrawing()
		rl.DrawFPS(0, 0)
		rl.ClearBackground(rl.Black)

		uiEventChannel := make(chan w.UIEvent, 10)

		for _, element := range elementsToDraw {
			if element.dirty {
				element.component.Calculate(screenBounds)
				element.dirty = false
			}

			element.component.Draw(uiEventChannel)
		}

		// Draw popups
		ctx.Popup.Draw(uiEventChannel)
		ctx.Popup.Update()

		elementsToDraw = elementsToDraw[:0]

		rl.EndDrawing()

		close(uiEventChannel)
		for event := range uiEventChannel {
			handleUIEvent(ctx, event)
		}
	}

	// --- Shutdown ---
	ctx.ShouldClose = true
	ctx.UserInputChan <- EvtQuit{} // Wake up game thread

	fmt.Println("Main: Waiting for GameThread to shut down...")
	<-ctx.DoneChan
	fmt.Println("Main: Shutdown complete.")
}
