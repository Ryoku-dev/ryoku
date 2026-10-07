<script lang="ts">
  import Button from "$lib/ui/Button.svelte";
  import Fold from "$lib/ui/Fold.svelte";
  import type { ModelRow } from "../types";
  import { toggleModel, toggleProvider } from "./routing";

  interface Props {
    provider: string;
    models: ModelRow[];
    order: number[];
    onchange: (order: number[]) => void;
  }

  let { provider, models, order, onchange }: Props = $props();
  let open = $state(false);
  const selected = $derived(new Set(order));
  const selectedCount = $derived(models.filter((model) => selected.has(model.id)).length);

  function contextLabel(value: number | null): string {
    if (!value) return "Unknown context";
    if (value >= 1_000_000) return `${Number((value / 1_000_000).toFixed(1))}m context`;
    return `${Math.round(value / 1_000)}k context`;
  }
</script>

<div class="provider-group">
  <Fold bind:open>
    {#snippet summary()}
      <span class="provider-summary">
        <strong>{provider}</strong>
        <small>{selectedCount} of {models.length} selected</small>
      </span>
    {/snippet}
    <div class="provider-actions">
      <Button size="sm" variant="quiet" onclick={() => onchange(toggleProvider(order, models))}>
        {selectedCount === models.length ? "Clear provider" : "Select provider"}
      </Button>
    </div>
    <div class="model-list">
      {#each models as model (model.id)}
        <button
          class="model-row"
          class:on={selected.has(model.id)}
          type="button"
          aria-pressed={selected.has(model.id)}
          onclick={() => onchange(toggleModel(order, model.id))}
        >
          <span class="check" aria-hidden="true">{selected.has(model.id) ? "✓" : ""}</span>
          <span class="model-name"><strong>{model.displayName}</strong><small>{model.modelId}</small></span>
          <span class="fact">{contextLabel(model.contextWindow)}</span>
          <span class="fact">{model.access}</span>
          <span class="fact">Intelligence {model.intelligenceRank}</span>
          {#if !model.enabled}<span class="disabled-note">Enables on save</span>{/if}
        </button>
      {/each}
    </div>
  </Fold>
</div>

<style>
  .provider-group { padding: var(--s2) var(--s3); border: 1px solid var(--line-soft); border-radius: var(--radius); }
  .provider-summary { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); width: 100%; }
  .provider-summary strong { color: var(--ink); font-size: var(--f-body); font-weight: 500; text-transform: capitalize; }
  .provider-summary small { color: var(--ink-faint); font-size: var(--f-small); }
  .provider-actions { display: flex; justify-content: flex-end; padding: var(--s1) 0 var(--s2); }
  .model-list { padding-top: var(--s1); }
  .model-row { display: grid; grid-template-columns: 22px minmax(180px, 1.5fr) repeat(3, minmax(90px, .7fr)) auto; align-items: center; gap: var(--s2); width: 100%; min-height: 46px; padding: var(--s2); border-top: 1px solid var(--line-soft); border-radius: 0; color: var(--ink-dim); text-align: left; }
  .model-row:hover { background: var(--tint5); }
  .model-row.on { background: var(--tint10); color: var(--ink); }
  .check { display: grid; place-items: center; width: 18px; height: 18px; border: 1px solid var(--line-strong); border-radius: 4px; color: var(--ink-on-bone); font-size: var(--f-small); }
  .model-row.on .check { border-color: var(--bone); background: var(--bone); }
  .model-name { min-width: 0; }
  .model-name strong, .model-name small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .model-name strong { color: inherit; font-weight: 500; }
  .model-name small { margin-top: 2px; color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-tiny); }
  .fact, .disabled-note { color: var(--ink-mute); font-size: var(--f-small); text-transform: capitalize; }
  .disabled-note { color: var(--ink); white-space: nowrap; }
  @media (max-width: 1000px) {
    .model-row { grid-template-columns: 22px minmax(150px, 1fr) repeat(2, minmax(80px, .6fr)); }
    .model-row .fact:last-of-type, .disabled-note { grid-column: 2 / -1; }
  }
  @media (max-width: 700px) {
    .provider-summary small { display: none; }
    .model-row { grid-template-columns: 22px minmax(0, 1fr); }
    .fact, .disabled-note { grid-column: 2; }
  }
</style>
