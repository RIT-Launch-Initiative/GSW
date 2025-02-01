package main

import (
	"fmt"
    "net"
)

const (
    radio_port = 12000
)

func OpenConn() (*net.UDPConn, error) {
    host := "localhost"
    addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%v:%v", host, radio_port))
    if err != nil {
        return nil, fmt.Errorf("Error resolving UDP address: %v", err)
    }
    conn, err := net.DialUDP("udp", nil, addr)
    if err != nil {
        return nil, fmt.Errorf("Error connecting to UDP address: %v", err)
    }
    return conn, nil;
}

func SendToRadio(conn *net.UDPConn, packet []byte) error {
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
    fmt.Println("Hello world")
    data := []byte{0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9}

    conn, err := OpenConn()
    if err != nil {
        fmt.Println(err)
    }
    defer CloseConn(conn)

    err = SendToRadio(conn, data)
    if err != nil {
        fmt.Println(err)
    }
}
