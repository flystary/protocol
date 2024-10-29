package v4

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
	FlagTypeDupAck           // 快速重传包
)

type Packet struct {
	Seq        int      // 序列号
	Ack        int      // 确认号
	SAck       string   // SAck 区间
	Data       string   // 数据内容
	Flag       FlagType // 标志位
	Retransmit bool     // 重传标志位
}

// Packet 数据包编码
// 使用字符串拼接作为简单实现
func encode(p *Packet) []byte {
	return []byte(fmt.Sprintf("%d|%d|%q|%q|%d|%t", p.Seq, p.Ack, p.SAck, p.Data, p.Flag, p.Retransmit))
}

// Packet 数据包解码
func decode(data []byte) Packet {
	var p Packet
	_, _ = fmt.Sscanf(string(data), "%d|%d|%q|%q|%d|%t", &p.Seq, &p.Ack, &p.SAck, &p.Data, &p.Flag, &p.Retransmit)
	return p
}

// 模拟 WireShark 的输出格式
func serialization(p *Packet) string {
	var sb strings.Builder

	if p.Retransmit {
		sb.WriteString("[TCP Retransmit] ")
	}

	if p.Flag == FlagTypeData {
		// 无需任何标志位渲染
		// 输出占位符美化终端显示
		if !p.Retransmit {
			sb.WriteString("     ")
		}
	} else if p.Flag == FlagTypeAck {
		sb.WriteString("[ACK]")
	} else if p.Flag == FlagTypeDupAck {
		sb.WriteString("[TCP Dup ACK]")
	} else {
		sb.WriteString("[Unknown]")
	}

	sb.WriteString(" Seq=")
	sb.WriteString(strconv.Itoa(p.Seq))

	if p.Flag == FlagTypeAck || p.Flag == FlagTypeDupAck {
		sb.WriteString(" Ack=")
		sb.WriteString(strconv.Itoa(p.Ack))

		if len(p.SAck) > 0 {
			sb.WriteString(" SAck=")
			sb.WriteString(p.SAck)
		}
	}

	sb.WriteString(" Len=")
	sb.WriteString(strconv.Itoa(len(p.Data)))

	if p.Flag == FlagTypeData {
		sb.WriteString(" Data=")
		sb.WriteString(p.Data)
	}

	return sb.String()
}
