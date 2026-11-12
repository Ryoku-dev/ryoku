.pragma library

var title = "Rashin";
var lead = "The system assistant: ask with free models, hand work to your coding agent and search your system and projects. It is optional, and while it is off nothing of it runs.";

var sections = [
    { id: "rashin", title: "Rashin" },
    { id: "status", title: "Status" },
    { id: "open", title: "Open Rashin" }
];

var rows = [
    { id: "rashin-enabled", section: "rashin", label: "Use Rashin",
      detail: "Off. Rashin runs nothing at all until you turn it on",
      keywords: ["rashin", "assistant", "ai", "agent", "on", "off", "install", "remove", "opt in"],
      control: "custom", activatable: true },

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
