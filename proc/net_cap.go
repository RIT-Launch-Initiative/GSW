package proc

import (
	"fmt"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"

	"os"
	"strings"
	"time"
)

func getFilter() string {
	ports := []string{}
	for _, packet := range GswConfig.TelemetryPackets {
		ports = append(ports, fmt.Sprintf("udp port %d", packet.Port))
	}

	filter := strings.Join(ports, " or ")
	return filter
}

func createOutputFile() (*os.File, error) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	// TODO: Configurable output directory
	if _, err := os.Stat("captures"); os.IsNotExist(err) {
		err := os.Mkdir("captures", 0755)
		if err != nil {
			return nil, fmt.Errorf("error creating captures directory: %v", err)
		}
	}

	filename := fmt.Sprintf("captures/%s_%s.pcap", GswConfig.Name, timestamp)

	return os.Create(filename)
}

func NetworkCapture() {
	filter := getFilter()

	handle, err := pcap.OpenLive("any", 1600, true, pcap.BlockForever)
	if err != nil {
		fmt.Printf("Error opening pcap handle: %v\n", err)
		return
	}
	defer handle.Close()

	pcapFile, err := createOutputFile()
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		return
	}
	defer pcapFile.Close()

	writer := pcapgo.NewWriter(pcapFile)

}
