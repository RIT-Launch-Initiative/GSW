package main

import (
	"os"
	"fmt"
    "net"
	"strconv"
	"strings"
	"bufio"
)

func openConn(host string, port int) (*net.UDPConn, error) {
    addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%v:%v", host, port))
    if err != nil {
        return nil, fmt.Errorf("Error resolving UDP address: %v", err)
    }
    conn, err := net.DialUDP("udp", nil, addr)
    if err != nil {
        return nil, fmt.Errorf("Error connecting to UDP address: %v", err)
    }
    return conn, nil;
}

func sendOverUDP(conn *net.UDPConn, packet []byte) error {
    _, err := conn.Write(packet)
    if err != nil {
        return err
    }
    return nil
}

func closeConn(conn *net.UDPConn) {
   err := conn.Close()
   if err != nil {
       fmt.Println("Error closing connection:", err)
   }
}

func main() {
	var host string
	var port string
	scanner := bufio.NewScanner(os.Stdin)

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
    defer closeConn(conn)
	fmt.Println("Connection opened.\n")

	PayloadLoop:
	for {
		fmt.Print("Payload: ")
		scanner.Scan()
		if err := scanner.Err(); err != nil {
			fmt.Println("\tError reading payload:", err)
			continue
		}
		payloadStr := scanner.Text()

		payload := make([]byte, 0, 10)
		for _, str := range strings.Fields(payloadStr) {
			parsedInt64, err := strconv.ParseUint(str, 0, 8)
			if err != nil {
				fmt.Println("\tError parsing payload:", err)
				continue PayloadLoop
			}
			payload = append(payload, uint8(parsedInt64))
		}
		fmt.Printf("\tSending payload %# x\n", payload)
		if err := sendOverUDP(conn, payload); err != nil {
			fmt.Println("\tError sending payload:", err)
			continue
		}
	}
}
