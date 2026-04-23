<!-- add time since last transmission, add url parameters for: add customizable background color, add call sign, ground station location -->
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

    const DEFAULT_GROUND_STATION = { lon: -77.67641, lat: 43.08348 };
    const DEFAULT_BACKGROUND_COLOR = "#ffffff";
    const DEFAULT_CALL_SIGN = "KE2EGW";

    let groundStationPosition = { lon: -77.67641, lat: 43.08348 };
    let backgroundColor = DEFAULT_BACKGROUND_COLOR;
    let callSign = DEFAULT_CALL_SIGN;

    function parseGroundStationParam(value: string | null) {
        if (!value) return null;
        const [latRaw, lonRaw] = value.split(",").map((part) => part.trim());
        if (!latRaw || !lonRaw) return null;

        const lat = Number(latRaw);
        const lon = Number(lonRaw);
        if (!Number.isFinite(lat) || !Number.isFinite(lon)) return null;
        if (lat < -90 || lat > 90 || lon < -180 || lon > 180) return null;

        return { lat, lon };
    }

    function applyUrlParameters() {
        if (typeof window === "undefined") return;

        const query = new URLSearchParams(window.location.search);
        const bgParam = query.get("bg");
        const callSignParam = query.get("callSign");
        const groundStationParam = query.get("groundStation");

        if (bgParam && /^#(?:[0-9a-fA-F]{3}){1,2}$/.test(bgParam)) {
            backgroundColor = bgParam;
        }

        if (callSignParam) {
            callSign = callSignParam;
        }

        const parsedGroundStation = parseGroundStationParam(groundStationParam);
        if (parsedGroundStation) {
            groundStationPosition = parsedGroundStation;
        }
    }

    let rocketPosition = { ...DEFAULT_GROUND_STATION };
    let satCount: number | null = null;
    let altitude: number | null = null;
    let altitudeMax: number = 0.0;
    let battCurrent: number | null = null;
    let battVoltage: number | null = null;
    let receiverSnr: number | null = null;
    let temperature: number | null = null;
    let accelX: number | null = null;
    let accelY: number | null = null;
    let accelZ: number | null = null;
    let gForce: number | null = null;
    let gForceMax: number = 0.0;
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
    let lastTransmission: number | null = null;
    let nowMs = Date.now();
    let secondsSinceLastTransmission: number | null = null;
    let nowTimer: ReturnType<typeof setInterval> | null = null;

    onMount(() => {
        applyUrlParameters();
        connectMqtt();
        startSineWaveMqttGenerator({ intervalMs: 3000, channel: mqttChannel });
        nowTimer = setInterval(() => {
            nowMs = Date.now();
        }, 500);
    });

    onDestroy(() => {
        if (nowTimer) {
            clearInterval(nowTimer);
            nowTimer = null;
        }
        stopSineWaveMqttGenerator();
        disconnectMqtt();
    });

    $: altitudePercent = altitude === null
        ? 0
        : Math.max(0, Math.min(100, (altitude / ALTITUDE_MAX) * 100));

    $: altitudeMax = Math.max(altitudeMax, altitudePercent)
    
    $: gForce = accelX !== null && accelY !== null && accelZ !== null
        ? Math.sqrt(accelX ** 2 + accelY ** 2 + accelZ ** 2)
        : null;

    $: gForcePercent = gForce === null
        ? 0
        : Math.max(0, Math.min(100, (gForce / G_FORCE_MAX) * 100));
        
    $: gForceMax = Math.max(gForceMax, gForcePercent)

    $: topLeftMetricStubs = [
        { label: "mA", value: battCurrent !== null ? `${(battCurrent * 1000).toFixed(1)}` : "--" },
        { label: "dB", value: receiverSnr !== null ? `${receiverSnr.toFixed(1)}` : "--" },
        { label: "V", value: battVoltage !== null ? `${battVoltage.toFixed(2)}` : "--" },
        { label: "°C", value: temperature !== null ? `${temperature.toFixed(2)}` : "--" }
    ];

    $: secondsSinceLastTransmission = lastTransmission === null
        ? null
        : Math.max(0, Math.floor((nowMs - lastTransmission) / 1000));

    // Subscribe to MQTT data changes
    $: {
        const data = getDataByPacket($mqttData) as Record<string, unknown>;
        const prefix = getChannelPayload(data, mqttChannel);
        if (prefix) {
            lastTransmission = Date.now();
            const gnsscoordinates = getObjectCaseInsensitive(prefix, "gnsscoordinates");
            const powermodule = getObjectCaseInsensitive(prefix, "powermodule");
            const receiverstats = getObjectCaseInsensitive(prefix, "receiverstats");
            const sensormodule = getObjectCaseInsensitive(prefix, "sensormodule");

            const latitude = getNumberCaseInsensitive(gnsscoordinates, "latitude");
            const longitude = getNumberCaseInsensitive(gnsscoordinates, "longitude");
            if (latitude !== null && longitude !== null) {
                rocketPosition = { lon: longitude, lat: latitude };
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

<div class="relative flex w-full min-h-screen" style="background-color: {backgroundColor};">
    <div class="absolute left-0 top-0 z-50 w-64 rounded border border-gray-300 bg-white/95 p-2 shadow">
        <div class="grid grid-cols-2 gap-2">
            {#each topLeftMetricStubs as metric (metric.label)}
                <div class="rounded border border-gray-200 bg-gray-50 p-2">
                    <div class="text-[10px] font-semibold tracking-wide text-gray-600"></div>
                    <div class="mt-1 text-base font-mono font-bold text-gray-900">{metric.value} {metric.label}</div>
                </div>
            {/each}
        </div>
    </div>

    <div
        class="font-guardians absolute left-1/2 top-0 z-60 flex h-20 w-72 -translate-x-1/2 items-center justify-center bg-red-600/90 text-4xl font-black tracking-widest text-white shadow-lg"
        style="clip-path: polygon(0% 0%, 100% 0%, 80% 100%, 20% 100%);"
    >
        RISK
    </div>

    <div
        class="font-guardians absolute left-1/7 right-1/7 bottom-0 z-60 flex h-20 items-center justify-center bg-red-600/90 text-3xl font-normal text-white shadow-lg"
        style="clip-path: polygon(95% 0%, 5% 0%, 0% 100%, 100% 100%);"
    >
        <div class="flex w-full items-center">
            <span class="w-1/2 pr-12 text-right">RIT LAUNCH</span>
            <span class="w-1/2 pl-12 text-left">INITIATIVE</span>
        </div>
        <img
            src="/ritlaunch.png"
            alt="RIT Launch logo"
            class="pointer-events-none absolute left-1/2 h-15 w-15 -translate-x-1/2 object-contain"
        />
    </div>

    <div class="absolute bottom-10 left-2 top-[13rem] z-50 rounded bg-black/70 px-2 py-1 text-white">
        <div class="flex h-full flex-col items-center">
            <div class="mb-2 text-center text-xs font-semibold">Altitude</div>
            <div class="relative w-3 flex-1 rounded bg-white/20">
                <div
                    class="absolute left-1/2 h-3 w-3 -translate-x-1/2 rounded-full border border-white bg-red-200"
                    style={`bottom: calc(${altitudeMax}% - 0.375rem)`}
                ></div>
                <div
                    class="absolute bottom-0 left-0 w-full rounded bg-red-500"
                    style={`height: ${altitudePercent}%`}
                ></div>
                <div
                    class="absolute left-1/2 h-3 w-3 -translate-x-1/2 rounded-full border border-white bg-red-200"
                    style={`bottom: calc(${altitudePercent}% - 0.375rem)`}
                ></div>
            </div>
            <div class="mt-2 text-center text-xs tabular-nums">{altitude !== null ? `${altitude.toFixed(0)}ft` : "--"}</div>
        </div>
    </div>

    <div class="absolute bottom-10 right-2 top-[13rem] z-50 rounded bg-black/70 px-2 py-1 text-white">
        <div class="flex h-full flex-col items-center">
            <div class="mb-2 text-center text-xs font-semibold">G Force</div>
            <div class="relative w-3 flex-1 rounded bg-white/20">
                <div
                    class="absolute left-1/2 h-3 w-3 -translate-x-1/2 rounded-full border border-white bg-red-200"
                    style={`bottom: calc(${gForceMax}% - 0.375rem)`}
                ></div>
                <div
                    class="absolute bottom-0 left-0 w-full rounded bg-red-500"
                    style={`height: ${gForcePercent}%`}
                ></div>
                <div
                    class="absolute left-1/2 h-3 w-3 -translate-x-1/2 rounded-full border border-white bg-red-200"
                    style={`bottom: calc(${gForcePercent}% - 0.375rem)`}
                ></div>
            </div>
            <div class="mt-2 text-center text-xs tabular-nums">
                {gForce !== null ? `${gForce.toFixed(2)}g` : "--"}
            </div>
        </div>
    </div>

    <div class="absolute left-2 bottom-1 z-10 w-40 flex">
        <div class="mt-2 rounded bg-black/70 px-2 py-1 text-sm text-white">
            {callSign ? callSign : "------"}
        </div>
    </div>

    <div class="absolute right-2 bottom-1 z-10 w-30">
        <div class="mt-2 rounded bg-black/70 px-2 py-1 text-sm text-white">
            Last trans: {secondsSinceLastTransmission !== null ? `${secondsSinceLastTransmission}s` : "------"}
        </div>
    </div>
    
    <div class="absolute right-0 top-0 z-10 w-70">
        <div class="relative h-40 w-70">
            <MapLibre
            class="h-40 w-70"
            style="/src/style.aliflux.json"
            zoom={10.0}
            center={groundStationPosition}
            attributionControl={false}
            >
                <Marker lnglat={rocketPosition} anchor="bottom">
                    <img src="/ritlaunch.png"
                    alt="Launch logo"
                    class="h-10 w-10 object-contain"
                    />
                </Marker>
                <Marker lnglat={groundStationPosition} anchor="bottom">
                    <img src="/ritlaunch.png"
                    alt="Launch logo"
                    class="h-10 w-10 object-contain"
                    />
                </Marker>

            </MapLibre>
            <div class="absolute left-2 top-2 rounded bg-black/70 px-2 py-1 text-sm text-white">
                SAT: {satCount !== null ? `${satCount}` : "--"}
            </div>
        </div>
        <div class="mt-2 rounded bg-black/70 px-2 py-1 text-sm text-white">
            LAT: {rocketPosition.lat !== null ? `${rocketPosition.lat.toFixed(5)}` : "--.-----"},
            LON: {rocketPosition.lon !== null ? `${rocketPosition.lon.toFixed(5)}` : "--.-----"}
        </div>
    </div>
</div>
