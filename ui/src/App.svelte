<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import { mqttData, connectMqtt, disconnectMqtt, getDataByPacket } from "./stores";
    import { MapLibre, NavigationControl, ScaleControl, GlobeControl, Marker } from 'svelte-maplibre-gl';
    import "maplibre-gl/dist/maplibre-gl.css";

    let markerPosition = { lng: -77.67641, lat: 43.08348 };
    let satCount = 0

    onMount(() => {
        connectMqtt();
    });

    onDestroy(() => {
        disconnectMqtt();
    });

    // Subscribe to MQTT data changes
    $: {
        const data = getDataByPacket($mqttData);
        const prefix = data.b
        if (prefix) {
            const gnsscoordinates = prefix.gnsscoordinates as
                | { latitude?: number; longitude?: number, sat_count?: number }
                | undefined;
            if (gnsscoordinates && typeof gnsscoordinates.latitude === 'number' && typeof gnsscoordinates.longitude === 'number') {
                markerPosition = { lng: gnsscoordinates.longitude, lat: gnsscoordinates.latitude };
            }
            if (gnsscoordinates && typeof gnsscoordinates.sat_count === 'number') {
                satCount = gnsscoordinates.sat_count
            }
        }
    }
</script>

<div class="relative flex w-full min-h-screen" style="">
    <div class="flex max-w-[40rem] flex-col gap-3 font-mono pr-[34rem]">
        {#each Object.entries(getDataByPacket($mqttData)) as [packetName, measurements] (packetName)}
            {@const packetMeasurements = measurements as Record<string, unknown>}
            <section class="rounded border border-gray-300 p-3">
                <h2 class="mb-2 font-bold">{packetName}</h2>
                {#each Object.entries(packetMeasurements) as [measurementName, measurementValue] (measurementName)}
                    <div>{measurementName}: <span>{JSON.stringify(measurementValue)}</span></div>
                {/each}
            </section>
        {/each}

    </div>
    <MapLibre
    class="absolute right-0 top-0 h-[20%] w-[20rem]"
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
    <div class="absolute right-[16rem] top-[14%] rounded bg-black/70 px-2 py-1 text-s text-white">
        Sat: {satCount !== null ? `${satCount}` : "--"}
    </div>
    <div class="absolute right-2 top-[20%] rounded bg-black/70 px-2 py-1 text-s text-white">
        LAT: {markerPosition.lat !== null ? `${markerPosition.lat.toFixed(5)}` : "--.-----"},
        LONG: {markerPosition.lng !== null ? `${markerPosition.lng.toFixed(5)}` : "--.-----"}
    </div>
</div>
