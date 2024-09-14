package main

import (
	"encoding/binary"
	"fmt"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AarC10/GSW-V2/proc"
)

func calculateTimestamps(startLine int, packet tlm.TelemetryPacket, rcvChan chan []byte) {
	var averageDiff uint64

	udpTimestampMeas, ok := proc.GswConfig.Measurements["UdpSendTimestamp"]
	if !ok {
		fmt.Printf("\t\tMeasurement 'UdpSendTimestamp' not found\n")
		return
	}

	for {
		data := <-rcvChan

		timestamp := uint64(time.Now().UnixNano())
		udpTimestamp := binary.BigEndian.Uint64(data)
		shmTimestamp := binary.BigEndian.Uint64(data[udpTimestampMeas.Size:])

		// Calculate the difference between the two timestamps
		udpShmDiff := shmTimestamp - udpTimestamp
		benchShmDiff := timestamp - shmTimestamp
		totalDiff := timestamp - udpTimestamp
		if averageDiff == 0 {
			averageDiff = totalDiff
		}

		averageDiff = (averageDiff + totalDiff) / 2

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\033[%d;0H", startLine))
		sb.WriteString(packet.Name + ":\n")
		sb.WriteString(fmt.Sprintf("\tUDP Timestamp: %d\n", udpTimestamp))
		sb.WriteString(fmt.Sprintf("\tSHM Timestamp: %d\n", shmTimestamp))
		sb.WriteString(fmt.Sprintf("\tBench Timestamp: %d\n", timestamp))
		sb.WriteString(fmt.Sprintf("\tUDP-SHM Diff: %d\n", udpShmDiff))
		sb.WriteString(fmt.Sprintf("\tBench-SHM Diff: %d\n", benchShmDiff))
		sb.WriteString(fmt.Sprintf("\tTotal Diff: %d\n", totalDiff))
		sb.WriteString(fmt.Sprintf("\tAverage Diff: %d\n", averageDiff))
		fmt.Print(sb.String())

		time.Sleep(3 * time.Second)
	}
}

func main() {
	_, err := proc.ParseConfig("data/test/benchmark.yaml")
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return
	}

	// Clear screen
	fmt.Print("\033[2J")

	// Hide the cursor
	fmt.Print("\033[?25l")

	for i, packet := range proc.GswConfig.TelemetryPackets {
		outChan := make(chan []byte)
		go proc.TelemetryPacketReader(packet, outChan)
		go calculateTimestamps(i*9, packet, outChan)
	}

	// Set up channel to catch interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Print("\033[2J")
	fmt.Print("\033[?25h")
}
