package main

import (
	"fmt"
	"github.com/AarC10/GSW-V2/proc"
)

func printTelemetryPackets(cfg *proc.Configuration) {
	fmt.Println("Telemetry Packets:")
	for _, packet := range cfg.TelemetryPackets {
		fmt.Printf("\tName: %s\n\tPort: %d\n", packet.Name, packet.Port)
		if len(packet.Measurements) > 0 {
			fmt.Println("\tMeasurements:")
			for _, measurementName := range packet.Measurements {
				measurement, err := findMeasurementByName(cfg.Measurements, measurementName)
				if err != nil {
					fmt.Printf("\t\tMeasurement '%s' not found: %v\n", measurementName, err)
					continue
				}
				fmt.Printf("\t\t%s\n", measurement.String())
			}
		} else {
			fmt.Println("\t\tNo measurements defined.")
		}
	}
}

func findMeasurementByName(measurements []proc.Measurement, name string) (*proc.Measurement, error) {
	for _, m := range measurements {
		if m.Name == name {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("measurement '%s' not found", name)
}

func main() {
	cfg, err := proc.ParseYAML("data/config/backplane.yaml")
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return
	}

	printTelemetryPackets(cfg)
}
