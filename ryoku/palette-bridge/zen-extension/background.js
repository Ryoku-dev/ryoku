"use strict";

const endpoint = "http://127.0.0.1:47616/v1/events";
let retry;
let pendingTheme;
let applying = false;
let lastTheme = "";

async function applyLatest(theme) {
  pendingTheme = theme;
  if (applying) return;
  applying = true;
  try {
    while (pendingTheme) {
      const next = pendingTheme;
      pendingTheme = null;
      const signature = JSON.stringify(next);
      if (signature === lastTheme) continue;
      try {
        await browser.theme.update(next);
        lastTheme = signature;
      } catch (error) {
        console.error("[ryoku-zen-palette] rejected palette", error);
      }
    }
  } finally {
    applying = false;
  }
}

function connect() {
  const events = new EventSource(endpoint);
  events.onmessage = async event => {
    try {
      await applyLatest(paletteToTheme(JSON.parse(event.data)));
    } catch (error) {
      console.error("[ryoku-zen-palette] rejected palette", error);
    }
  };
  events.onerror = () => {
    events.close();
    clearTimeout(retry);
    retry = setTimeout(connect, 1000);
  };
}

connect();
