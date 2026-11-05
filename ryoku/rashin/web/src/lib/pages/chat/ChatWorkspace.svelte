<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "$lib/api/client";
  import Page from "$lib/app/Page.svelte";
  import type { ChatLane, ChatStore } from "$lib/chat/store.svelte";
  import ApprovalDock from "./ApprovalDock.svelte";
  import Composer from "./Composer.svelte";
  import Inspector from "./Inspector.svelte";
  import SessionsPane from "./SessionsPane.svelte";
  import Transcript from "./Transcript.svelte";
  import Icon from "$lib/ui/Icon.svelte";
  import Kbd from "$lib/ui/Kbd.svelte";

  interface Props {
    store: ChatStore;
    lane: ChatLane;
  }

  let { store, lane }: Props = $props();

  const preferenceKey = (name: string) => `rashin.${lane}.${name}`;
  let sessionsOpen = $state(localStorage.getItem(preferenceKey("sessions-open")) !== "0");
  let inspectorOpen = $state(localStorage.getItem(preferenceKey("inspector-open")) !== "0");
  let composerField: HTMLTextAreaElement | undefined = $state();
  let lastSessionStamp = "";
  let terminalBusy = $state(false);
  let terminalError = $state("");

  const currentSession = $derived(store.state.history.find((session) => session.id === store.state.session.id));
  const title = $derived(lane === "ryoku" ? "Agent" : "Chat");
  const gloss = $derived(lane === "ryoku" ? "作業" : "対話");
  const handoffPending = $derived.by(() => {
    const items = store.state.items;
    return items.length > 0 && items[items.length - 1]?.kind === "handoff";
  });

  function setSessionsOpen(open: boolean): void {
    sessionsOpen = open;
    localStorage.setItem(preferenceKey("sessions-open"), open ? "1" : "0");
  }

  function setInspectorOpen(open: boolean): void {
    inspectorOpen = open;
    localStorage.setItem(preferenceKey("inspector-open"), open ? "1" : "0");
  }

  async function openTerminal(): Promise<void> {
    if (terminalBusy) return;
    terminalBusy = true;
    terminalError = "";
    try {
      await api.chatTerminal(lane);
    } catch (cause) {
      terminalError = cause instanceof Error ? cause.message : "The session could not be opened in a terminal.";
    } finally {
      terminalBusy = false;
    }
  }

  function newChat(): void {
    terminalError = "";
    store.newChat();
  }

  function handleShortcut(event: KeyboardEvent): void {
    if (event.key === "Escape" && store.state.busy && !event.defaultPrevented) {
      event.preventDefault();
      store.cancel();
      return;
    }
    if (!event.ctrlKey || event.altKey || event.metaKey) return;
    const key = event.key.toLowerCase();
    if (!["n", "k", "b", "i"].includes(key)) return;
    event.preventDefault();
    if (key === "n") newChat();
    else if (key === "k") composerField?.focus();
    else if (key === "b") setSessionsOpen(!sessionsOpen);
    else setInspectorOpen(!inspectorOpen);
  }

  $effect(() => {
    const stamp = `${store.state.session.id}:${store.state.session.title}`;
    if (!stamp || stamp === ":" || stamp === lastSessionStamp) return;
    lastSessionStamp = stamp;
    store.loadSessions();
  });

  onMount(() => {
    store.loadSessions();
    window.addEventListener("keydown", handleShortcut);
    return () => window.removeEventListener("keydown", handleShortcut);
  });
</script>

<Page {title} {gloss} bare>
  <div class="workspace">
    {#if sessionsOpen}
      <SessionsPane
        {lane}
        sessions={store.state.history}
        currentId={store.state.session.id}
        {terminalBusy}
        {terminalError}
        onnew={newChat}
        onswitch={(id) => store.switchSession(id)}
        oncollapse={() => setSessionsOpen(false)}
        onterminal={() => void openTerminal()}
      />
    {/if}

    <section class="conversation" aria-label={lane === "ryoku" ? "Chat with the Needle" : `Chat with ${store.state.agent || "Hermes"}`}>
      {#if !sessionsOpen || !inspectorOpen}
        <div class="pane-reveals">
          {#if !sessionsOpen}
            <button type="button" onclick={() => setSessionsOpen(true)}>
              <Icon name="sidebar" size={14} />
              Conversations
              <Kbd keys={["Ctrl", "B"]} />
            </button>
          {/if}
          <span></span>
          {#if !inspectorOpen}
            <button type="button" onclick={() => setInspectorOpen(true)}>
              Inspector
              <Kbd keys={["Ctrl", "I"]} />
              <Icon name="sliders" size={14} />
            </button>
          {/if}
        </div>
      {/if}

      <div class="transcript-wrap">
        <Transcript
          {lane}
          items={store.state.items}
          seed={store.state.session.id || "the needle"}
          harnessName={store.state.agent}
          permissions={store.state.permissions}
          banner={store.state.banner}
          replaying={store.state.replaying}
          waitingForSession={store.loadingSessionId !== ""}
          busy={store.state.busy}
          onnew={newChat}
          onanswer={(requestId, optionId) => store.answerPermission(requestId, optionId)}
          onsuggest={(text) => {
            store.setDraft(text);
            composerField?.focus();
          }}
        />
      </div>

      {#if store.state.permissions.length > 0}
        <div class="dock-wrap">
          <ApprovalDock
            permissions={store.state.permissions}
            onanswer={(requestId, optionId) => store.answerPermission(requestId, optionId)}
          />
        </div>
      {/if}

      <Composer
        {lane}
        bind:focusTarget={composerField}
        draft={store.draft}
        busy={store.state.busy}
        connected={store.connected}
        activity={store.state.activity}
        commands={store.state.commands}
        models={store.state.models}
        currentModel={store.state.currentModel}
        pick={store.state.pick}
        pickUnavailable={store.state.pickUnavailable}
        {handoffPending}
        agent={store.state.agent}
        approvals={store.state.approvals}
        waiting={store.state.permissions.length > 0}
        usage={store.state.usage}
        ondraft={(value) => store.setDraft(value)}
        onsend={(text, images) => store.send(text, images)}
        oncancel={() => store.cancel()}
        onmodel={(id) => store.setModel(id)}
        onapprovals={(mode) => store.setApprovals(mode)}
      />
    </section>

    {#if inspectorOpen}
      <Inspector
        session={store.state.session}
        cwd={currentSession?.cwd}
        agent={store.state.agent}
        currentModel={store.state.currentModel}
        pick={store.state.pick}
        pickUnavailable={store.state.pickUnavailable}
        models={store.state.models}
        usage={store.state.usage}
        items={store.state.items}
        busy={store.state.busy}
        permissions={store.state.permissions}
        commands={store.state.commands}
        oncollapse={() => setInspectorOpen(false)}
      />
    {/if}
  </div>
</Page>

<style>
  .workspace { position: relative; display: grid; grid-template-columns: auto minmax(0, 1fr) auto; height: 100%; min-height: 0; overflow: hidden; }
  .conversation { display: flex; min-width: 0; min-height: 0; flex-direction: column; background: var(--paper); }
  .transcript-wrap { position: relative; display: flex; flex: 1; min-height: 0; }
  .dock-wrap { flex: none; padding: 0 var(--s5) var(--s3); }
  .pane-reveals { display: grid; grid-template-columns: auto 1fr auto; align-items: center; min-height: 44px; padding: var(--s2) var(--s3); border-bottom: 1px solid var(--line-soft); }
  .pane-reveals button { display: inline-flex; align-items: center; gap: var(--s2); min-height: 28px; padding: 0 var(--s2); border-radius: var(--radius); color: var(--ink-mute); font-size: var(--f-small); transition: color var(--t-fast) var(--ease), background-color var(--t-fast) var(--ease); }
  .pane-reveals button:hover { color: var(--ink); background: var(--tint5); }

  @container (max-width: 1050px) {
    .workspace { grid-template-columns: minmax(0, 1fr); }
    .workspace :global(.sessions), .workspace :global(.inspector) { position: absolute; inset-block: 0; z-index: 20; border-color: var(--line-strong); }
    .workspace :global(.sessions) { left: 0; }
    .workspace :global(.inspector) { right: 0; }
  }
</style>
