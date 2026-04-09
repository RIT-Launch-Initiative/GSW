<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import {
        mqttData,
        connectMqtt,
        disconnectMqtt,
        getDataByPacket,
        getObjectCaseInsensitive,
        getNumberCaseInsensitive,
        getChannelPayload,
        startSineWaveMqttGenerator,
        stopSineWaveMqttGenerator
    } from "./stores";
    import { MapLibre, NavigationControl, ScaleControl, GlobeControl, Marker } from 'svelte-maplibre-gl';
    import "maplibre-gl/dist/maplibre-gl.css";

    let markerPosition = { lng: -77.67641, lat: 43.08348 };
    let satCount: number | null = null;
    let altitude: number | null = null;
    let battCurrent: number | null = null;
    let battVoltage: number | null = null;
    let receiverSnr: number | null = null;
    let temperature: number | null = null;
    let accelX: number | null = null;
    let accelY: number | null = null;
    let accelZ: number | null = null;
    let gForce: number | null = null;
    const mqttChannel: string | number = "1";
    const ALTITUDE_MAX = 12000;
    const G_FORCE_MAX = 20;
    let altitudePercent = 0;
    let gForcePercent = 0;
    let topLeftMetricStubs = [
        { label: "BATT mA", value: "--" },
        { label: "RCV SNR", value: "--" },
        { label: "BATT V", value: "--" },
        { label: "TEMP", value: "--" }
    ];

    onMount(() => {
        connectMqtt();
        startSineWaveMqttGenerator({ intervalMs: 1000, channel: mqttChannel });
    });

    onDestroy(() => {
        stopSineWaveMqttGenerator();
        disconnectMqtt();
    });

    $: altitudePercent = altitude === null
        ? 0
        : Math.max(0, Math.min(100, (altitude / ALTITUDE_MAX) * 100));

    $: gForce = accelX !== null && accelY !== null && accelZ !== null
        ? Math.sqrt(accelX ** 2 + accelY ** 2 + accelZ ** 2)
        : null;

    $: gForcePercent = gForce === null
        ? 0
        : Math.max(0, Math.min(100, (gForce / G_FORCE_MAX) * 100));

    $: topLeftMetricStubs = [
        { label: "BATT mA", value: battCurrent !== null ? `${(battCurrent * 1000).toFixed(1)}` : "--" },
        { label: "SNR dB", value: receiverSnr !== null ? `${receiverSnr.toFixed(1)}` : "--" },
        { label: "BATT V", value: battVoltage !== null ? `${battVoltage.toFixed(2)}` : "--" },
        { label: "TEMP", value: temperature !== null ? `${temperature.toFixed(2)}` : "--" }
    ];

    // Subscribe to MQTT data changes
    $: {
        const data = getDataByPacket($mqttData) as Record<string, unknown>;
        const prefix = getChannelPayload(data, mqttChannel);
        if (prefix) {
            const gnsscoordinates = getObjectCaseInsensitive(prefix, "gnsscoordinates");
            const powermodule = getObjectCaseInsensitive(prefix, "powermodule");
            const receiverstats = getObjectCaseInsensitive(prefix, "receiverstats");
            const sensormodule = getObjectCaseInsensitive(prefix, "sensormodule");

            const latitude = getNumberCaseInsensitive(gnsscoordinates, "latitude");
            const longitude = getNumberCaseInsensitive(gnsscoordinates, "longitude");
            if (latitude !== null && longitude !== null) {
                markerPosition = { lng: longitude, lat: latitude };
            }

            satCount = getNumberCaseInsensitive(gnsscoordinates, "sat_count");
            altitude = getNumberCaseInsensitive(gnsscoordinates, "altitude");
            battCurrent = getNumberCaseInsensitive(powermodule, "CURR_BATT");
            battVoltage = getNumberCaseInsensitive(powermodule, "VOLT_BATT");
            receiverSnr = getNumberCaseInsensitive(receiverstats, "SNR", "RCV_SNR", "snr");
            temperature = getNumberCaseInsensitive(sensormodule, "temperature");
            accelX = getNumberCaseInsensitive(sensormodule, "ACCEL_X", "ADX_ACCEL_X", "LSM_ACCEL_X");
            accelY = getNumberCaseInsensitive(sensormodule, "ACCEL_Y", "ADX_ACCEL_Y", "LSM_ACCEL_Y");
            accelZ = getNumberCaseInsensitive(sensormodule, "ACCEL_Z", "ADX_ACCEL_Z", "LSM_ACCEL_Z");
        }
    }
</script>

<div class="relative flex w-full min-h-screen" style="">
    <div class="absolute left-2 top-2 z-50 w-64 rounded border border-gray-300 bg-white/95 p-2 shadow">
        <div class="grid grid-cols-2 gap-2">
            {#each topLeftMetricStubs as metric (metric.label)}
                <div class="rounded border border-gray-200 bg-gray-50 p-2">
                    <div class="text-[10px] font-semibold tracking-wide text-gray-600">{metric.label}</div>
                    <div class="mt-1 text-base font-mono font-bold text-gray-900">{metric.value}</div>
                </div>
            {/each}
        </div>
    </div>

    <div
        class="absolute left-1/2 top-2 z-60 flex h-20 w-72 -translate-x-1/2 items-center justify-center bg-red-600/90 text-4xl font-black tracking-widest text-white shadow-lg"
        style="clip-path: polygon(0% 0%, 100% 0%, 80% 100%, 20% 100%);"
    >
        RISK
    </div>

    <div class="absolute bottom-2 left-2 top-[13rem] z-50 rounded bg-black/70 px-2 py-1 text-white">
        <div class="flex h-full flex-col items-center">
            <div class="mb-2 text-center text-xs font-semibold">Altitude</div>
            <div class="relative w-3 flex-1 rounded bg-white/20">
                <div
                    class="absolute bottom-0 left-0 w-full rounded bg-lime-400"
                    style={`height: ${altitudePercent}%`}
                ></div>
                <div
                    class="absolute left-1/2 h-3 w-3 -translate-x-1/2 rounded-full border border-white bg-lime-200"
                    style={`bottom: calc(${altitudePercent}% - 0.375rem)`}
                ></div>
            </div>
            <div class="mt-2 text-center text-xs tabular-nums">{altitude !== null ? `${altitude.toFixed(0)}ft` : "--"}</div>
        </div>
    </div>

    <div class="absolute bottom-2 right-2 top-[13rem] z-50 rounded bg-black/70 px-2 py-1 text-white">
        <div class="flex h-full flex-col items-center">
            <div class="mb-2 text-center text-xs font-semibold">G Force</div>
            <div class="relative w-3 flex-1 rounded bg-white/20">
                <div
                    class="absolute bottom-0 left-0 w-full rounded bg-sky-400"
                    style={`height: ${gForcePercent}%`}
                ></div>
                <div
                    class="absolute left-1/2 h-3 w-3 -translate-x-1/2 rounded-full border border-white bg-sky-200"
                    style={`bottom: calc(${gForcePercent}% - 0.375rem)`}
                ></div>
            </div>
            <div class="mt-2 text-center text-xs tabular-nums">
                {gForce !== null ? `${gForce.toFixed(2)}g` : "--"}
            </div>
        </div>
    </div>

    <div class="absolute right-0 top-0 z-10 w-70">
        <div class="relative h-40 w-70">
            <MapLibre
            class="h-40 w-70"
            style="/src/style.aliflux.json"
            zoom={3.5}
            center={{ lng: -77.67641, lat: 43.08348 }}
            attributionControl={false}
            >
                <!-- <ScaleControl /> -->
                <Marker
                lnglat={markerPosition}
                />
            </MapLibre>
            <div class="absolute left-2 top-2 rounded bg-black/70 px-2 py-1 text-sm text-white">
                SAT: {satCount !== null ? `${satCount}` : "--"}
            </div>
        </div>
        <div class="mt-2 rounded bg-black/70 px-2 py-1 text-sm text-white">
            LAT: {markerPosition.lat !== null ? `${markerPosition.lat.toFixed(5)}` : "--.-----"},
            LNG: {markerPosition.lng !== null ? `${markerPosition.lng.toFixed(5)}` : "--.-----"}
        </div>
    </div>
</div>
