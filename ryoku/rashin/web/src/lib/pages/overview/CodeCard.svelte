<script lang="ts">
  import Card from "$lib/ui/Card.svelte";
  import Button from "$lib/ui/Button.svelte";
  import Chip from "$lib/ui/Chip.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Field from "$lib/ui/Field.svelte";
  import type { CodeStatus, ProwlDaemonStatus, ProwlHit } from "./types";
  import { formatTokens, repoName } from "./types";

  interface Props {
    daemon: ProwlDaemonStatus | null;
    status: CodeStatus | null;
    loading: boolean;
    error?: string;
    hits: ProwlHit[] | null;
    searching: boolean;
    searchError?: string;
    onsearch: (query: string) => void;
  }

  let { daemon, status, loading, error, hits, searching, searchError, onsearch }: Props = $props();
  let query = $state("");
  const saved = $derived(status?.savings.saved_tokens ?? 0);
  const coverage = $derived.by(() => {
    const chunks = Math.max(0, status?.semantic.chunks ?? 0);
    const embedded = Math.min(chunks, Math.max(0, status?.semantic.embedded ?? 0));
    return chunks > 0 ? Math.round((embedded / chunks) * 100) : 0;
  });

  function submit(event: SubmitEvent): void {
    event.preventDefault();
    const next = query.trim();
    if (next) onsearch(next);
  }
</script>

<Card title="Code intelligence" gloss="索引" lead="Measured context the index kept out of agent prompts." class="code-card">
  {#snippet tools()}
    {#if daemon?.repo}<Chip mono>{repoName(daemon.repo)}{daemon.running ? " · live" : " · stopped"}</Chip>{/if}
  {/snippet}

  {#if loading}
    <p class="loading">Reading the code index…</p>
  {:else if error}
    <Empty title="Code intelligence is unavailable" body={`${error} Check Prowl's gateway, then reload this sheet.`} icon="search" />
  {:else if !daemon?.installed}
    <Empty title="Prowl is not installed" body="Install Prowl to index code and measure how much cited answers save." icon="search" />
  {:else if !daemon.running}
    <Empty title="Prowl is not running" body={daemon.error || "Start Rashin to bring up Prowl's gateway, then reload this sheet."} icon="search" />
  {:else if !status}
    <Empty title="No live index yet" body={`Add ${repoName(daemon.repo) || "this repository"} under Prowl > Projects to build its index.`} icon="search" />
  {:else}
    <div class="saved">
      <strong>{formatTokens(saved)}</strong>
      <span>{saved ? "tokens saved by indexed answers" : "No measured savings yet. Indexed answers record themselves here."}</span>
    </div>

    <div class="ledger" aria-label="Code intelligence measurements">
      <div><strong>{status.counts.files}</strong><span>Files indexed</span></div>
      <div><strong>{status.counts.symbols}</strong><span>Symbols mapped</span></div>
      <div><strong>{status.savings.queries}</strong><span>Indexed answers</span></div>
      <div><strong>{formatTokens(status.savings.answer_tokens)}</strong><span>Answer tokens</span></div>
    </div>

    <div class="signals">
      <Chip tone={status.semantic.complete ? "plate" : "line"}>Semantic {coverage}%</Chip>
      <Chip>{status.counts.edges} dependency edges</Chip>
      <Chip>{status.last_index ? "Index current" : "Awaiting first index"}</Chip>
    </div>

    <form class="search" onsubmit={submit}>
      <Field label="Search the indexed source" placeholder="Describe a symbol or behavior" bind:value={query} autocomplete="off" />
      <Button type="submit" variant="plate" icon="search" armed={!!query.trim()} busy={searching}>Search</Button>
    </form>

    {#if searchError}
      <p class="search-error">{searchError} Try a shorter query or check Prowl's gateway.</p>
    {:else if hits !== null}
      <div class="results" aria-live="polite">
        {#if hits.length}
          {#each hits as hit (`${hit.file}:${hit.start_line}:${hit.snippet ?? ""}`)}
            <article>
              <p><code>{hit.file}:{hit.start_line}{hit.end_line > hit.start_line ? `-${hit.end_line}` : ""}</code></p>
              <span>{hit.snippet || "Match found in the indexed source."}</span>
            </article>
          {/each}
        {:else}
          <Empty title="No cited matches" body="Try a symbol name or a more specific behavior." icon="search" />
        {/if}
      </div>
    {/if}
  {/if}
</Card>

<style>
  :global(.code-card) { min-height: 100%; }
  .loading { color: var(--ink-mute); }
  .saved { display: flex; align-items: baseline; gap: var(--s4); padding-bottom: var(--s5); border-bottom: 1px solid var(--line-soft); }
  .saved strong { flex: none; font-family: var(--display); font-size: clamp(46px, 6vw, 76px); font-weight: 300; line-height: .9; letter-spacing: -.04em; color: var(--ink); font-variant-numeric: tabular-nums; }
  .saved span { max-width: 27ch; color: var(--ink-mute); font-size: var(--f-small); }
  .ledger { display: grid; grid-template-columns: repeat(4, 1fr); margin: var(--s5) 0; border-block: 1px solid var(--line-soft); }
  .ledger div { display: flex; flex-direction: column; gap: var(--s1); padding: var(--s3); border-right: 1px solid var(--line-soft); }
  .ledger div:first-child { padding-left: 0; }
  .ledger div:last-child { border-right: 0; }
  .ledger strong { color: var(--ink); font-size: 18px; font-weight: 500; }
  .ledger span { color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-tiny); letter-spacing: var(--track-label); text-transform: uppercase; }
  .signals { display: flex; flex-wrap: wrap; gap: var(--s2); }
  .search { display: grid; grid-template-columns: 1fr auto; align-items: end; gap: var(--s2); margin-top: var(--s5); padding-top: var(--s5); border-top: 1px solid var(--line-soft); }
  .search-error { margin-top: var(--s3); color: var(--alert); font-size: var(--f-small); }
  .results { display: flex; flex-direction: column; gap: var(--s2); margin-top: var(--s4); }
  .results article { padding: var(--s3); border: 1px solid var(--line-soft); border-radius: var(--radius); background: var(--tint5); }
  .results code { color: var(--ink); font-size: var(--f-micro); }
  .results span { display: block; margin-top: var(--s1); color: var(--ink-mute); font-size: var(--f-small); line-height: 1.45; }
  @media (max-width: 680px) {
    .saved { align-items: flex-start; flex-direction: column; }
    .ledger { grid-template-columns: repeat(2, 1fr); }
    .ledger div:nth-child(2) { border-right: 0; }
    .ledger div:nth-child(-n+2) { border-bottom: 1px solid var(--line-soft); }
  }
  @media (max-width: 520px) {
    .ledger { grid-template-columns: 1fr; }
    .ledger div { border-right: 0; border-bottom: 1px solid var(--line-soft); padding-inline: 0; }
    .ledger div:nth-child(2) { border-bottom: 1px solid var(--line-soft); }
    .ledger div:last-child { border-bottom: 0; }
    .search { grid-template-columns: 1fr; }
  }
</style>
