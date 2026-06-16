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
	"sync"
	"time"

	"golang.org/x/term"
)

// periodicState holds shared state between the input loop and the sender goroutine.
type periodicState struct {
	mu      sync.Mutex
	payload []byte
	paused  bool
}

func (ps *periodicState) setPayload(p []byte) {
	ps.mu.Lock()
	ps.payload = make([]byte, len(p))
	copy(ps.payload, p)
	ps.mu.Unlock()
}

func (ps *periodicState) togglePause() bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.paused = !ps.paused
	return ps.paused
}

func (ps *periodicState) isPaused() bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.paused
}

func runPeriodicSender(conn *net.UDPConn, interval time.Duration, state *periodicState) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		state.mu.Lock()
		paused := state.paused
		payload := make([]byte, len(state.payload))
		copy(payload, state.payload)
		state.mu.Unlock()
		if !paused && len(payload) > 0 {
			if _, err := conn.Write(payload); err != nil {
				fmt.Printf("\r\n\tPeriodic send error: %v\r\n", err)
			}
		}
	}
}

func printHelpMessage() {
	fmt.Print("\r\n")
	fmt.Print("Specify the bytes of the payload to send by entering a list of integers separated by spaces.\r\n")
	fmt.Print("Integers can be in base 10, binary (start with 0b), or hex (start with 0x).\r\n")
	fmt.Print("To toggle between ASCII and byte modes, type 'mode'.\r\n")
	fmt.Print("In ASCII mode, text will be sent directly as bytes.\r\n")
	fmt.Print("In ASCII mode, you can use escape sequences like \\x00 to include specific byte values.\r\n")
	fmt.Print("To resend the most recent payload, press the up arrow followed by Enter.\r\n")
	fmt.Print("To view the history of previously sent payloads, type 'h'.\r\n")
	fmt.Print("h <#> - resends the payload with the given history item number.\r\n")
	fmt.Print("Ctrl+P - pause/unpause periodic sending.\r\n")
	fmt.Print("\r\n")
}

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

func sendOverUDP(conn *net.UDPConn, packet []byte) error {
	_, err := conn.Write(packet)
	return err
}

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
	parsedPort, err := strconv.ParseInt(scanner.Text(), 0, 0)
	if err != nil {
		return "", -1, fmt.Errorf("error parsing port: %v", err)
	}
	return host, int(parsedPort), nil
}

func promptInterval(scanner *bufio.Scanner) (time.Duration, error) {
	fmt.Print("Periodic send interval (e.g. 1s, 500ms; 0 or blank to disable): ")
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading interval: %v", err)
	}
	text := strings.TrimSpace(scanner.Text())
	if text == "0" || text == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(text)
	if err != nil {
		return 0, fmt.Errorf("error parsing interval: %v", err)
	}
	return d, nil
}

func parseAsciiEscapeSequences(input string) ([]byte, error) {
	result := []byte{}
	for i := 0; i < len(input); i++ {
		if input[i] == '\\' && i+1 < len(input) {
			if i+3 < len(input) && input[i+1] == 'x' {
				if hexByte, err := strconv.ParseUint(input[i+2:i+4], 16, 8); err == nil {
					result = append(result, byte(hexByte))
					i += 3
					continue
				}
			}
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
				result = append(result, input[i+1])
			}
			i++
		} else {
			result = append(result, input[i])
		}
	}
	return result, nil
}

func parsePayload(input string, isAsciiMode bool) ([]byte, error) {
	if isAsciiMode {
		return parseAsciiEscapeSequences(input)
	}
	payload := make([]byte, 0, 10)
	for _, str := range strings.Fields(input) {
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

type lineKind int

const (
	lineNormal  lineKind = iota
	lineCtrlP            // Ctrl+P: toggle pause
	lineCtrlC            // Ctrl+C: quit
	lineUpArrow          // Up arrow: resend last payload
)

type lineResult struct {
	kind lineKind
	text string
}

func readByte() (byte, error) {
	b := make([]byte, 1)
	_, err := os.Stdin.Read(b)
	return b[0], err
}

// readRawLine reads a line in raw terminal mode, echoing characters manually.
// Returns when Enter, Ctrl+P, Ctrl+C, or an arrow key is pressed.
func readRawLine(prompt string) (lineResult, error) {
	fmt.Print(prompt)
	var buf []byte
	for {
		ch, err := readByte()
		if err != nil {
			return lineResult{}, err
		}
		switch ch {
		case 0x03: // Ctrl+C
			fmt.Print("\r\n")
			return lineResult{kind: lineCtrlC}, nil
		case 0x10: // Ctrl+P
			fmt.Print("\r\n")
			return lineResult{kind: lineCtrlP}, nil
		case 0x0d, 0x0a: // Enter
			fmt.Print("\r\n")
			return lineResult{kind: lineNormal, text: string(buf)}, nil
		case 0x7f, 0x08: // Backspace / DEL
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Print("\b \b")
			}
		case 0x1b: // ESC — possible arrow key sequence
			next, err := readByte()
			if err != nil {
				return lineResult{}, err
			}
			if next == '[' {
				arrow, err := readByte()
				if err != nil {
					return lineResult{}, err
				}
				if arrow == 'A' { // up arrow
					fmt.Print("\r\n")
					return lineResult{kind: lineUpArrow}, nil
				}
			}
		default:
			if ch >= 0x20 { // printable ASCII
				buf = append(buf, ch)
				fmt.Printf("%c", ch)
			}
		}
	}
}

func mainInputLoop(conn *net.UDPConn, periodic *periodicState) {
	history := make([][]byte, 0, 20)
	isAsciiMode := false

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Println("Warning: could not set raw terminal mode:", err)
	} else {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	sendPayload := func(payload []byte) {
		if isAsciiMode {
			fmt.Printf("\tSending ASCII: '%s'\r\n", payload)
			fmt.Printf("\tAs bytes: %# x\r\n", payload)
		} else {
			fmt.Printf("\tSending payload %# x\r\n", payload)
		}
		if err := sendOverUDP(conn, payload); err != nil {
			fmt.Printf("\tError sending payload: %v\r\n", err)
		}
		if periodic != nil {
			periodic.setPayload(payload)
		}
	}

	for {
		modeStr := map[bool]string{true: "ASCII", false: "Bytes"}[isAsciiMode]
		var statusStr string
		if periodic != nil {
			if periodic.isPaused() {
				statusStr = " [PAUSED]"
			} else {
				statusStr = " [SENDING]"
			}
		}
		prompt := fmt.Sprintf("Mode: %s%s\r\nPayload: ", modeStr, statusStr)

		result, err := readRawLine(prompt)
		if err != nil {
			fmt.Printf("\r\nError reading input: %v\r\n", err)
			continue
		}

		switch result.kind {
		case lineCtrlC:
			fmt.Print("Exiting.\r\n")
			return

		case lineCtrlP:
			if periodic == nil {
				fmt.Print("Periodic sending is disabled.\r\n")
			} else {
				if periodic.togglePause() {
					fmt.Print("Periodic sending paused.\r\n")
				} else {
					fmt.Print("Periodic sending resumed.\r\n")
				}
			}
			continue

		case lineUpArrow:
			if len(history) == 0 {
				fmt.Print("History is empty.\r\n")
				continue
			}
			sendPayload(history[len(history)-1])
			continue
		}

		input := result.text
		if len(input) == 0 {
			fmt.Print("No input.\r\n")
			continue
		}

		if input == "help" {
			printHelpMessage()
			continue
		}

		if input == "mode" {
			isAsciiMode = !isAsciiMode
			fmt.Printf("Switched to %s mode\r\n", map[bool]string{true: "ASCII", false: "Bytes"}[isAsciiMode])
			continue
		}

		if strings.HasPrefix(input, "h") && (len(input) == 1 || input[1] == ' ') {
			tokens := strings.Fields(input)
			if len(tokens) == 1 {
				if len(history) == 0 {
					fmt.Print("History is empty.\r\n")
				} else {
					fmt.Print("History:\r\n")
					for i, payload := range history {
						if len(payload) > 6 {
							fmt.Printf("\t%d\t[%#x %#x %#x ... %#x %#x %#x]\r\n", i+1,
								payload[0], payload[1], payload[2],
								payload[len(payload)-3], payload[len(payload)-2], payload[len(payload)-1])
						} else {
							fmt.Printf("\t%d\t%# x\r\n", i+1, payload)
						}
					}
				}
				continue
			} else if len(tokens) == 2 {
				index, err := strconv.ParseInt(tokens[1], 10, 0)
				index-- // history indexing starts at 1
				if err != nil {
					fmt.Printf("Error parsing history number: %v\r\n", err)
					continue
				}
				if index < 0 || int(index) >= len(history) {
					fmt.Printf("Invalid history number: %v\r\n", index+1)
					continue
				}
				sendPayload(history[index])
				continue
			}
			fmt.Print("Usage: h [item number]\r\n")
			continue
		}

		payload, err := parsePayload(input, isAsciiMode)
		if err != nil {
			fmt.Printf("Error reading input: %v\r\n", err)
			continue
		}

		if len(history) == 0 || !slices.Equal(payload, history[len(history)-1]) {
			history = append(history, payload)
		}
		sendPayload(payload)
	}
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	host, port, err := promptConnInfo(scanner)
	if err != nil {
		fmt.Println("Error reading connection info:", err)
		return
	}

	interval, err := promptInterval(scanner)
	if err != nil {
		fmt.Println("Error reading interval:", err)
		return
	}

	conn, err := openConn(host, port)
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return
	}
	fmt.Print("Connection opened.\n\n")
	fmt.Println("** For usage info, type 'help'. **")

	var periodic *periodicState
	if interval > 0 {
		periodic = &periodicState{}
		go runPeriodicSender(conn, interval, periodic)
		fmt.Printf("Periodic sending every %v. Press Ctrl+P to pause/unpause.\n\n", interval)
	}

	mainInputLoop(conn, periodic)
}
