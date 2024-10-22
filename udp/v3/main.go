package v2

import (
	"fmt"
	"net"
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
	const ackDelay = 200 * time.Millisecond

	var (
		lastAck     int
		seqList     = [][2]int{}
		lastAckTime = time.Now()
		clientAddr  *net.UDPAddr
	)

	go func() {
		for {
			if time.Since(lastAckTime) >= ackDelay && len(seqList) > 0 {

				lastAck = seqList[0][1]
				lastAckChange := false

				mergedSeqList := [][2]int{
					seqList[0],
				}
				for i := 1; i < len(seqList); i++ {
					if seqList[i][0] == mergedSeqList[len(mergedSeqList)-1][1] {
						mergedSeqList[len(mergedSeqList)-1][1] = seqList[i][1]

						if !lastAckChange {
							lastAck = mergedSeqList[len(mergedSeqList)-1][1]
						}
					} else {
						lastAckChange = true
						mergedSeqList = append(mergedSeqList, seqList[i])
					}
				}

				for _, seq := range mergedSeqList {
					ackPacket := Packet{
						Seq:  1,
						Ack:  lastAck,
						SAck: fmt.Sprintf("%d-%d", seq[0], seq[1]),
						Data: "",
						Flag: FlagTypeAck,
					}
					ackData := encode(&ackPacket)
					conn.WriteToUDP(ackData, clientAddr)
				}
				lastAckTime = time.Now()
				seqList = seqList[:0]
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	for {
		_, clientAddr, err = conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			continue
		}

		recvPacket := decode(buffer[:])
		fmt.Printf("client -> server %s\n", serialization(&recvPacket))

		// lastAck = recvPacket.Seq + len(recvPacket.Data)
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



	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		buffer := make([]byte, 1024)
		_, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			return
		}

		recvAckPacket := decode(buffer[:])
		fmt.Printf("server -> client %s\n", serialization(&recvAckPacket))

	}()

	packet := Packet{
		Seq:  1,
		Ack:  1,
		Data: "Hello Server",
		Flag: FlagTypeData,
	}

	for i := 0; i < 5; i++ {
		data := encode(&packet)
		conn.Write(data)

		// 更新下次发送数据包的 Seq 值

	}
	packet.Seq += len(packet.Data)
	wg.Wait()
}

func Run() {
	go startServer()

	time.Sleep(200 * time.Millisecond)

	startClient()
}
