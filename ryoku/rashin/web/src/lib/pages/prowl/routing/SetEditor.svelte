<script lang="ts">
  import Page from "$lib/app/Page.svelte";
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Field from "$lib/ui/Field.svelte";
  import Seg from "$lib/ui/Seg.svelte";
  import type { ModelRow, Profile } from "../types";
  import ProviderGroup from "./ProviderGroup.svelte";
  import { groupAvailableModels, reorderMember, setAllShown } from "./routing";

  interface Props {
    profile: Profile | null;
    models: ModelRow[];
    order: number[];
    loading?: boolean;
    saving?: boolean;
    dirty?: boolean;
    error?: string;
    onchange: (order: number[]) => void;
    onsave: () => void;
    onretry: () => void;
  }

  let {
    profile,
    models,
    order,
    loading = false,
    saving = false,
    dirty = false,
    error = "",
    onchange,
    onsave,
    onretry,
  }: Props = $props();
  let query = $state("");
  let access = $state("all");

  const filteredModels = $derived.by(() => {
    const needle = query.trim().toLowerCase();
    return models.filter((model) => model.available
      && (access === "all" || model.access === access)
      && (!needle || [model.platform, model.displayName, model.modelId, model.access, model.sizeLabel]
        .some((value) => value.toLowerCase().includes(needle))));
  });
  const shown = $derived(filteredModels);
  const groups = $derived(groupAvailableModels(filteredModels));
  const selected = $derived(new Set(order));
  const selectedModels = $derived(order
    .map((id) => models.find((model) => model.id === id))
    .filter((model): model is ModelRow => Boolean(model?.available)));
  const selectedModelIds = $derived(selectedModels.map((model) => model.id));
  const unavailableMembers = $derived(order.length - selectedModels.length);
  const allShownSelected = $derived(shown.length > 0 && shown.every((model) => selected.has(model.id)));

</script>

<Page
  title={profile ? `Routing › ${profile.name}` : "Routing › Set"}
  gloss="組合"
  lead="Choose connected models, then put them in the order this set should try."
>
  {#snippet tools()}
    <Button variant="quiet" onclick={() => (location.hash = "#/prowl/routing")}>Back to sets</Button>
    <Button variant="plate" busy={saving} armed={dirty && Boolean(profile)} onclick={onsave}>Save set</Button>
  {/snippet}

  {#if loading}
    <Empty title="Reading the set" body="Prowl is loading its connected models and saved priority." />
  {:else if error}
    <div class="notice" role="alert"><span>{error}</span><Button size="sm" onclick={onretry}>Retry</Button></div>
  {:else if profile}
    <div class="editor-grid">
      <Card title="Priority" gloss="順序" lead="Connected members in the order this set will try.">
        {#if selectedModels.length === 0}
          <Empty
            title={unavailableMembers > 0 ? "No connected members" : "This set is empty"}
            body={unavailableMembers > 0 ? `${unavailableMembers} saved member${unavailableMembers === 1 ? "" : "s"} will reappear when their providers reconnect.` : "Open a provider below and select at least one model, then save the set."}
          />
        {:else}
          <ol class="priority-list">
            {#each selectedModels as model, index (model.id)}
              <li>
                <span class="position">{index + 1}</span>
                <span class="priority-name"><strong>{model.displayName}</strong><small>{model.platform} · {model.modelId}</small></span>
                <Button size="sm" variant="quiet" armed={index > 0} onclick={() => onchange(reorderMember(order, model.id, -1, selectedModelIds))}>Up</Button>
                <Button size="sm" variant="quiet" armed={index < selectedModels.length - 1} onclick={() => onchange(reorderMember(order, model.id, 1, selectedModelIds))}>Down</Button>
              </li>
            {/each}
          </ol>
        {/if}
      </Card>

      <Card title="Connected models" gloss="模型" lead="Providers start folded. Selecting a disabled model enables it when you save.">
        {#snippet tools()}
          <Button
            size="sm"
            variant="line"
            armed={shown.length > 0}
            onclick={() => onchange(setAllShown(order, shown, !allShownSelected))}
          >{allShownSelected ? "Clear all shown" : "Select all shown"}</Button>
        {/snippet}
      {#if unavailableMembers > 0}
        <p class="unavailable-note">{unavailableMembers} saved member{unavailableMembers === 1 ? "" : "s"} from disconnected providers remain in this set and are preserved when you save.</p>
      {/if}
        <div class="model-filters">
          <Field label="Search connected models" placeholder="Provider or model" bind:value={query} />
          <Seg
            label="Model access filter"
            size="sm"
            options={[
              { value: "all", label: "All" },
              { value: "free", label: "Free" },
              { value: "paid", label: "Paid" },
              { value: "subscription", label: "Subscription" },
            ]}
            value={access}
            onchange={(value) => (access = value)}
          />
        </div>
        {#if groups.length === 0}
          <Empty
            title={query || access !== "all" ? "No connected models match" : "No connected models"}
            body={query || access !== "all" ? "Clear the search or change the access filter." : "Connect a provider on the Providers page, then return here to build this set."}
          />
        {:else}
          <div class="provider-groups">
            {#each groups as group (group.provider)}
              <ProviderGroup
                provider={group.provider}
                models={group.models}
                {order}
                {onchange}
              />
            {/each}
          </div>
        {/if}
      </Card>
    </div>
  {:else}
    <Empty title="Set not found" body="Return to Routing and choose one of the sets Prowl currently has." />
  {/if}
</Page>

<style>
  .notice { display: flex; align-items: center; justify-content: space-between; gap: var(--s4); padding: var(--s3) var(--s4); border: 1px solid var(--line); border-radius: var(--radius); color: var(--ink-dim); }
  .editor-grid { display: grid; gap: var(--s4); align-content: start; }
  .priority-list { display: flex; flex-direction: column; }
  .priority-list li { display: grid; grid-template-columns: 28px minmax(0, 1fr) auto auto; align-items: center; gap: var(--s2); min-height: 48px; border-bottom: 1px solid var(--line-soft); }
  .priority-list li:last-child { border-bottom: 0; }
  .position { color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-micro); }
  .priority-name { min-width: 0; }
  .priority-name strong, .priority-name small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .priority-name strong { color: var(--ink); font-size: var(--f-body); font-weight: 500; }
  .priority-name small { margin-top: 2px; color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-tiny); }
  .provider-groups { display: flex; flex-direction: column; gap: var(--s2); }
  .unavailable-note { padding: var(--s3) var(--s4); border: 1px dashed var(--line); border-radius: var(--radius); color: var(--ink-mute); font-size: var(--f-small); }
  .model-filters { display: flex; align-items: end; justify-content: space-between; gap: var(--s4); margin-bottom: var(--s4); }
  .model-filters :global(.field) { flex: 1; max-width: 420px; }
  @media (max-width: 720px) { .model-filters { align-items: stretch; flex-direction: column; } }
</style>
