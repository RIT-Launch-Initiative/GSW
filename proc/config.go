package proc

import (
	"fmt"

	"github.com/AarC10/GSW-V2/lib/ipc"
)

const telemetryConfigKey = "telemetry-config"

func WriteTelemetryConfigToShm(shmDir string, data []byte) (cleanup func(), err error) {
	configWriter, err := ipc.NewShmHandler(telemetryConfigKey, len(data), true, shmDir)
	if err != nil {
		return nil, fmt.Errorf("creating shm handler: %w", err)
	}
	if configWriter.Write(data) != nil {
		configWriter.Cleanup()
		return nil, fmt.Errorf("writing to shm handler: %w", err)
	}
	return configWriter.Cleanup, nil
}

func ReadTelemetryConfigFromShm(shmDir string) ([]byte, error) {
	configReader, err := ipc.CreateShmReader(telemetryConfigKey, shmDir)
	if err != nil {
		return nil, fmt.Errorf("creating shm handler: %w", err)
	}
	data, err := configReader.ReadRaw()
	if err != nil {
		return nil, fmt.Errorf("reading from shm handler: %w", err)

	}
	return data, nil
}
