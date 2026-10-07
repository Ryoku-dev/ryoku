<script lang="ts">
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Field from "$lib/ui/Field.svelte";
  import Seg from "$lib/ui/Seg.svelte";
  import { formatCost, formatLatency, formatTokens, relativeTime } from "../format";
  import type { UsageRequestRow } from "../types";

  interface Props {
    rows: UsageRequestRow[];
    failuresOnly: boolean;
    loading?: boolean;
    onfilter: (failuresOnly: boolean) => void;
    oncopy: (row: UsageRequestRow) => void;
  }

  let { rows, failuresOnly, loading = false, onfilter, oncopy }: Props = $props();
  let query = $state("");
  let expanded = $state<number | null>(null);
  const filter = $derived(failuresOnly ? "failures" : "all");
  const filtered = $derived.by(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return rows;
    return rows.filter((row) => [
      row.platform,
      row.model,
      row.outcome,
      row.errorKind,
      row.error,
      row.class,
      row.effort,
      row.routedFrom,
    ].some((value) => value?.toLowerCase().includes(needle)));
  });
</script>

<Card title="Recent requests" gloss="履歴" lead="The route Prowl chose, its accounting quality, and any upstream refusal." pad={false}>
  {#snippet tools()}
    <Seg
      size="sm"
      label="Request outcome filter"
      options={[{ value: "all", label: "All" }, { value: "failures", label: "Failures" }]}
      value={filter}
      onchange={(value) => onfilter(value === "failures")}
    />
  {/snippet}
  <div class="request-search"><Field label="Search requests" placeholder="Provider, model, route or error" bind:value={query} /></div>
  {#if loading}
    <Empty title="Reading request history" body="Prowl is loading the most recent routed calls." />
  {:else if filtered.length === 0}
    <Empty
      title={failuresOnly ? "No failures recorded" : "No requests have crossed Prowl"}
      body={query ? "Clear the search to see the rest of this request history." : failuresOnly ? "Nothing needs debugging in the recent trail." : "Send a Quick request or use a connected harness, then return here."}
    />
  {:else}
    <div class="table-wrap">
      <table class="data requests">
        <thead><tr><th>When</th><th>Outcome</th><th>Provider · model</th><th>Route</th><th class="r">Tokens</th><th class="r">Latency</th><th class="r">Cost</th><th class="r">Attempts</th><th>Error kind</th><th></th></tr></thead>
        <tbody>
          {#each filtered as row (row.id)}
            <tr class:failed={row.outcome !== "success"}>
              <td class="mono">{relativeTime(row.createdAt)}</td>
              <td><span class="outcome">{row.outcome}</span></td>
              <td class="route"><strong>{row.platform}</strong><span>{row.model}</span></td>
              <td>{[row.routedFrom, row.class, row.effort].filter(Boolean).join(" · ") || "—"}</td>
              <td class="r mono">{row.usageQuality === "unavailable" ? "—" : `${row.estimated || row.usageQuality === "estimated" ? "~" : ""}${formatTokens(row.inputTokens)} / ${formatTokens(row.outputTokens)}`}</td>
              <td class="r mono">{formatLatency(row.latencyMs)}</td>
              <td class="r mono">{formatCost(row.costUsd, row.costKnown)}</td>
              <td class="r mono">{row.attempts}</td>
              <td>{row.errorKind || "—"}</td>
              <td><Button size="sm" variant="quiet" onclick={() => (expanded = expanded === row.id ? null : row.id)}>{expanded === row.id ? "Close" : "Read"}</Button></td>
            </tr>
            {#if expanded === row.id}
              <tr class="detail-row">
                <td colspan="10">
                  <div class="detail">
                    <dl>
                      <div><dt>Status</dt><dd>{row.status || "—"}</dd></div>
                      <div><dt>Route</dt><dd>{row.class || "—"} · {row.effort || "—"}{row.routedFrom ? ` · from ${row.routedFrom}` : ""}</dd></div>
                      <div><dt>Tries</dt><dd>{row.attempts} attempt{row.attempts === 1 ? "" : "s"} · {formatLatency(row.latencyMs)}</dd></div>
                    </dl>
                    <pre>{row.error || `The upstream reported no message; the outcome was ${row.outcome}.`}</pre>
                    {#if row.error}<Button size="sm" icon="copy" onclick={() => oncopy(row)}>Copy error</Button>{/if}
                  </div>
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</Card>

<style>
  .request-search { max-width: 420px; padding: var(--s3) var(--s5); border-bottom: 1px solid var(--line-soft); }
  .table-wrap { overflow-x: auto; padding: 0 var(--s5) var(--s4); }
  .requests { min-width: 1120px; }
  .requests td { vertical-align: middle; }
  .requests tr.failed .outcome, .requests tr.failed td:nth-last-child(2) { color: var(--alert); }
  .outcome { text-transform: capitalize; }
  .route { max-width: 220px; }
  .route strong, .route span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .route strong { color: var(--ink); font-weight: 500; text-transform: capitalize; }
  .route span { margin-top: 2px; color: var(--ink-mute); font-family: var(--mono); font-size: var(--f-tiny); }
  .detail-row:hover { background: transparent; }
  .detail-row td { padding: 0 var(--s3) var(--s4); }
  .detail { display: grid; gap: var(--s3); padding: var(--s4); border: 1px solid var(--line-soft); border-radius: var(--radius); background: var(--paper-lift); }
  .detail dl { display: flex; flex-wrap: wrap; gap: var(--s3) var(--s5); }
  .detail dl div { display: flex; gap: var(--s2); }
  .detail dt { color: var(--ink-faint); }
  .detail dd { margin: 0; color: var(--ink-dim); }
  .detail pre { max-height: 220px; overflow: auto; white-space: pre-wrap; color: var(--ink); font-family: var(--mono); font-size: var(--f-small); line-height: 1.5; }
  .detail :global(button) { justify-self: start; }
</style>
