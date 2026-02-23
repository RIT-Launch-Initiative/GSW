package proc

import "strconv"

func NetworkCapture() {
	ports := []string{}
	for _, packet := range GswConfig.TelemetryPackets {
		ports = append(ports, strconv.Itoa(packet.Port))
	}

	for {

	}
}
