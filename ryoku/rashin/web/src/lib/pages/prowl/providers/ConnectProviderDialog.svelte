<script lang="ts">
  import { untrack } from "svelte";
  import Button from "$lib/ui/Button.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Field from "$lib/ui/Field.svelte";
  import type { ProviderBundle } from "./providers";

  interface Props {
    open: boolean;
    bundle: ProviderBundle | null;
    busy?: boolean;
    error?: string;
    onconnect: (apiKey: string, label: string) => void;
  }

  let { open = $bindable(false), bundle, busy = false, error = "", onconnect }: Props = $props();
  let apiKey = $state("");
  let label = $state("");
  let openedFor = "";

  $effect(() => {
    const id = open && bundle ? bundle.provider.id : "";
    untrack(() => {
      if (!id) {
        apiKey = "";
        label = "";
        openedFor = "";
        return;
      }
      if (id !== openedFor) {
        apiKey = "";
        label = "";
      }
      openedFor = id;
    });
  });
</script>

<Dialog bind:open title={bundle ? `Connect ${bundle.provider.name}` : "Connect provider"} description={bundle?.provider.keyless ? "The API key is optional. Leave it empty to use this provider's free tier." : "Store an API key in Prowl's encrypted credential vault."}>
  {#if bundle}
    <form class="connect-form" onsubmit={(event) => { event.preventDefault(); onconnect(apiKey.trim(), label.trim()); }}>
      {#if error}<p class="action-error" role="alert">{error}</p>{/if}
      <Field label={bundle.provider.keyless ? "API key (optional)" : "API key"} type="password" bind:value={apiKey} required={!bundle.provider.keyless} autofocus autocomplete="off" mono placeholder={bundle.provider.keyless ? "Leave empty for free tier" : "Paste the provider key"} />
      <Field label="Label (optional)" bind:value={label} placeholder="Main, work, backup" />
      {#if bundle.provider.apiKeyUrl || bundle.provider.docsUrl}
        <a class="signup" href={bundle.provider.apiKeyUrl || bundle.provider.docsUrl} target="_blank" rel="noreferrer">Open {bundle.provider.name}'s signup page</a>
      {/if}
      <div class="form-actions">
        <Button type="button" variant="quiet" onclick={() => (open = false)}>Cancel</Button>
        <Button type="submit" variant="plate" busy={busy} armed={bundle.provider.keyless || Boolean(apiKey.trim())}>{bundle.provider.keyless && !apiKey.trim() ? "Use free tier" : "Add key"}</Button>
      </div>
    </form>
  {/if}
</Dialog>

<style>
  .connect-form { display: flex; flex-direction: column; gap: var(--s4); }
  .action-error { color: var(--alert); padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); }
  .signup { align-self: flex-start; color: var(--ink-dim); font-size: var(--f-small); border-bottom: 1px solid var(--line-strong); }
  .signup:hover { color: var(--ink); border-color: var(--bone); }
  .form-actions { display: flex; justify-content: flex-end; gap: var(--s2); padding-top: var(--s2); }
</style>
