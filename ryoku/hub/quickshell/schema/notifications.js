.pragma library

var title = "Notifications & pop-ups";
var lead = "How notification cards and the volume and brightness pop-ups look, and what notifications show.";

var sections = [
    { id: "design", title: "Notification design" },
    { id: "popups", title: "Volume and brightness pop-ups" }
];

function s(key, fallback) {
    return { store: "settings", key: key, fallback: fallback };
}

// template rows feed search only; the page draws the galleries
var rows = [
    { id: "notification-design", section: "design", label: "Notification design", template: true,
      detail: "Choose how notification cards look",
      keywords: ["notification cards", "toasts", "banners", "theme", "style", "look", "store design", "install"] },
    { id: "show-images", section: "design", label: "Show images",
      detail: "Show a picture the app sends, or its icon when there is none",
      keywords: ["notification images", "pictures", "icons", "thumbnails", "privacy", "show_images"],
      control: "toggle", bind: s("notifications.show_images", false), "default": false },
    { id: "test-notification", section: "design", label: "Send a test notification",
      detail: "See the design in use on your desktop",
      keywords: ["try", "preview", "sample", "notify-send"], control: "custom", activatable: true },
    { id: "browse-notifications", section: "design", label: "Find more notification designs",
      detail: "Browse and install designs in RyoStore",
      keywords: ["store", "download", "ryostore"], control: "handoff",
      command: ["ryostore", "open", "notifications"], actionText: "Open" },

    { id: "popup-design", section: "popups", label: "Pop-up design", template: true,
      detail: "Choose how the volume and brightness pop-ups look",
      keywords: ["osd", "volume popup", "brightness popup", "on-screen display", "overlay", "style", "look", "install"] },
    { id: "test-popup", section: "popups", label: "Show a test pop-up",
      detail: "See the design in use on your desktop",
      keywords: ["try", "preview", "sample", "osd"], control: "custom", activatable: true },
    { id: "browse-popups", section: "popups", label: "Find more pop-up designs",
      detail: "Browse and install designs in RyoStore",
      keywords: ["store", "download", "ryostore", "osd"], control: "handoff",
      command: ["ryostore", "open", "osds"], actionText: "Open" }
];

var related = ["sounds", "quick-settings", "bar"];
