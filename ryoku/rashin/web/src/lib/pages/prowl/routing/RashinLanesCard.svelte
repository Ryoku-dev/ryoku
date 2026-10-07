<script lang="ts">
  import type { QuickResponse } from "$lib/api/client";
  import { chat } from "$lib/chat/store.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Chip from "$lib/ui/Chip.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Select from "$lib/ui/Select.svelte";

  interface Props {
    quick: QuickResponse | null;
    busy?: boolean;
    error?: string;
    onset: (route: string) => void;
  }

  let { quick, busy = false, error = "", onset }: Props = $props();
  const quickOptions = $derived((quick?.routes ?? []).map((route) => ({
    value: route.id,
    label: route.label,
    hint: route.sub,
  })));
  const chatOptions = $derived(chat.state.models.map((model) => ({
    value: model.id,
    label: model.name || model.id,
    hint: model.id,
  })));
  const chatModel = $derived(chat.state.models.find((model) => model.id === chat.state.currentModel));
  const chatProwlPending = $derived(chat.state.prowl === "pending");
</script>

<Card title="Rashin lanes" gloss="羅針" lead="Choose how short asks and the running chat session reach a model.">
  <div class="lanes">
    <section class="lane">
      <header>
        <div>
          <span class="t-label">Fast lane</span>
          <h3>{quick?.label || "Route unavailable"}</h3>
        </div>
        <Chip tone={quick?.ready ? "plate" : "alert"}>{quick?.ready ? "Ready" : "Not ready"}</Chip>
      </header>
      {#if error}
        <Empty title="Fast lane state is unavailable" body="Start the Rashin service, then return here to choose a Prowl route." />
      {:else if quick}
        <p>{quick.ready ? "Short asks use this route before a full agent wakes." : (quick.reason || "Connect a provider before sending a Quick request.")}</p>
        <dl>
          <div><dt>Gateway</dt><dd>{quick.gateway.running ? quick.gateway.url : "Stopped"}</dd></div>
          <div><dt>Route</dt><dd>{quick.route}</dd></div>
        </dl>
        <Select label="Fast lane route" options={quickOptions} value={quick.route} placeholder="Choose route" disabled={busy} onchange={onset} />
      {/if}
    </section>

    <section class="lane">
      <header>
        <div>
          <span class="t-label">Chat lane</span>
          <h3>{chatModel?.name || chat.state.currentModel || "Agent default"}</h3>
        </div>
        <Chip tone={chatProwlPending ? "alert" : chat.connected ? "line" : "quiet"}>
          {chatProwlPending ? "Waiting on Prowl" : chat.connected ? "Connected" : "Connecting"}
        </Chip>
      </header>
      {#if chatProwlPending}
        <p class="prowl-waiting">
          Chat is waiting for Prowl. {chat.state.prowlReason || "Connect a provider before Prowl can route the active chat agent."}
          <a href="#/prowl/providers">Open Providers</a>
        </p>
      {/if}
      {#if chatOptions.length === 0 && !chatProwlPending}
        <Empty title="Waiting for chat models" body="Open Chat or wait for the active agent to finish starting." />
      {:else if chatOptions.length > 0}
        <p>{chatModel?.description || "This choice belongs to the running chat agent session."}</p>
        <dl>
          <div><dt>Agent</dt><dd>{chat.state.agent || "Starting"}</dd></div>
          <div><dt>Models</dt><dd>{chatOptions.length} advertised</dd></div>
        </dl>
        <Select
          label="Chat lane model"
          options={chatOptions}
          value={chat.state.currentModel}
          placeholder="Choose model"
          disabled={!chat.connected}
          onchange={(id) => chat.setModel(id)}
        />
      {/if}
    </section>
  </div>
</Card>

<style>
  .lanes { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--s5); }
  .lane { min-width: 0; }
  .lane + .lane { padding-left: var(--s5); border-left: 1px solid var(--line-soft); }
  header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--s3); }
  header > div { min-width: 0; }
  h3 { margin-top: var(--s1); overflow: hidden; color: var(--ink); font-family: var(--display); font-size: 24px; font-weight: 400; line-height: 1.2; text-overflow: ellipsis; white-space: nowrap; }
  .lane > p { min-height: 42px; margin-top: var(--s2); color: var(--ink-mute); font-size: var(--f-small); line-height: 1.5; }
  .lane > .prowl-waiting { min-height: 0; padding: var(--s3); border: 1px solid color-mix(in srgb, var(--alert) 45%, transparent); border-radius: var(--radius); color: var(--alert); }
  .prowl-waiting a { margin-left: var(--s2); color: inherit; text-decoration: underline; text-underline-offset: 3px; }
  dl { display: grid; gap: var(--s2); margin: var(--s4) 0; padding-top: var(--s3); border-top: 1px solid var(--line-soft); }
  dl div { display: grid; grid-template-columns: 72px minmax(0, 1fr); gap: var(--s3); }
  dt { color: var(--ink-faint); font-size: var(--f-small); }
  dd { margin: 0; overflow: hidden; color: var(--ink-dim); font-family: var(--mono); font-size: var(--f-micro); text-overflow: ellipsis; white-space: nowrap; }
  @media (max-width: 1100px) {
    .lanes { grid-template-columns: 1fr; }
    .lane + .lane { padding: var(--s5) 0 0; border-left: 0; border-top: 1px solid var(--line-soft); }
  }
</style>
