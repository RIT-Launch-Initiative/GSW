package db

import (
	"fmt"
	"net"

	"github.com/AarC10/GSW-V2/lib/logger"
	"go.uber.org/zap"
)

// InfluxDBV1Handler is a DB Handler implementation for InfluxDB v1
type InfluxDBV1Handler struct {
	conn net.UDPConn // UDP connection to InfluxDB
	addr string      // IP address and port of InfluxDB
}

// Initialize sets up the InfluxDB UDP connection
func (h *InfluxDBV1Handler) Initialize(host string, port int) error {
	h.addr = fmt.Sprintf("%s:%d", host, port)
	addr, err := net.ResolveUDPAddr("udp", h.addr)
	if err != nil {
		return fmt.Errorf("resolving db address: %w", err)
	}

	logger.Info("resolved database address", zap.String("url", addr.String()))

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}

	h.conn = *conn

	return nil
}

// CreateQuery generates InfluxDB query for measurement group
func (h *InfluxDBV1Handler) CreateQuery(measurements MeasurementGroup) string {
	return CreateQuery(measurements)
}

// CreateQuery generates InfluxDB query for measurement group
func CreateQuery(measurements MeasurementGroup) string {
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
