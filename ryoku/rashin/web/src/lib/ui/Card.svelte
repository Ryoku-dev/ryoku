<!-- A flat plate with a hairline. The head pairs the Latin title with its
     kanji gloss when the card is a section; a bare card is just a frame. -->
<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    title?: string;
    gloss?: string;
    lead?: string;
    tools?: Snippet;
    children: Snippet;
    pad?: boolean;
    class?: string;
  }

  let { title, gloss, lead, tools, children, pad = true, class: cls = "" }: Props = $props();
</script>

<section class="card {cls}" class:pad>
  {#if title || tools}
    <header class="card-head">
      <div class="card-name">
        {#if title}<h2 class="card-title">{title}</h2>{/if}
        {#if gloss}<span class="card-gloss t-jp">{gloss}</span>{/if}
        {#if lead}<p class="card-lead">{lead}</p>{/if}
      </div>
      {#if tools}<div class="card-tools">{@render tools()}</div>{/if}
    </header>
  {/if}
  <div class="card-body">{@render children()}</div>
</section>

<style>
  .card {
    display: flex;
    flex-direction: column;
    min-width: 0;
    border: 1px solid var(--line-soft);
    border-radius: var(--radius);
    background: var(--paper);
  }
  /* The head keeps its inset even when the body is flush (a table, a
     toolbar), and the body starts a full step below the lead: a lead
     pressed against the first control read as one crowded block. */
  .card-head { padding: var(--s4) var(--s5) 0; }
  .card.pad .card-body { padding: var(--s4) var(--s5) var(--s5); }
  .card-head + .card-body { margin-top: var(--s4); }
  .card.pad .card-head + .card-body { margin-top: 0; }
  .card-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--s4); }
  .card-name { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: baseline; column-gap: var(--s2); }
  .card-title { font-size: var(--f-row); font-weight: 500; color: var(--ink); }
  .card-lead { grid-column: 1 / -1; margin-top: var(--s2); color: var(--ink-mute); font-size: var(--f-small); line-height: 1.5; max-width: 70ch; }
  .card-tools { display: flex; align-items: center; gap: var(--s2); flex: none; }
  .card-body { min-width: 0; }
</style>
