<script lang="ts">
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Chip from "$lib/ui/Chip.svelte";
  import { formatTokens, relativeTime } from "../format";
  import type { Project } from "../types";
  import { deriveProjectState, semanticCoverage, stateLabel } from "./projects";

  interface Props {
    project: Project;
    busy?: boolean;
    copied?: boolean;
    onopen: (root: string) => void;
    oncopy: (root: string) => void;
    onreindex: (root: string) => void;
    onremove: (project: Project) => void;
  }

  let { project, busy = false, copied = false, onopen, oncopy, onreindex, onremove }: Props = $props();
  const state = $derived(deriveProjectState(project));
  const coverage = $derived(semanticCoverage(project.status));
  const tone = $derived(state === "ready" ? "plate" : state === "error" ? "alert" : "line");
</script>

<Card class="project-card">
  <div class="project-head">
    <div>
      <div class="project-title">
        <h2>{project.name}</h2>
        <Chip tone={tone} mono>{stateLabel(state)}</Chip>
      </div>
      <code title={project.root}>{project.root}</code>
    </div>
    <Button size="sm" onclick={() => onopen(project.root)}>Details</Button>
  </div>

  {#if project.error || project.job?.error}
    <p class="project-error" role="alert">{project.job?.error || project.error}</p>
  {/if}

  <dl class="metrics">
    <div><dt>Files</dt><dd>{project.status?.counts.files ?? 0}</dd></div>
    <div><dt>Symbols</dt><dd>{project.status?.counts.symbols ?? 0}</dd></div>
    <div><dt>Edges</dt><dd>{project.status?.counts.edges ?? 0}</dd></div>
    <div><dt>Savings</dt><dd>{project.status?.savings.queries ? `~${formatTokens(project.status.savings.saved_tokens)}` : "None yet"}</dd></div>
  </dl>

  <div class="semantic">
    <div>
      <span>{coverage.label}</span>
      <strong>{coverage.percent}%</strong>
    </div>
    <div class="bar" aria-label={`Semantic coverage ${coverage.percent}%`}><i style:width={`${coverage.percent}%`}></i></div>
  </div>

  <dl class="facts">
    <div><dt>Embedding model</dt><dd>{project.embedModel}</dd></div>
    <div><dt>Last index</dt><dd>{project.status?.last_index ? relativeTime(project.status.last_index) : "Never"}</dd></div>
  </dl>

  <div class="actions">
    <Button size="sm" variant="quiet" onclick={() => oncopy(project.root)}>{copied ? "Copied" : "Copy path"}</Button>
    <Button size="sm" busy={busy} armed={state !== "indexing"} onclick={() => onreindex(project.root)}>Reindex</Button>
    <Button size="sm" variant="quiet" armed={!busy && state !== "indexing"} onclick={() => onremove(project)}>Remove</Button>
  </div>
</Card>

<style>
  .project-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--s4); }
  .project-head > div { min-width: 0; }
  .project-title { display: flex; align-items: center; flex-wrap: wrap; gap: var(--s2); }
  h2 { color: var(--ink); font-size: var(--f-row); font-weight: 500; }
  code { display: block; margin-top: var(--s1); overflow: hidden; color: var(--ink-mute); font-size: var(--f-micro); text-overflow: ellipsis; white-space: nowrap; }
  .project-error { margin-top: var(--s3); padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); color: var(--alert); font-size: var(--f-small); }
  .metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); margin-top: var(--s4); border-block: 1px solid var(--line-soft); }
  .metrics div { padding: var(--s3); border-right: 1px solid var(--line-soft); }
  .metrics div:first-child { padding-left: 0; }
  .metrics div:last-child { border-right: 0; }
  dt { color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-tiny); letter-spacing: var(--track-label); text-transform: uppercase; }
  dd { margin-top: var(--s1); color: var(--ink); font-variant-numeric: tabular-nums; }
  .semantic { display: grid; gap: var(--s2); margin-top: var(--s4); }
  .semantic > div:first-child { display: flex; justify-content: space-between; gap: var(--s3); color: var(--ink-mute); font-size: var(--f-small); }
  .semantic strong { color: var(--ink); font-weight: 500; }
  .facts { display: grid; gap: var(--s2); margin-top: var(--s4); }
  .facts div { display: grid; grid-template-columns: 128px minmax(0, 1fr); gap: var(--s3); align-items: baseline; }
  .facts dd { overflow: hidden; color: var(--ink-dim); font-family: var(--mono); font-size: var(--f-micro); text-overflow: ellipsis; white-space: nowrap; }
  .actions { display: flex; justify-content: flex-end; gap: var(--s2); margin-top: var(--s4); padding-top: var(--s3); border-top: 1px solid var(--line-soft); }
  @media (max-width: 620px) {
    .metrics { grid-template-columns: repeat(2, 1fr); }
    .metrics div:nth-child(2) { border-right: 0; }
    .metrics div:nth-child(-n+2) { border-bottom: 1px solid var(--line-soft); }
    .facts div { grid-template-columns: 1fr; gap: var(--s1); }
  }
</style>
