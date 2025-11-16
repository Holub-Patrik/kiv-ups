package main

import (
	unet "poker-client/ups_net"
	w "poker-client/window"
	"sync"
)

type UIScreen int

const (
	ScreenMainMenu UIScreen = iota
	ScreenServerSelect
	ScreenConnecting
	ScreenRoomSelect
	ScreenInGame
)

type Room struct {
	ID             int
	Name           string
	CurrentPlayers int // small int (2B)
	MaxPlayers     int // small int (2B)
}

type GameState struct {
	Screen       UIScreen
	Rooms        map[int]Room
	IsConnecting bool

	ServerIP   string
	ServerPort string
}

type UserInputEvent any

type EvtConnectClicked struct {
	Host string
	Port string
}

type EvtCancelConnectClicked struct{}

type EvtRoomJoinClicked struct {
	RoomID string
}

type EvtQuitClicked struct{}

type UIStore struct {
	MainMenu     w.RGComponent
	ServerSelect w.RGComponent
	Connecting   w.RGComponent
}

type ProgCtx struct {
	State      GameState
	StateMutex sync.RWMutex

	UserInputChan chan UserInputEvent // Render -> Game
	NetMsgInChan  chan unet.NetMsg    // Network -> Game
	NetMsgOutChan chan unet.NetMsg    // Game -> Network
	DoneChan      chan bool           // Game -> Main (to signal shutdown)

	NetHandler  unet.NetHandler
	ShouldClose bool

	UI    UIStore
	Popup PopupManager
}
