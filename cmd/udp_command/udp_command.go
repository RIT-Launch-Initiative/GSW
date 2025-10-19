package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Prints usage info for the application.
func printHelpMessage() {
	fmt.Println()
	fmt.Println("Specify the bytes of the payload to send by entering a list of integers separated by spaces.")
	fmt.Println("Integers can be in base 10, binary (start with 0b), or hex (start with 0x).")
	fmt.Println("To toggle between ASCII and byte modes, type 'mode'.")
	fmt.Println("In ASCII mode, text will be sent directly as bytes.")
	fmt.Println("In ASCII mode, you can use escape sequences like \\x00 to include specific byte values.")
	fmt.Println("To resend the most recent payload, press the up arrow followed by Enter.")
	fmt.Println("To view the history of previously sent payloads, type 'h'.")
	fmt.Println("h <#> - resends the payload with the given history item number.")
	fmt.Println()
}

// "Opens a connection" over UDP with the specified host and port.
func openConn(host string, port int) (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%v:%v", host, port))
	if err != nil {
		return nil, fmt.Errorf("error resolving UDP address: %v", err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("error connecting to UDP address: %v", err)
	}
	return conn, nil
}

// Sends the given packet over the given connection.
func sendOverUDP(conn *net.UDPConn, packet []byte) error {
	_, err := conn.Write(packet)
	if err != nil {
		return err
	}
	return nil
}

// Prompts for user input specifying the host and port to send payloads to.
// Returns the host and port.
func promptConnInfo(scanner *bufio.Scanner) (string, int, error) {
	fmt.Print("IP address: ")
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return "", -1, fmt.Errorf("error reading IP address: %v", err)
	}
	host := scanner.Text()
	fmt.Print("Port: ")
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return "", -1, fmt.Errorf("error reading port: %v", err)
	}
	port := scanner.Text()

	// Parse port to int
	parsedPort, err := strconv.ParseInt(port, 0, 0)
	if err != nil {
		return "", -1, fmt.Errorf("error parsing port: %v", err)
	}
	return host, int(parsedPort), nil
}

// Parses ASCII escape sequences in the input string and returns the corresponding byte array
func parseAsciiEscapeSequences(input string) ([]byte, error) {
	// In ASCII mode, process escape sequences
	result := []byte{}
	for i := 0; i < len(input); i++ {
		if input[i] == '\\' && i+1 < len(input) {
			if i+3 < len(input) && input[i+1] == 'x' {
				// Handle \xHH escape sequence
				if hexByte, err := strconv.ParseUint(input[i+2:i+4], 16, 8); err == nil {
					result = append(result, byte(hexByte))
					i += 3 // Skip the next 3 characters (\xHH)
					continue
				}
			}
			// Handle other escape sequences
			switch input[i+1] {
			case 'n':
				result = append(result, '\n')
			case 'r':
				result = append(result, '\r')
			case 't':
				result = append(result, '\t')
			case '\\':
				result = append(result, '\\')
			case '0':
				result = append(result, 0)
			default:
				// If not a recognized escape, just add the character
				result = append(result, input[i+1])
			}
			i++ // Skip the next character (the one after \)
		} else {
			// Regular character
			result = append(result, input[i])
		}
	}
	return result, nil
}

// Prompts for user input, either a payload or other command.
// Returns the payload (if one was given) formatted as a byte array.
func promptInput(scanner *bufio.Scanner, history [][]byte, isAsciiMode bool) ([]byte, error) {
	fmt.Print("Payload: ")
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning input: %v", err)
	}

	input := scanner.Text()
	inputTokens := strings.Fields(input)
	if len(input) == 0 {
		return nil, fmt.Errorf("no input")
	}

	// Print help message
	if input == "help" {
		printHelpMessage()
		return nil, nil
	}

	// Toggle mode command
	if input == "mode" {
		return []byte("__MODE_TOGGLE__"), nil
	}

	// Check for history commands
	if strings.HasPrefix(input, "h") && (len(input) == 1 || input[1] == ' ') {
		inputTokens = strings.Fields(input)
		if len(inputTokens) == 1 {
			// print history
			if len(history) == 0 {
				fmt.Println("History is empty.")
			} else {
				fmt.Println("History:")
			}
			for i, payload := range history {
				if len(payload) > 6 {
					truncatedPayloadStr := fmt.Sprintf("[%#x %#x %#x ... %#x %#x %#x]", payload[0], payload[1],
						payload[2], payload[len(payload)-3], payload[len(payload)-2], payload[len(payload)-1])
					fmt.Printf("\t%d\t%v\n", i+1, truncatedPayloadStr)
				} else {
					fmt.Printf("\t%d\t%# x\n", i+1, payload)
				}
			}
			return nil, nil
		} else if len(inputTokens) == 2 {
			// resend a previous payload
			index, err := strconv.ParseInt(inputTokens[1], 10, 0)
			index-- // history indexing starts at 1
			if err != nil {
				return nil, fmt.Errorf("error parsing history number: %v", err)
			}
			if index < 0 || int(index) >= len(history) {
				return nil, fmt.Errorf("invalid history number: %v", index+1)
			}
			return history[index], nil
		} else {
			// print usage info
			return nil, fmt.Errorf("usage: h [item number]")
		}
	} else if input == "\x1b[A" { // read up arrow
		// resend most recent payload
		if len(history) == 0 {
			fmt.Println("History is empty.")
			return nil, nil
		}
		return history[len(history)-1], nil
	}

	// Process payload based on current mode
	if isAsciiMode {
		return parseAsciiEscapeSequences(input)
	} else {
		// Byte mode - original behavior
		// Loop adding byte from input line to payload
		payload := make([]byte, 0, 10)
		for _, str := range inputTokens {
			parsedInt64, err := strconv.ParseUint(str, 0, 64)
			if err != nil {
				return nil, fmt.Errorf("error parsing payload: %v", err)
			}
			byteBuffer := make([]byte, 8)
			binary.BigEndian.PutUint64(byteBuffer, parsedInt64)
			payload = append(payload, bytes.TrimLeft(byteBuffer, "\x00")...)
		}
		return payload, nil
	}
}

// Loop prompting for input.
func mainInputLoop(scanner *bufio.Scanner, conn *net.UDPConn) {
	history := make([][]byte, 0, 20)
	isAsciiMode := false
	for {
		// Show mode in prompt
		if isAsciiMode {
			fmt.Print("Mode: ASCII\n")
		} else {
			fmt.Print("Mode: Bytes\n")
		}

		payload, err := promptInput(scanner, history, isAsciiMode)
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}
		if payload == nil { // no payload given
			continue
		}

		// Handle mode toggle command
		if bytes.Equal(payload, []byte("__MODE_TOGGLE__")) {
			isAsciiMode = !isAsciiMode
			fmt.Printf("Switched to %s mode\n", map[bool]string{true: "ASCII", false: "Bytes"}[isAsciiMode])
			continue
		}

		// Add payload to history (if not repeat of most recent payload)
		if len(history) == 0 || !slices.Equal(payload, history[len(history)-1]) {
			history = append(history, payload)
		}
		// Send payload over connection
		if isAsciiMode {
			fmt.Printf("\tSending ASCII: '%s'\n", payload)
			fmt.Printf("\tAs bytes: %# x\n", payload)
		} else {
			fmt.Printf("\tSending payload %# x\n", payload)
		}
		if err := sendOverUDP(conn, payload); err != nil {
			fmt.Println("\tError sending payload:", err)
		}
	}
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	// Read in host and port, then open connection
	host, port, err := promptConnInfo(scanner)
	if err != nil {
		fmt.Println("Error reading connection info:", err)
		return
	}
	conn, err := openConn(host, port)
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return
	}
	fmt.Print("Connection opened.\n\n")
	fmt.Println("** For usage info, type 'help'. **")
	mainInputLoop(scanner, conn)
}
