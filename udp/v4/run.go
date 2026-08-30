package v4

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
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

	buffer := make([]byte, 32)

	// 延迟 200 毫秒发送 ACK
	const ackDelay = 200 * time.Millisecond

	var (
		// 延迟 Ack
		lastAck int

		// 记录接收到的区间 Seq
		// [0]: 区间起始 Seq
		// [1]: 区间结束 Seq, Seq + Data.Len()
		seqList = [][2]int{}

		// 记录历史接收到的所有区间 Seq
		seqRecord = [][2]int{}

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
			// 超过延迟时间，发送 Ack 确认包
			if time.Since(lastAckTime) >= ackDelay && len(seqList) > 0 {
				// 超过延迟时间，发送 Ack 确认包
				// 构造 Ack 包并发送

				lastAck = seqList[0][1]
				lastAckChanged := false

				// 因为丢包，可能存在多个区间 Ack 确认包
				// 所以需要分开单独发送
				// 根据 Seq 合并区间
				mergedSeqList := [][2]int{
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
					ackPacket := Packet{
						// 因为这个示例中
						// 服务端不主动发送数据
						// 所以 Seq 固定为 1
						Seq:  1,
						Ack:  lastAck,
						SAck: fmt.Sprintf("%d-%d", seq[0], seq[1]),
						Data: "",
						Flag: FlagTypeAck,
					}

					ackData := encode(&ackPacket)
					conn.WriteToUDP(ackData, clientAddr)
				}

				// 更新最后发送 Ack 的时间
				lastAckTime = time.Now()

				// 重置区间 Seq
				seqList = seqList[:0]
			}

			// 短暂休眠，避免占用过多 CPU 资源
			time.Sleep(100 * time.Millisecond)
		}
	}()

	for {
		_, clientAddr, err = conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			continue
		}

		// 解析接收到的数据包
		recvPacket := decode(buffer[:])

		fmt.Printf("client -> server %s\n", serialization(&recvPacket))

		// 记录历史区间 Seq
		seqRecord = append(seqRecord, [2]int{
			recvPacket.Seq,
			recvPacket.Seq + len(recvPacket.Data),
		})

		// 这里假设重传的数据包 100% 接收成功
		// 服务端直接返回确认 Ack 报文
		// 简化对重传数据包的再次 Ack 的实现机制
		if recvPacket.Retransmit {
			// 排序合并后的区间
			sort.Slice(seqRecord, func(i, j int) bool {
				return seqRecord[i][0] < seqRecord[j][0] && seqRecord[i][1] < seqRecord[j][1]
			})
			// 合并重复区间
			// 合并重复区间
			uniqueIndex := 0
			for i := 1; i < len(seqRecord); i++ {
				if seqRecord[i][0] == seqRecord[uniqueIndex][1] {
					seqRecord[uniqueIndex][1] = seqRecord[i][1]
				} else {
					uniqueIndex++
				}
			}
			seqRecord = seqRecord[:uniqueIndex+1]

			// 更新已经接收到连续区间最大 Ack
			lastAck = seqRecord[0][1]

			recvPacket.SAck = fmt.Sprintf("%d-%d", recvPacket.Seq, recvPacket.Seq+len(recvPacket.Data))
			recvPacket.Ack = lastAck

			recvPacket.Seq = 1
			recvPacket.Flag = FlagTypeAck
			conn.WriteToUDP(encode(&recvPacket), clientAddr)
			continue
		}

		// 记录接收到的区间 Seq
		seqList = append(seqList, [2]int{
			recvPacket.Seq,
			recvPacket.Seq + len(recvPacket.Data),
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
				// 发送的数据包已经被接收方全部确认
				// 无需重传
				if len(sentPackets) == len(receivedPackets) {
					continue
				}

				// 通过区间差集算法
				// 同时考虑 选择性确认 的情况
				lostPackets := []*Packet{}
				receivedAckList := [][2]int{}
				for _, val := range receivedPackets {
					ackBlock := strings.Split(val.SAck, "-")
					start, _ := strconv.ParseInt(ackBlock[0], 10, 64)
					end, _ := strconv.ParseInt(ackBlock[1], 10, 64)
					receivedAckList = append(receivedAckList, [2]int{
						int(start),
						int(end),
					})
				}

				// 排序合并后的区间
				sort.Slice(receivedAckList, func(i, j int) bool {
					return receivedAckList[i][0] < receivedAckList[j][0] && receivedAckList[i][1] < receivedAckList[j][1]
				})
				// 合并重复区间
				uniqueIndex := 0
				for i := 1; i < len(receivedAckList); i++ {
					if receivedAckList[i][0] == receivedAckList[uniqueIndex][1] {
						receivedAckList[uniqueIndex][1] = receivedAckList[i][1]
					} else {
						uniqueIndex++
					}
				}
				receivedAckList = receivedAckList[:uniqueIndex+1]

				// 计算丢失的数据包
				curRecvIndex := 0
				for i, val := range sentPackets {
					if curRecvIndex >= len(receivedPackets) {
						lostPackets = append(lostPackets, val)
						continue
					}
					if val.Seq > receivedAckList[curRecvIndex][1] {
						curRecvIndex++
						lostPackets = append(lostPackets, sentPackets[i-1])
					}
				}

				for _, val := range lostPackets {
					// 构建 1 个 UDP 数据包
					packet := Packet{
						Seq:        val.Seq,
						Ack:        1,
						Data:       "Hello Server",
						Flag:       FlagTypeData,
						Retransmit: true,
					}

					data := encode(&packet)
					conn.Write(data)
				}
			default:
				// 接收 Ack 包
				buffer := make([]byte, 32)

				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				_, _, err := conn.ReadFromUDP(buffer)
				if err != nil {
					continue
				}

				recvAckPacket := decode(buffer[:])
				fmt.Printf("server -> client %s\n", serialization(&recvAckPacket))

				// 更新接收到的数据包 Seq
				receivedPackets = append(receivedPackets, &recvAckPacket)
			}
		}
	}()

	//  客户端 Seq 值从 1 开始
	curSeq := 1

	// 连续发送 5 个 UDP 数据包
	for i := 0; i < 5; i++ {
		// 构建 1 个 UDP 数据包
		packet := Packet{
			Seq:  curSeq,
			Ack:  1,
			Data: "Hello Server",
			Flag: FlagTypeData,
		}

		// 更新发送过的数据包 Seq
		sentPackets = append(sentPackets, &packet)

		// 第 4 个数据包模拟丢包
		if i != 3 {
			data := encode(&packet)
			conn.Write(data)
		}

		// 更新下次发送数据包的 Seq 值
		curSeq += len(packet.Data)
	}

	// 等待 Ack 报文接收完成
	wg.Wait()
}

func Run() {
	go startServer()

	time.Sleep(200 * time.Millisecond)

	startClient()
}
