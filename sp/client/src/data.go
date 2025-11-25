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
	RoundBet  int // how much each player bet
	Cards     []Card
	IsMyTurn  bool
}

type PokerTable struct {
	MyHand         []Card
	CommunityCards []Card

	Pot      int
	RoundBet int // per round bet total, added to pot
	MyTurn   bool
	Players  map[int]PlayerData
}

type PlayerConfig struct {
	NickName      string
	StartingChips int
}

type GameState struct {
	Screen UIScreen
	Rooms  map[int]Room

	IsConnecting bool

	ServerIP   string
	ServerPort string
	PlayerCfg  PlayerConfig

	Table     PokerTable
	BetAmount string
}

type UserInputEvent any

type EvtGameAction struct {
	Action string // "FOLD", "CALL", "BETT", "CHCK", "RDY1"
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
	RoomSelect   UIElement
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
