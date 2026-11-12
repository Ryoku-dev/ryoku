.pragma library

var title = "Lock screen";
var lead = "The design your lock screen and sign-in screen wear. A design changes the look, never how you unlock.";

var sections = [
    { id: "design", title: "Lock screen design" },
    { id: "this-design", title: "This design" },
    { id: "unlocking", title: "Unlocking" }
];

var designBind = { store: "lockdesign", key: "active" };

// template rows feed search only; the page draws them itself
var rows = [
    { id: "design-gallery", section: "design", label: "Lock screen design", template: true,
      detail: "Choose the design of the lock screen and the sign-in screen",
      keywords: ["skin", "theme", "look", "style", "greeter", "sddm", "qylock", "orbital", "tape", "clockwork", "lockscreen"] },
    { id: "browse-store", section: "design", label: "Get more designs",
      detail: "Find more lock screen designs in RyoStore",
      keywords: ["store", "download", "install", "ryostore", "skins"], control: "handoff",
      command: ["ryostore", "open", "lockscreens"], actionText: "Open" },

    { id: "preview", section: "this-design", label: "Preview", template: true,
      detail: "Try the design on the real lock screen, then unlock to come back",
      keywords: ["test", "try", "lock now", "see"] },
    { id: "key-prompts", section: "this-design", label: "Security key prompts", template: true,
      detail: "Whether this design can ask for a security key's PIN",
      keywords: ["yubikey", "fido", "pin", "passkey"] },
    { id: "clock", section: "this-design", label: "Clock", template: true,
      detail: "The lock screen's clock follows your 24-hour time setting",
      keywords: ["24-hour", "12-hour", "time format", "am", "pm"] },

    { id: "unlocking", section: "unlocking", label: "Unlocking", template: true,
      detail: "Password, fingerprint and security keys, in Sign-in & security",
      keywords: ["password", "fingerprint", "security key", "unlock", "pam"] }
];

var related = ["power", "time-language", "wallpaper-colours"];
