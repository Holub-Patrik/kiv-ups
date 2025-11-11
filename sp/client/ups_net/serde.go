package ups_net

import (
	"fmt"
	"strings"
)

// use net.Dial("tcp", host+":"+port)

const (
	MSG_CODE_SIZE    uint64 = 4
	PAYLOAD_LEN_SIZE uint64 = 4
	SIZESTR_LEN      uint64 = 4
)

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
	payload     strings.Builder
	code        strings.Builder
	phase       MainPart
	msg_type    MsgType
	size_index  uint64
	code_index  uint64
	payload_len uint64
}

func InitParser() Parser {
	return Parser{
		payload:     strings.Builder{},
		code:        strings.Builder{},
		phase:       Magic_1,
		msg_type:    'P',
		size_index:  0,
		code_index:  0,
		payload_len: 0,
	}
}

func (p *Parser) ResetParser() {
	p.payload.Reset()
	p.code.Reset()
	p.phase = Magic_1
	p.msg_type = 'P'
	p.size_index = 0
	p.code_index = 0
	p.payload_len = 0
}

func (p *Parser) ParseByte(char byte) ParserState {
	switch p.phase {
	case Magic_1:
		if char != 'P' {
			fmt.Println("Invalid Magic")
			return Invalid
		}
		p.phase = Magic_2

	case Magic_2:
		if char != 'K' {
			fmt.Println("Invalid Magic")
			return Invalid
		}
		p.phase = Magic_3

	case Magic_3:
		if char != 'R' {
			fmt.Println("Invalid Magic")
			return Invalid
		}
		p.phase = Type

	case Type:
		if char != 'N' && char != 'P' {
			fmt.Println("Unknown message type")
			return Invalid
		}
		p.phase = Code

	case Code:
		p.code.WriteByte(char)
		p.code_index++

		if p.code_index >= MSG_CODE_SIZE {
			if p.msg_type == NoPayloadMsg {
				p.phase = Endline
			} else {
				p.phase = Size
			}
		}

	case Size:
		if char < '0' || char > '9' {
			fmt.Println("Non numeric character in size")
			return Invalid
		}

		p.payload_len = p.payload_len*10 + (uint64(char) - '0')
		p.size_index++

	case Payload:
		p.payload.WriteByte(char)

		if uint64(p.payload.Len()) == p.payload_len {
			p.phase = Endline
		}

	case Endline:
		if char == '\n' {
			return Done
		} else {
			return Invalid
		}
	}

	return OK
}

func (p *Parser) GetPayload() string {
	return p.payload.String()
}

func (p *Parser) ParseBytes(bytes []byte) ParseResults {
	res := ParseResults{}

	var i uint64 = 0
	for {
		if i >= uint64(len(bytes)) {
			break
		}

		if res.parser_done || res.error_occured {
			break
		}

		state := p.ParseByte(bytes[i])
		switch state {
		case OK:
			continue
		case Done:
			res.parser_done = true
		case Invalid:
			res.error_occured = true
		}
	}

	if res.parser_done {
		res.msg_type = p.msg_type
		res.code = p.code.String()
		if p.msg_type == PayloadMsg {
			res.payload = p.GetPayload()
		}
	}

	res.bytes_parsed = i
	return res
}
