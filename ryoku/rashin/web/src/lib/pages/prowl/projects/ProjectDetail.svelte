<script lang="ts">
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Chip from "$lib/ui/Chip.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import { formatTokens, parseInstant, relativeTime } from "../format";
  import type { Project } from "../types";
  import type { ProjectOverview } from "./projects";
  import { deriveProjectState, semanticCoverage, stateLabel } from "./projects";

  interface Props {
    project: Project;
    overview: ProjectOverview | null;
    overviewError?: string;
    busy?: boolean;
    copied?: boolean;
    onback: () => void;
    oncopy: (root: string) => void;
    onreindex: (root: string) => void;
    onremove: (project: Project) => void;
  }

  let { project, overview, overviewError = "", busy = false, copied = false, onback, oncopy, onreindex, onremove }: Props = $props();
  const state = $derived(deriveProjectState(project));
  const coverage = $derived(semanticCoverage(project.status));
  const languages = $derived(Object.entries(project.status?.counts.langs ?? {}).sort((a, b) => b[1] - a[1]));

  function indexedAt(value: string | undefined): string {
    if (!value) return "Never";
    const instant = parseInstant(value);
    return instant ? `${relativeTime(value)} · ${instant.toLocaleString()}` : "Unknown";
  }
</script>

<div class="detail">
  <div class="detail-nav">
    <Button variant="quiet" size="sm" onclick={onback}>All projects</Button>
    <div>
      <Button size="sm" variant="quiet" onclick={() => oncopy(project.root)}>{copied ? "Copied" : "Copy path"}</Button>
      <Button size="sm" busy={busy} armed={state !== "indexing"} onclick={() => onreindex(project.root)}>Reindex</Button>
      <Button size="sm" variant="quiet" armed={!busy && state !== "indexing"} onclick={() => onremove(project)}>Remove</Button>
    </div>
  </div>

  <Card title={project.name} gloss="索引" lead={project.root}>
    {#snippet tools()}<Chip tone={state === "error" ? "alert" : state === "ready" ? "plate" : "line"} mono>{stateLabel(state)}</Chip>{/snippet}
    {#if project.error || project.job?.error}<p class="detail-error" role="alert">{project.job?.error || project.error}</p>{/if}
    <dl class="status-grid">
      <div><dt>Files</dt><dd>{project.status?.counts.files ?? 0}</dd></div>
      <div><dt>Symbols</dt><dd>{project.status?.counts.symbols ?? 0}</dd></div>
      <div><dt>Resources</dt><dd>{project.status?.counts.resources ?? 0}</dd></div>
      <div><dt>Chunks</dt><dd>{project.status?.counts.chunks ?? 0}</dd></div>
      <div><dt>Edges</dt><dd>{project.status?.counts.edges ?? 0}</dd></div>
      <div><dt>Resolved</dt><dd>{project.status?.counts.resolved_edges ?? 0}</dd></div>
      <div><dt>External</dt><dd>{project.status?.counts.external_edges ?? 0}</dd></div>
      <div><dt>Unresolved</dt><dd>{project.status?.counts.unresolved_edges ?? 0}</dd></div>
    </dl>
    <div class="coverage">
      <div><span>{coverage.label}</span><strong>{coverage.percent}%</strong></div>
      <div class="bar"><i style:width={`${coverage.percent}%`}></i></div>
    </div>
    <dl class="details-list">
      <div><dt>AI assist</dt><dd>{project.status?.ai_enabled ? "On" : "Off"}</dd></div>
      <div><dt>Embedding model</dt><dd>{project.embedModel}</dd></div>
      <div><dt>Last index</dt><dd>{indexedAt(project.status?.last_index)}</dd></div>
      <div><dt>Measured savings</dt><dd>{project.status?.savings.queries ? `~${formatTokens(project.status.savings.saved_tokens)} tokens across ${project.status.savings.queries} queries` : "No indexed answers yet"}</dd></div>
    </dl>
  </Card>

  <div class="map-grid">
    <Card title="Languages" gloss="言語" lead="Files in the current index.">
      {#if languages.length}
        <div class="ranked-list">
          {#each languages as language (language[0])}<div><span>{language[0]}</span><strong>{language[1]}</strong></div>{/each}
        </div>
      {:else}<Empty title="No languages indexed" body="Reindex after adding source files to this project." />{/if}
    </Card>

    <Card title="Repository map" gloss="構造" lead="Prowl's compact overview of this codebase.">
      {#if overviewError}
        <Empty title="The repository map is unavailable" body={`${overviewError} Reindex the project, then retry.`} />
      {:else if overview}
        <dl class="details-list map-facts">
          <div><dt>Subsystems</dt><dd>{overview.clusters?.length ?? 0}</dd></div>
          <div><dt>Entrypoints</dt><dd>{overview.entrypoint_count ?? overview.entrypoints?.length ?? 0}</dd></div>
          <div><dt>Documentation</dt><dd>{overview.docs?.length ?? 0}</dd></div>
          <div><dt>Keybinds</dt><dd>{overview.keybinds ?? 0}</dd></div>
        </dl>
        {#if overview.clusters?.length}
          <div class="ranked-list sections">
            {#each overview.clusters.slice(0, 8) as cluster (cluster.label)}
              <div><span>{cluster.label}<small>{cluster.lang}</small></span><strong>{cluster.files}</strong></div>
            {/each}
          </div>
        {/if}
      {:else}
        <p class="loading-map">Reading the repository map…</p>
      {/if}
    </Card>
  </div>

  <a class="toolkit-link" href="#/prowl/toolkit">How Prowl indexes and queries a project</a>
</div>

<style>
  .detail { display: grid; gap: var(--s4); }
  .detail-nav { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); }
  .detail-nav > div { display: flex; gap: var(--s2); }
  .detail-error { margin-bottom: var(--s3); color: var(--alert); font-size: var(--f-small); }
  .status-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border-block: 1px solid var(--line-soft); }
  .status-grid div { padding: var(--s3); border-right: 1px solid var(--line-soft); border-bottom: 1px solid var(--line-soft); }
  .status-grid div:nth-child(4n) { border-right: 0; }
  .status-grid div:nth-last-child(-n+4) { border-bottom: 0; }
  dt { color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-tiny); letter-spacing: var(--track-label); text-transform: uppercase; }
  dd { margin-top: var(--s1); color: var(--ink); font-variant-numeric: tabular-nums; }
  .coverage { display: grid; gap: var(--s2); margin-top: var(--s4); }
  .coverage > div:first-child { display: flex; justify-content: space-between; gap: var(--s3); color: var(--ink-mute); font-size: var(--f-small); }
  .coverage strong { color: var(--ink); font-weight: 500; }
  .details-list { display: grid; gap: var(--s2); margin-top: var(--s4); }
  .details-list div { display: grid; grid-template-columns: 148px minmax(0, 1fr); align-items: baseline; gap: var(--s3); }
  .details-list dd { color: var(--ink-dim); font-size: var(--f-small); }
  .map-grid { display: grid; grid-template-columns: minmax(260px, .75fr) minmax(340px, 1.25fr); gap: var(--s4); align-items: start; }
  .ranked-list { display: grid; border-top: 1px solid var(--line-soft); }
  .ranked-list > div { display: flex; align-items: baseline; justify-content: space-between; gap: var(--s3); min-height: var(--row-h); padding: var(--s2) 0; border-bottom: 1px solid var(--line-soft); }
  .ranked-list span { min-width: 0; overflow: hidden; color: var(--ink-dim); text-overflow: ellipsis; white-space: nowrap; }
  .ranked-list strong { color: var(--ink); font-family: var(--mono); font-size: var(--f-small); font-weight: 500; }
  .ranked-list small { margin-left: var(--s2); color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-tiny); }
  .map-facts { margin-top: 0; }
  .sections { margin-top: var(--s4); }
  .loading-map { color: var(--ink-mute); font-size: var(--f-small); }
  .toolkit-link { justify-self: start; color: var(--ink-dim); font-size: var(--f-small); text-decoration: underline; text-decoration-color: var(--line-strong); text-underline-offset: 3px; }
  .toolkit-link:hover { color: var(--ink); }
  @media (max-width: 820px) { .map-grid { grid-template-columns: 1fr; } }
  @media (max-width: 620px) {
    .status-grid { grid-template-columns: repeat(2, 1fr); }
    .status-grid div:nth-child(2n) { border-right: 0; }
    .status-grid div:nth-last-child(-n+4) { border-bottom: 1px solid var(--line-soft); }
    .status-grid div:nth-last-child(-n+2) { border-bottom: 0; }
    .details-list div { grid-template-columns: 1fr; gap: var(--s1); }
  }
</style>
