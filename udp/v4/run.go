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

	// 因为 conn.ReadFromUDP 方法是阻塞接收操作
	// 所以这里启动一个新的 goroutine
	// 来完成延迟 Ack 操作
	go func() {
		for {
			// 短暂休眠，避免占用过多 CPU 资源
			time.Sleep(100 * time.Millisecond)

			// 未拿到客户端地址或列表为空
			if clientAddr == nil || len(seqList) == 0 {
				continue
			}
			// 延迟时间未到
			if time.Since(lastAckTime) < ackDelay {
				continue
			}

			lastAck = seqList[0][1]
			lastAckChanged := false

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
						Seq:       1,
						Ack:       lastAck,
						Flag:      FlagTypeAck,
						SAckCount: uint8(len(sackList)),
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
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			continue
		}

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

		// 这里假设重传的数据包 100% 接收成功
		// 服务端直接返回确认 Ack 报文
		// 简化对重传数据包的再次 Ack 的实现机制
		if recvPacket.Header.Retransmit {
			// 排序合并后的区间
			sort.Slice(seqRecord, func(i, j int) bool {
				if seqRecord[i][0] != seqRecord[j][0] {
					return seqRecord[i][0] < seqRecord[j][0]
				}
				return seqRecord[i][1] < seqRecord[j][1]
			})

			// 合并重复区间
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
				Data: nil,
			}
			conn.WriteToUDP(ackPacket.Encode(), clientAddr)
			continue
		}
		// 记录常规接收到的区间 Seq
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

	sentPackets := []*Packet{}
	receivedPackets := []*Packet{}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		timeout := time.NewTimer(1 * time.Second)
		defer timeout.Stop()

		ticket := time.NewTicker(300 * time.Millisecond)
		defer ticket.Stop()

		for {
			select {
			case <-timeout.C:
				return
			case <-ticket.C:
				// 错误前置 5: 包全接收完无需重传
				if len(sentPackets) == len(receivedPackets) {
					continue
				}

				lostPackets := []*Packet{}
				receivedAckList := [][2]int32{}

				for _, val := range receivedPackets {
					receivedAckList = append(receivedAckList, val.SAck...)
				}

				// 无有效 ACK 时，直接重传全部已发送包
				if len(receivedAckList) == 0 {
					lostPackets = append(lostPackets, sentPackets...)
				} else {
					// 排序并合并接收到的 ACK 块
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

					// 找出掉包的 Packet
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

				// 执行重传发送
				for _, val := range lostPackets {
					retransmitPkt := Packet{
						Header: Header{
							Seq:        val.Header.Seq,
							Ack:        1,
							Flag:       FlagTypeData,
							Retransmit: true,
						},
						Data: []byte("Hello Server"),
					}
					conn.Write(retransmitPkt.Encode())
				}

			default:
				buffer := make([]byte, 1024)
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, _, err := conn.ReadFromUDP(buffer)
				// 网络超时/读取错误提前返回
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					return
				}

				recvAckPacket, err := Decode(buffer[:n])
				// ACK 数据包无法正常解码直接跳过
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
