<script lang="ts">
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Field from "$lib/ui/Field.svelte";
  import { parseInstant } from "../format";
  import type { ServerLog } from "../types";

  interface Props {
    logs: ServerLog[];
    loading?: boolean;
    onclear: () => void;
  }

  let { logs, loading = false, onclear }: Props = $props();
  let query = $state("");
  const filtered = $derived.by(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return logs;
    return logs.filter((entry) => [entry.level, entry.source, entry.provider, entry.model, entry.event, entry.message]
      .some((value) => value.toLowerCase().includes(needle)));
  });

  function clock(timestamp: number): string {
    const date = parseInstant(timestamp);
    return date ? date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }) : "—";
  }
</script>

<Card title="Gateway logs" gloss="記録" lead="The durable warning and error tail from Prowl's gateway." pad={false}>
  {#snippet tools()}<Button size="sm" variant="quiet" armed={logs.length > 0} onclick={onclear}>Clear logs</Button>{/snippet}
  <div class="log-search"><Field label="Search logs" placeholder="Provider, event or message" bind:value={query} /></div>
  {#if loading}
    <Empty title="Reading gateway logs" body="Prowl is loading its durable warning and error tail." />
  {:else if filtered.length === 0}
    <Empty
      title={query ? "No logs match" : "Router quiet"}
      body={query ? "Clear the search to see the rest of the gateway tail." : "No gateway warnings or errors are stored."}
    />
  {:else}
    <div class="log-list">
      {#each filtered as entry (entry.id)}
        <article>
          <time>{clock(entry.ts)}</time>
          <span class:bad={entry.level === "error"} class="level">{entry.level}</span>
          <div><strong>{entry.provider || entry.source}{entry.event ? ` · ${entry.event}` : ""}</strong><p>{entry.message}</p></div>
          <code>{entry.model}</code>
        </article>
      {/each}
    </div>
  {/if}
</Card>

<style>
  .log-search { max-width: 420px; padding: var(--s3) var(--s5); border-bottom: 1px solid var(--line-soft); }
  .log-list { max-height: 360px; overflow: auto; padding: 0 var(--s5) var(--s4); }
  article { display: grid; grid-template-columns: 74px 68px minmax(180px, 1fr) minmax(100px, .4fr); align-items: start; gap: var(--s3); padding: var(--s3) 0; border-bottom: 1px solid var(--line-soft); }
  article:last-child { border-bottom: 0; }
  time, code { color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-tiny); }
  .level { color: var(--ink-mute); font-family: var(--mono); font-size: var(--f-micro); text-transform: uppercase; }
  .level.bad { color: var(--alert); }
  article strong { color: var(--ink); font-size: var(--f-small); font-weight: 500; }
  article p { margin-top: var(--s1); color: var(--ink-mute); font-size: var(--f-small); line-height: 1.45; overflow-wrap: anywhere; }
  article code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; text-align: right; }
  @media (max-width: 720px) { article { grid-template-columns: 64px 60px minmax(0, 1fr); } article code { display: none; } }
</style>
