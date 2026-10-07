<script lang="ts">
  import { untrack } from "svelte";
  import Button from "$lib/ui/Button.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Field from "$lib/ui/Field.svelte";
  import { formatLatency } from "../format";
  import type { CustomModelInput, CustomProbeResponse, DiscoverModelsResponse, DiscoveredModel } from "../types";

  interface Props {
    open: boolean;
    discovery: DiscoverModelsResponse | null;
    probe: CustomProbeResponse | null;
    busy?: string;
    error?: string;
    onreset: () => void;
    ondiscover: (baseUrl: string, apiKey: string) => void;
    onprobe: (baseUrl: string, apiKey: string) => void;
    onadd: (baseUrl: string, apiKey: string, label: string, models: CustomModelInput[]) => void;
  }

  let { open = $bindable(false), discovery, probe, busy = "", error = "", onreset, ondiscover, onprobe, onadd }: Props = $props();
  let baseUrl = $state("");
  let apiKey = $state("");
  let label = $state("");
  let selected = $state<string[]>([]);
  let wasOpen = false;
  let discoveredSignature = "";

  const chatModels = $derived((discovery?.models ?? []).filter((model) => !model.kind || model.kind === "chat"));
  const selectedIDs = $derived(new Set(selected));

  function resetDiscovery() {
    selected = [];
    discoveredSignature = "";
    onreset();
  }

  function clearForm() {
    baseUrl = "";
    apiKey = "";
    label = "";
    selected = [];
    discoveredSignature = "";
  }

  function setCredential(field: "base" | "key", value: string) {
    if (field === "base") baseUrl = value;
    else apiKey = value;
    if (discovery || probe) resetDiscovery();
  }

  function toggleModel(model: DiscoveredModel, checked: boolean) {
    if (checked && !selectedIDs.has(model.id)) selected = [...selected, model.id];
    else if (!checked && selectedIDs.has(model.id)) selected = selected.filter((id) => id !== model.id);
  }

  function addEndpoint() {
    const models: CustomModelInput[] = [];
    for (const model of chatModels) {
      if (!selectedIDs.has(model.id)) continue;
      models.push({ model: model.id, displayName: model.id, supportsVision: model.vision ?? undefined });
    }
    onadd(baseUrl.trim(), apiKey.trim(), label.trim(), models);
  }

  $effect(() => {
    const isOpen = open;
    untrack(() => {
      if (isOpen !== wasOpen) {
        clearForm();
        onreset();
      }
      wasOpen = isOpen;
    });
  });

  $effect(() => {
    const signature = discovery ? `${discovery.baseUrl}:${discovery.models.map((model) => model.id).join("\u0000")}` : "";
    untrack(() => {
      if (signature && signature !== discoveredSignature) {
        selected = chatModels.map((model) => model.id);
        discoveredSignature = signature;
      }
    });
  });
</script>

<Dialog bind:open title="Custom OpenAI-compatible endpoint" description="Discover its catalogue, prove one model answers, then add the selected chat models." width={760}>
  <div class="custom-flow">
    {#if error}<p class="action-error" role="alert">{error}</p>{/if}
    <div class="credentials">
      <Field label="Base URL" value={baseUrl} oninput={(event) => setCredential("base", event.currentTarget.value)} placeholder="https://gateway.example/v1" mono autofocus required />
      <Field label="API key" type="password" value={apiKey} oninput={(event) => setCredential("key", event.currentTarget.value)} placeholder="Paste the endpoint key" mono autocomplete="off" required />
      <div class="label-field"><Field label="Label (optional)" bind:value={label} placeholder="Lab relay" /></div>
    </div>

    <section class="step">
      <div class="step-copy"><span>1</span><div><h3>Discover models</h3><p>Read the endpoint's model catalogue without storing the key.</p></div></div>
      <Button size="sm" variant={discovery ? "line" : "plate"} busy={busy === "discover"} armed={Boolean(baseUrl.trim() && apiKey.trim())} onclick={() => ondiscover(baseUrl.trim(), apiKey.trim())}>{discovery ? "Discover again" : "Discover"}</Button>
    </section>

    {#if discovery}
      <div class="models">
        <div class="model-head">
          <span>{selected.length} of {chatModels.length} chat models selected</span>
          <div><Button size="sm" variant="quiet" onclick={() => (selected = chatModels.map((model) => model.id))}>Select all</Button><Button size="sm" variant="quiet" onclick={() => (selected = [])}>Clear</Button></div>
        </div>
        {#if chatModels.length === 0}
          <Empty title="No chat models were discovered" body="Check that the URL points at an OpenAI-compatible /v1 endpoint, then discover again." />
        {:else}
          <div class="model-list">
            {#each chatModels as model (model.id)}
              <label class="model-row">
                <input type="checkbox" checked={selectedIDs.has(model.id)} onchange={(event) => toggleModel(model, event.currentTarget.checked)} />
                <span><strong>{model.id}</strong><small>{model.ownedBy || "Owner not published"}{model.contextWindow ? `, ${model.contextWindow.toLocaleString()} context` : ""}</small></span>
                {#if model.registered}<em>Already registered</em>{:else if model.priceNote}<em>{model.priceNote}</em>{/if}
              </label>
            {/each}
          </div>
        {/if}
      </div>

      <section class="step">
        <div class="step-copy"><span>2</span><div><h3>Probe the endpoint</h3><p>Send one bounded request before the credential is saved.</p></div></div>
        <Button size="sm" variant={probe ? "line" : "plate"} busy={busy === "probe"} armed={selected.length > 0} onclick={() => onprobe(baseUrl.trim(), apiKey.trim())}>{probe ? "Probe again" : "Probe"}</Button>
      </section>
    {/if}

    {#if probe}
      <p class="probe-result">Answered with <code>{probe.modelId}</code> in {formatLatency(probe.latencyMs)}{probe.toolCalls === true ? ", tool calls verified" : ""}{probe.reasoning === true ? ", reasoning verified" : ""}.</p>
      <section class="step final">
        <div class="step-copy"><span>3</span><div><h3>Add endpoint</h3><p>Store the credential and make the selected models available to routing sets.</p></div></div>
        <Button size="sm" variant="plate" busy={busy === "add"} armed={selected.length > 0} onclick={addEndpoint}>Add {selected.length} model{selected.length === 1 ? "" : "s"}</Button>
      </section>
    {/if}
  </div>
</Dialog>

<style>
  .custom-flow { display: flex; flex-direction: column; gap: var(--s4); }
  .action-error { color: var(--alert); padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); }
  .credentials { display: grid; grid-template-columns: minmax(0, 1.5fr) minmax(0, 1fr); gap: var(--s3); }
  .label-field { grid-column: 1 / -1; }
  .step { display: flex; align-items: center; justify-content: space-between; gap: var(--s4); padding-block: var(--s3); border-block: 1px solid var(--line-soft); }
  .step.final { border-top: 0; }
  .step-copy { display: flex; align-items: flex-start; gap: var(--s3); }
  .step-copy > span { display: grid; place-items: center; width: 22px; height: 22px; border: 1px solid var(--line); border-radius: var(--radius); color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-micro); }
  .step h3 { color: var(--ink); font-size: var(--f-row); font-weight: 500; }
  .step p { margin-top: var(--s1); color: var(--ink-mute); font-size: var(--f-small); }
  .models { border: 1px solid var(--line-soft); border-radius: var(--radius); }
  .model-head { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); padding: var(--s2) var(--s3); border-bottom: 1px solid var(--line-soft); color: var(--ink-mute); font-size: var(--f-small); }
  .model-head > div { display: flex; gap: var(--s1); }
  .model-list { max-height: 260px; overflow: auto; }
  .model-row { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--s3); min-height: 46px; padding: var(--s2) var(--s3); border-bottom: 1px solid var(--line-soft); }
  .model-row:last-child { border-bottom: 0; }
  .model-row input { accent-color: var(--bone); }
  .model-row strong { display: block; color: var(--ink); font-family: var(--mono); font-size: var(--f-small); font-weight: 400; overflow-wrap: anywhere; }
  .model-row small { display: block; margin-top: 2px; color: var(--ink-faint); }
  .model-row em { color: var(--ink-mute); font-size: var(--f-micro); font-style: normal; }
  .probe-result { padding: var(--s3); border-left: 1px solid var(--bone); color: var(--ink-dim); font-size: var(--f-small); }
  .probe-result code { color: var(--ink); }
  @media (max-width: 650px) {
    .credentials { grid-template-columns: 1fr; }
    .label-field { grid-column: auto; }
    .step { align-items: flex-start; flex-direction: column; }
  }
</style>
