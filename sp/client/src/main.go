package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	unet "poker-client/ups_net"
	w "poker-client/window"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func initProgCtx(nick string, chips int) *ProgCtx {
	ctx := ProgCtx{}

	ctx.UserInputChan = make(chan UserInputEvent, 10) // Buffered
	ctx.DoneChan = make(chan bool)

	ctx.State.Screen = ScreenMainMenu
	ctx.State.Rooms = make(map[int]Room)
	ctx.StateMutex = sync.RWMutex{}
	ctx.State.ServerIP = "127.0.0.1"
	ctx.State.ServerPort = "8080"
	ctx.State.BetAmount = ""

	// Initialize Player Config
	ctx.State.PlayerCfg = PlayerConfig{
		NickName:      nick,
		StartingChips: chips,
	}

	ctx.NetHandler = unet.NetHandler{}
	ctx.Popup = NewPopupManager()

	buildUI(&ctx)

	return &ctx
}

func handleUIEvent(ctx *ProgCtx, event w.UIEvent) {
	switch event.SourceID {
	case "MainMenu_ConnectBtn":
		ctx.StateMutex.Lock()
		ctx.State.Screen = ScreenServerSelect
		ctx.StateMutex.Unlock()

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

	case "RoomSelect_BackBtn":
		ctx.UserInputChan <- EvtBackToMain{}

	case "Game_Bet":
		ctx.StateMutex.RLock()
		betStr := strings.TrimSpace(ctx.State.BetAmount)
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

		ctx.StateMutex.Lock()
		ctx.State.BetAmount = ""
		ctx.StateMutex.Unlock()

		ctx.UserInputChan <- EvtGameAction{Action: "BETT", Amount: fmt.Sprintf("%04d", amount)}

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

	// Parse Command Line Arguments
	nickPtr := flag.String("nick", "Guest", "Player Nickname")
	chipsPtr := flag.Int("chips", 1000, "Starting Chip Count")
	flag.Parse()

	rl.SetConfigFlags(rl.FlagWindowResizable)

	const (
		screenWidth  int32 = 1600
		screenHeight int32 = 1000
	)

	rl.InitWindow(screenWidth, screenHeight, "Poker Client - "+*nickPtr)
	defer rl.CloseWindow()

	// rl.SetTargetFPS(60)

	ctx := initProgCtx(*nickPtr, *chipsPtr)

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

		case ScreenServerSelect:
			elementsToDraw = append(elementsToDraw, ctx.UI.MainMenu, ctx.UI.ServerSelect)

		case ScreenConnecting, ScreenWaitingForRooms: // Reuse connecting screen for waiting
			elementsToDraw = append(elementsToDraw, ctx.UI.MainMenu, ctx.UI.Connecting)

		case ScreenRoomSelect:
			roomSelect := buildRoomSelectUI(ctx)
			roomSelect.component.Rebuild(ctx.UI.RoomSelect.component)
			ctx.UI.RoomSelect = roomSelect

			elementsToDraw = append(elementsToDraw, ctx.UI.MainMenu, ctx.UI.RoomSelect)

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
