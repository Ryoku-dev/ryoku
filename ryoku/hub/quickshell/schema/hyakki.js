.pragma library

var title = "Hyakki";
var lead = "Optional answers and agent sessions, with Oni on the desktop. Hyakki owns its models, features and privacy settings.";

var sections = [
    { id: "hyakki", title: "Hyakki" },
    { id: "oni", title: "Oni Vision" },
    { id: "settings", title: "Native settings" }
];

function settings(view) {
    return ["hyakki", "open", "--tab", "settings", "--view", view];
}

var rows = [
    { id: "hyakki-enabled", section: "hyakki", label: "Use Hyakki",
      detail: "Turning it on installs Hyakki when needed. Turning it off stops it and removes the app; your conversations and settings stay",
      control: "custom", activatable: true, keywords: ["hyakki", "install", "on", "off", "optional"] },
    { id: "hyakki-open", section: "hyakki", label: "Open Hyakki",
      detail: "Agent sessions and conversations in the native app. Super+Alt+Space focuses its window",
      control: "handoff", actionText: "Open", command: ["hyakki", "open"], keywords: ["agent", "chat", "sessions"] },
    { id: "hyakki-shortcuts", section: "hyakki", label: "Desktop shortcuts",
      detail: "Alt+Space opens Quick Ask, web search and downloads. Change desktop shortcuts in Ryoku Settings",
      control: "link", link: "shortcuts", keywords: ["keys", "alt space", "super alt space"] },
    { id: "oni-vision", section: "oni", label: "Vision and screen",
      detail: "Open Oni's native screen capabilities and controls. Pick chooses one clicked spot; it does not freeze the screen for a dragged box",
      control: "handoff", actionText: "Open in Hyakki", command: settings("oni-screen"), keywords: ["oni", "vision", "screen", "pick", "spot"] },
    { id: "oni-settings", section: "oni", label: "Oni on the desktop",
      detail: "Choose Oni's character and turn its capabilities on in the native app",
      control: "handoff", actionText: "Open in Hyakki", command: settings("oni"), keywords: ["oni", "character", "capabilities"] },
    { id: "oni-privacy", section: "oni", label: "Oni privacy",
      detail: "Review capture disclosure and consent in the native app. Opening this page does not enable capture, listening or input access",
      control: "handoff", actionText: "Open in Hyakki", command: settings("oni-privacy"), keywords: ["privacy", "consent", "capture"] },
    { id: "oni-writing", section: "settings", label: "Writing",
      detail: "Oni's native writing controls and capabilities",
      control: "handoff", actionText: "Open in Hyakki", command: settings("oni-writing"), keywords: ["writing", "rewrite"] },
    { id: "oni-voice", section: "settings", label: "Oni voice",
      detail: "Oni's own voice controls. Ryoku's separate voice typing stays on the Voice typing page",
      control: "handoff", actionText: "Open in Hyakki", command: settings("oni-voice"), keywords: ["voice", "microphone", "speech"] },
    { id: "hyakki-features", section: "settings", label: "Hyakki app features",
      detail: "Optional features are configured only in the native app",
      control: "handoff", actionText: "Open in Hyakki", command: settings("features"), keywords: ["features", "optional"] },
    { id: "hyakki-providers", section: "settings", label: "Models and providers",
      detail: "Manage the models and providers Hyakki uses in its native settings",
      control: "handoff", actionText: "Open in Hyakki", command: settings("providers"), keywords: ["models", "providers", "keys"] }
];

var related = ["shortcuts", "voice-typing", "screen-capture"];
