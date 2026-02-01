# `ui`

Ground station UI and overlays implemented in [Svelte](https://svelte.dev/).

Communicates with GSW through the `mqtt_producer` app and an MQTT server. This project uses [`pnpm`](https://pnpm.io/) over `npm` for its performance benefits.

## Development

```shell
docker compose -f compose-services.yaml up --build
go run ./cmd/mqtt_producer
pnpm run dev
```
