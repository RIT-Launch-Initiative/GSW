package proc

import (
	"fmt"

	"github.com/AarC10/GSW-V2/lib/ipc"
)

const telemetryConfigKey = "telemetry-config"

// WriteTelemetryConfigToShm writes to the config in SHM,
// returning a cleanup function to remove it.
func WriteTelemetryConfigToShm(shmDir string, data []byte) (cleanup func(), err error) {
	configWriter, err := ipc.NewShmHandler(telemetryConfigKey, len(data), true, shmDir)
	if err != nil {
		return nil, fmt.Errorf("creating shm handler: %w", err)
	}
	if err := configWriter.Write(data); err != nil {
		configWriter.Cleanup()
		return nil, fmt.Errorf("writing to shm handler: %w", err)
	}
	return configWriter.Cleanup, nil
}

// ReadTelemetryConfigFromShm reads the config from SHM and returns it.
func ReadTelemetryConfigFromShm(shmDir string) ([]byte, error) {
	configReader, err := ipc.CreateShmReader(telemetryConfigKey, shmDir)
	if err != nil {
		return nil, fmt.Errorf("creating shm handler: %w", err)
	}
	defer configReader.Cleanup()

	data, err := configReader.ReadRaw()
	if err != nil {
		return nil, fmt.Errorf("reading from shm handler: %w", err)
	}
	return data, nil
}
