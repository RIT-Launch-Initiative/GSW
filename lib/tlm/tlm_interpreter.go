package tlm

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

type Measurement struct {
	Name       string `yaml:"name"`
	Size       int    `yaml:"size"`
	Type       string `yaml:"type,omitempty"`
	Unsigned   bool   `yaml:"unsigned,omitempty"`
	Endianness string `yaml:"endianness,omitempty"`
}

type TelemetryPacket struct {
	Name         string   `yaml:"name"`
	Port         int      `yaml:"port"`
	Measurements []string `yaml:"measurements"`
}

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
		fmt.Printf("Unsupported data length: %d\n", len(data))
		return nil, fmt.Errorf("unsupported data length: %d", len(data))
	}
	// TODO: Support non-aligned bytes less than 8?
}

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
