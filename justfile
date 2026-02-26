# List available recipes
default:
    @just --list

# Glootie from Rick and Morty: just build <app>
build app:
    go build -o bin/{{app}} ./cmd/{{app}}

# Build all apps
build-all:
    for app in cmd/*/; do just build $(basename $app); done

# Run any app: just run <app>
run app:
    go run ./cmd/{{app}}

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
