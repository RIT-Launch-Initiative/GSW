package main

import (
	"bytes"
	"flag"
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
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

var shmDir = flag.String("shm", "/dev/shm", "directory to use for shared memory")
var configFilepath = flag.String("c", "grafana_live", "name of config file")

// streamTelemetryPacket streams telemetry packet data to Grafana Live as it is received on the channel.
func streamTelemetryPacket(packet tlm.TelemetryPacket, packetChan chan []byte, config *viper.Viper, authToken string) {
	// read config values
	grafanaChannelPath := config.GetString("channel_path")
	liveAddr := config.GetString("live_addr")

	// set up MeasurementGroup
	measurements := make([]db.Measurement, len(packet.Measurements))
	measurementGroup := db.MeasurementGroup{DatabaseName: grafanaChannelPath, Measurements: measurements}
	for i, measurementName := range packet.Measurements {
		measurements[i].Name = measurementName
	}

	// stream data
	for packetData := range packetChan {
		proc.UpdateMeasurementGroup(packet, measurementGroup, packetData)
		query := db.CreateQuery(measurementGroup)
		if err := sendQuery(query, liveAddr, authToken); err != nil {
			fmt.Printf("Error streaming data: %v\n", err)
		}
	}
}

// sendQuery sends the query string containing telemetry data to Grafana Live.
func sendQuery(query string, liveAddr string, authToken string) error {
	// Convert the query string to bytes
	data := []byte(query)
	// Send the query data over HTTP to Grafana Live
	body := bytes.NewReader(data)
	request, err := http.NewRequest(http.MethodPost, liveAddr, body)
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
		return fmt.Errorf("error reading HTTP response body: %v", err)
	}
	err = response.Body.Close()
	if err != nil {
		return fmt.Errorf("error closing response body: %v", err)
	}
	return nil
}

// readConfigFiles reads the configuration file for grafana_live.go as well as the
// telemetry configuration from shared memory. It returns a Viper config object.
func readConfigFiles() (*viper.Viper, error) {
	configReader, err := ipc.CreateIpcShmReader("telemetry-config", *shmDir)
	if err != nil {
		fmt.Println("*** Error accessing config file. Make sure the GSW service is running. ***")
		return nil, err
	}
	data, err := configReader.ReadNoTimestamp()
	if err != nil {
		return nil, fmt.Errorf("error reading shared memory: %v", err)
	}
	_, err = proc.ParseConfigBytes(data)
	if err != nil {
		return nil, fmt.Errorf("error parsing telemetry YAML: %v", err)
	}

	liveConfig := viper.New()
	liveConfig.SetConfigName(*configFilepath)
	liveConfig.SetConfigType("yaml")
	liveConfig.AddConfigPath("data/config/")
	err = liveConfig.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading Grafana Live config: %v", err)
	}

	return liveConfig, nil
}

func main() {
	flag.Parse()
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Error reading .env file: %v\n", err)
		// Program can continue if env variable was set elsewhere
	}
	authToken := os.Getenv("GRAFANA_LIVE_TOKEN")
	if authToken == "" {
		fmt.Println("Error: GRAFANA_LIVE_TOKEN environment variable empty or not set.")
		return
	}
	liveConfig, err := readConfigFiles()
	if err != nil {
		fmt.Printf("Error reading config files: %v\n", err)
		return
	}

	fmt.Println("Starting Grafana Live streaming.")
	for _, packet := range proc.GswConfig.TelemetryPackets {
		packetChan := make(chan []byte)
		go proc.TelemetryPacketReader(packet, packetChan, *shmDir)
		go streamTelemetryPacket(packet, packetChan, liveConfig, authToken)
	}

	// Catch interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\nShutting down Grafana Live streaming.")
}
