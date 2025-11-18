package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
)

const (
	PRINT_INTERVAL = 3
)

var totalPacketsReceived atomic.Uint64

func packetReader(packet tlm.TelemetryPacket) {
	reader, err := proc.NewIpcShmReaderForPacket(packet, "/dev/shm")
	if err != nil {
		log.Fatal(fmt.Errorf("couldn't create reader for packet: %w", err))
	}

	defer reader.Cleanup()

	var totalPacketsReader uint64
	var packetsLost uint64

	var averageDiff uint64
	var averageUdpShmDiff uint64
	var averageBenchShmDiff uint64

	go func() {
		var lastPacketsReceived uint64
		var lastPacketsLost uint64
		for {
			time.Sleep(time.Second * PRINT_INTERVAL)
			var sb strings.Builder

			packetsInterval := totalPacketsReader - lastPacketsReceived
			packetsLostInterval := packetsLost - lastPacketsLost

			sb.WriteString(fmt.Sprintf("[%s] Packets/s: %d\n", packet.Name, packetsInterval/PRINT_INTERVAL))
			sb.WriteString(fmt.Sprintf("[%s] Reader lost/s: %d (%.3f%%)\n", packet.Name, packetsLostInterval/PRINT_INTERVAL, (float64(packetsLostInterval)/float64(packetsInterval+packetsLostInterval))*100))
			sb.WriteString(fmt.Sprintf("[%s] Average Diff: %d\n", packet.Name, averageDiff))
			sb.WriteString(fmt.Sprintf("[%s] Average UDP SHM Diff: %d\n", packet.Name, averageUdpShmDiff))
			sb.WriteString(fmt.Sprintf("[%s] Average Bench SHM Diff: %d\n", packet.Name, averageBenchShmDiff))

			lastPacketsReceived = totalPacketsReader
			lastPacketsLost = packetsLost

			fmt.Print(sb.String())
		}
	}()

	var lastPacketSequence uint64

	for {
		p, err := reader.Read()
		if err != nil {
			log.Fatal(fmt.Errorf("couldn't read packet: %w", err))
		}
		shmPacket, ok := p.(*ipc.ShmReaderMessage)
		if !ok {
			log.Fatal(fmt.Errorf("packet is not from shm IPC reader"))
		}

		data := shmPacket.Data()
		receiveTimestamp := shmPacket.ReceiveTimestamp()
		packetSequence := binary.BigEndian.Uint64(data[8:16])

		udpTimestamp := binary.BigEndian.Uint64(data[0:8])
		timestamp := uint64(time.Now().UnixNano())

		udpShmDiff := receiveTimestamp - udpTimestamp
		benchShmDiff := timestamp - receiveTimestamp
		totalDiff := timestamp - udpTimestamp
		if averageDiff == 0 {
			averageDiff = totalDiff
		}

		averageDiff = (averageDiff + totalDiff) / 2
		averageUdpShmDiff = (averageUdpShmDiff + udpShmDiff) / 2
		averageBenchShmDiff = (averageBenchShmDiff + benchShmDiff) / 2

		// the latter condition is a simplification that could mean that some
		// lost packets are not accounted for during an overflow.
		if lastPacketSequence != 0 && lastPacketSequence <= packetSequence {
			packetsLost += packetSequence - lastPacketSequence - 1
		}
		lastPacketSequence = packetSequence

		totalPacketsReader += 1
		totalPacketsReceived.Add(1)
	}
}

func reader(packets []*tlm.TelemetryPacket) {
	go func() {
		var lastPacketsReceived uint64
		for {
			time.Sleep(time.Second * PRINT_INTERVAL)
			totalLoaded := totalPacketsReceived.Load()
			fmt.Printf("Total: %d packets/second\n", (totalLoaded-lastPacketsReceived)/PRINT_INTERVAL)

			lastPacketsReceived = totalLoaded
		}
	}()

	for _, packet := range packets {
		go packetReader(*packet)
	}
}
