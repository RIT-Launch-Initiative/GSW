package db

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
)

// InfluxDBV1Handler is a DB Handler implementation for InfluxDB v1
type InfluxDBV1Handler struct {
	conn net.UDPConn // UDP connection to InfluxDB
	addr string      // IP address and port of InfluxDB
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

	// Add timestamp if it exists. Otherwise, Influx will default to current nano time
	if measurements.Timestamp != 0 {
		query += fmt.Sprintf(" %d", measurements.Timestamp)
	}

	query += "\n"

	// TODO: Make a debug logger?

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

	// Send the query data over HTTP to Grafana Live
	// TODO: config!!!
	liveAddr := "http://localhost:3000/api/live/push/custom_stream_id/"
	body := bytes.NewReader(data)
	request, err := http.NewRequest(http.MethodPost, liveAddr, body)
	auth_token := "REDACTED" // TODO auth_token config
	request.Header.Set("Authorization", "Bearer "+auth_token)
	if err != nil {
		return fmt.Errorf("error forming HTTP request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("error returned by HTTP request: %v", err)
	}
	fmt.Printf("Response recieved.")
	fmt.Printf("Status code: %d\n", response.StatusCode)
	resBodyContents, err := io.ReadAll(response.Body)
	if err != nil {
		response.Body.Close()
		return fmt.Errorf("error returned by HTTP request: %v", err)
	}
	err = response.Body.Close()
	if err != nil {
		return fmt.Errorf("error closing response body: %v", err)
	}
	fmt.Printf("Response body: %s\n", resBodyContents)

	return nil
}

// Close closes the InfluxDB UDP client when done
func (h *InfluxDBV1Handler) Close() error {
	err := h.conn.Close()
	if err != nil {
		return fmt.Errorf("error closing InfluxDB UDP client: %w", err)
	}

	return nil
}
