package v4

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

var (
	serverAddr = net.UDPAddr{
		Port: 8080,
		IP:   net.ParseIP("127.0.0.1"),
	}
)

func startServer() {
	conn, err := net.ListenUDP("udp", &serverAddr)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer conn.Close()

	buffer := make([]byte, 1024)

	// 延迟 200 毫秒发送 ACK
	const ackDelay = 200 * time.Millisecond

	var (
		// 延迟 Ack
		lastAck int32

		// 记录接收到的区间 Seq
		// [0]: 区间起始 Seq
		// [1]: 区间结束 Seq, Seq + Data.Len()
		seqList = [][2]int32{}

		// 记录历史接收到的所有区间 Seq
		seqRecord = [][2]int32{}

		// 最后发送 Ack 报文的时间
		lastAckTime = time.Now()
		// 客户端的 UDP 地址
		clientAddr *net.UDPAddr
	)

	// 延迟 Ack 及重复 Ack (DupACK) 发送
	go func() {
		for {
			// 短暂休眠，避免占用过多 CPU 资源
			time.Sleep(100 * time.Millisecond)

			// 未拿到客户端地址或列表为空跳过
			if clientAddr == nil || len(seqList) == 0 {
				continue
			}

			// 延迟时间未达到阈值跳过
			if time.Since(lastAckTime) < ackDelay {
				continue
			}

			lastAck = seqList[0][1]
			lastAckChanged := false

			// 反转检测（模拟乱序/快速重传探测）
			for i, j := 1, len(seqList)-1; i < j; i, j = i+1, j-1 {
				seqList[i], seqList[j] = seqList[j], seqList[i]
			}

			for _, val := range seqList {
				if val[0] > lastAck {
					dupAckPacket := Packet{
						Header: Header{
							Seq:  1,
							Ack:  lastAck,
							Flag: FlagTypeDupAck,
						},
					}
					ackData := dupAckPacket.Encode()
					conn.WriteToUDP(ackData, clientAddr)
				} else {
					lastAck = val[1]
				}
			}

			// 按 Seq 区间排序
			sort.Slice(seqList, func(i, j int) bool {
				if seqList[i][0] != seqList[j][0] {
					return seqList[i][0] < seqList[j][0]
				}
				return seqList[i][1] < seqList[j][1]
			})

			// 合并连续区间
			mergedSeqList := [][2]int32{
				seqList[0],
			}

			for i := 1; i < len(seqList); i++ {
				// 数据包 Seq 是连续的，直接合并两个区间
				if seqList[i][0] == mergedSeqList[len(mergedSeqList)-1][1] {
					mergedSeqList[len(mergedSeqList)-1][1] = seqList[i][1]

					// 更新最后接收到的确认号
					if !lastAckChanged {
						lastAck = mergedSeqList[len(mergedSeqList)-1][1]
					}
				} else {
					lastAckChanged = true

					// 数据包 Seq 不是连续的，有中间数据包还未收到
					mergedSeqList = append(mergedSeqList, seqList[i])
				}
			}

			for _, seq := range mergedSeqList {
				sackList := [][2]int32{seq}
				ackPacket := Packet{
					Header: Header{
						Seq: 1,
						Ack: lastAck,

						Flag: FlagTypeAck,
					},
					SAck: sackList,
					Data: nil,
				}
				ackData := ackPacket.Encode()
				conn.WriteToUDP(ackData, clientAddr)
			}

			// 更新最后发送 Ack 的时间
			lastAckTime = time.Now()

			// 重置区间 Seq
			seqList = seqList[:0]
		}
	}()

	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			continue
		}
		clientAddr = addr

		// 解析接收到的数据包
		recvPacket, err := Decode(buffer[:n])
		if err != nil {
			fmt.Println("Error decoding:", err)
			continue
		}

		fmt.Printf("client -> server %s\n", recvPacket.String())

		// 记录历史区间 Seq
		seqRecord = append(seqRecord, [2]int32{
			recvPacket.Header.Seq,
			recvPacket.Header.Seq + int32(len(recvPacket.Data)),
		})

		// 重传数据包处理
		if recvPacket.Header.Retransmit {
			// 排序合并后的区间
			sort.Slice(seqRecord, func(i, j int) bool {
				if seqRecord[i][0] != seqRecord[j][0] {
					return seqRecord[i][0] < seqRecord[j][0]
				}
				return seqRecord[i][1] < seqRecord[j][1]
			})

			uniqueIndex := 0
			for i := 1; i < len(seqRecord); i++ {
				if seqRecord[i][0] <= seqRecord[uniqueIndex][1] {
					if seqRecord[i][1] > seqRecord[uniqueIndex][1] {
						seqRecord[uniqueIndex][1] = seqRecord[i][1]
					}
				} else {
					uniqueIndex++
					seqRecord[uniqueIndex] = seqRecord[i]
				}
			}
			seqRecord = seqRecord[:uniqueIndex+1]

			// 更新已经接收到连续区间最大 Ack
			lastAck = seqRecord[0][1]

			ackRange := [2]int32{recvPacket.Header.Seq, recvPacket.Header.Seq + int32(len(recvPacket.Data))}
			ackPacket := Packet{
				Header: Header{
					Seq:        1,
					Ack:        lastAck,
					Flag:       FlagTypeAck,
					SAckCount:  1,
					Retransmit: true,
				},
				SAck: [][2]int32{ackRange},
			}
			conn.WriteToUDP(ackPacket.Encode(), clientAddr)
			continue
		}

		// 记录接收到的区间 Seq
		seqList = append(seqList, [2]int32{
			recvPacket.Header.Seq,
			recvPacket.Header.Seq + int32(len(recvPacket.Data)),
		})
	}
}

func startClient() {
	conn, err := net.DialUDP("udp", nil, &serverAddr)
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	// 记录客户端已经发送过的数据包 Seq 列表
	sentPackets := []*Packet{}
	// 记录客户端已经接收到的数据包 Seq 列表
	receivedPackets := []*Packet{}

	var wg sync.WaitGroup
	wg.Add(1)

	// 这里启动一个新的 goroutine
	// 1. 完成超时重传
	// 2. 完成接收 Ack 操作
	go func() {
		defer wg.Done()

		// 超时退出
		timeout := time.NewTimer(1 * time.Second)
		defer timeout.Stop()

		// 超时重传定时器
		// 硬编码为 300 毫秒
		ticket := time.NewTicker(300 * time.Millisecond)
		defer ticket.Stop()

		for {
			select {
			case <-timeout.C:
				return
			case <-ticket.C:
				// 已收到的确认包数量等于已发送包时无需重传 无需重传
				if len(sentPackets) == len(receivedPackets) {
					continue
				}

				// 通过区间差集算法
				// 同时考虑 选择性确认 的情况
				lostPackets := []*Packet{}
				receivedAckList := [][2]int32{}

				for _, val := range receivedPackets {
					receivedAckList = append(receivedAckList, val.SAck...)
				}

				if len(receivedAckList) == 0 {
					lostPackets = append(lostPackets, sentPackets...)
				} else {
					sort.Slice(receivedAckList, func(i, j int) bool {
						if receivedAckList[i][0] != receivedAckList[j][0] {
							return receivedAckList[i][0] < receivedAckList[j][0]
						}
						return receivedAckList[i][1] < receivedAckList[j][1]
					})

					uniqueIndex := 0
					for i := 1; i < len(receivedAckList); i++ {
						if receivedAckList[i][0] <= receivedAckList[uniqueIndex][1] {
							if receivedAckList[i][1] > receivedAckList[uniqueIndex][1] {
								receivedAckList[uniqueIndex][1] = receivedAckList[i][1]
							}
						} else {
							uniqueIndex++
							receivedAckList[uniqueIndex] = receivedAckList[i]
						}
					}
					receivedAckList = receivedAckList[:uniqueIndex+1]

					for _, pkt := range sentPackets {
						pktEnd := pkt.Header.Seq + int32(len(pkt.Data))
						acked := false
						for _, ackRange := range receivedAckList {
							if pkt.Header.Seq >= ackRange[0] && pktEnd <= ackRange[1] {
								acked = true
								break
							}
						}
						if !acked {
							lostPackets = append(lostPackets, pkt)
						}
					}
				}

				for _, val := range lostPackets {
					packet := Packet{
						Header: Header{
							Seq:        val.Header.Seq,
							Ack:        1,
							Flag:       FlagTypeData,
							Retransmit: true,
						},
						Data: []byte("Hello Server"),
					}
					conn.Write(packet.Encode())
				}
			default:
				buffer := make([]byte, 1024)
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, _, err := conn.ReadFromUDP(buffer)
				// 超时及读取异常判定
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					return
				}

				recvAckPacket, err := Decode(buffer[:n])
				// 非法/损坏的 Ack 数据包抛弃
				if err != nil {
					continue
				}

				fmt.Printf("server -> client %s\n", recvAckPacket.String())
				receivedPackets = append(receivedPackets, &recvAckPacket)
			}
		}
	}()

	curSeq := int32(1)

	for i := 0; i < 5; i++ {
		packet := Packet{
			Header: Header{
				Seq:  curSeq,
				Ack:  1,
				Flag: FlagTypeData,
			},
			Data: []byte("Hello Server"),
		}

		sentPackets = append(sentPackets, &packet)

		// 模拟第 4 个包丢包
		if i != 3 {
			conn.Write(packet.Encode())
		}

		curSeq += int32(len(packet.Data))
	}

	wg.Wait()
}

func Run() {
	go startServer()

	time.Sleep(200 * time.Millisecond)

	startClient()
}
