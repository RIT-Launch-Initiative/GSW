# List available recipes
default:
    @just --list

# Glootie from Rick and Morty: just build <app>
build app:
    go build -o bin/{{app}} ./cmd/{{app}}
    {{ if app == "pkt_cap" { "sudo setcap 'cap_net_raw,cap_net_admin=eip' bin/pkt_cap" } else { "" } }}

# Build all apps
build-all:
    for app in cmd/*/; do just build $(basename $app); done

# Run any app: just run <app>
run app:
    {{ if app == "pkt_cap" { "just build pkt_cap && ./bin/pkt_cap" } else { "go run ./cmd/" + app } }}

# Run GSW service
run-gsw:
    go run ./cmd/gsw_service.go

# Run all tests
# TODO: Maybe have individual test recipes for each package?
test:
    go test ./...

# Format all Go code
fmt:
    gofmt -w .
