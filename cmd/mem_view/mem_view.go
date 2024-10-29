package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/lib/util"
	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/proc"
)

func buildString(packet tlm.TelemetryPacket, data []byte, startLine int) string {
	var sb strings.Builder
	offset := 0

	// Print the measurement name, base-10 value, and base-16 value. One for each line
	// Format: MeasurementName: Value (Base-10) [(Base-16)]
	sb.WriteString(fmt.Sprintf("\033[%d;0H", startLine))
	for _, measurementName := range packet.Measurements {
		measurement, ok := proc.GswConfig.Measurements[measurementName]
		if !ok {
			fmt.Printf("\t\tMeasurement '%s' not found\n", measurementName)
			continue
		}

		value, err := tlm.InterpretMeasurementValue(measurement, data[offset:offset+measurement.Size])
		if err != nil {
			continue
		}

		sb.WriteString(fmt.Sprintf("%s: %v [%s]          \n", measurementName, value, util.Base16String(data[offset:offset+measurement.Size], 1)))
		offset += measurement.Size
	}

	return sb.String()
}

func printTelemetryPacket(startLine int, packet tlm.TelemetryPacket, rcvChan chan []byte) {
	fmt.Print(buildString(packet, make([]byte, proc.GetPacketSize(packet)), startLine))

	for {
		data := <-rcvChan
		buildString(packet, data, startLine)
		fmt.Print(buildString(packet, data, startLine))
	}
}

func main() {
	configReader, err := ipc.CreateIpcShmReader("config")
	if err != nil {
		fmt.Printf("Error creating shared memory handler: %v\n", err)
		return
	}
	data, err := configReader.ReadNoTimestamp()
	if err != nil {
		fmt.Printf("Error reading shared memory: %v\n", err)
		return
	}
	_, err = proc.ParseConfigBytes(data)
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return
	}

	// Clear screen
	fmt.Print("\033[2J")

	// Hide the cursor
	fmt.Print("\033[?25l")

	startLine := 0
	for _, packet := range proc.GswConfig.TelemetryPackets {
		outChan := make(chan []byte)
		go proc.TelemetryPacketReader(packet, outChan)
		go printTelemetryPacket(startLine, packet, outChan)
		startLine += len(packet.Measurements) + 1
	}

	// Set up channel to catch interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Print("\033[2J")
	fmt.Print("\033[?25h")
}
