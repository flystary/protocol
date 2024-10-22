package v1

import (
	"fmt"
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
			fmt.Println("Error reading:", err)
			continue
		}

		recvPacket := decode(buffer[:n])
		fmt.Printf("client -> server %s\n", serialization(&recvPacket))

		ackPacket := Packet{
			Seq:  1,
			Ack:  recvPacket.Seq + len(recvPacket.Data),
			Data: "",
			Flag: FlagTypeAck,
		}
		ackData := encode(&ackPacket)
		conn.WriteToUDP(ackData, clientAddr)

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

	for i := 0; i < 5; i++ {
		data := encode(&packet)
		conn.Write(data)

		buffer := make([]byte, 1024)
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			return
		}

		recvAckPacket := decode(buffer[:n])
		fmt.Printf("server -> client %s\n", serialization(&recvAckPacket))

		// 更新下次发送数据包的 Seq 值
		packet.Seq = recvAckPacket.Ack
	}
}

func Run() {
	go startServer()

	time.Sleep(200 * time.Millisecond)

	startClient()
}
