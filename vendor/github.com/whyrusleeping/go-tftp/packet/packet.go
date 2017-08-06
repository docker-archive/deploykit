// package packet implements methods to serialize and deserialize TFTP protocol packets
package packet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
)

var _ = log.Fatal

// ErrInvalidPacket is returned when given malformed data
var ErrInvalidPacket = errors.New("invalid packet")

// ErrPacketType is returned when given an invalid packet type value
var ErrPacketType = errors.New("unrecognized packet type")

// Packet Type codes as defined in rfc 1350
const (
	RRQ = uint16(iota + 1)
	WRQ
	DATA
	ACK
	ERROR
	OACK
)

// Packet represents any TFTP packet
type Packet interface {
	// GetType returns the packet type
	GetType() uint16

	// Bytes serializes the packet
	Bytes() []byte
}

type ReqPacket struct {
	Filename  string
	Mode      string
	Type      uint16
	BlockSize int
}

func (p *ReqPacket) GetType() uint16 {
	return p.Type
}

// we will never need to serialize a Request Packet
// as the server, so it is safe to return nil just to
// satisfy the interface (consider adding this later for
// client use)
func (p *ReqPacket) Bytes() []byte {
	buf := new(bytes.Buffer)
	opcode := make([]byte, 2)
	binary.BigEndian.PutUint16(opcode, p.GetType())
	buf.Write(opcode)
	buf.WriteString(p.Filename)
	buf.WriteByte(0)
	buf.WriteString(p.Mode)
	buf.WriteByte(0)
	if p.BlockSize != 0 && p.BlockSize != 512 {
		buf.WriteString("blksize")
		buf.WriteByte(0)
		buf.WriteString(fmt.Sprint(p.BlockSize))
		buf.WriteByte(0)
	}
	return buf.Bytes()
}

type DataPacket struct {
	Data     []byte
	BlockNum uint16
}

func (p *DataPacket) GetType() uint16 {
	return DATA
}

func (p *DataPacket) Bytes() []byte {
	buf := make([]byte, 4+len(p.Data))
	binary.BigEndian.PutUint16(buf[:2], DATA)
	binary.BigEndian.PutUint16(buf[2:4], p.BlockNum)
	copy(buf[4:], p.Data)
	return buf
}

type AckPacket uint16

func NewAck(blknum uint16) *AckPacket {
	a := AckPacket(blknum)
	return &a
}

func (p *AckPacket) GetType() uint16 {
	return ACK
}

func (p *AckPacket) GetBlocknum() uint16 {
	return uint16(*p)
}

func (p *AckPacket) Bytes() []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint16(buf[:2], ACK)
	binary.BigEndian.PutUint16(buf[2:], uint16(*p))
	return buf
}

type ErrorPacket struct {
	Code  uint16
	Value string
}

const (
	TFTPErrUndefined uint16 = iota
	TFTPErrNotFound
	TFTPErrAccessViolation
	TFTPErrDiskFull
	TFTPErrIllegalOp
	TFTPErrUnknownTID
	TFTPErrAlreadyExists
	TFTPErrNoSuchUser
)

func (p *ErrorPacket) Error() string {
	return p.Value
}

func (p *ErrorPacket) Bytes() []byte {
	buf := make([]byte, 4+len(p.Value))
	binary.BigEndian.PutUint16(buf[:2], ERROR)
	binary.BigEndian.PutUint16(buf[2:4], p.Code)
	copy(buf[4:], p.Value)
	return buf
}

func (p *ErrorPacket) GetType() uint16 {
	return ERROR
}

type OAckPacket struct {
	Options map[string]string
}

func NewOAckPacket() *OAckPacket {
	return &OAckPacket{
		Options: make(map[string]string),
	}
}

func (oa *OAckPacket) Bytes() []byte {
	panic("Not yet implemented!")
}

func (oa *OAckPacket) GetType() uint16 {
	return OACK
}

// ReadPacket deserializes a packet from the given buffer
func ParsePacket(buf []byte) (Packet, error) {
	if len(buf) < 2 {
		return nil, ErrInvalidPacket
	}

	pktType := binary.BigEndian.Uint16(buf[0:2])
	switch pktType {
	case RRQ, WRQ:
		vals := bytes.Split(buf[2:], []byte{0})
		if len(vals) < 2 {
			return nil, ErrInvalidPacket
		}
		return &ReqPacket{
			Type:     pktType,
			Filename: string(vals[0]),
			Mode:     string(vals[1]),
		}, nil
	case ACK:
		blknum := binary.BigEndian.Uint16(buf[2:4])
		return NewAck(blknum), nil
	case DATA:
		blknum := binary.BigEndian.Uint16(buf[2:4])
		return &DataPacket{
			BlockNum: blknum,
			Data:     buf[4:],
		}, nil
	case ERROR:
		errcode := binary.BigEndian.Uint16(buf[2:4])
		return &ErrorPacket{
			Code:  errcode,
			Value: string(buf[4:]),
		}, nil
	case OACK:
		oack := NewOAckPacket()
		vals := bytes.Split(buf[2:], []byte{0})
		for i := 0; i+1 < len(vals); i += 2 {
			oack.Options[string(vals[i])] = string(vals[i+1])
		}
		return oack, nil
	default:
		return nil, ErrPacketType
	}
}
