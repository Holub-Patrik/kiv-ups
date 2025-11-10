package main

// use net.Dial("tcp", host+":"+port)

type Msg struct {
	has_payload bool
	code        string
	payload     string
}

type ConnManager struct {
	msg_chan chan Msg
}

type MsgReader struct {
	msg_chan chan Msg
}

type MainPart int
type ParserState int
type MsgType byte

const (
	Magic_1 MainPart = iota
	Magic_2
	Magic_3
	Type
	Code
	Size
	Payload
	Endline
)

const (
	OK ParserState = iota
	Done
	Invalid
)

const (
	PayloadMsg   MsgType = 'P'
	NoPayloadMsg MsgType = 'N'
)

type ParseResults struct {
	error_occured bool
	parser_done   bool
	code          string
	bytes_parsed  uint64
	msg_type      MsgType
	payload       string
}

type Parser struct {
}
