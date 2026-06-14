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
    import { MapLibre, Marker } from "svelte-maplibre-gl";
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
        const mockMqttParam = query.get("mockMqtt");
        const mqttAddressParam = query.get("mqttAddress");
        const mqttChannelParam = query.get("mqttChannel");
        const teamNumberParam = query.get("teamNumber");
        const altitudeOffsetParam = query.get("altitudeOffset");

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

        if (mockMqttParam && mockMqttParam.toLowerCase() !== "false") {
            mockMqtt = true;
        }

        if (mqttAddressParam) {
            mqttAddress = mqttAddressParam;
        }

        if (mqttChannelParam && /^\d+$/.test(mqttChannelParam)) {
            mqttChannel = mqttChannelParam;
        }

        if (teamNumberParam) {
            teamNumber = teamNumberParam;
        }

        if (altitudeOffsetParam) {
            const parsed = Number(altitudeOffsetParam);
            if (Number.isFinite(parsed)) {
                altitudeOffsetFt = parsed;
            }
        }
    }

    let rocketPosition = { ...DEFAULT_GROUND_STATION };
    let satCount: number | null = null;
    let altitude: number | null = null;
    let altitudePeak: number | null = null;
    let altitudeMax: number = 0.0;
    let battCurrent: number | null = null;
    let battVoltage: number | null = null;
    let receiverSnr: number | null = null;
    let temperature: number | null = null;
    let accelX: number | null = null;
    let accelY: number | null = null;
    let accelZ: number | null = null;
    let gForce: number | null = null;
    let gForcePeak: number | null = null;
    let gForceMax: number = 0.0;
    const ALTITUDE_MAX = 13000;
    const G_FORCE_MAX = 20;
    const altitudeTicks = [
        { value: 0, label: "0" },
        { value: 2500, label: "2,500" },
        { value: 5000, label: "5,000" },
        { value: 7500, label: "7,500" },
        { value: 10000, label: "10,000", emphasis: true },
        { value: 12500, label: "12,500" },
        { value: 13000, label: "13,000" }
    ];
    const gForceTicks = [
        { value: 0, label: "0" },
        { value: 5, label: "5" },
        { value: 10, label: "10" },
        { value: 15, label: "15" },
        { value: 20, label: "20" }
    ];
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
    let mockMqtt = false;
    let mqttAddress = "";
    let mqttChannel = "0";
    let teamNumber = "2";
    let altitudeOffsetFt = 0;

    onMount(() => {
        applyUrlParameters();
        connectMqtt(mqttAddress || undefined);
        if (mockMqtt) {
            startSineWaveMqttGenerator({ intervalMs: 3000, channel: mqttChannel });
        }
        nowTimer = setInterval(() => {
            nowMs = Date.now();
        }, 500);
    });

    onDestroy(() => {
        if (nowTimer) {
            clearInterval(nowTimer);
            nowTimer = null;
        }
        if (mockMqtt) {
            stopSineWaveMqttGenerator();
        }

        disconnectMqtt();
    });

    $: altitudePercent = altitude === null
        ? 0
        : Math.max(0, Math.min(100, (altitude / ALTITUDE_MAX) * 100));

    $: if (altitude !== null) {
        altitudePeak = altitudePeak === null ? altitude : Math.max(altitudePeak, altitude);
    }

    $: altitudeMax = altitudePeak === null
        ? 0
        : Math.max(0, Math.min(100, (altitudePeak / ALTITUDE_MAX) * 100));
    
    $: gForce = accelX !== null && accelY !== null && accelZ !== null
        ? Math.sqrt(accelX ** 2 + accelY ** 2 + accelZ ** 2)
        : null;

    $: gForcePercent = gForce === null
        ? 0
        : Math.max(0, Math.min(100, (gForce / G_FORCE_MAX) * 100));

    $: if (gForce !== null) {
        gForcePeak = gForcePeak === null ? gForce : Math.max(gForcePeak, gForce);
    }

    $: gForceMax = gForcePeak === null
        ? 0
        : Math.max(0, Math.min(100, (gForcePeak / G_FORCE_MAX) * 100));

    $: topLeftMetricStubs = [
        { label: "mA", value: battCurrent !== null ? `${(battCurrent * 1000).toFixed(1)}` : "--" },
        { label: "dB", value: receiverSnr !== null ? `${receiverSnr.toFixed(1)}` : "--" },
        { label: "V", value: battVoltage !== null ? `${battVoltage.toFixed(2)}` : "--" },
        { label: "°C", value: temperature !== null ? `${temperature.toFixed(2)}` : "--" }
    ];

    $: secondsSinceLastTransmission = lastTransmission === null
        ? null
        : Math.max(0, Math.floor((nowMs - lastTransmission) / 1000));

    function meterPercent(value: number, max: number) {
        return Math.max(0, Math.min(100, (value / max) * 100));
    }

    // Standard barometric formula. Assumes pressure in kPa, returns feet.
    function pressureToAltitudeFt(pressureKpa: number): number {
        const altMeters = 44330 * (1 - Math.pow(pressureKpa / 101.325, 1 / 5.255));
        return altMeters * 3.28084;
    }

    // Subscribe to MQTT data changes
    $: {
        const data = getDataByPacket($mqttData) as Record<string, unknown>;
        // Channel-based lookup (mock: gsw/{channel}/{packetType} → object) falls back to
        // direct packet-name lookup (real: gsw/{PacketName}/{field} → scalar).
        const lookup = getChannelPayload(data, mqttChannel) ?? data;

        const gnsscoordinates = getObjectCaseInsensitive(lookup, "gnsscoordinates");
        const powermodule = getObjectCaseInsensitive(lookup, "powermodule");
        const receiverstats = getObjectCaseInsensitive(lookup, "receiverstats");
        const sensormodule = getObjectCaseInsensitive(lookup, "sensormodule");

        if (gnsscoordinates || powermodule || receiverstats || sensormodule) {
            lastTransmission = Date.now();

            const latitude = getNumberCaseInsensitive(gnsscoordinates, "latitude");
            const longitude = getNumberCaseInsensitive(gnsscoordinates, "longitude");
            if (latitude !== null && longitude !== null) {
                rocketPosition = { lon: longitude, lat: latitude };
            }

            satCount = getNumberCaseInsensitive(gnsscoordinates, "sat_count");
            const pressureKpa = getNumberCaseInsensitive(sensormodule, "PRESS_BMP388", "PRESS_MS5611")
                ?? getNumberCaseInsensitive(gnsscoordinates, "altitude");
            altitude = pressureKpa !== null ? pressureToAltitudeFt(pressureKpa) - altitudeOffsetFt : null;
            battCurrent = getNumberCaseInsensitive(powermodule, "CURR_BATT");
            battVoltage = getNumberCaseInsensitive(powermodule, "VOLT_BATT");
            receiverSnr = getNumberCaseInsensitive(receiverstats, "SNR", "RCV_SNR", "snr");
            temperature = getNumberCaseInsensitive(sensormodule, "temperature", "TEMP_BMP388", "TEMP_MS5611", "TEMP_TMP117");
            const rawAccelX = getNumberCaseInsensitive(sensormodule, "LSM_ACCEL_X", "accel_x", "ACCEL_X", "ADX_ACCEL_X");
            const rawAccelY = getNumberCaseInsensitive(sensormodule, "LSM_ACCEL_Y", "accel_y", "ACCEL_Y", "ADX_ACCEL_Y");
            const rawAccelZ = getNumberCaseInsensitive(sensormodule, "LSM_ACCEL_Z", "accel_z", "ACCEL_Z", "ADX_ACCEL_Z");
            accelX = rawAccelX !== null ? rawAccelX / 9.81 : null;
            accelY = rawAccelY !== null ? rawAccelY / 9.81 : null;
            accelZ = rawAccelZ !== null ? rawAccelZ / 9.81 : null;
        }
    }
</script>

<div class="relative flex w-full min-h-screen" style="background-color: {backgroundColor};">
    <div class="absolute left-0 top-0 z-50 w-64 rounded border border-gray-300 bg-black p-2 shadow">
        <div class="grid grid-cols-2 gap-2">
            {#each topLeftMetricStubs as metric (metric.label)}
                <div class="rounded border border-black bg-white/20 p-2">
                    <div class="text-[10px] font-semibold tracking-wide text-white"></div>
                    <div class="mt-1 text-base font-mono font-bold text-white">{metric.value} {metric.label}</div>
                </div>
            {/each}
        </div>
    </div>

    <div
        class="font-guardians absolute left-1/2 top-0 z-60 flex h-15 w-95 -translate-x-1/2 items-center justify-center bg-red-600 text-3xl font-black tracking-widest text-white shadow-lg"
        style="clip-path: polygon(0% 0%, 100% 0%, 90% 100%, 10% 100%);"
    >
        RISK #{teamNumber}
    </div>

    <div
        class="font-guardians absolute left-1/7 right-1/7 bottom-0 z-60 flex h-20 items-center justify-center bg-red-600 text-3xl font-normal text-white shadow-lg"
        style="clip-path: polygon(95% 0%, 5% 0%, 0% 100%, 100% 100%);"
    >
        <div class="flex w-full items-center">
            <span class="w-1/2 pr-12 text-right">RIT LAUNCH</span>
            <span class="w-1/2 pl-12 text-left">INITIATIVE</span>
        </div>
        <img
            src="/launchwhite.png"
            alt="RIT Launch logo"
            class="pointer-events-none absolute left-1/2 h-15 w-15 -translate-x-1/2 object-contain"
        />
    </div>

    <div class="absolute bottom-10 left-2 top-[13rem] z-50 rounded bg-black px-2 py-1 text-white">
        <div class="flex h-full flex-col items-start pl-2 pr-5">
            <div class="mb-2 text-center text-xs font-semibold">Altitude</div>
            <div class="relative w-3 flex-1 rounded bg-white/20">
                {#each altitudeTicks as tick (tick.value)}
                    <div
                        class={`absolute left-full ml-1 flex items-center gap-1 ${tick.emphasis ? "font-black text-red-300" : "text-white/80"}`}
                        style={`bottom: calc(${meterPercent(tick.value, ALTITUDE_MAX)}% - 0.5px)`}
                    >
                        <div class={`h-px ${tick.emphasis ? "w-3 bg-red-300" : "w-2 bg-white/70"}`}></div>
                        <span class="text-[10px] leading-none tabular-nums">{tick.label}</span>
                    </div>
                {/each}
                <div
                    class="absolute left-1/2 h-3 w-3 -translate-x-1/2 rounded-full border border-white bg-red-200"
                    style={`bottom: calc(${altitudeMax}% - 0.375rem)`}
                ></div>
                <div
                    class="absolute left-full ml-2 -translate-y-1/2 rounded bg-red-600/80 px-1 py-[1px] text-[10px] font-bold leading-none tabular-nums text-white"
                    style={`bottom: calc(${altitudeMax}% - 0.375rem)`}
                >
                    MAX {altitudePeak !== null ? `${altitudePeak.toFixed(0)}ft` : "--"}
                </div>
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

    <div class="absolute bottom-10 right-2 top-[13rem] z-50 rounded bg-black px-2 py-1 text-white">
        <div class="flex h-full flex-col items-end pl-2 pr-2">
            <div class="mb-2 text-center text-xs font-semibold">G Force</div>
            <div class="relative w-3 flex-1 rounded bg-white/20">
                {#each gForceTicks as tick (tick.value)}
                    <div
                        class="absolute right-full mr-1 flex items-center gap-1 text-white/80"
                        style={`bottom: calc(${meterPercent(tick.value, G_FORCE_MAX)}% - 0.5px)`}
                    >
                        <span class="text-[10px] leading-none tabular-nums">{tick.label}</span>
                        <div class="h-px w-2 bg-white/70"></div>
                    </div>
                {/each}
                <div
                    class="absolute left-1/2 h-3 w-3 -translate-x-1/2 rounded-full border border-white bg-red-200"
                    style={`bottom: calc(${gForceMax}% - 0.375rem)`}
                ></div>
                <div
                    class="absolute right-full mr-2 -translate-y-1/2 rounded bg-red-600/80 px-1 py-[1px] text-[10px] font-bold leading-none tabular-nums text-white"
                    style={`bottom: calc(${gForceMax}% - 0.375rem)`}
                >
                    MAX {gForcePeak !== null ? `${gForcePeak.toFixed(2)}g` : "--"}
                </div>
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
        <div class="mt-2 rounded bg-black px-2 py-1 text-sm text-white">
            {callSign ? callSign : "------"}
        </div>
    </div>

    <div class="absolute right-2 bottom-1 z-10 w-30">
        <div class="mt-2 rounded bg-black px-2 py-1 text-sm text-white">
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
            <div class="absolute left-2 top-2 rounded bg-black px-2 py-1 text-sm text-white">
                SAT: {satCount !== null ? `${satCount}` : "--"}
            </div>
            <div class="absolute left-2 bottom-2 mt-2 rounded bg-black px-2 py-1 text-sm text-white">
                LAT: {rocketPosition.lat !== null ? `${rocketPosition.lat.toFixed(5)}` : "--.-----"},
                LON: {rocketPosition.lon !== null ? `${rocketPosition.lon.toFixed(5)}` : "--.-----"}
            </div>
        </div>
    </div>
</div>
