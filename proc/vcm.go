package proc

import (
	"fmt"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"gopkg.in/yaml.v2"
	"os"
)

// Configuration is a struct that holds the configuration for the GSW
type Configuration struct {
	Name             string                     `yaml:"name"`              // Name of the configuration
	Measurements     map[string]tlm.Measurement `yaml:"measurements"`      // Map of measurements
	TelemetryPackets []tlm.TelemetryPacket      `yaml:"telemetry_packets"` // List of telemetry packets
}

// TODO: Make global safer
var GswConfig Configuration

// ResetConfig resets the global configuration
func ResetConfig() {
	GswConfig = Configuration{}
}

// ParseConfig parses a YAML configuration file and returns a Configuration struct
func ParseConfig(filename string) (*Configuration, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading YAML file: %v", err)
	}
	return ParseConfigBytes(data)
}

// ParseConfigBytes parses a YAML formatted byte slice and returns a Configuration struct
func ParseConfigBytes(data []byte) (*Configuration, error) {
	// Unmarshalling doesn't seem to lead to errors with bad data. Better to check result config
	err := yaml.Unmarshal(data, &GswConfig)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling YAML: %v", err)
	}
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

// GetPacketSize returns the size of a telemetry packet in bytes
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
