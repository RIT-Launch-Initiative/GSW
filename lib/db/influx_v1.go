package db

import (
	"fmt"
	"net"
)

type InfluxDBV1Handler struct {
	conn net.UDPConn
	addr string
}

// Initialize sets up the InfluxDB UDP connection
func (h *InfluxDBV1Handler) Initialize() error {
	h.addr = "localhost:8089" // TODO: Make this IP and port configurable

	addr, err := net.ResolveUDPAddr("udp", h.addr)
	if err != nil {
		fmt.Println("Error creating InfluxDB UDP client:", err)
		return err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Println("Error creating InfluxDB UDP client:", err)
		return err
	}

	h.conn = *conn
	return nil
}

// CreateQuery Generates InfluxDB query for measurement group
func (h *InfluxDBV1Handler) CreateQuery(measurements MeasurementGroup) string {
	query := measurements.DatabaseName + " "

	for _, measurement := range measurements.Measurements {
		query += fmt.Sprintf("%s=%s,", measurement.Name, measurement.Value)
	}

	// Don't check if string is empty. We expect the Name and the measurements to be non-empty.
	query = query[:len(query)-1]

	query += fmt.Sprintf(" %d", measurements.Timestamp)

	fmt.Printf("%s\n", query)
	return query
}

// Insert sends the measurement group data to InfluxDB using UDP
func (h *InfluxDBV1Handler) Insert(measurements MeasurementGroup) error {
	// Generate the InfluxDB line protocol query
	query := h.CreateQuery(measurements)

	// Convert the query string to bytes
	data := []byte(query)

	// Send the query data over UDP
	_, err := h.conn.Write(data)
	if err != nil {
		return fmt.Errorf("error sending data to InfluxDB over UDP: %w", err)
	}

	return nil
}

// Close closes the InfluxDB UDP client when done
func (h *InfluxDBV1Handler) Close() {
	err := h.conn.Close()
	if err != nil {
		fmt.Println("Error closing InfluxDB UDP client:", err)
		return
	}
}
