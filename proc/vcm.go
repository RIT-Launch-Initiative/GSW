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

var GswConfig Configuration // TODO: Make global safer

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
	_ = yaml.Unmarshal(data, &GswConfig)
	if GswConfig.Name == "" {
		return nil, fmt.Errorf("no configuration name provided")
	}

	if len(GswConfig.Measurements) == 0 {
		return nil, fmt.Errorf("no measurements found in configuration")
	}

	if len(GswConfig.TelemetryPackets) == 0 {
		return nil, fmt.Errorf("no telemetry packets found in configuration")
	}

	// Set default values for measurements if not specified
	for k := range GswConfig.Measurements {
		// TODO: More strict checks of configuration and input handling
		if GswConfig.Measurements[k].Name == "" {
			return nil, fmt.Errorf("measurement name missing in configuration")
		}

		if GswConfig.Measurements[k].Endianness == "" {
			entry := GswConfig.Measurements[k] // Workaround to avoid UnaddressableFieldAssign
			entry.Endianness = "big"           // Default to big endian
			GswConfig.Measurements[k] = entry
		} else if GswConfig.Measurements[k].Endianness != "little" && GswConfig.Measurements[k].Endianness != "big" {
			return nil, fmt.Errorf("endianness specified as %s, instead of big or little", GswConfig.Measurements[k].Endianness)
		}

		if GswConfig.Measurements[k].Scaling == 0 {
			entry := GswConfig.Measurements[k] // Workaround to avoid UnaddressableFieldAssign
			entry.Scaling = 1.0                // Default scaling factor
			GswConfig.Measurements[k] = entry
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
