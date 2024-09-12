package tlm

import (
	"encoding/binary"
	"fmt"
	"github.com/AarC10/GSW-V2/proc"
	"math"
)

func InterpretUnsignedInteger(data []byte, endianness string) interface{} {
	switch len(data) {
	case 1:
		return data[0]
	case 2:
		if endianness == "little" {
			return binary.LittleEndian.Uint16(data)
		}
		return binary.BigEndian.Uint16(data)
	case 4:
		if endianness == "little" {
			return binary.LittleEndian.Uint32(data)
		}
		return binary.BigEndian.Uint32(data)
	case 8:
		if endianness == "little" {
			return binary.LittleEndian.Uint64(data)
		}
		return binary.BigEndian.Uint64(data)
	default:
		fmt.Printf("Unsupported data length: %d\n", len(data))
		return nil
	}
	// TODO: Support non-aligned bytes less than 8?
}

func InterpretSignedInteger(data []byte, endianness string) interface{} {
	unsigned := InterpretUnsignedInteger(data, endianness)

	switch v := unsigned.(type) {
	case uint8:
		return int8(v)
	case uint16:
		return int16(v)
	case uint32:
		return int32(v)
	case uint64:
		return int64(v)
	default:
		fmt.Printf("Unsupported unsigned integer type: %T\n", v)
		return nil
	}
}

func InterpretFloat(data []byte, endianness string) interface{} {
	unsigned := InterpretUnsignedInteger(data, endianness)

	switch v := unsigned.(type) {
	case uint32:
		return math.Float32frombits(v)
	case uint64:
		return math.Float64frombits(v)
	default:
		fmt.Printf("Unsupported unsigned integer type for float conversion: %T\n", v)
		return nil
	}
}

func InterpretMeasurementValue(measurement proc.Measurement, data []byte) interface{} {
	switch measurement.Type {
	case "int":
		if measurement.Unsigned {
			return InterpretUnsignedInteger(data, measurement.Endianness)
		}
		return InterpretSignedInteger(data, measurement.Endianness)
	case "float":
		return InterpretFloat(data, measurement.Endianness)
	default:
		fmt.Printf("Unsupported type for measurement: %s\n", measurement.Type)
		return nil
	}
}

func InterpretMeasurementValueString(measurement proc.Measurement, data []byte) string {
	switch measurement.Type {
	case "int":
		if measurement.Unsigned {
			return fmt.Sprintf("%d", InterpretUnsignedInteger(data, measurement.Endianness))
		}
		return fmt.Sprintf("%d", InterpretSignedInteger(data, measurement.Endianness))
	case "float":
		return fmt.Sprintf("%f", InterpretFloat(data, measurement.Endianness))
	default:
		fmt.Printf("Unsupported type for measurement: %s\n", measurement.Type)
		return ""
	}
}
