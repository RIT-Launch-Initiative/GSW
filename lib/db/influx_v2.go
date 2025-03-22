package db

import (
	"fmt"
	"github.com/influxdata/influxdb-client-go/v2"
)

// InfluxDBV2Handler is a handler for InfluxDB v2
type InfluxDBV2Handler struct {
	client influxdb2.Client // InfluxDB client
	org    string           // Organization string
	bucket string           // Bucket string
}

// Initialize sets up the InfluxDB client
func (h *InfluxDBV2Handler) Initialize() {
	// TODO: Get URL and token from config
	//h.client = influxdb2.NewClient(url, token)
}

// CreateQuery generates the InfluxDB line protocol query for measurementGroup
func (h *InfluxDBV2Handler) CreateQuery(measurementGroup MeasurementGroup) string {
	var query string

	for _, measurement := range measurementGroup.Measurements {
		query += fmt.Sprintf("%s,value=%s %d\n", measurement.Name, measurement.Value, measurementGroup.Timestamp)
	}
	return query
}

// Insert sends the measurement data to InfluxDB
func (h *InfluxDBV2Handler) Insert(measurementGroup MeasurementGroup) error {
	// TODO: Implement
	//query := h.CreateQuery(measurementGroup)

	return nil
}

// Close closes the InfluxDB client when done
func (h *InfluxDBV2Handler) Close() error {
	h.client.Close()
	return nil
}
