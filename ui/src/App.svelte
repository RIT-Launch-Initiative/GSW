<script lang="ts">
    import mqtt from "mqtt";
    import { onDestroy } from "svelte";
    import { SvelteMap } from "svelte/reactivity";
    let state: SvelteMap<string, unknown> = new SvelteMap();
    const client = mqtt.connect("ws://localhost:1880");
    client.on("connect", async () => {
        console.log("connected, subscribing to gsw topic");
        await client.subscribeAsync("gsw/#");
    });
    client.on("message", (topic, body) => {
        const value = JSON.parse(body.toString());
        state.set(topic, value);
    });
    onDestroy(() => {
        client.end(true, () => {
            console.log("closed client");
        });
    });
</script>

<div class="flex p-4">
    <div class="flex flex-col gap-2">
        {#each state.keys() as topic (topic)}
            <div class="font-mono">{topic}: <span>{state.get(topic)}</span></div>
        {/each}
    </div>
</div>
