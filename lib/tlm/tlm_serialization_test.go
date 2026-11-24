package tlm

import (
	"bytes"
	"testing"
)

func TestSerializeUnsignedInteger(t *testing.T) {
	tests := []struct {
		name       string
		value      interface{}
		size       int
		endianness string
		expected   []byte
		wantErr    bool
	}{
		{"uint8", uint8(0x12), 1, "", []byte{0x12}, false},
		{"uint16 little endian", uint16(0x3412), 2, "little", []byte{0x12, 0x34}, false},
		{"uint16 big endian", uint16(0x1234), 2, "big", []byte{0x12, 0x34}, false},
		{"uint32 little endian", uint32(0x78563412), 4, "little", []byte{0x12, 0x34, 0x56, 0x78}, false},
		{"uint32 big endian", uint32(0x12345678), 4, "big", []byte{0x12, 0x34, 0x56, 0x78}, false},
		{"uint64 little endian", uint64(0xF0DEBC9A78563412), 8, "little", []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0}, false},
		{"uint64 big endian", uint64(0x123456789ABCDEF0), 8, "big", []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0}, false},
		{"unsupported length", uint64(0x12345678), 16, "", nil, true},
		{"zero length", uint8(0x12), 0, "", nil, true},
		{"unsupported type", "invalid", 4, "little", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SerializeUnsignedInteger(tt.value, tt.size, tt.endianness)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error state: got err=%v, wantErr=%v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSerializeSignedInteger(t *testing.T) {
	tests := []struct {
		name       string
		value      interface{}
		size       int
		endianness string
		expected   []byte
		wantErr    bool
	}{
		{"int8", int8(-126), 1, "", []byte{0x82}, false},
		{"int16 little endian", int16(-126), 2, "little", []byte{0x82, 0xFF}, false},
		{"int16 big endian", int16(-126), 2, "big", []byte{0xFF, 0x82}, false},
		{"int32 little endian", int32(-126), 4, "little", []byte{0x82, 0xFF, 0xFF, 0xFF}, false},
		{"int32 big endian", int32(-126), 4, "big", []byte{0xFF, 0xFF, 0xFF, 0x82}, false},
		{"int64 little endian", int64(-126), 8, "little", []byte{0x82, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, false},
		{"int64 big endian", int64(-126), 8, "big", []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x82}, false},
		{"unsupported length", int64(-126), 16, "", nil, true},
		{"unsupported type", float32(1.0), 4, "little", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SerializeSignedInteger(tt.value, tt.size, tt.endianness)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error state: got err=%v, wantErr=%v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSerializeFloat(t *testing.T) {
	tests := []struct {
		name       string
		value      interface{}
		size       int
		endianness string
		expected   []byte
		wantErr    bool
	}{
		{"float32 little endian", float32(1.0), 4, "little", []byte{0x00, 0x00, 0x80, 0x3F}, false},
		{"float32 big endian", float32(1.0), 4, "big", []byte{0x3F, 0x80, 0x00, 0x00}, false},
		{"float64 little endian", 1.0, 8, "little", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F}, false},
		{"float64 big endian", 1.0, 8, "big", []byte{0x3F, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, false},
		{"unsupported length", 1.0, 16, "", nil, true},
		{"unsupported type", "1.0", 4, "little", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SerializeFloat(tt.value, tt.size, tt.endianness)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error state: got err=%v, wantErr=%v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSerializeMeasurementValue(t *testing.T) {
	tests := []struct {
		name        string
		measurement Measurement
		value       interface{}
		expected    []byte
		wantErr     bool
	}{
		{"unsigned int", Measurement{Size: 1, Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 1.0}, uint8(0x12), []byte{0x12}, false},
		{"signed int", Measurement{Size: 1, Type: "int", Unsigned: false, Endianness: "little", ScalingFactor: 1.0}, int8(-126), []byte{0x82}, false},
		{"float", Measurement{Size: 4, Type: "float", Endianness: "little", ScalingFactor: 1.0}, float32(1.0), []byte{0x00, 0x00, 0x80, 0x3F}, false},
		{"unsupported type", Measurement{Size: 1, Type: "string", Endianness: "little", ScalingFactor: 1.0}, uint8(0x12), nil, true},
		{"scaled unsigned int", Measurement{Size: 2, Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 0.5}, float64(5.0), []byte{0x0A, 0x00}, false},
		{"scaled signed int", Measurement{Size: 2, Type: "int", Unsigned: false, Endianness: "big", ScalingFactor: 0.1}, float64(-12.6), []byte{0xFF, 0x82}, false},
		{"scaled float32", Measurement{Size: 4, Type: "float", Endianness: "little", ScalingFactor: 2.0}, float64(2.0), []byte{0x00, 0x00, 0x80, 0x3F}, false},
		{"zero scaling factor", Measurement{Size: 2, Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 0.0}, float64(5.0), nil, true},
		{"negative scaled unsigned int", Measurement{Size: 2, Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 0.5}, float64(-5.0), nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SerializeMeasurementValue(tt.measurement, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error state: got err=%v, wantErr=%v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSerializeUnalignedMeasurementValue(t *testing.T) {
	tests := []struct {
		name        string
		measurement Measurement
		value       interface{}
		expected    []byte
	}{
		{"unsigned 3 byte", Measurement{Size: 3, Type: "int", Unsigned: true, Endianness: "big", ScalingFactor: 1.0}, uint32(0x010203), []byte{0x01, 0x02, 0x03}},
		{"signed 3 byte", Measurement{Size: 3, Type: "int", Unsigned: false, Endianness: "big", ScalingFactor: 1.0}, int32(-2183681), []byte{0xDE, 0xAD, 0xFF}},

		{"signed 5 byte", Measurement{Size: 5, Type: "int", Unsigned: false, Endianness: "little", ScalingFactor: 1.0}, int64(0x0504030201), []byte{0x01, 0x02, 0x03, 0x04, 0x05}},
		{"unsigned 5 byte", Measurement{Size: 5, Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 1.0}, uint64(0xFFEFBEADDE), []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xFF}},

		{"signed 6 byte", Measurement{Size: 6, Type: "int", Unsigned: false, Endianness: "big", ScalingFactor: 1.0}, int64(0x010203040506), []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}},
		{"unsigned 6 byte", Measurement{Size: 6, Type: "int", Unsigned: true, Endianness: "big", ScalingFactor: 1.0}, uint64(0xDEADBEEFFEED), []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xFE, 0xED}},

		{"signed 7 byte", Measurement{Size: 7, Type: "int", Unsigned: false, Endianness: "little", ScalingFactor: 1.0}, int64(0x07060504030201), []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}},
		{"unsigned 7 byte", Measurement{Size: 7, Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 1.0}, uint64(0xD0BABBBEBAFECA), []byte{0xCA, 0xFE, 0xBA, 0xBE, 0xBB, 0xBA, 0xD0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SerializeMeasurementValue(tt.measurement, tt.value)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSerializeMeasurementValueString(t *testing.T) {
	tests := []struct {
		name        string
		measurement Measurement
		valueStr    string
		expected    []byte
		wantErr     bool
	}{
		{"unsigned int", Measurement{Size: 2, Type: "int", Unsigned: true, Endianness: "big", ScalingFactor: 1.0}, "4660", []byte{0x12, 0x34}, false},
		{"signed int", Measurement{Size: 2, Type: "int", Unsigned: false, Endianness: "little", ScalingFactor: 1.0}, "-126", []byte{0x82, 0xFF}, false},
		{"float32", Measurement{Size: 4, Type: "float", Endianness: "big", ScalingFactor: 1.0}, "1.0", []byte{0x3F, 0x80, 0x00, 0x00}, false},
		{"invalid int string", Measurement{Size: 2, Type: "int", Unsigned: true, Endianness: "big", ScalingFactor: 1.0}, "not_a_number", nil, true},
		{"unsupported type", Measurement{Size: 1, Type: "string", Endianness: "little", ScalingFactor: 1.0}, "123", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SerializeMeasurementValueString(tt.measurement, tt.valueStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error state: got err=%v, wantErr=%v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		measurement Measurement
		data        []byte
	}{
		{"uint16 little", Measurement{Size: 2, Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 1.0}, []byte{0x12, 0x34}},
		{"int32 big", Measurement{Size: 4, Type: "int", Unsigned: false, Endianness: "big", ScalingFactor: 1.0}, []byte{0xFF, 0xFF, 0xFF, 0x82}},
		{"int 3 byte big", Measurement{Size: 3, Type: "int", Unsigned: false, Endianness: "big", ScalingFactor: 1.0}, []byte{0xDE, 0xAD, 0xFF}},
		{"float64 little", Measurement{Size: 8, Type: "float", Endianness: "little", ScalingFactor: 1.0}, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F}},
		{"scaled uint16", Measurement{Size: 2, Type: "int", Unsigned: true, Endianness: "little", ScalingFactor: 0.25}, []byte{0x10, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := InterpretMeasurementValue(tt.measurement, tt.data)
			if err != nil {
				t.Fatalf("interpret failed: %v", err)
			}

			out, err := SerializeMeasurementValue(tt.measurement, val)
			if err != nil {
				t.Fatalf("serialize failed: %v", err)
			}

			if !bytes.Equal(out, tt.data) {
				t.Errorf("round-trip mismatch: expected %v, got %v", tt.data, out)
			}
		})
	}
}
