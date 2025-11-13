package main

import (
	unet "poker-client/ups_net" // Using alias 'unet' from your main.go
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// UIScreen represents the current view the user should see.
type UIScreen int

const (
	ScreenMainMenu UIScreen = iota
	ScreenServerSelect
	ScreenConnecting
	ScreenRoomSelect
	ScreenInGame
	ScreenError
)

// Room defines the data for a single game room.
// You'll need to write a function to parse this from your [RoomData] payload.
type Room struct {
	ID             string // A unique ID for the room
	Name           string
	CurrentPlayers int
	MaxPlayers     int
	// Add other fields as needed
}

// GameState holds all data that the renderer needs.
// It must be protected by a mutex.
type GameState struct {
	Screen       UIScreen
	Rooms        map[string]Room // Map[RoomID] -> Room
	ErrorMessage string
	IsConnecting bool

	// UI-specific state
	ServerIP   string
	ServerPort string
}

// UserInputEvent is the interface for events from the Render thread to the Game thread.
type UserInputEvent interface{}

// --- Specific User Input Events ---

// EvtConnectClicked is sent when the user confirms host/port.
type EvtConnectClicked struct {
	Host string
	Port string
}

// EvtCancelConnectClicked is sent from the "Connecting" screen.
type EvtCancelConnectClicked struct{}

// EvtRoomJoinClicked is sent when the user clicks a room.
type EvtRoomJoinClicked struct {
	RoomID string
}

// EvtQuitClicked is sent when the user clicks the "Close" button.
type EvtQuitClicked struct{}

// ProgCtx holds the global application context.
type ProgCtx struct {
	// Shared State
	State      GameState
	StateMutex sync.RWMutex // Protects State

	// Channels
	UserInputChan chan UserInputEvent     // Render -> Game
	NetMsgInChan  chan unet.NetMsg        // Network -> Game
	NetMsgOutChan chan unet.NetMsg        // Game -> Network
	DoneChan      chan bool               // Game -> Main (to signal shutdown)
	NetHandler    unet.NetHandler         // Your network handler
	ShouldClose   bool                    // Flag to signal all goroutines to stop
	Layout        map[string]rl.Rectangle // A simple map for UI rects
}
