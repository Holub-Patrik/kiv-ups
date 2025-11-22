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
	ScreenWaitingForRooms
	ScreenRoomSelect
	ScreenInGame
)

type Room struct {
	ID             int
	Name           string
	CurrentPlayers int // small int (2B)
	MaxPlayers     int // small int (2B)
}

type Card struct {
	ID     int
	Symbol string
	Hidden bool
}

type PlayerData struct {
	Name      string
	ChipCount int
	Status    string
}

type PokerTable struct {
	MyHand         []Card
	CommunityCards []Card

	Pot     int
	MyTurn  bool
	Players map[int]PlayerData
}

type GameState struct {
	Screen       UIScreen
	Rooms        map[int]Room
	IsConnecting bool

	ServerIP   string
	ServerPort string

	Table PokerTable
}

type UserInputEvent any

type EvtGameAction struct {
	Action string // "FOLD", "CALL", "BETT", "CHCK"
	Amount string // For betting
}

type EvtConnect struct {
	Host string
	Port string
}

type EvtCancelConnect struct{}

type EvtRoomJoin struct {
	RoomID string
}

type EvtQuit struct{}

type EvtBackToMain struct{}

type UIElement struct {
	dirty     bool
	component w.RGComponent
}

type UIStore struct {
	MainMenu     UIElement
	ServerSelect UIElement
	Connecting   UIElement
	Game         UIElement
}

func (store *UIStore) SetDirty() {
	store.MainMenu.dirty = true
	store.ServerSelect.dirty = true
	store.Connecting.dirty = true
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
