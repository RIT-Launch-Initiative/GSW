package tlm

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// Measurement represents a single measurement in a telemetry packet.
type Measurement struct {
	Name       string  `yaml:"name"`                 // Name of the measurement
	Size       int     `yaml:"size"`                 // Size of the measurement in bytes
	Type       string  `yaml:"type,omitempty"`       // Type of the measurement (int, float)
	Unsigned   bool    `yaml:"unsigned,omitempty"`   // Whether the measurement is unsigned
	Endianness string  `yaml:"endianness,omitempty"` // Endianness of the measurement (big, little)
	Scaling    float64 `yaml:"scaling,omitempty"`    // Scaling factor for the measurement (optional)
}

// TelemetryPacket represents information about a telemetry packet received over Ethernet.
type TelemetryPacket struct {
	Name         string   `yaml:"name"`         // Name of the telemetry packet
	Port         int      `yaml:"port"`         // Port number for the telemetry packet
	Measurements []string `yaml:"measurements"` // List of measurements in the telemetry packet
}

// InterpretUnsignedInteger interprets a byte slice as an unsigned integer.
// The endianness parameter specifies the byte order of the data.
// Size of the data must be 1, 2, 4, or 8 bytes.
func InterpretUnsignedInteger(data []byte, endianness string) (interface{}, error) {
	switch len(data) {
	case 1:
		return data[0], nil
	case 2:
		if endianness == "little" {
			return binary.LittleEndian.Uint16(data), nil
		}
		return binary.BigEndian.Uint16(data), nil
	case 4:
		if endianness == "little" {
			return binary.LittleEndian.Uint32(data), nil
		}
		return binary.BigEndian.Uint32(data), nil
	case 8:
		if endianness == "little" {
			return binary.LittleEndian.Uint64(data), nil
		}
		return binary.BigEndian.Uint64(data), nil
	default:
		return nil, fmt.Errorf("unsupported data length: %d", len(data))
	}
	// TODO: Support non-aligned bytes less than 8?
}

// InterpretSignedInteger interprets a byte slice as a signed integer.
// The endianness parameter specifies the byte order of the data.
// Size of the data must be 1, 2, 4, or 8 bytes.
func InterpretSignedInteger(data []byte, endianness string) (interface{}, error) {
	unsigned, err := InterpretUnsignedInteger(data, endianness)
	if err != nil {
		return nil, err
	}

	switch v := unsigned.(type) {
	case uint8:
		return int8(v), nil
	case uint16:
		return int16(v), nil
	case uint32:
		return int32(v), nil
	case uint64:
		return int64(v), nil
	default:
		return nil, fmt.Errorf("unsupported integer type for signed conversion: %T", v)
	}
}

// InterpretFloat interprets a byte slice as a floating point number.
// The endianness parameter specifies the byte order of the data.
// Size of the data must be 4 or 8 bytes.
func InterpretFloat(data []byte, endianness string) (interface{}, error) {
	unsigned, err := InterpretUnsignedInteger(data, endianness)
	if err != nil {
		return nil, err
	}

	switch v := unsigned.(type) {
	case uint32:
		return math.Float32frombits(v), nil
	case uint64:
		return math.Float64frombits(v), nil
	default:
		return nil, fmt.Errorf("unsupported type for float conversion: %T", v)
	}
}

// InterpretMeasurementValue interprets a byte slice as a value for a measurement.
// The measurement parameter specifies the type and endianness of the data.
// The function returns the interpreted value and an error if the interpretation fails.
func InterpretMeasurementValue(measurement Measurement, data []byte) (interface{}, error) {
	switch measurement.Type {
	case "int":
		if measurement.Unsigned {
			return InterpretUnsignedInteger(data, measurement.Endianness)
		}
		return InterpretSignedInteger(data, measurement.Endianness)
	case "float":
		return InterpretFloat(data, measurement.Endianness)
	default:
		return nil, fmt.Errorf("unsupported type for measurement: %s", measurement.Type)
	}
}

// InterpretMeasurementValueString interprets a byte slice as a value for a measurement and returns a string representation.
// The measurement parameter specifies the type and endianness of the data.
// The function returns the interpreted value as a string and an error if the interpretation fails.
func InterpretMeasurementValueString(measurement Measurement, data []byte) (string, error) {
	switch measurement.Type {
	case "int":
		if measurement.Unsigned {
			measurementValue, err := InterpretUnsignedInteger(data, measurement.Endianness)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d", measurementValue), nil
		}

		measurementValue, err := InterpretSignedInteger(data, measurement.Endianness)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("%d", measurementValue), nil
	case "float":
		measurementValue, err := InterpretFloat(data, measurement.Endianness)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%f", measurementValue), nil
	default:
		return "", fmt.Errorf("unsupported type for measurement: %s", measurement.Type)
	}
}

// String returns a string representation of the measurement.
func (m Measurement) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Name: %s, Size: %d", m.Name, m.Size))
	if m.Type != "" {
		sb.WriteString(fmt.Sprintf(", Type: %s", m.Type))
	}

	if m.Unsigned {
		sb.WriteString(", Unsigned")
	} else {
		sb.WriteString(", Signed")
	}
	sb.WriteString(fmt.Sprintf(", Endianness: %s", m.Endianness))
	return sb.String()
}
