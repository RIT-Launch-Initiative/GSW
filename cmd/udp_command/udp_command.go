package main

import (
	"os"
	"fmt"
    "net"
	"strconv"
	"encoding/binary"
	"strings"
	"bufio"
)

const (
    radio_port = 12000
)

func OpenConn(host string, port int) (*net.UDPConn, error) {
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

func SendOverUDP(conn *net.UDPConn, packet []byte) error {
    _, err := conn.Write(packet)
    if err != nil {
        return fmt.Errorf("Error sending data to radio: %v", err)
    }
    return nil
}

func CloseConn(conn *net.UDPConn) {
   err := conn.Close()
   if err != nil {
       fmt.Printf("Error closing connection: %v", err)
   }
}

func main() {
	var host string
	var port string

	fmt.Print("IP Address: ")
	_, err := fmt.Scanln(&host)
	if err != nil {
        fmt.Println(err)
		return
	}
	fmt.Print("Port: ")
	_, err = fmt.Scanln(&port)
	if err != nil {
        fmt.Println(err)
		return
	}

	fmt.Print("Payload: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	err = scanner.Err()
	if err != nil {
		fmt.Println(err)
		return
	}
	payloadStr := scanner.Text()

	payload := make([]byte, 0, 10)
	byteToAdd :=  make([]byte, 8)
	for _, str := range strings.Fields(payloadStr) {
		parsedInt, err := strconv.ParseUint(str, 0, 64)
		if err != nil {
        	fmt.Println(err)
			return
		}
		binary.LittleEndian.PutUint64(byteToAdd, parsedInt)
		payload = append(payload, byteToAdd[0])
	}

	parsedPort, err := strconv.ParseInt(port, 0, 0)
	if err != nil {
        fmt.Println(err)
		return
	}
    conn, err := OpenConn(host, int(parsedPort))
    if err != nil {
        fmt.Println(err)
		return
    }
    defer CloseConn(conn)

	fmt.Printf("Sending payload %v\n", payload)
    err = SendOverUDP(conn, payload)
    if err != nil {
        fmt.Println(err)
		return
    }
}
