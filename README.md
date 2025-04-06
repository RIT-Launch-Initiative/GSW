# Launch Ground Software
Ground software code for RIT Launch Initiative. Responsible for receiving telemetry over Ethernet and publishing to shared memory for other applications to use.

## Compiling
By Golang convention, all files meant to be executed are stored in the cmd folder. 
* The GSW service can be compiled by running `go build cmd/gsw_service.go` from the project root directory
* GSW applications are stored in subdirectories within the cmd folder and the `go build` command can be run on go files within those subdirectories

## Running
You can always run the GSW service by doing a `./gsw_service` after building. For running any Go program though, instead of doing `go build (FILE_PATH)` you can do `go run (FILE_PATH)` instead.

### Compatibility
Some machines do not have a /dev/shm directory. The directory used for shared memory can be changed with the flag `-shm (DIRECTORY_NAME)`. For example, `go run cmd/mem_view/mem_view.go -shm /someDirectory/RAMDrive`.

## Unit Tests
There are several unit tests that can be run. You can do a `go test ./...` from the root project directory to execute all tests. It is also recommended to run with the -cover
flag to get coverage statements.

## Configuration
By default, the GSW service is configured using the file `gsw_service.yaml` in the `data/config` directory. If the flag `-c (FILE_NAME)` is used, the GSW service will instead parse the configuration file at `data/config/(FILE_NAME).yaml`.

### Keys
* `telemetry_config`: Path to the telemetry config file. This flag *must* be specified for the service to run. Example: `telemetry_config: data/config/backplane.yaml`

## Create Service Script (for Linux)
The script must be run from the /scripts directory.
gsw_service must be built prior to the script being run (and it must exist for the service to work).

### Running the Service
Once the script has been run, start the service with:
`sudo systemctl start gsw`

Check the status of the service with:
`sudo systemctl status gsw`

Stop the service with:
`sudo systemctl stop gsw`

If you want the service to run on startup:
`sudo systemctl enable gsw`

## Grafana Live
The grafana_live application can be run from the root directory (with `go run cmd/grafana_live/grafana_live.go`) to stream live data to Grafana.
To set up live data streaming, perform the following steps:
1. Import a new dashboard into Grafana using the JSON file located at `data/grafana/Backplane-Live.json`. The dashboard can have any name and UID.
2. Create a service account at the following URL: [http://localhost:3000/org/serviceaccounts/create](http://localhost:3000/org/serviceaccounts/create) (replace `http://localhost:3000` if you are using a different host).
The service account can have any display name but must be given the Admin role ([More information about creating service accounts](https://grafana.com/docs/grafana/latest/administration/service-accounts/)).
3. Click "Add service account token" and then "Generate token". The token can have any display name. Don't set an expiration date unless you want the token to become invalid after that date. 
Copy the token to the clipboard. If you lose it, you will need to generate a new token.
4. Set the environment variable `GRAFANA_LIVE_TOKEN` to the service account token from the previous step. This can be done by creating a file called `.env` in the root directory.
The file contents should be `GRAFANA_LIVE_TOKEN="<token>"`, replacing `<token>` with the service account token.

The grafana_live application setup should now be complete. Make sure the GSW service is running before starting the application.

Note: Live data is not currently set up for radio module.

### Configuration
By default, the application is configured using the file `grafana_live.yaml` in the `data/config` directory. If the flag `-c (FILE_NAME)` is used, the application will instead parse the configuration file at `data/config/(FILE_NAME).yaml`.

#### Keys (must be specified)
* `channel_path`: Final portion of the Grafana Live channel string (e.g. "backplane" in `stream/telemetry/backplane`).
* `live_addr`: Address where data is pushed to Grafana Live. If data is being streamed to a local Grafana instance,
this will probably start with `http://localhost:3000`, but this can be replaced with the address of any Grafana instance.
The second part of the address should always be `/api/live/push/telemetry/`, unless you have a reason to change it.