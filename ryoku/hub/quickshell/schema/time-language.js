.pragma library

var title = "Time & language";
var lead = "The language Ryoku speaks, how dates and times read, your time zone, and where the weather comes from.";

var sections = [
    { id: "language", title: "Language" },
    { id: "formats", title: "Formats and clock" },
    { id: "time-zone", title: "Time zone" },
    { id: "location", title: "Location and units" }
];

function s(key, fallback) {
    return fallback === undefined ? { store: "settings", key: key } : { store: "settings", key: key, fallback: fallback };
}

// the page fills the language and region options from the shipped language table
var rows = [
    { id: "language", section: "language", label: "Language",
      detail: "The language of Settings, the bar, the panels and every Ryoku app; Auto follows your system",
      keywords: ["locale", "translation", "interface language", "ui language", "english", "deutsch", "español", "français", "polski", "português", "日本語"],
      control: "picker", options: [], bind: s("language", "auto") },

    { id: "formats", section: "formats", label: "Regional formats",
      detail: "How dates, numbers and month names are written",
      keywords: ["locale", "region", "date format", "number format", "lc_time", "country"],
      control: "picker", options: [], bind: s("formatLocale", ""), "default": "" },
    { id: "clock-24h", section: "formats", label: "24-hour clock",
      detail: "Show times as 18:30 everywhere in Ryoku; off shows 6:30 PM",
      keywords: ["24 hour", "12 hour", "am pm", "clock format", "military time", "time format"],
      control: "toggle", bind: s("general.clock_format_24_h", false), "default": false },

    { id: "time-zone", section: "time-zone", label: "Time zone",
      detail: "The zone the system clock keeps; every clock follows it",
      keywords: ["timezone", "tz", "utc", "gmt", "zoneinfo", "localtime", "world map", "clock"],
      control: "custom", activatable: true, bind: { store: "timezone", key: "zone" },
      changeLabel: "Time zone" },

    { id: "location", section: "location", label: "Location",
      detail: "The city the weather is for; leave it empty to find it from your connection",
      keywords: ["city", "weather location", "place", "geolocation", "ip"],
      control: "custom", bind: s("weatherLocation", ""), "default": "" },
    { id: "units", section: "location", label: "Temperature units",
      detail: "Used by the bar, the launcher, the widgets and every weather card; Auto follows your regional formats",
      keywords: ["weather units", "celsius", "fahrenheit", "°c", "°f", "metric", "imperial", "temperature"],
      control: "segmented", bind: s("weatherUnit", "auto"), "default": "auto",
      options: [
          { label: "Auto", value: "auto" },
          { label: "Celsius", value: "celsius" },
          { label: "Fahrenheit", value: "fahrenheit" }
      ] }
];

var related = ["fonts-icons", "launcher", "lock-screen"];
