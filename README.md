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

## Docker
It may be easier to run the GSW in a docker container. This might be better for compatibility and easier for people on Windows hosts (as docker desktop will natively use a WLS 2 backend).

A dockerfile for GSW is provided in `./cmd/Containerfile` and can be built and run with:
```shell
$ docker build -t launch-gsw -f ./cmd/Containerfile .
```

And the container could be started with:
```shell
$ docker run --name gsw-service \
    -p 11020:11020/udp \
    -p 13020:13020/udp \
    -p 12005:12005/udp \
    -p 12006:12006/udp \
    -p 12002:12002/udp \
    launch-gsw
```

For simplicity, a `docker-compose` file is provided for building and running GSW, Grafana, and InfluxDB:
```shell
$ docker compose up --build
```

### Attaching to the container

If you need to run any of the apps in `cmd/`, you can get a shell into the container using:
```shell
$ docker exec -it gsw-service sh
```

All binaries are in PATH as the names of their folders in `cmd/`.

### Exporting telemetry from docker-compose InfluxDB

You can access the InfluxDB CLI using `docker compose exec -it influxdb influx`. 
For example, to export all receiver telemetry as a CSV, run:
```shell
$ docker compose exec influxdb influx \
    -database "gsw" \
    -format csv \
    -execute "SELECT * FROM receiver"
```

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
**To set up live data streaming to Grafana, the setup utility can be run from the root directory with `go run cmd/live_setup/live_setup.go`**

Once set up, the grafana_live application can be run from the root directory (with `go run cmd/grafana_live/grafana_live.go`) to stream live data to Grafana.
Make sure the GSW service is running before starting the application.

In case the setup utility does not work, live data streaming can be set up manually as follows:
1. Import a new dashboard into Grafana using the JSON file located at `data/grafana/dashboards/Backplane-Live.json`. The dashboard can have any name and UID.
2. Create a service account at the following URL: [http://localhost:3000/org/serviceaccounts/create](http://localhost:3000/org/serviceaccounts/create) (replace `http://localhost:3000` if you are using a different host).
The service account can have any display name but must be given the Admin role ([More information about creating service accounts](https://grafana.com/docs/grafana/latest/administration/service-accounts/)).
3. Click "Add service account token" and then "Generate token". The token can have any display name. Don't set an expiration date unless you want the token to become invalid after that date. 
Copy the token to the clipboard. If you lose it, you will need to generate a new token.
4. Set the environment variable `GRAFANA_LIVE_TOKEN` to the service account token from the previous step. This can be done by creating a file called `.env` in the root directory.
The file contents should be `GRAFANA_LIVE_TOKEN="<token>"`, replacing `<token>` with the service account token.

### Configuration
By default, the application is configured using the file `grafana_live.yaml` in the `data/config` directory. If the flag `-c (FILE_NAME)` is used, the application will instead parse the configuration file at `data/config/(FILE_NAME).yaml`.

The Grafana Live dashboard is not currently set up for radio module.

#### Keys (must be specified)
* `channel_path`: Final portion of the Grafana Live channel string (e.g. "backplane" in `stream/telemetry/backplane`).
* `websocket_addr`: Address where data is pushed to Grafana Live over WebSocket. If data is being streamed to a local Grafana instance,
this will probably start with `ws://localhost:3000`, but this can be replaced with the address of any Grafana instance.
The second part of the address should always be `/api/live/push/telemetry/`, unless you have a reason to change it.
* `http_addr`: Address where data is pushed to Grafana Live over HTTP. Make sure the protocol is `http://` or `https://`.
* `use_websocket`: Whether to use WebSocket for data streaming. If disabled, HTTP will be used. WebSocket is recommended for better performance.
* `use_http`: Whether to use HTTP for data streaming. If `use_websocket` is also true, HTTP will be used as backup if the WebSocket fails.