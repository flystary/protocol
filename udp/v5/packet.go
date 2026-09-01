package v4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type FlagType uint8

const (
	FlagTypeInvalid FlagType = iota
	FlagTypeData             // 数据包
	FlagTypeAck              // 确认包
	FlagTypeDupAck           // 快速重传确认包
)

// HeaderSize 固定 11 字节: Seq(4B) + Ack(4B) + Flag(1B) + SAckCount(1B) + Retransmit(1B)
const HeaderSize = 11

type Header struct {
	Seq        int32    // 序列号
	Ack        int32    // 确认号
	Flag       FlagType // 标志位
	SAckCount  uint8    // SAck 区间数量（最多支持 255 个）
	Retransmit bool     // 重传标志位
}

// Packet 完整数据包
// 物理内存布局: [Header(11B)] + [SAck 数组(SAckCount * 8B)] + [Data(变长，在最末尾)]
type Packet struct {
	Header Header
	SAck   [][2]int32 // SAck 块列表，例如 [[10, 20], [30, 40]]
	Data   []byte     // 变长负载，永远放在最后
}

// Packet 数据包编码
// Encode 将 Packet 序列化为二进制流
func (p *Packet) Encode() []byte {
	sackLen := len(p.SAck)
	if sackLen > 255 {
		sackLen = 255 // 防止溢出 uint8
	}

	totalLen := HeaderSize + (sackLen * 8) + len(p.Data)
	buf := make([]byte, totalLen)

	// 写入 Header
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.Header.Seq))
	binary.BigEndian.PutUint32(buf[4:8], uint32(p.Header.Ack))
	buf[8] = byte(p.Header.Flag)
	buf[9] = uint8(sackLen)
	if p.Header.Retransmit {
		buf[10] = 1
	} else {
		buf[10] = 0
	}

	// 写入 SAck 区间 (每个区间占 8 字节: Left 4B + Right 4B)
	offset := HeaderSize
	for i := 0; i < sackLen; i++ {
		binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(p.SAck[i][0]))
		binary.BigEndian.PutUint32(buf[offset+4:offset+8], uint32(p.SAck[i][1]))
		offset += 8
	}

	// 写入最末尾的变长 Data
	copy(buf[offset:], p.Data)

	return buf
}

// Packet 数据包解码
func Decode(data []byte) (Packet, error) {
	// 错误前置 1：校验头部最小字节数
	if len(data) < HeaderSize {
		return Packet{}, errors.New("packet data too short for header")
	}

	header := Header{
		Seq:        int32(binary.BigEndian.Uint32(data[0:4])),
		Ack:        int32(binary.BigEndian.Uint32(data[4:8])),
		Flag:       FlagType(data[8]),
		SAckCount:  data[9],
		Retransmit: data[10] == 1,
	}

	sackBytesLen := int(header.SAckCount) * 8
	// 错误前置 2：校验 SAck 区间数据完整性
	if len(data) < HeaderSize+sackBytesLen {
		return Packet{}, errors.New("packet data too short for SAck ranges")
	}

	// 解析 SAck 区间列表
	var sack [][2]int32
	if header.SAckCount > 0 {
		sack = make([][2]int32, header.SAckCount)
		offset := HeaderSize
		for i := 0; i < int(header.SAckCount); i++ {
			sack[i][0] = int32(binary.BigEndian.Uint32(data[offset : offset+4]))
			sack[i][1] = int32(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
			offset += 8
		}
	}

	// 剩余部分完全归为 Data
	dataOffset := HeaderSize + sackBytesLen
	payload := data[dataOffset:]

	return Packet{
		Header: header,
		SAck:   sack,
		Data:   payload,
	}, nil
}

// String 实现 fmt.Stringer 接口，模拟 Wireshark 输出格式
func (p Packet) String() string {
	var sb strings.Builder
	sb.Grow(64)

	if p.Header.Retransmit {
		sb.WriteString("[TCP Retransmit] ")
	}

	switch p.Header.Flag {
	case FlagTypeData:
		if !p.Header.Retransmit {
			sb.WriteString("     ")
		}
	case FlagTypeAck:
		sb.WriteString("[ACK]")
	case FlagTypeDupAck:
		sb.WriteString("[TCP Dup ACK]")
	default:
		sb.WriteString("[Unknown]")
	}

	sb.WriteString(" Seq=")
	sb.WriteString(strconv.FormatInt(int64(p.Header.Seq), 10))

	if p.Header.Flag == FlagTypeAck || p.Header.Flag == FlagTypeDupAck {
		sb.WriteString(" Ack=")
		sb.WriteString(strconv.FormatInt(int64(p.Header.Ack), 10))

		if len(p.SAck) > 0 {
			sb.WriteString(" SAck=")
			sb.WriteString(formatSAck(p.SAck))
		}
	}

	sb.WriteString(" Len=")
	sb.WriteString(strconv.Itoa(len(p.Data)))

	if p.Header.Flag == FlagTypeData {
		sb.WriteString(" Data=")
		sb.WriteString(string(p.Data))
	}

	return sb.String()
}

func formatSAck(sack [][2]int32) string {
	var parts []string
	for _, rng := range sack {
		parts = append(parts, fmt.Sprintf("[%d-%d]", rng[0], rng[1]))
	}
	return strings.Join(parts, ",")
}
