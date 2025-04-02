package main

import (
	"bytes"
	"fmt"
	"github.com/AarC10/GSW-V2/lib/db"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
)

const grafanaChannelName = "backplane" // appended to the end of the address

// streamTelemetryPacket streams telemetry packet data to Grafana Live as it is received on the channel.
func streamTelemetryPacket(packet tlm.TelemetryPacket, packetChan chan []byte) {
	// set up MeasurementGroup
	measurements := make([]db.Measurement, len(packet.Measurements))
	measurementGroup := db.MeasurementGroup{DatabaseName: grafanaChannelName, Measurements: measurements}
	for i, measurementName := range packet.Measurements {
		measurements[i].Name = measurementName
	}

	// stream data
	for packetData := range packetChan {
		proc.UpdateMeasurementGroup(packet, measurementGroup, packetData)
		query := db.CreateQuery(measurementGroup)
		if err := sendQuery(query); err != nil {
			fmt.Printf("Error streaming data: %v\n", err)
		}
	}
}

// sendQuery sends the query string containing telemetry data to Grafana Live.
func sendQuery(query string) error {
	// Convert the query string to bytes
	data := []byte(query)
	// Send the query data over HTTP to Grafana Live
	// TODO: config!!!
	liveAddr := "http://localhost:3000/api/live/push/custom_stream_id/"
	body := bytes.NewReader(data)
	request, err := http.NewRequest(http.MethodPost, liveAddr, body)
	authToken := "REDACTED" // TODO auth_token config
	request.Header.Set("Authorization", "Bearer "+authToken)
	if err != nil {
		return fmt.Errorf("error forming HTTP request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("error returned by HTTP request: %v", err)
	}
	if response.StatusCode != 200 {
		fmt.Printf("Possible unexpected status code: %d\n", response.StatusCode)
	}

	_, err = io.ReadAll(response.Body)
	if err != nil {
		response.Body.Close()
		return fmt.Errorf("error returned by HTTP request: %v", err)
	}
	err = response.Body.Close()
	if err != nil {
		return fmt.Errorf("error closing response body: %v", err)
	}
	return nil
}

func main() {
	configReader, err := ipc.CreateIpcShmReader("telemetry-config")
	if err != nil {
		fmt.Println("*** Error accessing config file. Make sure the GSW service is running. ***")
		fmt.Printf("(%v)\n", err)
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

	fmt.Println("Starting Grafana Live service.")
	for _, packet := range proc.GswConfig.TelemetryPackets {
		packetChan := make(chan []byte)
		go proc.TelemetryPacketReader(packet, packetChan)
		go streamTelemetryPacket(packet, packetChan)
	}

	// Catch interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("Shutting down Grafana Live service.")
}
