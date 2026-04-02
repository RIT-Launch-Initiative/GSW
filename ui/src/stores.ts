import mqtt from "mqtt";
import { writable } from "svelte/store";
import type { Writable } from "svelte/store";

export const mqttData: Writable<Map<string, unknown>> = writable(new Map());

let client: mqtt.MqttClient | null = null;

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
