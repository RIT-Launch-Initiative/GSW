package util

import (
	"fmt"
	"strings"
)

// Base2String returns a string representation of the byte slice in base 2
func Base2String(b []byte, groupSize int) string {
	return formatBytes(b, 2, groupSize, true)
}

// Base16String returns a string representation of the byte slice in base 16
func Base16String(b []byte, groupSize int) string {
	return formatBytes(b, 16, groupSize, true)
}

// Base2StringNoHeader returns a string representation of the byte slice in base 2 without the header
func Base2StringNoHeader(b []byte, groupSize int) string {
	return formatBytes(b, 2, groupSize, false)
}

// Base16StringNoHeader returns a string representation of the byte slice in base 16 without the header
func Base16StringNoHeader(b []byte, groupSize int) string {
	return formatBytes(b, 16, groupSize, false)
}

// formatBytes formats the byte slice into a string representation
// base can only be 2 or 16
// groupSize is the number of bytes to group together
// includePrefix determines whether to include the prefix (0b or 0x)
func formatBytes(b []byte, base int, groupSize int, includePrefix bool) string {
	var result strings.Builder
	var prefix string
	var formatString string

	switch base {
	case 2:
		prefix = "0b"
		formatString = fmt.Sprintf("%%0%db", groupSize)
	case 16:
		prefix = "0x"
		formatString = fmt.Sprintf("%%0%dX", groupSize)
	default:
		panic("Unsupported base")
	}

	bytesForNumWritten := 0
	for _, byteValue := range b {
		if bytesForNumWritten >= groupSize {
			result.WriteString(" ")
			bytesForNumWritten = 0

		}

		if bytesForNumWritten == 0 && includePrefix {
			result.WriteString(prefix)
		}

		bytesForNumWritten, _ = result.WriteString(fmt.Sprintf(formatString, byteValue))
	}

	return result.String()
}
