.pragma library

var title = "Displays";
var lead = "Arrange your screens, choose how each one runs, and warm them at night.";

var sections = [
    { id: "arrangement", title: "Arrangement" },
    { id: "display", title: "Each display" },
    { id: "desktop-size", title: "Desktop size" },
    { id: "layouts", title: "Saved layouts" },
    { id: "night-light", title: "Night light", missing: "Night light needs a window manager that can tint the screen" }
];

function nl(key) { return { store: "nightlight", key: key }; }

var rows = [
    { id: "arrange", section: "arrangement", label: "Arrange displays",
      detail: "Drag a display to where it sits on your desk; edges snap together",
      keywords: ["arrangement", "layout", "position", "drag", "multi monitor", "left", "right", "above"] },
    { id: "mirror-all", section: "arrangement", label: "Mirror",
      detail: "Show the main display's picture on every display",
      keywords: ["duplicate", "clone", "same picture", "presentation", "projector"], needs: { cap: "outputMirror" } },
    { id: "extend", section: "arrangement", label: "Extend",
      detail: "Line every display up side by side as one wide desktop",
      keywords: ["side by side", "span", "spread"] },
    { id: "match-scale", section: "arrangement", label: "Match scale",
      detail: "Give each display the scale that suits its pixel density",
      keywords: ["dpi", "auto scale", "autoscale", "hidpi", "density"] },
    { id: "set-main", section: "arrangement", label: "Set as main",
      detail: "The main display holds the pointer and new windows when you sign in",
      keywords: ["primary", "main display", "default monitor", "origin"] },

    { id: "enabled", section: "display", label: "Use this display",
      detail: "Turned off, it stays dark until you turn it back on",
      keywords: ["enable", "disable", "turn off", "off", "disconnect"] },
    { id: "resolution", section: "display", label: "Resolution",
      detail: "How many pixels the display shows",
      keywords: ["mode", "pixels", "custom resolution", "1080p", "4k"] },
    { id: "refresh", section: "display", label: "Refresh rate",
      detail: "How many times a second the picture redraws",
      keywords: ["hz", "hertz", "fps", "144hz", "frame rate"] },
    { id: "scale", section: "display", label: "Scale",
      detail: "How large apps, windows and text draw on this display",
      keywords: ["hidpi", "zoom", "dpi", "fractional scaling", "bigger", "smaller"] },
    { id: "rotation", section: "display", label: "Rotation",
      detail: "Turn the picture for a display standing on its side",
      keywords: ["rotate", "portrait", "landscape", "transform", "orientation"] },
    { id: "vrr", section: "display", label: "Adaptive sync",
      detail: "The refresh rate follows games and video so motion does not tear",
      keywords: ["vrr", "freesync", "g-sync", "gsync", "variable refresh rate", "tearing"] },
    { id: "colour", section: "display", label: "Colour",
      detail: "Standard colour, a wider range of colour, or HDR",
      keywords: ["hdr", "wide gamut", "srgb", "bt2020", "10 bit", "color"], needs: { cap: "outputHdr" } },
    { id: "sdr-brightness", section: "display", label: "Everyday content brightness",
      detail: "How bright windows without HDR look while HDR is on",
      keywords: ["sdr brightness", "nits", "hdr", "dim"], needs: { cap: "outputHdr" } },
    { id: "mirror-of", section: "display", label: "Show a copy of",
      detail: "Make this display repeat another display's picture",
      keywords: ["mirror", "duplicate", "clone"], needs: { cap: "outputMirror" } },
    { id: "position", section: "display", label: "Exact position",
      detail: "In pixels from the main display's top-left corner",
      keywords: ["x", "y", "coordinates", "offset"], more: true },

    { id: "desktop-size", section: "desktop-size", label: "Size on all displays",
      detail: "How large the desktop's own bars, panels and text are on every display. Apps keep each display's scale.",
      keywords: ["desktop size", "interface scale", "ui scale", "shell size", "panel size", "bar size", "font scale", "bigger", "smaller"],
      control: "slider", min: 0.8, max: 1.5, step: 0.05, pct: true, "default": 1.3,
      bind: { store: "settings", key: "fontScale" } },
    { id: "desktop-size-display", section: "desktop-size", label: "Desktop size on one display",
      detail: "Make the desktop's bars, panels and text larger or smaller on a single display",
      keywords: ["per monitor", "ui scale", "interface scale", "per display"] },

    { id: "save-layout", section: "layouts", label: "Save the current layout",
      detail: "Keeps where your displays sit and how each one runs, as they are now",
      keywords: ["profile", "monitor profile", "save", "docked", "undocked"] },
    { id: "save-resolution", section: "layouts", label: "Save this display's resolution",
      detail: "Keeps the selected display's resolution and scale, to use again on any display",
      keywords: ["preset", "custom resolution", "save"] },
    { id: "saved", section: "layouts", label: "Your saved layouts",
      detail: "Apply a saved layout or remove it",
      keywords: ["profiles", "presets", "apply", "delete", "remove"] },

    { id: "night-on", section: "night-light", label: "Warm the screen",
      detail: "Cut blue light with a warmer tint",
      keywords: ["night light", "blue light", "warm", "redshift", "gammastep", "wlsunset", "night shift", "eye strain"],
      control: "toggle", bind: nl("on"), "default": false, needs: { cap: "nightLight" } },
    { id: "night-temp", section: "night-light", label: "Warmth",
      detail: "Lower numbers are warmer",
      keywords: ["temperature", "kelvin", "colour temperature", "color temperature"],
      control: "stepper", min: 1000, max: 6500, step: 100, unit: "K",
      bind: nl("temperature"), "default": 4000, needs: { cap: "nightLight" } },
    { id: "night-schedule", section: "night-light", label: "Turn on by itself",
      detail: "Follow the sun, or warm the screen between hours you choose",
      keywords: ["schedule", "sunset", "sunrise", "automatic", "follow the sun", "timer"],
      control: "segmented", options: [
          { label: "Never", value: "off" },
          { label: "Sunset to sunrise", value: "sun" },
          { label: "Set hours", value: "clock" }
      ],
      bind: nl("schedule"), "default": "off", needs: { cap: "nightLight" } },
    { id: "night-start", section: "night-light", label: "Starts at",
      keywords: ["start time", "bedtime", "evening"], control: "time",
      bind: nl("startAt"), "default": 1320, needs: { cap: "nightLight" },
      when: { bind: nl("schedule"), is: ["clock"] } },
    { id: "night-end", section: "night-light", label: "Ends at",
      keywords: ["end time", "wake", "morning"], control: "time",
      bind: nl("stopAt"), "default": 420, needs: { cap: "nightLight" },
      when: { bind: nl("schedule"), is: ["clock"] } },
    { id: "night-margin", section: "night-light", label: "Wait after sunset",
      detail: "Starts this long after sunset and ends this long before sunrise",
      keywords: ["edge margin", "margin", "delay", "offset"], control: "stepper", min: 0, max: 180, step: 15, unit: "min",
      bind: nl("marginMin"), "default": 60, needs: { cap: "nightLight" },
      when: { bind: nl("schedule"), is: ["sun"] } }
];

var related = ["bar", "desktop-dock", "graphics", "power"];

function row(id) {
    for (var i = 0; i < rows.length; i++)
        if (rows[i].id === id)
            return rows[i];
    return null;
}
