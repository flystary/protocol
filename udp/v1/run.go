package v1

import (
	"fmt"
	"log"
	"net"
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

	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v\n", err)
			continue
		}

		recvPacket, err := Decode(buffer[:n])
		if err != nil {
			log.Printf("Error decoding packet: %v\n", err)
			continue
		}
		fmt.Printf("client -> server %s\n", recvPacket)

		// 构造响应 ACK 包
		ackPacket := Packet{
			Header: Header{
				Seq:  1,
				Ack:  recvPacket.Header.Seq + int32(len(recvPacket.Data)),
				Flag: FlagTypeAck,
			},
			Data: nil, // ACK 包无变长负载 Data
		}

		ackData := ackPacket.Encode()
		if _, err := conn.WriteToUDP(ackData, clientAddr); err != nil {
			log.Printf("Error sending ACK: %v\n", err)
		}
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

	buffer := make([]byte, 1024)

	for i := 0; i < 5; i++ {
		data := packet.Encode()
		if _, err := conn.Write(data); err != nil {
			log.Printf("Error sending data: %v\n", err)
			return
		}

		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("Error reading ACK: %v\n", err)
			return
		}

		recvAckPacket, err := Decode(buffer[:n])
		if err != nil {
			log.Printf("Error decoding ACK packet: %v\n", err)
			return
		}

		fmt.Printf("server -> client %s\n", recvAckPacket)

		// 更新下次发送数据包的 Seq
		packet.Header.Seq = recvAckPacket.Header.Ack
	}
}

func Run() {
	go startServer()

	time.Sleep(200 * time.Millisecond)

	startClient()
}
