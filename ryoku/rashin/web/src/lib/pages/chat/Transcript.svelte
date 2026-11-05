<script lang="ts">
  import { tick } from "svelte";
  import type { Item, Permission } from "$chatstate";
  import type { BannerState } from "$lib/chat/protocol";
  import type { ChatLane } from "$lib/chat/store.svelte";
  import NeedleFace from "$lib/fx/NeedleFace.svelte";
  import ThinkingOrb from "$lib/fx/ThinkingOrb.svelte";
  import Button from "$lib/ui/Button.svelte";
  import Icon from "$lib/ui/Icon.svelte";
  import HandoffCard from "./HandoffCard.svelte";
  import MessageItem from "./MessageItem.svelte";
  import PermissionRequest from "./PermissionRequest.svelte";
  import ToolItem from "./ToolItem.svelte";

  interface Props {
    lane: ChatLane;
    items: Item[];
    /** the conversation id the Needle's face is seeded from */
    seed: string;
    harnessName: string;
    permissions: Permission[];
    banner: { state: BannerState; error: string };
    replaying: boolean;
    waitingForSession: boolean;
    busy: boolean;
    onnew: () => void;
    onanswer: (requestId: string, optionId: string) => void;
    onsuggest: (text: string) => void;
  }

  let { lane, items, seed, harnessName, permissions, banner, replaying, waitingForSession, busy, onnew, onanswer, onsuggest }: Props = $props();
  let scroller: HTMLDivElement | undefined = $state();
  let pinned = $state(true);
  let held = $state(true);
  let settleGeneration = 0;

  const unmatchedPermissions = $derived.by(() => permissions.filter((permission) =>
    !items.some((item) => item.kind === "tool" && item.id === permission.toolId),
  ));
  const restoring = $derived(replaying || waitingForSession);

  function jumpToLatest(behavior: ScrollBehavior = "smooth") {
    if (!scroller) return;
    pinned = true;
    scroller.scrollTo({ top: scroller.scrollHeight, behavior });
  }

  function noteScroll() {
    if (!scroller) return;
    pinned = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 72;
  }

  $effect(() => {
    void items;
    void permissions;
    const generation = ++settleGeneration;
    if (restoring) {
      held = true;
      pinned = true;
      return;
    }
    if (!held && !pinned) return;
    void tick().then(() => {
      if (generation !== settleGeneration || restoring || (!held && !pinned)) return;
      jumpToLatest("instant");
      held = false;
    });
  });
</script>

{#if held && restoring}
  <section class="replay-status" aria-live="polite">
    <ThinkingOrb mode="weaving" size={20} label="Restoring" />
    <span>Restoring this conversation</span>
  </section>
{/if}

<div class="transcript" class:held bind:this={scroller} onscroll={noteScroll} aria-hidden={held}>
  <div class="transcript-inner">
    {#if banner.state === "dead"}
      <section class="banner dead" role="alert">
        <div>
          <strong>The agent stopped</strong>
          <p>{banner.error || "Start a new chat to wake it again."}</p>
        </div>
        <Button variant="plate" size="sm" icon="plus" onclick={onnew}>New chat</Button>
      </section>
    {:else if banner.state === "starting" && items.length === 0}
      <section class="banner starting" aria-live="polite">
        <ThinkingOrb mode="connecting" size={20} label="Starting" />
        <span>Waking the agent</span>
      </section>
    {/if}

    {#if items.length === 0 && banner.state !== "starting" && !restoring}
      <section class="empty-chat">
        {#if lane === "ryoku"}
          <NeedleFace {seed} size={72} mood={banner.state === "dead" ? "sleeping" : "idle"} label="The Needle" />
          <p>What should we look at on this machine?</p>
          <div class="suggestions" aria-label="Suggested prompts">
            <button type="button" onclick={() => onsuggest("What is using the most memory?")}>What is using the most memory?</button>
            <button type="button" onclick={() => onsuggest("Explain my keybinds")}>Explain my keybinds</button>
            <button type="button" onclick={() => onsuggest("Run the doctor")}>Run the doctor</button>
          </div>
          <a class="lane-link" href="#/wiki">New here? Start with the wiki</a>
        {:else}
          <p>Chat with {harnessName || "Hermes"}</p>
          <span class="empty-copy">This lane knows nothing about this machine by design.</span>
          <a class="lane-link" href="#/agent">Open Agent for machine work</a>
        {/if}
      </section>
    {:else}
      <div class="items" aria-live={busy ? "polite" : "off"}>
        {#each items as item (item.id)}
          {#if item.kind === "msg"}
            <MessageItem {item} {seed} />
          {:else if item.kind === "handoff"}
            <HandoffCard {item} />
          {:else}
            <ToolItem
              {item}
              permissions={permissions.filter((permission) => permission.toolId === item.id)}
              {onanswer}
            />
          {/if}
        {/each}
        {#each unmatchedPermissions as permission (permission.requestId)}
          <div class="unmatched-permission"><PermissionRequest {permission} {onanswer} /></div>
        {/each}
      </div>
    {/if}
  </div>
</div>

{#if !pinned && !held}
  <button class="jump" type="button" onclick={() => jumpToLatest()}>
    <Icon name="chevronDown" size={14} />
    Jump to latest
  </button>
{/if}

<style>
  .transcript { position: relative; flex: 1; min-height: 0; overflow-y: auto; overscroll-behavior: contain; }
  .transcript.held { visibility: hidden; }
  .replay-status {
    position: absolute;
    inset: 0;
    z-index: 3;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--s3);
    background: var(--paper);
    color: var(--ink-mute);
    font-size: var(--f-small);
  }
  .transcript-inner { width: min(100%, var(--measure)); min-height: 100%; margin: 0 auto; padding: var(--s6) var(--s5) var(--s7); }
  .items { display: flex; flex-direction: column; gap: var(--s5); }
  .banner {
    display: flex;
    align-items: center;
    gap: var(--s3);
    margin-bottom: var(--s5);
    padding: var(--s3) var(--s4);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    color: var(--ink-dim);
  }
  .banner.dead { justify-content: space-between; border-color: color-mix(in srgb, var(--alert) 55%, transparent); }
  .banner.dead strong { color: var(--alert); font-weight: 500; }
  .banner.dead p { margin-top: 2px; color: var(--ink-mute); font-size: var(--f-small); }
  .banner.starting { width: fit-content; color: var(--ink-mute); font-size: var(--f-small); }
  .empty-chat { display: flex; flex-direction: column; align-items: flex-start; gap: var(--s4); padding-top: var(--s7); }
  .empty-chat p { color: var(--ink); font-family: var(--display); font-size: 24px; font-weight: 400; }
  .suggestions { display: flex; flex-wrap: wrap; gap: var(--s2); }
  .suggestions button {
    min-height: 28px;
    padding: var(--s1) var(--s3);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    color: var(--ink-mute);
    font-size: var(--f-small);
    transition: color var(--t-fast) var(--ease), border-color var(--t-fast) var(--ease), background-color var(--t-fast) var(--ease);
  }
  .suggestions button:hover { border-color: var(--line-strong); background: var(--tint5); color: var(--ink); }
  .empty-copy { color: var(--ink-mute); font-size: var(--f-small); }
  .lane-link { color: var(--ink-mute); font-size: var(--f-small); text-decoration: underline; text-decoration-color: var(--line-strong); text-underline-offset: 3px; }
  .lane-link:hover { color: var(--ink); }
  .unmatched-permission { margin-left: 40px; }
  .jump {
    position: absolute;
    right: var(--s5);
    bottom: var(--s3);
    z-index: 4;
    display: inline-flex;
    align-items: center;
    gap: var(--s2);
    height: 28px;
    padding: 0 var(--s3);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    background: var(--bone);
    color: var(--ink-on-bone);
    font-size: var(--f-small);
    animation: jump-in var(--t-mid) var(--ease-out);
  }
  @keyframes jump-in { from { opacity: 0; transform: translateY(8px); } }
  @media (max-width: 760px) {
    .transcript-inner { padding: var(--s5) var(--s4) var(--s6); }
    .unmatched-permission { margin-left: 0; }
  }
</style>
