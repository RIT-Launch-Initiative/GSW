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

// Prompts for user input over stdin, either a payload or other command.
// Returns the payload (if one was given) formatted as a byte array.
func promptInput(scanner *bufio.Scanner, history [][]byte) ([]byte, error) {
	fmt.Print("Payload: ")
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning input: %v", err)
	}
	inputTokens := strings.Fields(scanner.Text())
	if len(inputTokens) == 0 {
		return nil, fmt.Errorf("no input")
	}

	// Check for history commands
	if inputTokens[0] == "h" {
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
	} else if inputTokens[0] == "\x1b[A" && len(inputTokens) == 1 { // read up arrow
		// resend most recent payload
		if len(history) == 0 {
			fmt.Println("History is empty.")
			return nil, nil
		}
		return history[len(history)-1], nil
	}

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

func main() {
	var host string
	var port string
	scanner := bufio.NewScanner(os.Stdin)

	// Read in host and port
	fmt.Print("IP address: ")
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading IP address:", err)
		return
	}
	host = scanner.Text()
	fmt.Print("Port: ")
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading port:", err)
		return
	}
	port = scanner.Text()

	// Parse port to int + open connection
	parsedPort, err := strconv.ParseInt(port, 0, 0)
	if err != nil {
		fmt.Println("Error parsing port:", err)
		return
	}
	conn, err := openConn(host, int(parsedPort))
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return
	}
	fmt.Print("Connection opened.\n\n")

	// Loop prompting for input
	history := make([][]byte, 0, 20)
	for {
		payload, err := promptInput(scanner, history)
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}
		if payload == nil { // no payload given
			continue
		}
		// Add payload to history (if not repeat of most recent payload)
		if len(history) == 0 || !slices.Equal(payload, history[len(history)-1]) {
			history = append(history, payload)
		}

		// Send payload over connection
		fmt.Printf("\tSending payload %# x\n", payload)
		if err := sendOverUDP(conn, payload); err != nil {
			fmt.Println("\tError sending payload:", err)
		}
	}
}
