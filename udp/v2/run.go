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
		lastAckTime = time.Now()
		clientAddr  *net.UDPAddr
	)

	go func() {
		for {
			if time.Since(lastAckTime) >= ackDelay {
				ackPacket := Packet{
					Seq:  1,
					Ack:  lastAck,
					Data: "",
					Flag: FlagTypeAck,
				}
				ackData := encode(&ackPacket)
				conn.WriteToUDP(ackData, clientAddr)
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

		recvPacket := decode(buffer[:])
		fmt.Printf("client -> server %s\n", serialization(&recvPacket))

		lastAck = recvPacket.Seq + len(recvPacket.Data)
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
		Seq:  1,
		Ack:  1,
		Data: "Hello Server",
		Flag: FlagTypeData,
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

		recvAckPacket := decode(buffer[:])
		fmt.Printf("server -> client %s\n", serialization(&recvAckPacket))

	}()

	for i := 0; i < 5; i++ {
		data := encode(&packet)
		conn.Write(data)

		// 更新下次发送数据包的 Seq 值
		packet.Seq += len(packet.Data)
	}
	wg.Wait()
}

func Run() {
	go startServer()

	time.Sleep(200 * time.Millisecond)

	startClient()
}
