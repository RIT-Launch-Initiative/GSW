# `ui`

Ground station UI and overlays implemented in [Svelte](https://svelte.dev/).

Communicates with GSW through the `mqtt_producer` app and an MQTT server. This project uses [`pnpm`](https://pnpm.io/) over `npm` for its performance benefits.

## Usage

Example URL format: `http://localhost:5173/?bg=%23fafbf2&callSign=KE2EGW-1&groundStation=43.08348,-77.67641&mqttAddress=127.0.0.1:3000&mqttChannel=3&teamNumber=52`

Available parameters:
- bg: %23\[hex code here]
- callSign
- groundStation: lat,lon
- mockMqtt: true if mocking MQTT data
- mqttAddress: address of MQTT
- mqttChannel: channel for MQTT data
- teamNumber

## Development

```shell
# mosquitto container
docker compose -f compose-services.yaml up --build

go run ./cmd/mqtt_producer

# ui development server
cd ui/ && pnpm run dev
```
