<script lang="ts">
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Chip from "$lib/ui/Chip.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import type { ChatAgentInfo } from "./harnesses";

  interface ChatAgentRouting {
    active: boolean;
    pending: boolean;
    reason?: string;
  }

  interface ChatAgentSwitchResponse {
    routing: ChatAgentRouting;
  }

  interface Props {
    agents: ChatAgentInfo[];
    busy?: boolean;
    error?: string;
    onswitch: (id: string) => Promise<ChatAgentSwitchResponse | void>;
  }

  let { agents, busy = false, error = "", onswitch }: Props = $props();
  let open = $state(false);
  let pending = $state<ChatAgentInfo | null>(null);
  let routing = $state<ChatAgentRouting | null>(null);
  let switchedName = $state("");

  function choose(agent: ChatAgentInfo) {
    if (!agent.available || agent.active) return;
    pending = agent;
    open = true;
  }

  async function confirm() {
    if (!pending) return;
    const selected = pending;
    routing = null;
    open = false;
    const response = await onswitch(selected.id);
    if (response?.routing) {
      switchedName = selected.name;
      routing = response.routing;
    }
  }
</script>

<Card title="Rashin chat agent" gloss="対話" lead="Choose which supported harness answers Rashin's chat lane.">
  {#if error}
    <Empty title="Chat agents are unavailable" body={error} />
  {:else if agents.length === 0}
    <Empty title="No chat harness is available" body="Install a supported ACP harness, then return here to choose it for chat." />
  {:else}
    <div class="plates">
      {#each agents as agent (agent.id)}
        <button class="plate" class:on={agent.active} type="button" disabled={!agent.available || agent.active || busy} onclick={() => choose(agent)}>
          <span class="agent-name">{agent.name}</span>
          {#if agent.recommended}<Chip tone={agent.active ? "quiet" : "line"}>Recommended</Chip>{/if}
          <span class="state">{agent.active ? "Answering chat" : agent.available ? "Available" : "Not connected"}</span>
        </button>
      {/each}
    </div>
    {#if routing}
      <p class:waiting={routing.pending} class="routing-result" aria-live="polite">
        {#if routing.active}
          {switchedName} is routed through Prowl.
        {:else}
          {switchedName} is waiting for Prowl. {routing.reason || "Connect a provider before Prowl can route this chat agent."}
          <a href="#/prowl/providers">Open Providers</a>
        {/if}
      </p>
    {/if}
  {/if}
</Card>

<Dialog bind:open title="Switch the chat agent?" description="The running chat session restarts when the harness changes.">
  <p>The current transcript stays on disk. New turns will be answered by <strong>{pending?.name}</strong>.</p>
  {#snippet footer()}
    <Button variant="quiet" onclick={() => (open = false)}>Keep current</Button>
    <Button variant="plate" autofocus busy={busy} onclick={confirm}>Switch agent</Button>
  {/snippet}
</Dialog>

<style>
  .agent-name { flex: 1; color: inherit; font-weight: 500; }
  .state { min-width: 96px; text-align: right; color: var(--ink-faint); font-size: var(--f-small); }
  .plate.on .state { color: color-mix(in srgb, var(--ink-on-bone) 65%, transparent); }
  .plate:disabled:not(.on) { cursor: default; opacity: 0.5; }
  .routing-result { margin-top: var(--s3); padding: var(--s3); border: 1px solid var(--line-soft); border-radius: var(--radius); color: var(--ink-dim); font-size: var(--f-small); }
  .routing-result.waiting { border-color: color-mix(in srgb, var(--alert) 45%, transparent); color: var(--alert); }
  .routing-result a { margin-left: var(--s2); color: inherit; text-decoration: underline; text-underline-offset: 3px; }
  p { color: var(--ink-dim); }
  strong { color: var(--ink); font-weight: 500; }
  @media (max-width: 560px) { .state { min-width: 0; } }
</style>
