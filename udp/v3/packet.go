package v2

import (
	"fmt"
	"strconv"
	"strings"
)

type FlagType uint8

const (
	FlagTypeInvalid FlagType = iota
	FlagTypeData             // 数据包
	FlagTypeAck              // 确认包
)

type Packet struct {
	Seq  int      // 序列号
	Ack  int      // 确认号
	SAck string   // SAck 区间
	Data string   // 数据内容
	Flag FlagType //标志位
}

func encode(p *Packet) []byte {
	return []byte(fmt.Sprintf("%d|%d|%q|%d", p.Seq, p.Ack, p.Data, p.Flag))
}

func decode(data []byte) Packet {
	var p Packet
	_, _ = fmt.Sscanf(string(data), "%d|%d|%q|%d", &p.Seq, &p.Ack, &p.Data, &p.Flag)
	return p
}

func serialization(p *Packet) string {
	var sb strings.Builder

	if p.Flag == FlagTypeData {
		// 无需任何标志位渲染
		// 输出占位符美化终端显示
		sb.WriteString("    ")
	} else if p.Flag == FlagTypeAck {
		sb.WriteString("[ACK]")
	} else {
		sb.WriteString("[Unknown]")
	}

	sb.WriteString(" Seq=")
	sb.WriteString(strconv.Itoa(p.Seq))

	if p.Flag == FlagTypeAck {
		sb.WriteString(" Ack=")
		sb.WriteString(strconv.Itoa(p.Ack))
	}

	sb.WriteString(" Len=")
	sb.WriteString(strconv.Itoa(len(p.Data)))

	if p.Flag == FlagTypeData {
		sb.WriteString(" Data=")
		sb.WriteString(p.Data)
	}

	return sb.String()
}
