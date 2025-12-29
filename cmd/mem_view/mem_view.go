package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/lib/util"
	"github.com/AarC10/GSW-V2/proc"
)

var shmDir = flag.String("shm", "/dev/shm", "directory to use for shared memory")

// buildString creates a string representation of the telemetry packet data
// Format: MeasurementName: Value (Base-10) [(Base-16)]
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

// printTelemetryPacket prints the telemetry packet data to the console
// Written to the console at the specified start line and updated as new data is received
func printTelemetryPacket(startLine int, packet tlm.TelemetryPacket) {
	reader, err := proc.NewIpcShmReaderForPacket(packet, *shmDir)
	if err != nil {
		fmt.Printf("Error creating reader: %v\n", err)
		return
	}
	defer reader.Cleanup()

	fmt.Print(buildString(packet, make([]byte, proc.GetPacketSize(packet)), startLine))

	for {
		p, err := reader.Read(context.TODO())
		if err != nil {
			fmt.Printf("Error reading packet: %v\n", err)
			continue
		}
		data := p.Data()

		fmt.Print(buildString(packet, data, startLine))
	}
}

func main() {
	flag.Parse()

	configData, err := proc.ReadTelemetryConfigFromShm(*shmDir)
	if err != nil {
		fmt.Println("*** Error accessing config file. Make sure the GSW service is running. ***")
		fmt.Printf("(%v)\n", err)
		return
	}
	_, err = proc.ParseConfigBytes(configData)
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
		go printTelemetryPacket(startLine, packet)
		startLine += len(packet.Measurements) + 1
	}

	// Set up channel to catch interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Print("\033[2J")
	fmt.Print("\033[?25h")
}
