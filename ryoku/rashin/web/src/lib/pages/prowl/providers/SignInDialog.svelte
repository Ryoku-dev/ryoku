<script lang="ts">
  import { untrack } from "svelte";
  import Button from "$lib/ui/Button.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Lamp from "$lib/ui/Lamp.svelte";
  import type { SignInSession } from "../types";

  interface Props {
    open: boolean;
    session: SignInSession | null;
    providerName?: string;
    busy?: boolean;
    error?: string;
    oncancel: () => void;
  }

  let { open = $bindable(false), session, providerName = "provider", busy = false, error = "", oncancel }: Props = $props();
  let copied = $state("");
  let copyError = $state("");
  let wasOpen = false;

  async function copy(value: string, label: string) {
    copyError = "";
    try {
      await navigator.clipboard.writeText(value);
      copied = label;
      window.setTimeout(() => {
        if (copied === label) copied = "";
      }, 1_500);
    } catch {
      copyError = `${label} could not be copied. Select it and copy it manually.`;
    }
  }

  $effect(() => {
    const isOpen = open;
    untrack(() => {
      if (wasOpen && !isOpen && session) oncancel();
      wasOpen = isOpen;
    });
  });
</script>

<Dialog bind:open title={`Continue with ${providerName}`} description="Finish signing in with the provider. Prowl will enroll the account when the browser flow completes.">
  {#if session}
    {@const active = session}
    <div class="signin">
      <p class="waiting"><Lamp state="busy" /><span>Waiting for the browser sign-in to finish.</span></p>
      {#if active.user_code}
        <div class="verification">
          <span class="t-label">Verification code</span>
          <code>{active.user_code}</code>
          <Button size="sm" onclick={() => void copy(active.user_code, "Code")}>{copied === "Code" ? "Copied" : "Copy code"}</Button>
        </div>
      {/if}
      <div class="verification url">
        <span class="t-label">Verification URL</span>
        <code>{active.url}</code>
        <Button size="sm" onclick={() => void copy(active.url, "URL")}>{copied === "URL" ? "Copied" : "Copy URL"}</Button>
      </div>
      <a class="browser-link" href={active.url} target="_blank" rel="noreferrer">Open verification page in your browser</a>
      {#if active.error || error}<p class="action-error" role="alert">{active.error || error}</p>{/if}
      {#if copyError}<p class="action-error" role="alert">{copyError}</p>{/if}
      <div class="signin-actions">
        <Button variant="quiet" busy={busy} onclick={oncancel}>Cancel sign-in</Button>
      </div>
    </div>
  {/if}
</Dialog>

<style>
  .signin { display: flex; flex-direction: column; gap: var(--s4); }
  .waiting { display: flex; align-items: center; gap: var(--s2); color: var(--ink-dim); }
  .verification { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: var(--s2); align-items: center; padding: var(--s3); border: 1px solid var(--line-soft); border-radius: var(--radius); }
  .verification .t-label { grid-column: 1 / -1; }
  .verification code { color: var(--ink); font-size: var(--f-row); overflow-wrap: anywhere; }
  .verification.url code { font-size: var(--f-small); }
  .browser-link { align-self: flex-start; color: var(--ink); border-bottom: 1px solid var(--line-strong); }
  .browser-link:hover { border-color: var(--bone); }
  .action-error { color: var(--alert); padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); }
  .signin-actions { display: flex; justify-content: flex-end; }
</style>
