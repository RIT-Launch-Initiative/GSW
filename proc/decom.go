package proc

import (
	"fmt"
	"net"
)

func getPacketSize(packet TelemetryPacket) int {
	size := 0
	for _, measurementName := range packet.Measurements {
		measurement, err := FindMeasurementByName(GswConfig.Measurements, measurementName)
		if err != nil {
			fmt.Printf("\t\tMeasurement '%s' not found: %v\n", measurementName, err)
			continue
		}
		size += measurement.Size
	}
	return size
}

func byteSwap(data []byte, startIndex int, stopIndex int) {
	for i, j := startIndex, stopIndex; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
}

func PacketListener(packet TelemetryPacket, channel chan []byte) {
	packetSize := getPacketSize(packet)
	fmt.Printf("Packet size for port %d: %d\n", packet.Port, packetSize)

	// Listen over UDP
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

		if n == packetSize {
			channel <- buffer[:n] // Send data over channel
		} else {
			fmt.Printf("Received packet of incorrect size. Expected: %d, Received: %d\n", packetSize, n)
		}
	}
}

func EndianessConverter(packet TelemetryPacket, channel chan []byte) {
	byteIndicesToSwap := make([][]int, 0)

	startIndice := 0
	for _, measurementName := range packet.Measurements {
		measurement, err := FindMeasurementByName(GswConfig.Measurements, measurementName)
		if err != nil {
			fmt.Printf("\t\tMeasurement '%s' not found: %v\n", measurementName, err)
			continue
		}

		if measurement.Endianness == "little" {
			byteIndicesToSwap = append(byteIndicesToSwap, []int{startIndice, startIndice + measurement.Size - 1})
		}

		startIndice += measurement.Size
	}

	for {
		data := <-channel
		for _, byteIndices := range byteIndicesToSwap {
			byteSwap(data, byteIndices[0], byteIndices[1])
		}

		fmt.Printf("Received data: %v\n", data)
	}
}
