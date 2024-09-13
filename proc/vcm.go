package proc

import (
	"fmt"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"gopkg.in/yaml.v2"
	"os"
)

type Configuration struct {
	Name             string                     `yaml:"name"`
	Measurements     map[string]tlm.Measurement `yaml:"measurements"`
	TelemetryPackets []tlm.TelemetryPacket      `yaml:"telemetry_packets"`
}

// TODO: Make global safer
var GswConfig Configuration

func ResetConfig() {
	GswConfig = Configuration{}
}

func ParseConfig(filename string) (*Configuration, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading YAML file: %v", err)
	}

	// Unmarshalling doesn't seem to lead to errors with bad data. Better to check result config
	_ = yaml.Unmarshal(data, &GswConfig)
	if GswConfig.Name == "" {
		return nil, fmt.Errorf("Error parsing YAML. No config name.")
	}

	if len(GswConfig.Measurements) == 0 {
		return nil, fmt.Errorf("Error parsing YAML. No measurements.")
	}

	if len(GswConfig.TelemetryPackets) == 0 {
		return nil, fmt.Errorf("Error parsing YAML. No telemetry packets.")
	}

	// Set default values for measurements if not specified
	for k := range GswConfig.Measurements {
		// TODO: More strict checks of configuration and input handling
		if GswConfig.Measurements[k].Name == "" {
			return nil, fmt.Errorf("Measurement name missing")
		}

		if GswConfig.Measurements[k].Endianness == "" {
			entry := GswConfig.Measurements[k] // Workaround to avoid UnaddressableFieldAssign
			entry.Endianness = "big"           // Default to big endian
			GswConfig.Measurements[k] = entry
		} else if GswConfig.Measurements[k].Endianness != "little" && GswConfig.Measurements[k].Endianness != "big" {
			return nil, fmt.Errorf("Endianess not specified as big or little got %s", GswConfig.Measurements[k].Endianness)
		}
	}

	return &GswConfig, nil
}

func GetPacketSize(packet tlm.TelemetryPacket) int {
	size := 0
	for _, measurementName := range packet.Measurements {
		measurement, ok := GswConfig.Measurements[measurementName]
		if !ok {
			fmt.Printf("\t\tMeasurement '%s' not found\n", measurementName)
			continue
		}
		size += measurement.Size
	}
	return size
}
