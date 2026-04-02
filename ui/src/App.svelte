<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import { mqttData, connectMqtt, disconnectMqtt, getDataByPacket } from "./stores";
    import { MapLibre, NavigationControl, ScaleControl, GlobeControl, Marker } from 'svelte-maplibre-gl';
    import "maplibre-gl/dist/maplibre-gl.css";

    let markerPosition = { lng: -77.67641, lat: 43.08348 };

    onMount(() => {
        connectMqtt();
    });

    onDestroy(() => {
        disconnectMqtt();
    });

    // Subscribe to MQTT data changes
    $: {
        const data = getDataByPacket($mqttData);
        const gnsscoordinates = data.gnsscoordinates as
            | { latitude?: number; longitude?: number }
            | undefined;
        if (gnsscoordinates && typeof gnsscoordinates.latitude === 'number' && typeof gnsscoordinates.longitude === 'number') {
            markerPosition = { lng: gnsscoordinates.longitude, lat: gnsscoordinates.latitude };
        }
    }
</script>

<div class="flex w-full" style="">
    <div class="flex max-w-[40rem] flex-col gap-3 font-mono">
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
    class="h-[10pc] w-[5pc] min-h-[20%] min-w-[30%] right-[0px] top-[0px]"
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
</div>
