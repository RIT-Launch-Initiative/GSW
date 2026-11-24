package tlm

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

// SerializeUnsignedInteger serializes an unsigned integer into a byte slice.
// The size parameter specifies the output length (1-8 bytes) and endianness specifies the byte order.
func SerializeUnsignedInteger(value interface{}, size int, endianness string) ([]byte, error) {
	if size < 1 || size > 8 {
		return nil, fmt.Errorf("unsupported size for unsigned integer: %d bytes", size)
	}

	var val uint64
	switch v := value.(type) {
	case uint8:
		val = uint64(v)
	case uint16:
		val = uint64(v)
	case uint32:
		val = uint64(v)
	case uint64:
		val = v
	case uint:
		val = uint64(v)
	default:
		return nil, fmt.Errorf("unsupported type for unsigned integer: %T", value)
	}

	data := make([]byte, size)
	switch size {
	case 1:
		data[0] = byte(val)
	case 2:
		if endianness == "little" {
			binary.LittleEndian.PutUint16(data, uint16(val))
		} else {
			binary.BigEndian.PutUint16(data, uint16(val))
		}
	case 4:
		if endianness == "little" {
			binary.LittleEndian.PutUint32(data, uint32(val))
		} else {
			binary.BigEndian.PutUint32(data, uint32(val))
		}
	case 8:
		if endianness == "little" {
			binary.LittleEndian.PutUint64(data, val)
		} else {
			binary.BigEndian.PutUint64(data, val)
		}
	default:
		if endianness == "little" {
			for i := 0; i < size; i++ {
				data[i] = byte(val >> (8 * i))
			}
		} else {
			for i := 0; i < size; i++ {
				data[i] = byte(val >> (8 * (size - 1 - i)))
			}
		}
	}

	return data, nil
}

// SerializeSignedInteger serializes a signed integer into a byte slice.
// The size parameter specifies the output length (1-8 bytes) and endianness specifies the byte order.
func SerializeSignedInteger(value interface{}, size int, endianness string) ([]byte, error) {
	var val int64
	switch v := value.(type) {
	case int8:
		val = int64(v)
	case int16:
		val = int64(v)
	case int32:
		val = int64(v)
	case int64:
		val = v
	case int:
		val = int64(v)
	default:
		return nil, fmt.Errorf("unsupported integer type for signed conversion: %T", value)
	}

	return SerializeUnsignedInteger(uint64(val), size, endianness)
}

// SerializeFloat serializes a floating point number into a byte slice.
// Size must be 4 (float32) or 8 (float64) bytes.
func SerializeFloat(value interface{}, size int, endianness string) ([]byte, error) {
	var f64 float64
	switch v := value.(type) {
	case float32:
		f64 = float64(v)
	case float64:
		f64 = v
	default:
		return nil, fmt.Errorf("unsupported type for float conversion: %T", value)
	}

	switch size {
	case 4:
		return SerializeUnsignedInteger(math.Float32bits(float32(f64)), 4, endianness)
	case 8:
		return SerializeUnsignedInteger(math.Float64bits(f64), 8, endianness)
	default:
		return nil, fmt.Errorf("unsupported size for float: %d bytes", size)
	}
}

// SerializeMeasurementValue serializes a value into a byte slice according to the measurement specification.
// If ScalingFactor is not 1.0, the value is divided by ScalingFactor before serialization.
func SerializeMeasurementValue(measurement Measurement, value interface{}) ([]byte, error) {
	if measurement.ScalingFactor != 1.0 {
		if measurement.ScalingFactor == 0 {
			return nil, fmt.Errorf("scaling factor cannot be zero")
		}

		var f64 float64
		switch v := value.(type) {
		case int:
			f64 = float64(v)
		case int8:
			f64 = float64(v)
		case int16:
			f64 = float64(v)
		case int32:
			f64 = float64(v)
		case int64:
			f64 = float64(v)
		case uint:
			f64 = float64(v)
		case uint8:
			f64 = float64(v)
		case uint16:
			f64 = float64(v)
		case uint32:
			f64 = float64(v)
		case uint64:
			f64 = float64(v)
		case float32:
			f64 = float64(v)
		case float64:
			f64 = v
		default:
			return nil, fmt.Errorf("unsupported type for scaling: %T", value)
		}

		unscaled := f64 / measurement.ScalingFactor
		switch measurement.Type {
		case "int":
			if measurement.Unsigned {
				if unscaled < 0 {
					return nil, fmt.Errorf("cannot serialize negative value %f as unsigned integer", unscaled)
				}
				value = uint64(math.Round(unscaled))
			} else {
				value = int64(math.Round(unscaled))
			}
		case "float":
			value = unscaled
		}
	}

	switch measurement.Type {
	case "int":
		if measurement.Unsigned {
			return SerializeUnsignedInteger(value, measurement.Size, measurement.Endianness)
		}
		return SerializeSignedInteger(value, measurement.Size, measurement.Endianness)
	case "float":
		return SerializeFloat(value, measurement.Size, measurement.Endianness)
	default:
		return nil, fmt.Errorf("unsupported type for measurement: %s", measurement.Type)
	}
}

// SerializeMeasurementValueString parses a string representation of a measurement value and serializes it into a byte slice.
func SerializeMeasurementValueString(measurement Measurement, valueStr string) ([]byte, error) {
	switch measurement.Type {
	case "int":
		if measurement.Unsigned {
			val, err := strconv.ParseUint(valueStr, 10, 64)
			if err != nil {
				return nil, err
			}
			return SerializeUnsignedInteger(val, measurement.Size, measurement.Endianness)
		}

		val, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			return nil, err
		}
		return SerializeSignedInteger(val, measurement.Size, measurement.Endianness)
	case "float":
		val, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return nil, err
		}
		return SerializeFloat(val, measurement.Size, measurement.Endianness)
	default:
		return nil, fmt.Errorf("unsupported type for measurement: %s", measurement.Type)
	}
}
