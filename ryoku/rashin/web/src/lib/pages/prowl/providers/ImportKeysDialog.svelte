<script lang="ts">
  import { untrack } from "svelte";
  import Button from "$lib/ui/Button.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Select from "$lib/ui/Select.svelte";
  import type { DirectoryProvider, ImportPreviewResponse, ImportSelectedKey } from "../types";
  import { providerPlatform } from "./providers";

  interface Props {
    open: boolean;
    preview: ImportPreviewResponse | null;
    providers: DirectoryProvider[];
    busy?: string;
    error?: string;
    onreset: () => void;
    onpreview: (files: File[]) => void;
    onimport: (keys: ImportSelectedKey[]) => void;
  }

  let { open = $bindable(false), preview, providers, busy = "", error = "", onreset, onpreview, onimport }: Props = $props();
  let files = $state<File[]>([]);
  let selected = $state<number[]>([]);
  let platforms = $state<Record<number, string>>({});
  let wasOpen = false;
  let previewSignature = "";
  let fileInput: HTMLInputElement | undefined;

  const platformOptions = $derived.by(() => {
    const values = new Map<string, string>();
    values.set("custom", "Custom endpoint");
    for (const provider of providers) values.set(providerPlatform(provider), provider.name);
    const options: Array<{ value: string; label: string }> = [];
    for (const [value, label] of values) options.push({ value, label });
    return options;
  });
  const selectedIDs = $derived(new Set(selected));

  const chosen = $derived.by(() => {
    const rows: ImportSelectedKey[] = [];
    for (const [index, key] of (preview?.keys ?? []).entries()) {
      if (!selectedIDs.has(index)) continue;
      const platform = platforms[index] || key.detectedPlatform || "";
      if (!platform) continue;
      rows.push({
        keyName: key.keyName,
        keyValue: key.keyValue,
        platform,
        baseUrl: key.baseUrl ?? undefined,
        models: key.models,
      });
    }
    return rows;
  });

  function choose(index: number, checked: boolean) {
    if (checked && !selectedIDs.has(index)) selected = [...selected, index];
    else if (!checked && selectedIDs.has(index)) selected = selected.filter((value) => value !== index);
  }

  function setPlatform(index: number, platform: string) {
    platforms = { ...platforms, [index]: platform };
  }

  function clearForm() {
    if (fileInput) fileInput.value = "";
    files = [];
    selected = [];
    platforms = {};
    previewSignature = "";
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
    const current = preview;
    const signature = current ? current.keys.map((key) => `${key.keyName}:${key.keyValue}:${key.detectedPlatform}:${key.baseUrl}:${key.isDuplicate}`).join("\u0000") : "";
    untrack(() => {
      if (current && signature && signature !== previewSignature) {
        const nextPlatforms: Record<number, string> = {};
        current.keys.forEach((key, index) => {
          nextPlatforms[index] = key.detectedPlatform || (key.baseUrl ? "custom" : "");
        });
        selected = current.keys.flatMap((key, index) => key.isDuplicate ? [] : [index]);
        platforms = nextPlatforms;
        previewSignature = signature;
      }
    });
  });
</script>

<Dialog bind:open title="Import provider keys" description="Preview an exported key file, choose the credentials to keep, then import only those rows." width={760}>
  <div class="import-flow">
    {#if error}<p class="action-error" role="alert">{error}</p>{/if}
    <label class="file-picker">
      <span class="t-label">Export files</span>
      <input bind:this={fileInput} type="file" multiple onchange={(event) => (files = Array.from(event.currentTarget.files ?? []))} />
      <small>JSON, dotenv, shell exports, and supported gateway exports are read locally by Prowl.</small>
    </label>
    <div class="preview-actions">
      <span>{files.length ? `${files.length} file${files.length === 1 ? "" : "s"} selected` : "Choose at least one export file."}</span>
      <Button size="sm" variant="plate" busy={busy === "preview"} armed={files.length > 0} onclick={() => onpreview(files)}>Preview file{files.length === 1 ? "" : "s"}</Button>
    </div>

    {#if preview}
      {#if preview.keys.length === 0}
        <Empty title="No importable keys found" body="Choose an export containing provider credentials. Prowl lists unsupported entries below instead of pretending they imported." />
      {:else}
        <div class="import-list">
          <div class="list-head"><span>{selected.length} of {preview.total} selected</span><span>{preview.duplicates} duplicate{preview.duplicates === 1 ? "" : "s"} skipped by default</span></div>
          {#each preview.keys as key, index (`${key.keyName}:${index}`)}
            <div class="import-row" class:duplicate={key.isDuplicate}>
              <input type="checkbox" checked={selectedIDs.has(index)} onchange={(event) => choose(index, event.currentTarget.checked)} />
              <div class="key-copy"><strong>{key.keyName}</strong><code>{key.prefix || "Key detected"}</code>{#if key.isDuplicate}<small>Already stored</small>{/if}</div>
              <Select label={`Provider for ${key.keyName}`} size="sm" options={platformOptions} value={platforms[index] || ""} placeholder="Choose provider" onchange={(value) => setPlatform(index, value)} />
            </div>
          {/each}
        </div>
      {/if}

      {#if preview.skipped.length}
        <div class="skipped"><strong>Not imported from preview</strong>{#each preview.skipped as item}<p>{item}</p>{/each}</div>
      {/if}
      <div class="import-actions">
        <p>{selected.length !== chosen.length ? "Choose a provider for every selected key before importing." : `${chosen.length} credential${chosen.length === 1 ? "" : "s"} ready.`}</p>
        <Button variant="plate" busy={busy === "import"} armed={chosen.length > 0 && chosen.length === selected.length} onclick={() => onimport(chosen)}>Import selected</Button>
      </div>
    {/if}
  </div>
</Dialog>

<style>
  .import-flow { display: flex; flex-direction: column; gap: var(--s4); }
  .action-error { color: var(--alert); padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); }
  .file-picker { display: flex; flex-direction: column; gap: var(--s2); padding: var(--s4); border: 1px dashed var(--line); border-radius: var(--radius); }
  .file-picker input { color: var(--ink-dim); font-size: var(--f-small); }
  .file-picker input::file-selector-button { height: var(--ctl-h); margin-right: var(--s3); padding: 0 var(--s3); border: 1px solid var(--line); border-radius: var(--radius); background: transparent; color: var(--ink); font-family: var(--ui); }
  .file-picker small { color: var(--ink-faint); }
  .preview-actions, .list-head, .import-actions { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); color: var(--ink-mute); font-size: var(--f-small); }
  .import-list { border: 1px solid var(--line-soft); border-radius: var(--radius); }
  .list-head { padding: var(--s2) var(--s3); border-bottom: 1px solid var(--line-soft); }
  .import-row { display: grid; grid-template-columns: auto minmax(0, 1fr) minmax(150px, .6fr); align-items: center; gap: var(--s3); min-height: 52px; padding: var(--s2) var(--s3); border-bottom: 1px solid var(--line-soft); }
  .import-row:last-child { border-bottom: 0; }
  .import-row.duplicate { opacity: .65; }
  .import-row input { accent-color: var(--bone); }
  .key-copy { min-width: 0; }
  .key-copy strong, .key-copy code, .key-copy small { display: block; }
  .key-copy strong { color: var(--ink); font-weight: 500; }
  .key-copy code, .key-copy small { margin-top: 2px; color: var(--ink-faint); font-size: var(--f-micro); overflow-wrap: anywhere; }
  .skipped { padding: var(--s3); border-left: 1px solid var(--line); color: var(--ink-mute); font-size: var(--f-small); }
  .skipped strong { color: var(--ink-dim); font-weight: 500; }
  .skipped p { margin-top: var(--s1); }
  @media (max-width: 650px) {
    .import-row { grid-template-columns: auto minmax(0, 1fr); }
    .import-row > :last-child { grid-column: 2; }
    .preview-actions, .list-head, .import-actions { align-items: flex-start; flex-direction: column; }
  }
</style>
