package tlm

import (
	"testing"
)

func TestInterpretUnsignedInteger(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		endianness string
		expected   interface{}
	}{
		{"uint8", []byte{0x12}, "", uint8(0x12)},
		{"uint16 little endian", []byte{0x12, 0x34}, "little", uint16(0x3412)},
		{"uint16 big endian", []byte{0x12, 0x34}, "big", uint16(0x1234)},
		{"uint32 little endian", []byte{0x12, 0x34, 0x56, 0x78}, "little", uint32(0x78563412)},
		{"uint32 big endian", []byte{0x12, 0x34, 0x56, 0x78}, "big", uint32(0x12345678)},
		{"uint64 little endian", []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0}, "little", uint64(0xF0DEBC9A78563412)},
		{"uint64 big endian", []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0}, "big", uint64(0x123456789ABCDEF0)},
		{"unsupported length", []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xAB, 0xCD, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE, 0xDE, 0xAD, 0xBE, 0xE5}, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := InterpretUnsignedInteger(tt.data, tt.endianness)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestInterpretSignedInteger(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		endianness string
		expected   interface{}
	}{
		{"int8", []byte{0x82}, "", int8(-126)},
		{"int16 little endian", []byte{0x82, 0xFF}, "little", int16(-126)},
		{"int16 big endian", []byte{0xFF, 0x82}, "big", int16(-126)},
		{"int32 little endian", []byte{0x82, 0xFF, 0xFF, 0xFF}, "little", int32(-126)},
		{"int32 big endian", []byte{0xFF, 0xFF, 0xFF, 0x82}, "big", int32(-126)},
		{"int64 little endian", []byte{0x82, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, "little", int64(-126)},
		{"int64 big endian", []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x82}, "big", int64(-126)},
		{"unsupported length", []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xAB, 0xCD, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE, 0xDE, 0xAD, 0xBE, 0xE5}, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := InterpretSignedInteger(tt.data, tt.endianness)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestInterpretFloat(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		endianness string
		expected   interface{}
	}{
		{"float32 little endian", []byte{0x00, 0x00, 0x80, 0x3F}, "little", float32(1.0)},
		{"float32 big endian", []byte{0x3F, 0x80, 0x00, 0x00}, "big", float32(1.0)},
		{"float64 little endian", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F}, "little", 1.0},
		{"float64 big endian", []byte{0x3F, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "big", 1.0},
		{"unsupported length", []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xAB, 0xCD, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE, 0xDE, 0xAD, 0xBE, 0xE5}, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := InterpretFloat(tt.data, tt.endianness)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestInterpretMeasurementValue(t *testing.T) {
	tests := []struct {
		name        string
		measurement Measurement
		data        []byte
		expected    interface{}
	}{
		{"unsigned int", Measurement{Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 1.0}, []byte{0x12}, uint8(0x12)},
		{"signed int", Measurement{Type: "int", Unsigned: false, Endianness: "little", ScalingFactor: 1.0}, []byte{0x82}, int8(-126)},
		{"float", Measurement{Type: "float", Endianness: "little", ScalingFactor: 1.0}, []byte{0x00, 0x00, 0x80, 0x3F}, float32(1.0)},
		{"unsupported type", Measurement{Type: "string", Endianness: "little", ScalingFactor: 1.0}, []byte{0x12}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := InterpretMeasurementValue(tt.measurement, tt.data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestInterpretUnalignedMeasurementValue(t *testing.T) {
	tests := []struct {
		name        string
		measurement Measurement
		data        []byte
		expected    interface{}
	}{
		{"unsigned 3 byte", Measurement{Type: "int", Unsigned: true, Endianness: "big", ScalingFactor: 1.0}, []byte{0x01, 0x02, 0x03}, uint32(0x010203)},
		{"signed 3 byte", Measurement{Type: "int", Unsigned: false, Endianness: "big", ScalingFactor: 1.0}, []byte{0xDE, 0xAD, 0xFF}, int32(0xDEADFF)},

		{"unsigned 5 byte", Measurement{Type: "int", Unsigned: false, Endianness: "little", ScalingFactor: 1.0}, []byte{0x01, 0x02, 0x03, 0x04, 0x05}, int64(0x0504030201)},
		{"signed 5 byte", Measurement{Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 1.0}, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xFF}, uint64(0xFFEFBEADDE)},

		{"unsigned 6 byte", Measurement{Type: "int", Unsigned: false, Endianness: "big", ScalingFactor: 1.0}, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, int64(0x010203040506)},
		{"signed 6 byte", Measurement{Type: "int", Unsigned: true, Endianness: "big", ScalingFactor: 1.0}, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xFE, 0xED}, uint64(0xDEADBEEFFEED)},

		{"unsigned 7 byte", Measurement{Type: "int", Unsigned: false, Endianness: "little", ScalingFactor: 1.0}, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, int64(0x07060504030201)},
		{"signed 7 byte", Measurement{Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 1.0}, []byte{0xCA, 0xFE, 0xBA, 0xBE, 0xBB, 0xBA, 0xD0}, uint64(0xD0BABBBEBAFECA)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := InterpretMeasurementValue(tt.measurement, tt.data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
