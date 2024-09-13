package proc

import (
	"fmt"
	"github.com/AarC10/GSW-V2/lib/ipc"
	"net"
	"strconv"
)

func getIpcShmHandler(packet TelemetryPacket, write bool) (*ipc.IpcShmHandler, error) {
	handler, err := ipc.CreateIpcShmHandler(strconv.Itoa(packet.Port), GetPacketSize(packet), write)
	if err != nil {
		return nil, fmt.Errorf("error creating shared memory handler: %v", err)
	}

	return handler, nil
}

func TelemetryPacketWriter(packet TelemetryPacket) {
	packetSize := GetPacketSize(packet)
	shmWriter, _ := getIpcShmHandler(packet, true)
	if shmWriter == nil {
		fmt.Printf("Failed to create shared memory writer\n")
		return
	}
	defer shmWriter.Cleanup()

	fmt.Printf("Packet size for port %d: %d bytes %d bits\n", packet.Port, packetSize, packetSize*8)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", packet.Port))
	if err != nil {
		fmt.Printf("Error resolving UDP address: %v\n", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("Error listening on UDP: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("Listening on port %d for telemetry packet...\n", packet.Port)

	// Receive data
	buffer := make([]byte, packetSize)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("Error reading from UDP: %v\n", err)
			continue
		}

		// TODO: Make this a config
		//binary.BigEndian.PutUint64(buffer[8:], uint64(time.Now().UnixNano()))

		if n == packetSize {
			err := shmWriter.Write(buffer)
			if err != nil {
				fmt.Printf("Error writing to shared memory: %v\n", err)
			}
		} else {
			fmt.Printf("Received packet of incorrect size. Expected: %d, Received: %d\n", packetSize, n)
		}
	}
}

func TelemetryPacketReader(packet TelemetryPacket, outChannel chan []byte) {
	procReader, err := getIpcShmHandler(packet, false)
	if err != nil {
		fmt.Printf("Error creating proc handler: %v\n", err)
		return
	}
	defer procReader.Cleanup()

	lastUpdate := procReader.LastUpdate()
	for {
		latestUpdate := procReader.LastUpdate()
		if lastUpdate != latestUpdate {
			data, err := procReader.Read()
			if err != nil {
				fmt.Printf("Error reading from shared memory: %v\n", err)
				continue
			}
			lastUpdate = latestUpdate
			outChannel <- data
		}
	}
}
