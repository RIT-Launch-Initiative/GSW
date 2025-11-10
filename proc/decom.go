package proc

import (
	"fmt"
	"net"
	"strconv"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
)

// newIpcShmHandlerForPacket creates a shared memory IPC handler for a telemetry packet
// If write is true, the handler will be created for writing to shared memory
// If write is false, the handler will be created for reading from shared memory
func newIpcShmHandlerForPacket(packet tlm.TelemetryPacket, write bool, shmDir string) (*ipc.ShmHandler, error) {
	handler, err := ipc.NewShmHandler(strconv.Itoa(packet.Port), GetPacketSize(packet), write, shmDir)
	if err != nil {
		return nil, fmt.Errorf("error creating shared memory handler: %v", err)
	}

	return handler, nil
}

// TelemetryPacketWriter is a goroutine that listens for telemetry data on a UDP port and writes it to shared memory
func TelemetryPacketWriter(packet tlm.TelemetryPacket, outChannel chan []byte, shmDir string) {
	packetSize := GetPacketSize(packet)
	shmWriter, _ := newIpcShmHandlerForPacket(packet, true, shmDir)
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

func NewIpcShmReaderForPacket(packet tlm.TelemetryPacket, shmDir string) (ipc.Reader, error) {
	return newIpcShmHandlerForPacket(packet, false, shmDir)
}
