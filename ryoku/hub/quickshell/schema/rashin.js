.pragma library

var title = "Rashin";
var lead = "The system assistant: ask with free models, read the screen with Sight, hand work to your coding agent and search your system and projects. It is optional, and while it is off nothing of it runs.";

var sections = [
    { id: "rashin", title: "Rashin" },
    { id: "sight", title: "Sight" },
    { id: "status", title: "Status" },
    { id: "open", title: "Open Rashin" }
];

function sight(key, fallback) {
    return { store: "sight", key: key, fallback: fallback };
}

var rows = [
    { id: "rashin-enabled", section: "rashin", label: "Use Rashin",
      detail: "Off. Rashin runs nothing at all until you turn it on",
      keywords: ["rashin", "assistant", "ai", "agent", "on", "off", "install", "remove", "opt in"],
      control: "custom", activatable: true },

    { id: "sight-enabled", section: "sight", label: "Tap Right Alt three times",
      detail: "Then draw a box to explain, read or translate that part of the screen",
      keywords: ["sight", "screen", "tap", "shortcut", "explain", "translate", "ocr"],
      control: "toggle", bind: sight("enabled", true), "default": true },
    { id: "sight-highlight", section: "sight", label: "Hold Left Alt while highlighting",
      detail: "Release the key to show Translate, Explain and Ask beside the highlighted text",
      keywords: ["sight", "text", "highlight", "hold", "translate", "explain", "ask"],
      control: "toggle", bind: sight("highlight", true), "default": true },
    { id: "sight-mode", section: "sight", label: "Start with",
      detail: "What appears beside the box the moment you release it",
      keywords: ["sight", "screen", "explain", "text", "translate", "ask"],
      control: "segmented", options: [
          { label: "Explain", value: "explain" }, { label: "Text", value: "text" },
          { label: "Translate", value: "translate" }, { label: "Ask", value: "ask" }
      ], bind: sight("mode", "explain"), "default": "explain" },
    { id: "sight-target", section: "sight", label: "Translate into",
      detail: "The language used for translated screen text",
      keywords: ["sight", "screen", "translate", "language"],
      control: "picker", options: [], bind: sight("target", ""), "default": "" },
    { id: "sight-languages", section: "sight", label: "Text recognition languages",
      detail: "Missing languages install when enabled. Turning one off keeps its files",
      keywords: ["sight", "screen", "ocr", "text", "tesseract", "languages", "install"],
      control: "custom", bind: sight("ocrLanguages", ["eng"]), "default": ["eng"] },
    { id: "sight-local-only", section: "sight", label: "Local models only",
      detail: "Only models on this computer may see your screen; requires Rashin's providers",
      keywords: ["sight", "screen", "privacy", "local", "offline", "models"],
      control: "toggle", bind: sight("localOnly", false), "default": false },
    { id: "sight-engine", section: "sight", label: "Answers with",
      detail: "Use Ask's engine, Rashin's providers or an installed harness",
      keywords: ["sight", "screen", "engine", "providers", "harness", "ask"],
      control: "picker", options: [
          { label: "Same as Ask", value: "ask" }, { label: "Rashin's providers", value: "providers" }
      ], bind: sight("engine", "ask"), "default": "ask" },
    { id: "sight-model", section: "sight", label: "Model",
      detail: "Leave empty to use the engine's default model",
      keywords: ["sight", "screen", "model", "default"],
      control: "custom", bind: sight("model", ""), "default": "" },
    { id: "sight-shortcut", section: "sight", label: "Shortcuts",
      detail: "Choose the keys for screen boxes and highlighted text",
      keywords: ["sight", "screen", "tap", "hold", "shortcut", "keybind"],
      control: "custom", activatable: true },
    { id: "sight-status", section: "sight", label: "Sight status",
      detail: "Off",
      keywords: ["sight", "screen", "gestures", "keyboard", "access", "watching"] },

    { id: "status-ask", section: "status", label: "Ask",
      detail: "How Ask picks a model for your questions",
      keywords: ["ask", "models", "route", "free providers", "chat"] },
    { id: "status-agent", section: "status", label: "Coding agent",
      detail: "The coding agent Rashin hands work to",
      keywords: ["harness", "agent", "coding", "claude", "codex"] },
    { id: "status-index", section: "status", label: "Search index",
      detail: "What Rashin can search on this computer",
      keywords: ["index", "search", "projects", "files"] },

    { id: "rashin-open", section: "open", label: "Open Rashin",
      detail: "Ask, chat and work with agents in the Rashin app",
      keywords: ["app", "chat", "ask", "launch"], control: "handoff", command: ["rashin", "open"], actionText: "Open" },
    { id: "rashin-settings", section: "open", label: "Rashin's own settings",
      detail: "Models, providers, agents and indexes live in the Rashin app",
      keywords: ["providers", "models", "api keys", "ollama", "llama.cpp", "harness", "preferences"],
      control: "handoff", command: ["rashin", "open", "--tab", "settings"], actionText: "Open" }
];

var related = ["voice-typing", "default-apps", "add-ons"];
