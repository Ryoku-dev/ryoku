<script lang="ts">
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import { relativeTime } from "../format";
  import type { AccountUsage, LoginRow, SignInPlatform } from "../types";
  import { tightestCapacity } from "./providers";

  interface Props {
    logins: LoginRow[];
    accounts: AccountUsage[];
    platforms: SignInPlatform[];
    busy?: string;
    error?: string;
    onenroll: (login: LoginRow) => void;
    onwithdraw: (login: LoginRow) => void;
    onforget: (login: LoginRow) => void;
    onsignin: (platform: SignInPlatform) => void;
  }

  let { logins, accounts, platforms, busy = "", error = "", onenroll, onwithdraw, onforget, onsignin }: Props = $props();
  const usageByProvider = $derived.by(() => {
    const byProvider = new Map<string, AccountUsage>();
    for (const account of accounts) byProvider.set(account.provider.toLowerCase(), account);
    return byProvider;
  });
  const platformByID = $derived.by(() => {
    const byID = new Map<string, SignInPlatform>();
    for (const platform of platforms) byID.set(platform.id.toLowerCase(), platform);
    return byID;
  });
  const tightest = $derived(tightestCapacity(accounts));
  const remaining = $derived(tightest ? Math.max(0, 100 - tightest.utilization) : null);
</script>

<Card title="Subscription accounts" gloss="契約" lead="Browser sign-ins can contribute their account models without exposing a pasted key." class="wide">
  {#if error}<p class="action-error" role="alert">{error}</p>{/if}
  {#if tightest && remaining !== null}
    <div class="capacity-head">
      <span class="t-value">{remaining.toFixed(0)}%</span>
      <span class="t-small">remaining, {tightest.account} {tightest.label}</span>
    </div>
    <div class="bar" class:alert={remaining < 15}><i style:width={`${remaining}%`}></i></div>
  {/if}

  {#if logins.length === 0}
    <Empty title="No subscription accounts yet" body="Choose a sign-in provider in the directory below. Complete its browser flow, then the account will appear here." />
  {:else}
    <div class="accounts">
      {#each logins as login (login.id)}
        {@const usage = usageByProvider.get(login.id.toLowerCase())}
        {@const platform = platformByID.get(login.id.toLowerCase())}
        <article class="account">
          <header>
            <div>
              <h3>{login.name}</h3>
              <p>{login.detail || `${login.offered} models available to this account`}</p>
            </div>
            <span class="account-state">{login.enrolled ? `${login.models} models routing` : "Not routing"}</span>
          </header>

          <div class="windows">
            {#if usage?.error}
              <p class="usage-error">{usage.error} {usage.needsSignIn ? "Sign in again to refresh this account." : "Routing may still work."}</p>
            {:else if usage?.windows?.length}
              {#each usage.windows as window (window.key)}
                <div class="window-line">
                  <span>{window.label}</span>
                  <span>{(100 - Math.min(100, Math.max(0, window.utilization))).toFixed(0)}% remaining{window.resetsAt ? `, resets ${relativeTime(window.resetsAt)}` : ""}</span>
                </div>
              {/each}
            {:else if usage?.balance !== undefined && usage.balance !== null}
              <div class="window-line"><span>Balance</span><span>{usage.balance} {usage.unit || "credits"}{usage.cadence ? `, ${usage.cadence}` : ""}</span></div>
            {:else}
              <p class="no-allowance">This account does not publish an allowance report.</p>
            {/if}
            {#if usage?.note}<p class="no-allowance">{usage.note}</p>{/if}
          </div>

          <footer>
            {#if usage?.needsSignIn && platform}<Button size="sm" variant="plate" onclick={() => onsignin(platform)}>Sign in again</Button>{/if}
            {#if login.enrolled}
              <Button size="sm" busy={busy === `withdraw:${login.id}`} onclick={() => onwithdraw(login)}>Withdraw from routing</Button>
            {:else}
              <Button size="sm" variant="plate" busy={busy === `enroll:${login.id}`} onclick={() => onenroll(login)}>Enroll {login.offered} models</Button>
            {/if}
            <Button size="sm" variant="quiet" busy={busy === `forget:${login.id}`} onclick={() => onforget(login)}>Forget login</Button>
          </footer>
        </article>
      {/each}
    </div>
  {/if}
</Card>

<style>
  .action-error, .usage-error { color: var(--alert); }
  .action-error { margin-bottom: var(--s3); padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); }
  .capacity-head { display: flex; align-items: baseline; gap: var(--s3); margin-bottom: var(--s2); }
  .accounts { display: flex; flex-direction: column; margin-top: var(--s5); }
  .account { padding: var(--s4) 0; border-top: 1px solid var(--line-soft); }
  .account:first-child { padding-top: 0; border-top: 0; }
  .account header { display: flex; justify-content: space-between; align-items: flex-start; gap: var(--s4); }
  .account h3 { color: var(--ink); font-size: var(--f-row); font-weight: 500; }
  .account header p, .no-allowance, .usage-error { margin-top: var(--s1); color: var(--ink-mute); font-size: var(--f-small); }
  .usage-error { color: var(--alert); }
  .account-state { color: var(--ink-dim); font-size: var(--f-small); white-space: nowrap; }
  .windows { display: flex; flex-direction: column; margin-top: var(--s3); }
  .window-line { display: grid; grid-template-columns: minmax(120px, .5fr) minmax(0, 1fr); gap: var(--s4); padding: var(--s2) 0; border-bottom: 1px solid var(--line-soft); color: var(--ink-dim); font-size: var(--f-small); }
  .window-line span:last-child { color: var(--ink-mute); text-align: right; }
  .account footer { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: var(--s2); margin-top: var(--s3); }
  @media (max-width: 680px) {
    .account header, .window-line { grid-template-columns: 1fr; flex-direction: column; gap: var(--s1); }
    .window-line span:last-child { text-align: left; }
  }
</style>
