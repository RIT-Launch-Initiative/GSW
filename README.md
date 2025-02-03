# Launch Ground Software
Ground software code for RIT Launch Initiative. Responsible for receiving telemetry over Ethernet and publishing to shared memory for other applications to use.

## Compiling
By Golang convention, all files meant to be executed are stored in the cmd folder. 
* The GSW service can be compiled by running `go build cmd/gsw_service.go` from the project root directory
* GSW applications are stored in subdirectories within the cmd folder and the `go build` command can be ran on go files within those subdirectories

## Running
You can always run the GSW service by doing a `./gsw_service` after building. For running any Go program though, instead of doing `go build (FILE_PATH)` you can do `go run (FILE_PATH)` instead.

### Compatibility
Some machines do not have a /dev/shm directory. The directory used for shared memory can be changed with the flag `-shm (DIRECTORY_NAME)`. For example, `go run cmd/mem_view/mem_view.go -shm /someDirectory/RAMDrive`.

## Unit Tests
There are several unit tests that can be ran. You can do a `go test ./...` from the root project directory to execute all tests. It is also recommended to run with the -cover
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
