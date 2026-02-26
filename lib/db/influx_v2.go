package db

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/AarC10/GSW-V2/lib/logger"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"go.uber.org/zap"
)

// InfluxDBV2Handler is a BatchHandler implementation for InfluxDB v1
type InfluxDBV2Handler struct {
	client   influxdb2.Client
	writeAPI api.WriteAPI
	org      string
	bucket   string
	cfg      Config
}

// Initialize satisfies the Handler interface using host/port only so
// for V2 you most likey want InitializeWithConfig instead.
// This is a wrapper around InitializeWithConfig that fills in the URL and leaves
func (handler *InfluxDBV2Handler) Initialize(host string, port int) error {
	return handler.InitializeWithConfig(Config{
		URL:           fmt.Sprintf("http://%s:%d", host, port),
		Token:         "",
		Org:           "gsw",
		Bucket:        "gsw",
		BatchSize:     100,
		FlushInterval: 1000,
		Precision:     "ns",
	})
}

// InitializeWithConfig sets up the InfluxDB v2 client with full config
func (handler *InfluxDBV2Handler) InitializeWithConfig(cfg Config) error {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 1000
	}
	if cfg.Precision == "" {
		cfg.Precision = "ns"
	}

	handler.cfg = cfg
	handler.org = cfg.Org
	handler.bucket = cfg.Bucket

	options := influxdb2.DefaultOptions().
		SetBatchSize(cfg.BatchSize).
		SetFlushInterval(cfg.FlushInterval)

	handler.client = influxdb2.NewClientWithOptions(cfg.URL, cfg.Token, options)
	handler.writeAPI = handler.client.WriteAPI(cfg.Org, cfg.Bucket)

	// error reporter because that'll totally never happen
	go func() {
		for err := range handler.writeAPI.Errors() {
			logger.Error("InfluxDB V2 async write error", zap.Error(err))
		}
	}()

	logger.Info("InfluxDB V2 client initialized",
		zap.String("url", cfg.URL),
		zap.String("org", cfg.Org),
		zap.String("bucket", cfg.Bucket),
		zap.Uint("batchSize", cfg.BatchSize),
		zap.Uint("flushInterval", cfg.FlushInterval),
	)
	return nil
}

// CreateQuery generates InfluxDB line protocol for a MeasurementGroup.
// Reuses the same logic as V1 for consistency.
func (handler *InfluxDBV2Handler) CreateQuery(measurements MeasurementGroup) string {
	return CreateQuery(measurements)
}

// Insert writes a MeasurementGroup to InfluxDB v2 as a single point.
// The write is buffered and flushed based on batch size and flush interval.
func (handler *InfluxDBV2Handler) Insert(measurements MeasurementGroup) error {
	point := influxdb2.NewPointWithMeasurement(measurements.DatabaseName)

	var ts time.Time
	if measurements.Timestamp != 0 {
		ts = time.Unix(0, measurements.Timestamp)
	} else {
		ts = time.Now()
	}
	point.SetTime(ts)

	for _, m := range measurements.Measurements {
		// Try numeric first; fall back to string field.
		if f, err := strconv.ParseFloat(m.Value, 64); err == nil {
			point.AddField(m.Name, f)
		} else if i, err := strconv.ParseInt(m.Value, 10, 64); err == nil {
			point.AddField(m.Name, i)
		} else {
			point.AddField(m.Name, m.Value)
		}
	}

	handler.writeAPI.WritePoint(point)
	return nil
}

// InsertBatch writes multiple MeasurementGroups in one shot and flushes.
func (handler *InfluxDBV2Handler) InsertBatch(batch []MeasurementGroup) error {
	for _, measurementGroup := range batch {
		if err := handler.Insert(measurementGroup); err != nil {
			return err
		}
	}
	return handler.Flush()
}

// Flush forces all buffered points to be sent immediately.
func (handler *InfluxDBV2Handler) Flush() error {
	handler.writeAPI.Flush()
	return nil
}

// Close flushes pending writes and closes the client.
func (handler *InfluxDBV2Handler) Close() error {
	handler.writeAPI.Flush()
	handler.client.Close()
	return nil
}

// BlockingInsert writes a point using the blocking write API.
func (handler *InfluxDBV2Handler) BlockingInsert(ctx context.Context, measurements MeasurementGroup) error {
	blockingAPI := handler.client.WriteAPIBlocking(handler.org, handler.bucket)

	point := influxdb2.NewPointWithMeasurement(measurements.DatabaseName)
	var timestamp time.Time
	if measurements.Timestamp != 0 {
		timestamp = time.Unix(0, measurements.Timestamp)
	} else {
		timestamp = time.Now()
	}
	point.SetTime(timestamp)

	for _, measurement := range measurements.Measurements {
		if floatVal, err := strconv.ParseFloat(measurement.Value, 64); err == nil {
			point.AddField(measurement.Name, floatVal)
		} else if intVal, err := strconv.ParseInt(measurement.Value, 10, 64); err == nil {
			point.AddField(measurement.Name, intVal)
		} else {
			point.AddField(measurement.Name, measurement.Value)
		}
	}

	return blockingAPI.WritePoint(ctx, point)
}

// Ensure InfluxDBV2Handler satisfies BatchHandler at compile time.
var _ BatchHandler = (*InfluxDBV2Handler)(nil)

// writePoint is a helper implementing write.PointWriter for testing.
type writePoint = write.Point
