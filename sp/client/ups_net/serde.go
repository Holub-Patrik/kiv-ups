package ups_net

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

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
	Error       bool
	parser_done bool
	code        string
	BytesParsed uint64
	msg_type    MsgType
	payload     string
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

func (p *Parser) Init() {
	p.payload = strings.Builder{}
	p.code = strings.Builder{}
	p.phase = Magic_1
	p.msg_type = 'N'
	p.size_index = 0
	p.code_index = 0
	p.payload_len = 0
}

func (p *Parser) ResetParser() {
	p.payload.Reset()
	p.code.Reset()
	p.phase = Magic_1
	p.msg_type = 'N'
	p.size_index = 0
	p.code_index = 0
	p.payload_len = 0
}

func (p *Parser) ParseByte(char byte) ParserState {
	switch p.phase {
	case Magic_1:
		if char != 'P' {
			fmt.Println("Invalid Magic 1", fmt.Sprintf("%d", char))
			return Invalid
		}
		p.phase = Magic_2

	case Magic_2:
		if char != 'K' {
			fmt.Println("Invalid Magic 2", fmt.Sprintf("%d", char))
			return Invalid
		}
		p.phase = Magic_3

	case Magic_3:
		if char != 'R' {
			fmt.Println("Invalid Magic 3", fmt.Sprintf("%d", char))
			return Invalid
		}
		p.phase = Type

	case Type:
		if char == 'N' || char == 'P' {
			if char == 'N' {
				p.msg_type = NoPayloadMsg
			} else {
				p.msg_type = PayloadMsg
			}

			p.phase = Code
			return OK
		}

		fmt.Println("Unknown message type", fmt.Sprintf("%d", char))
		return Invalid

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
			fmt.Println("Non numeric character in size", fmt.Sprintf("%d", char))
			return Invalid
		}

		if p.size_index >= 3 {
			p.phase = Payload
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
			fmt.Println("Message was not terminated by an endline")
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

		if res.parser_done || res.Error {
			break
		}

		state := p.ParseByte(bytes[i])

		switch state {
		case OK:
		case Done:
			res.parser_done = true
		case Invalid:
			fmt.Println("Error occured in byte", i)
			res.Error = true
		}

		i++
	}

	if res.parser_done {
		res.msg_type = p.msg_type
		res.code = p.code.String()
		if p.msg_type == PayloadMsg {
			res.payload = p.GetPayload()
		}
	}

	res.BytesParsed = i
	return res
}

func ReadSmallInt(slice []byte) (int, bool) {
	fmt.Println("RSI: Reading:", string(slice))
	if len(slice) < 2 {
		return 0, false
	}

	var number int = 0
	for i := range 2 {
		char := slice[i]
		if char < '0' || char > '9' {
			fmt.Println("Non numeric:", string(char))
			return 0, false
		}

		number = number*10 + int(char-'0')
	}

	return number, true
}

func ReadBigInt(slice []byte) (int, bool) {
	fmt.Println("RBI: Reading:", string(slice))
	if len(slice) < 4 {
		return 0, false
	}

	var number int = 0
	for i := range 4 {
		char := slice[i]
		if char < '0' || char > '9' {
			fmt.Println("Non numeric:", string(char))
			return 0, false
		}

		number = number*10 + int(char-'0')
	}

	return number, true
}

func ReadString(slice []byte) (string, bool) {
	fmt.Println("RS: Reading: ", string(slice))
	stringLength, ok := ReadBigInt(slice)

	if !ok {
		return "", false
	}

	if len(slice) < 4+stringLength {
		return "", false
	}

	stringSlice := slice[4 : 4+stringLength]
	fmt.Println("RS: Reading string part: ", string(stringSlice))

	return string(stringSlice), true
}

func ReadVarInt(slice []byte) (int64, bool) {
	length, ok := ReadSmallInt(slice)
	if !ok {
		fmt.Println("Error during small int read")
		return 0, false
	}

	if len(slice) < 2+length {
		fmt.Println("Error during len check")
		return 0, false
	}

	intSlice := slice[2 : 2+length]
	number, err := strconv.ParseInt(string(intSlice), 10, 64)
	if err != nil {
		fmt.Println("Error during strconv")
		return 0, false
	}

	fmt.Println("Read var int went ok")
	return number, true
}

func WriteSmallInt(num int) (string, bool) {
	if num > 99 || num < 0 {
		return "", false
	}

	return fmt.Sprintf("%02d", num), true
}

func WriteBigInt(num int) (string, bool) {
	if num > 9999 || num < 0 {
		return "", false
	}

	return fmt.Sprintf("%04d", num), true
}

func WriteVarInt(num int) (string, bool) {
	digitCount := 0
	if num == 0 {
		digitCount = 1
	} else {
		absNum := math.Abs(float64(num))
		log10Floor := math.Floor(math.Log10(absNum)) + 1
		digitCount = int(log10Floor)
		if num < 1 {
			digitCount += 1
		}
	}

	digitCountStr, ok := WriteSmallInt(digitCount)
	if !ok {
		return "", false
	}

	return digitCountStr + fmt.Sprintf("%d", num), true
}

func WriteString(str string) (string, bool) {
	lenStr, ok := WriteBigInt(len(str))
	if !ok {
		return "", false
	}

	return lenStr + str, true
}
