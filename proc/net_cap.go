package proc

import (
	"fmt"
	"github.com/google/gopacket/pcap"
	"strings"
)

func NetworkCapture() {
	ports := []string{}
	for _, packet := range GswConfig.TelemetryPackets {
		ports = append(ports, fmt.Sprintf("udp port %d", packet.Port))
	}

	filter := strings.Join(ports, " or ")

	handle, err := pcap.OpenLive("any", 1600, true, pcap.BlockForever)
	if err != nil {
		fmt.Printf("Error opening pcap handle: %v\n", err)
		return
	}

}
