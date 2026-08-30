package v1

import (
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
)

type FlagType uint8

const (
	FlagTypeInvalid FlagType = iota
	FlagTypeData             // 数据包
	FlagTypeAck              // 确认包
)

// HeaderSize 固定为 9 字节: Seq(4B) + Ack(4B) + Flag(1B)
const HeaderSize = 9

// Header 协议头（固定长度）
type Header struct {
	Seq  int32    // 序列号
	Ack  int32    // 确认号
	Flag FlagType //标志位
}

// Packet 完整数据包：Header 在前，变长 Data 放在最后
type Packet struct {
	Header Header
	Data   []byte // 变长负载，放在包末尾
}

// Encode 将 Header 与 Data 编码为字节流
func (p *Packet) Encode() []byte {
	buf := make([]byte, HeaderSize+len(p.Data))

	// 写入 Header
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.Header.Seq))
	binary.BigEndian.PutUint32(buf[4:8], uint32(p.Header.Ack))
	buf[8] = byte(p.Header.Flag)

	// 写入末尾的 Data
	copy(buf[HeaderSize:], p.Data)
	return buf
}

// Decode 解析字节流为 Packet
func Decode(data []byte) (Packet, error) {
	if len(data) < HeaderSize {
		return Packet{}, errors.New("packet data too short for header")
	}

	header := Header{
		Seq:  int32(binary.BigEndian.Uint32(data[0:4])),
		Ack:  int32(binary.BigEndian.Uint32(data[4:8])),
		Flag: FlagType(data[8]),
	}

	// 9 字节之后的所有内容自动归为 Data
	payload := data[HeaderSize:]

	return Packet{
		Header: header,
		Data:   payload,
	}, nil
}

// String 实现 fmt.Stringer 接口用于日志打印
func (p Packet) String() string {
	var sb strings.Builder
	sb.Grow(64)

	switch p.Header.Flag {
	case FlagTypeData:
		sb.WriteString("    ")
	case FlagTypeAck:
		sb.WriteString("[ACK]")
	default:
		sb.WriteString("[Unknown]")
	}

	sb.WriteString(" Seq=")
	sb.WriteString(strconv.FormatInt(int64(p.Header.Seq), 10))

	if p.Header.Flag == FlagTypeAck {
		sb.WriteString(" Ack=")
		sb.WriteString(strconv.FormatInt(int64(p.Header.Ack), 10))
	}

	sb.WriteString(" Len=")
	sb.WriteString(strconv.Itoa(len(p.Data)))

	if p.Header.Flag == FlagTypeData {
		sb.WriteString(" Data=")
		sb.WriteString(string(p.Data))
	}

	return sb.String()
}
