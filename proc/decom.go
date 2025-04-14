package proc

import (
	"fmt"
	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"net"
	"strconv"
)

// getIpcShmHandler creates a shared memory IPC handler for a telemetry packet
// If write is true, the handler will be created for writing to shared memory
// If write is false, the handler will be created for reading from shared memory
func getIpcShmHandler(packet tlm.TelemetryPacket, write bool) (*ipc.IpcShmHandler, error) {
	handler, err := ipc.CreateIpcShmHandler(strconv.Itoa(packet.Port), GetPacketSize(packet), write)
	if err != nil {
		return nil, fmt.Errorf("error creating shared memory handler: %v", err)
	}

	return handler, nil
}

// TelemetryPacketWriter is a goroutine that listens for telemetry data on a UDP port and writes it to shared memory
func TelemetryPacketWriter(packet tlm.TelemetryPacket, outChannel chan []byte) {
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
	defer func(conn *net.UDPConn) {
		err := conn.Close()
		if err != nil {
			fmt.Printf("Error closing UDP connection: %v\n", err)
		}
	}(conn)

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

			select {
			case outChannel <- buffer:
				break
			default:
				break
			}
		} else {
			fmt.Printf("Received packet of incorrect size. Expected: %d, Received: %d\n", packetSize, n)
		}
	}
}

// TelemetryPacketReader is a goroutine that reads telemetry data from shared memory and sends it to an output channel
func TelemetryPacketReader(packet tlm.TelemetryPacket, outChannel chan []byte) {
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
