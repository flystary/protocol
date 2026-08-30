package v2

import (
	"fmt"
	"log"
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
		lastAck     int32
		lastAckTime = time.Now()
		clientAddr  *net.UDPAddr
	)

	go func() {
		for {
			if time.Since(lastAckTime) >= ackDelay {
				ackPacket := Packet{
					Header: Header{
						Seq:  1,
						Ack:  lastAck,
						Flag: FlagTypeAck,
					},
					Data: nil,
				}
				ackData := ackPacket.Encode()
				if _, err := conn.WriteToUDP(ackData, clientAddr); err != nil {
					log.Printf("Error sending ACK: %v\n", err)
				}
				lastAckTime = time.Now()
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
		recvPacket, err := Decode(buffer[:])
		if err != nil {
			fmt.Println("Error decodeing:", err)
			continue
		}
		fmt.Printf("client -> server %s\n", (recvPacket.String()))

		lastAck = recvPacket.Header.Seq + int32(len(recvPacket.Data))
	}
}

func startClient() {
	conn, err := net.DialUDP("udp", nil, &serverAddr)
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	packet := Packet{
		Header: Header{
			Seq:  1,
			Ack:  1,
			Flag: FlagTypeData,
		},
		Data: []byte("Hello Server"),
	}

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

		recvAckPacket, err := Decode(buffer[:])
		if err != nil {
			fmt.Println("Error decodeing:", err)
		}
		fmt.Printf("server -> client %s\n", recvAckPacket.String())
	}()

	for i := 0; i < 5; i++ {
		data := packet.Encode()
		conn.Write(data)

		// 更新下次发送数据包的 Seq 值
		packet.Header.Seq += int32(len(packet.Data))
	}
	wg.Wait()
}

func Run() {
	go startServer()

	time.Sleep(200 * time.Millisecond)

	startClient()
}
