// Update Spotify's color variables directly from Ryoku's wallpaper palette.
// This changes paint values in place: no renderer reload and no playback pause.
(function ryokuWallpaperColors() {
  if (window.__ryokuWallpaperColors) return;
  window.__ryokuWallpaperColors = true;
  const endpoint = "http://127.0.0.1:47616/v1/events";
  const roles = {
    text: "onSurface",
    subtext: "onSurfaceVariant",
    main: "background",
    "main-elevated": "surfaceContainer",
    "main-transition": "surface",
    highlight: "surfaceContainerHigh",
    "highlight-elevated": "surfaceContainerHighest",
    sidebar: "surfaceContainerLow",
    player: "background",
    card: "surfaceContainer",
    shadow: "shadow",
    "selected-row": "onSurface",
    button: "primary",
    "button-active": "primaryContainer",
    "button-disabled": "outlineVariant",
    "tab-active": "surfaceContainerHigh",
    notification: "tertiary",
    "notification-error": "error",
    misc: "surfaceVariant",
    "play-button": "primary",
    "play-button-active": "primaryContainer",
    "progress-fg": "secondary",
    "progress-bg": "surfaceVariant",
    heart: "error",
    "pagelink-active": "tertiary",
    "radio-btn-active": "primary",
  };

  // Modern Spotify surfaces use Encore tokens in addition to --spice-*.
  const bridgeCss = `
    :root,
    .encore-dark-theme,
    .encore-dark-theme .encore-base-set {
      --background-base: var(--spice-main) !important;
      --background-highlight: var(--spice-highlight) !important;
      --background-press: var(--spice-highlight-elevated) !important;
      --background-elevated-base: var(--spice-main-elevated) !important;
      --background-elevated-highlight: var(--spice-highlight-elevated) !important;
      --background-elevated-press: var(--spice-highlight-elevated) !important;
      --background-tinted-base: var(--spice-main-elevated) !important;
      --background-tinted-highlight: var(--spice-highlight) !important;
      --background-tinted-press: var(--spice-highlight-elevated) !important;
      --text-base: var(--spice-text) !important;
      --text-subdued: var(--spice-subtext) !important;
      --text-bright-accent: var(--spice-button) !important;
      --essential-base: var(--spice-text) !important;
      --essential-subdued: var(--spice-subtext) !important;
      --essential-bright-accent: var(--spice-button) !important;
      --decorative-base: var(--spice-text) !important;
      --decorative-subdued: var(--spice-subtext) !important;
      --encore-app-semantic-background-color: var(--spice-main) !important;
      --encore-background-highlight: var(--spice-highlight) !important;
      --encore-background-elevated-highlight: var(--spice-highlight-elevated) !important;
      --encore-box-shadow-color: var(--spice-shadow) !important;
      --encore-focus-outline-color: var(--spice-button) !important;
    }
    :root .encore-bright-accent-set {
      --background-base: var(--spice-button) !important;
      --background-highlight: var(--spice-button-active) !important;
      --background-press: var(--spice-button-active) !important;
      --text-base: var(--spice-main) !important;
      --essential-base: var(--spice-main) !important;
    }
  `;

  let bridge = document.getElementById("ryoku-wallpaper-token-bridge");
  if (!bridge) {
    bridge = document.createElement("style");
    bridge.id = "ryoku-wallpaper-token-bridge";
    bridge.textContent = bridgeCss;
    document.head.appendChild(bridge);
  }

  let last = "";
  const applied = new Map();
  function setColor(style, name, color) {
    if (applied.get(name) === color) return;
    style.setProperty(name, color);
    applied.set(name, color);
  }
  function rgb(hex) {
    const value = hex.replace("#", "");
    return [0, 2, 4].map((i) => parseInt(value.slice(i, i + 2), 16)).join(",");
  }
  function update(palette) {
    try {
      const signature = JSON.stringify(palette);
      if (signature === last) return;
      last = signature;
      const style = document.documentElement.style;
      for (const [name, role] of Object.entries(roles)) {
        const color = palette[role];
        if (!/^#[0-9a-f]{6}$/i.test(color || "")) continue;
        setColor(style, "--spice-" + name, color);
        setColor(style, "--spice-rgb-" + name, rgb(color));
      }
      if (/^#[0-9a-f]{6}$/i.test(palette.primary || "")) {
        setColor(style, "--spice-primary", palette.primary);
      }
      window.dispatchEvent(new CustomEvent("ryoku-palette-changed", { detail: palette }));
    } catch (_) {}
  }

  let reconnectTimer = null;
  function connect() {
    const events = new EventSource(endpoint);
    events.onmessage = (event) => {
      try {
        update(JSON.parse(event.data));
      } catch (_) {}
    };
    events.onerror = () => {
      events.close();
      if (reconnectTimer !== null) return;
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connect();
      }, 1000);
    };
  }
  connect();
})();
