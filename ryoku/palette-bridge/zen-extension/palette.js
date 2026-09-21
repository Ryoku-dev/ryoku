"use strict";

function paletteToTheme(palette) {
  const pick = (role, fallback) => palette[role] || palette[fallback];
  return { colors: {
    frame: palette.surface,
    tab_background_text: palette.onSurface,
    toolbar: palette.surface,
    toolbar_text: palette.onSurface,
    toolbar_field: pick("surfaceContainer", "surface"),
    toolbar_field_text: palette.onSurface,
    toolbar_field_focus: pick("surfaceContainerHigh", "surface"),
    toolbar_field_text_focus: palette.onSurface,
    toolbar_field_border: pick("outlineVariant", "outline"),
    toolbar_field_border_focus: palette.primary,
    toolbar_field_highlight: pick("primaryContainer", "primary"),
    toolbar_field_highlight_text: pick("onPrimaryContainer", "onSurface"),
    tab_selected: pick("surfaceContainerHigh", "surface"),
    tab_text: palette.onSurface,
    tab_line: palette.primary,
    popup: pick("surfaceContainer", "surface"),
    popup_text: palette.onSurface,
    popup_border: pick("outlineVariant", "outline"),
    popup_highlight: pick("primaryContainer", "primary"),
    popup_highlight_text: pick("onPrimaryContainer", "onSurface"),
    sidebar: palette.surface,
    sidebar_text: palette.onSurface,
    sidebar_highlight: pick("primaryContainer", "primary"),
    sidebar_highlight_text: pick("onPrimaryContainer", "onSurface"),
    sidebar_border: pick("outlineVariant", "surface"),
    icons: palette.onSurface,
    icons_attention: pick("tertiary", "primary"),
    button_background_hover: pick("surfaceContainerHigh", "surface"),
    button_background_active: pick("primaryContainer", "primary")
  }};
}

if (typeof module !== "undefined") module.exports = { paletteToTheme };
