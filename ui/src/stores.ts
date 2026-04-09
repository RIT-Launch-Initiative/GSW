import mqtt from "mqtt";
import { writable } from "svelte/store";
import type { Writable } from "svelte/store";

export const mqttData: Writable<Map<string, unknown>> = writable(new Map());

let client: mqtt.MqttClient | null = null;
let sineWaveGeneratorInterval: ReturnType<typeof setInterval> | null = null;

export type SineWaveGeneratorOptions = {
    intervalMs?: number;
    baseLat?: number;
    baseLng?: number;
    channel?: string | number;
};

function emitSyntheticMessage(topic: string, value: unknown) {
    // Mirror test samples directly into local state so UI updates immediately.
    mqttData.update((state) => {
        state.set(topic, value);
        return state;
    });

    if (client?.connected) {
        client.publish(topic, JSON.stringify(value));
    }
}

export function connectMqtt(brokerUrl: string = "ws://localhost:1880") {
    if (client) return;
    
    client = mqtt.connect(brokerUrl);
    
    client.on("connect", async () => {
        console.log("MQTT connected, subscribing to gsw topic");
        await client!.subscribeAsync("gsw/#");
    });
    
    client.on("message", (topic, body) => {
        try {
            const value = JSON.parse(body.toString());
            mqttData.update(state => {
                state.set(topic, value);
                return state;
            });
        } catch (e) {
            console.error(`Failed to parse message from ${topic}:`, e);
        }
    });
    
    client.on("error", (error) => {
        console.error("MQTT error:", error);
    });
}

export function disconnectMqtt() {
    if (client) {
        client.end(true, () => {
            console.log("MQTT client closed");
        });
        client = null;
    }
}

export function startSineWaveMqttGenerator(options: SineWaveGeneratorOptions = {}) {
    if (sineWaveGeneratorInterval) return;

    const intervalMs = options.intervalMs ?? 250;
    const baseLat = options.baseLat ?? 43.08348;
    const baseLng = options.baseLng ?? -77.67641;
    const channel = String(options.channel ?? "1");
    const topicRoot = `gsw/${channel}/`;
    const startTime = Date.now();

    sineWaveGeneratorInterval = setInterval(() => {
        const t = (Date.now() - startTime) / 1000;

        const latitude = baseLat + Math.sin(t * 0.3) * 0.02;
        const longitude = baseLng + Math.cos(t * 0.25) * 0.02;
        const altitude = 9000 + Math.sin(t * 0.5) * 1200;
        const satCount = Math.max(0, Math.round(8 + Math.sin(t * 0.4) * 3));

        const voltBatt = 11.8 + Math.sin(t * 0.35) * 0.6;
        const currBatt = 1.5 + Math.sin(t * 0.8) * 0.9;
        const snr = 20 + Math.sin(t * 0.6) * 5;
        const temperature = 24 + Math.sin(t * 0.22) * 4;

        const accel_x = 9.81 + Math.sin(t + 0.35)
        const accel_y = 9.81 + Math.sin(t + 0.35)
        const accel_z = 9.81 + Math.sin(t + 0.35)
        
        emitSyntheticMessage(topicRoot + "gnsscoordinates", {
            latitude,
            longitude,
            altitude,
            sat_count: satCount,
        });
        emitSyntheticMessage(topicRoot + "powermodule", {
            VOLT_BATT: voltBatt,
            CURR_BATT: currBatt,
        });
        emitSyntheticMessage(topicRoot + "receiverstats", {
            snr,
        });
        emitSyntheticMessage(topicRoot + "sensormodule", {
            temperature,
            accel_x,
            accel_y,
            accel_z,
        });
    }, intervalMs);
}

export function stopSineWaveMqttGenerator() {
    if (!sineWaveGeneratorInterval) return;

    clearInterval(sineWaveGeneratorInterval);
    sineWaveGeneratorInterval = null;
}

export type ParsedTopic = {
    prefix: string;
    packet: string;
    measurement: string;
};

export type PacketData = Record<string, Record<string, unknown>>;

export function parseTopic(topic: string): ParsedTopic | null {
    const parts = topic.split("/");
    if (parts.length < 3) {
        return null;
    }

    const [prefix, packet, measurement] = parts;
    if (!prefix || !packet || !measurement) {
        return null;
    }

    return {
        prefix,
        packet,
        measurement,
    };
}

export function getDataByPacket(mqttMap: Map<string, unknown>): PacketData {
    const grouped: PacketData = {};

    mqttMap.forEach((value, topic) => {
        const parsed = parseTopic(topic);
        if (!parsed) {
            return;
        }

        if (!grouped[parsed.packet]) {
            grouped[parsed.packet] = {};
        }
        grouped[parsed.packet][parsed.measurement] = value;
    });

    return grouped;
}

export function getValueCaseInsensitive(source: Record<string, unknown> | undefined, key: string): unknown {
    if (!source) return undefined;

    const keyLower = key.toLowerCase();
    for (const [entryKey, entryValue] of Object.entries(source)) {
        if (entryKey.toLowerCase() === keyLower) {
            return entryValue;
        }
    }

    return undefined;
}

export function getObjectCaseInsensitive(source: Record<string, unknown> | undefined, key: string): Record<string, unknown> | undefined {
    const value = getValueCaseInsensitive(source, key);
    if (value && typeof value === "object" && !Array.isArray(value)) {
        return value as Record<string, unknown>;
    }
    return undefined;
}

export function getNumberCaseInsensitive(source: Record<string, unknown> | undefined, ...keys: string[]): number | null {
    for (const key of keys) {
        const value = getValueCaseInsensitive(source, key);
        if (typeof value === "number") {
            return value;
        }
    }
    return null;
}

export function getChannelPayload(
    packetData: Record<string, unknown>,
    channel?: string | number,
): Record<string, unknown> | undefined {
    if (channel !== undefined) {
        return getObjectCaseInsensitive(packetData, String(channel));
    }

    for (const value of Object.values(packetData)) {
        if (value && typeof value === "object" && !Array.isArray(value)) {
            return value as Record<string, unknown>;
        }
    }

    return undefined;
}
