<script lang="ts">
  import { untrack } from "svelte";
  import Button from "$lib/ui/Button.svelte";
  import Chip from "$lib/ui/Chip.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Field from "$lib/ui/Field.svelte";
  import Lamp from "$lib/ui/Lamp.svelte";
  import { formatTokens, parseInstant, relativeTime } from "../format";
  import type { HealthResponse, KeyActivity, KeyView, ProviderProbeResponse } from "../types";
  import { deriveKeylessState, type ProviderBundle } from "./providers";

  interface Props {
    open: boolean;
    bundle: ProviderBundle | null;
    activity: Map<number, KeyActivity>;
    health: HealthResponse | null;
    probe: ProviderProbeResponse | null;
    probeLoading?: boolean;
    revealed: Record<number, string>;
    busy?: string;
    error?: string;
    onclose: () => void;
    onprobe: () => void;
    oncheck: (key: KeyView) => void;
    onreveal: (key: KeyView) => void;
    onclearcooldowns: (key: KeyView) => void;
    ontoggle: (key: KeyView) => void;
    onremove: (key: KeyView) => void;
    onrename: (key: KeyView, label: string) => void;
    onaddkey: () => void;
    onsignin: () => void;
  }

  let {
    open = $bindable(false), bundle, activity, health, probe, probeLoading = false,
    revealed, busy = "", error = "", onclose, onprobe, oncheck, onreveal,
    onclearcooldowns, ontoggle, onremove, onrename, onaddkey, onsignin,
  }: Props = $props();
  let labelDrafts = $state<Record<number, string>>({});
  let wasOpen = false;
  let draftOwner = "";

  const healthByKey = $derived.by(() => {
    const byKey = new Map<number, HealthResponse["keys"][number]>();
    for (const key of health?.keys ?? []) byKey.set(key.id, key);
    return byKey;
  });
  const keylessState = $derived(bundle ? deriveKeylessState(bundle.provider, bundle.keys) : "not-keyless");

  function setLabel(id: number, value: string) {
    labelDrafts = { ...labelDrafts, [id]: value };
  }

  function coolingText(key: KeyView, row?: KeyActivity): string {
    const seconds = row?.coolingUntil ?? null;
    const coolingUntil = parseInstant(seconds);
    if (coolingUntil && coolingUntil.getTime() > Date.now()) return relativeTime(seconds);
    const expires = key.cooldowns.reduce((latest, cooldown) => Math.max(latest, cooldown.expiresAtMs), 0);
    const expiresAt = parseInstant(expires);
    return expiresAt && expiresAt.getTime() > Date.now() ? relativeTime(expires) : "Not cooling";
  }

  function quotaText(reading: ProviderProbeResponse): string {
    if (reading.published && reading.remaining !== null) {
      const window = reading.window ? `${reading.window} left` : "left";
      const limit = reading.limit === null ? "" : ` of ${reading.limit}`;
      const reset = reading.resetAt ? `, resets ${relativeTime(reading.resetAt)}` : "";
      return `${reading.remaining} ${window}${limit}${reset}`;
    }
    return reading.message || "No published quota";
  }

  $effect(() => {
    const isOpen = open;
    const owner = isOpen && bundle ? bundle.provider.id : "";
    untrack(() => {
      if (owner && owner !== draftOwner) labelDrafts = {};
      draftOwner = owner;
      if (wasOpen && !isOpen) onclose();
      wasOpen = isOpen;
    });
  });
</script>

<Dialog bind:open title={bundle ? `Manage ${bundle.provider.name}` : "Manage provider"} description="Credentials, health, traffic, and provider-published quota." width={900}>
  {#if bundle}
    <div class="manage">
      {#if error}<p class="action-error" role="alert">{error}</p>{/if}
      {#if busy}<p class="working"><Lamp state="busy" />{busy}</p>{/if}

      <section class="summary">
        <div>
          <span class="t-label">Access</span>
          <strong>{bundle.provider.class || "provider"}</strong>
        </div>
        <div>
          <span class="t-label">Models</span>
          <strong>{bundle.login?.enrolled ? bundle.login.models : bundle.provider.modelCount}</strong>
        </div>
        <div>
          <span class="t-label">Connection</span>
          <strong>{keylessState === "free-tier" ? "Free tier" : bundle.signIn?.signed_in ? "Browser sign-in" : `${bundle.keys.length} key${bundle.keys.length === 1 ? "" : "s"}`}</strong>
        </div>
        <div>
          <span class="t-label">Setup</span>
          <strong>{bundle.provider.keyless ? "Key optional" : bundle.provider.friction || "API key"}</strong>
        </div>
      </section>

      {#if bundle.provider.apiKeyUrl || bundle.provider.docsUrl}
        <a class="provider-site" href={bundle.provider.apiKeyUrl || bundle.provider.docsUrl} target="_blank" rel="noreferrer">Open {bundle.provider.name}'s provider site</a>
      {/if}

      {#if bundle.connected && !bundle.provider.id.startsWith("custom:")}
        <section class="quota-block">
          <div>
            <h3>Provider quota</h3>
            <p>{probe ? quotaText(probe) : probeLoading ? "Reading the provider's published quota." : "Check what this provider publishes about remaining quota."}</p>
          </div>
          <Button size="sm" busy={probeLoading} onclick={onprobe}>{probe ? "Refresh quota" : "Check quota"}</Button>
        </section>
      {/if}

      {#if bundle.provider.note}<p class="note">{bundle.provider.note}</p>{/if}

      <section>
        <div class="section-head">
          <div><h3>Credentials</h3><p>Each key has its own health, traffic, and cooldown state.</p></div>
          <div class="section-actions">
            {#if bundle.signIn}<Button size="sm" onclick={onsignin}>{bundle.signIn.signed_in ? "Sign in again" : "Sign in"}</Button>{/if}
            {#if bundle.provider.adapter}<Button size="sm" onclick={onaddkey}>Add key</Button>{/if}
          </div>
        </div>

        {#if bundle.keys.length === 0}
          <Empty title="No API keys on this provider" body={bundle.signIn?.signed_in ? "This provider is connected by browser sign-in. Add a key only if you want another credential in its pool." : "Add a key, or close this panel and connect the provider from the directory."} />
        {:else}
          <div class="keys">
            {#each bundle.keys as key (key.id)}
              {@const stats = activity.get(key.id)}
              {@const checked = healthByKey.get(key.id)}
              {@const draft = labelDrafts[key.id] ?? key.label}
              <article class="key-row">
                <header>
                  <div class="key-name">
                    <strong>{key.label || "Unlabelled key"}</strong>
                    <code>{revealed[key.id] || key.maskedKey}</code>
                  </div>
                  <div class="key-state">
                    <Chip tone={key.enabled ? "plate" : "quiet"}>{key.enabled ? "Enabled" : "Paused"}</Chip>
                    <Chip tone={(checked?.status ?? key.status) === "error" ? "alert" : "line"}>{checked?.status ?? key.status}</Chip>
                  </div>
                </header>

                <dl class="key-facts">
                  <div><dt>Served</dt><dd>{stats?.served ?? 0}</dd></div>
                  <div><dt>Failed</dt><dd>{stats?.failed ?? 0}</dd></div>
                  <div><dt>Tokens</dt><dd>{formatTokens(stats?.tokens ?? 0)}</dd></div>
                  <div><dt>Last check</dt><dd>{relativeTime(checked?.lastCheckedAt ?? key.lastCheckedAt)}</dd></div>
                  <div><dt>Cooldown</dt><dd>{coolingText(key, stats)}</dd></div>
                </dl>

                {#if stats?.lastError || checked?.lastHealthError || key.lastHealthError}
                  <p class="last-error"><span>Last error</span>{stats?.lastError || checked?.lastHealthError || key.lastHealthError}</p>
                {/if}

                <div class="rename">
                  <Field label="Key label" value={draft} placeholder="Main, work, backup" oninput={(event) => setLabel(key.id, event.currentTarget.value)} />
                  <Button size="sm" armed={draft.trim() !== key.label} busy={busy === `Renaming ${key.id}`} onclick={() => onrename(key, draft.trim())}>Save label</Button>
                </div>

                <footer>
                  <Button size="sm" variant="plate" onclick={() => oncheck(key)}>Check health</Button>
                  <Button size="sm" onclick={() => ontoggle(key)}>{key.enabled ? "Pause" : "Resume"}</Button>
                  <Button size="sm" armed={!Boolean(revealed[key.id])} onclick={() => onreveal(key)}>{revealed[key.id] ? "Hides in 30s" : "Reveal key"}</Button>
                  <Button size="sm" onclick={() => onclearcooldowns(key)}>Clear cooldowns</Button>
                  <Button size="sm" variant="quiet" onclick={() => onremove(key)}>Remove key</Button>
                </footer>
              </article>
            {/each}
          </div>
        {/if}
      </section>
    </div>
  {/if}
</Dialog>

<style>
  .manage { display: flex; flex-direction: column; gap: var(--s5); }
  .action-error { color: var(--alert); padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); }
  .working { display: flex; align-items: center; gap: var(--s2); color: var(--ink-dim); }
  .summary { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border-block: 1px solid var(--line-soft); }
  .summary > div { display: flex; flex-direction: column; gap: var(--s1); padding: var(--s3); border-right: 1px solid var(--line-soft); }
  .summary > div:last-child { border-right: 0; }
  .summary strong { color: var(--ink); font-weight: 500; text-transform: capitalize; }
  .provider-site { align-self: flex-start; color: var(--ink-dim); font-size: var(--f-small); border-bottom: 1px solid var(--line-strong); }
  .provider-site:hover { color: var(--ink); border-color: var(--bone); }
  .quota-block, .section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--s4); }
  h3 { color: var(--ink); font-size: var(--f-row); font-weight: 500; }
  .quota-block p, .section-head p, .note { margin-top: var(--s1); color: var(--ink-mute); font-size: var(--f-small); }
  .note { padding-left: var(--s3); border-left: 1px solid var(--line); }
  .section-actions { display: flex; gap: var(--s2); }
  .keys { display: flex; flex-direction: column; gap: var(--s3); margin-top: var(--s3); }
  .key-row { padding: var(--s4); border: 1px solid var(--line-soft); border-radius: var(--radius); }
  .key-row header, .key-row footer { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); }
  .key-name { display: flex; align-items: baseline; gap: var(--s3); min-width: 0; }
  .key-name strong { color: var(--ink); font-weight: 500; }
  .key-name code { overflow: hidden; text-overflow: ellipsis; color: var(--ink-mute); font-size: var(--f-small); }
  .key-state, .key-row footer { display: flex; flex-wrap: wrap; gap: var(--s2); }
  .key-facts { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); margin-top: var(--s3); border-top: 1px solid var(--line-soft); }
  .key-facts > div { padding: var(--s2) var(--s2) 0 0; }
  .key-facts dt { color: var(--ink-faint); font-size: var(--f-micro); }
  .key-facts dd { margin: var(--s1) 0 0; color: var(--ink-dim); font-size: var(--f-small); }
  .last-error { display: grid; grid-template-columns: 80px 1fr; gap: var(--s2); margin-top: var(--s3); padding: var(--s2) 0; border-block: 1px solid var(--line-soft); color: var(--alert); font-size: var(--f-small); overflow-wrap: anywhere; }
  .last-error span { color: var(--ink-faint); }
  .rename { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: end; gap: var(--s2); margin-top: var(--s3); }
  .key-row footer { justify-content: flex-start; margin-top: var(--s3); }
  @media (max-width: 760px) {
    .summary { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .summary > div:nth-child(2) { border-right: 0; }
    .summary > div:nth-child(-n+2) { border-bottom: 1px solid var(--line-soft); }
    .key-facts { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .quota-block, .section-head { flex-direction: column; }
  }
</style>
