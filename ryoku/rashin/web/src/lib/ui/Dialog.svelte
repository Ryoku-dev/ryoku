<!-- Something that floats over the sheet: a scrim, then a plate with a
     hairline. The only shadow in the console is here, because this is the one
     surface that genuinely floats. -->
<script lang="ts">
  import type { Snippet } from "svelte";
  import { Dialog } from "bits-ui";
  import IconButton from "./IconButton.svelte";

  interface Props {
    open: boolean;
    title: string;
    description?: string;
    width?: number;
    children: Snippet;
    footer?: Snippet;
  }

  let { open = $bindable(false), title, description, width = 520, children, footer }: Props = $props();
  let content = $state<HTMLElement | null>(null);

  // The focus trap lands on the first focusable element, the close button,
  // which would make Enter dismiss a confirm. A control marked autofocus (the
  // dialog's action) takes the initial focus instead.
  function focusAction(event: Event) {
    const target = content?.querySelector<HTMLElement>("[autofocus]");
    if (!target) return;
    event.preventDefault();
    target.focus();
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Portal>
    <Dialog.Overlay class="dlg-scrim" />
    <Dialog.Content bind:ref={content} class="dlg" style="--dlg-w: {width}px" onOpenAutoFocus={focusAction}>
      <header class="dlg-head">
        <div>
          <Dialog.Title class="dlg-title">{title}</Dialog.Title>
          {#if description}<Dialog.Description class="dlg-desc">{description}</Dialog.Description>{/if}
        </div>
        <Dialog.Close>
          {#snippet child({ props })}
            <IconButton icon="close" label="Close" tip={false} {...props} />
          {/snippet}
        </Dialog.Close>
      </header>
      <div class="dlg-body">{@render children()}</div>
      {#if footer}<footer class="dlg-foot">{@render footer()}</footer>{/if}
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>

<style>
  :global(.dlg-scrim) {
    position: fixed;
    inset: 0;
    z-index: 40;
    background: var(--scrim);
    backdrop-filter: blur(6px);
    animation: scrim-in var(--t-mid) var(--ease);
  }
  :global(.dlg) {
    position: fixed;
    z-index: 41;
    top: 50%;
    left: 50%;
    width: min(var(--dlg-w), calc(100vw - var(--s6)));
    max-height: calc(100vh - var(--s6));
    display: flex;
    flex-direction: column;
    transform: translate(-50%, -50%);
    border: 1px solid var(--line-strong);
    border-radius: var(--island-radius);
    background: var(--paper-lift);
    box-shadow: 0 24px 64px color-mix(in srgb, var(--paper) 70%, transparent);
    animation: dlg-in var(--t-mid) var(--ease-out);
  }
  @keyframes scrim-in { from { opacity: 0; } }
  @keyframes dlg-in { from { opacity: 0; transform: translate(-50%, -48%) scale(0.98); } }
  .dlg-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--s4);
    padding: var(--s5) var(--s5) var(--s3);
  }
  :global(.dlg-title) { font-family: var(--display); font-size: 22px; font-weight: 400; color: var(--ink); line-height: 1.2; }
  :global(.dlg-desc) { margin-top: var(--s1); color: var(--ink-mute); font-size: var(--f-small); }
  .dlg-body { padding: 0 var(--s5) var(--s5); overflow: auto; }
  .dlg-foot { display: flex; justify-content: flex-end; gap: var(--s2); padding: var(--s3) var(--s5) var(--s5); border-top: 1px solid var(--line-soft); }
</style>
