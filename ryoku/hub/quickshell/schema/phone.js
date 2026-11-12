.pragma library

var title = "Phone";
var lead = "Link an Android phone to bring its notifications, screen, camera and microphone to this computer.";

var sections = [
    { id: "link", title: "Phone link" },
    { id: "notifications", title: "Notifications" },
    { id: "mirroring", title: "Screen mirroring" },
    { id: "webcam", title: "Webcam" },
    { id: "microphone", title: "Microphone" },
    { id: "paired", title: "Paired phones" }
];

function s(key, fallback) {
    return { store: "settings", key: key, fallback: fallback };
}

function p(key) {
    return { store: "phone", key: key };
}

// template rows feed search only; the page draws them itself
var rows = [
    { id: "phone-link", section: "link", label: "Phone link", template: true,
      detail: "Connect an Android phone to this computer; the first time, Ryoku installs the tools it needs",
      keywords: ["turn on", "enable", "install", "kde connect", "kdeconnect", "scrcpy", "adb", "android", "remove", "uninstall"] },
    { id: "set-up", section: "link", label: "Set up your phone", template: true,
      detail: "Pair a phone and allow USB debugging, step by step, in the Phone app",
      keywords: ["pair", "setup", "open phone", "kit", "usb debugging", "connect"] },

    { id: "show-notifications", section: "notifications", label: "Show phone notifications",
      detail: "Your phone's notifications appear on this computer",
      keywords: ["alerts", "messages", "mirror notifications", "desktop notifications"],
      control: "toggle", bind: s("phone.desktopNotifications", true), "default": true },
    { id: "contacts", section: "notifications", label: "Use your contacts",
      detail: "Show names instead of numbers for calls and messages",
      keywords: ["contacts", "names", "caller", "address book"],
      control: "toggle", bind: s("phone.contacts.enabled", true), "default": true },
    { id: "hide-unnamed", section: "notifications", label: "Hide unnamed contacts",
      detail: "Leave numbers without a saved name out of your contacts",
      keywords: ["nameless", "unknown numbers", "unsaved"],
      control: "toggle", bind: s("phone.contacts.hideUnnamed", true), "default": true, more: true,
      when: { bind: s("phone.contacts.enabled", true), is: [true] } },
    { id: "alarms", section: "notifications", label: "Phone alarms",
      detail: "Show the phone's next alarm and tell you when it rings",
      keywords: ["alarm", "clock", "wake up", "next alarm"],
      control: "toggle", bind: s("phone.alarm.enabled", true), "default": true, more: true },
    { id: "alarm-interval", section: "notifications", label: "Check alarms every",
      detail: "How often this computer asks the phone for its next alarm",
      keywords: ["poll", "interval", "minutes", "alarm check"],
      control: "stepper", min: 1, max: 1440, unit: "min",
      bind: s("phone.alarm.pollMinutes", 10), "default": 10, more: true,
      when: { bind: s("phone.alarm.enabled", true), is: [true] } },
    { id: "clock-apps", section: "notifications", label: "Clock apps", template: true, more: true,
      detail: "Apps whose notifications mean an alarm is ringing",
      keywords: ["alarm apps", "clock packages", "deskclock", "package names"] },
    { id: "ignored-apps", section: "notifications", label: "Apps that never count as alarms", template: true, more: true,
      detail: "Apps that set alarms for their own automations, not to wake you",
      keywords: ["ignored packages", "ignore", "automation", "alarm"] },

    { id: "quality", section: "mirroring", label: "Picture quality",
      detail: "Sharper pictures use more of your network and the phone's battery",
      keywords: ["resolution", "bitrate", "bit rate", "fps", "frame rate", "max size", "saver", "balanced", "sharp", "scrcpy"],
      control: "segmented", options: [
          { label: "Saver", value: { maxFps: 30, maxSize: 1024, videoBitRate: "4M" } },
          { label: "Balanced", value: { maxFps: 60, maxSize: 1600, videoBitRate: "8M" } },
          { label: "Full size", value: { maxFps: 60, maxSize: 0, videoBitRate: "8M" } },
          { label: "Sharp", value: { maxFps: 60, maxSize: 0, videoBitRate: "16M" } }
      ],
      bind: p("quality"), "default": { maxFps: 60, maxSize: 0, videoBitRate: "8M" } },
    { id: "screen-off", section: "mirroring", label: "Turn the phone screen off",
      detail: "Keep the phone's own screen dark while you mirror it",
      keywords: ["dark", "display off", "battery", "turn screen off"],
      control: "toggle", bind: s("phone.scrcpy.turnScreenOff", false), "default": false },
    { id: "phone-audio", section: "mirroring", label: "Play phone sound here",
      detail: "Hear the phone's audio through this computer",
      keywords: ["audio", "sound", "speakers", "forward audio"],
      control: "toggle", bind: s("phone.scrcpy.audio", true), "default": true },
    { id: "clipboard", section: "mirroring", label: "Share the clipboard",
      detail: "Copy on one device and paste on the other",
      keywords: ["copy", "paste", "clipboard sync"],
      control: "toggle", bind: s("phone.scrcpy.clipboardSync", true), "default": true },
    { id: "stay-awake", section: "mirroring", label: "Keep the phone awake",
      detail: "The phone does not sleep while it is connected",
      keywords: ["sleep", "screen timeout", "stay awake"],
      control: "toggle", bind: s("phone.scrcpy.stayAwake", true), "default": true },
    { id: "keyboard-mode", section: "mirroring", label: "Typing on the phone",
      detail: "Hardware keeps your keyboard layout and shortcuts; Compatible works on more phones",
      keywords: ["keyboard", "uhid", "sdk", "layout", "input"],
      control: "segmented", options: [
          { label: "Hardware", value: "uhid" },
          { label: "Compatible", value: "sdk" }
      ],
      bind: s("phone.scrcpy.keyboardMode", "uhid"), "default": "uhid", more: true },
    { id: "resume", section: "mirroring", label: "Reopen dropped windows",
      detail: "Phone windows that closed because the connection dropped open again when it is back",
      keywords: ["reconnect", "auto resume", "sessions"],
      control: "toggle", bind: s("phone.scrcpy.autoResume", true), "default": true, more: true },
    { id: "show-touches", section: "mirroring", label: "Show touches",
      detail: "Draw a dot where you touch the mirrored screen",
      keywords: ["touch points", "taps", "pointer"],
      control: "toggle", bind: s("phone.scrcpy.showTouches", false), "default": false, more: true },
    { id: "app-icons", section: "mirroring", label: "App icons",
      detail: "Load icons for phone apps you open as windows",
      keywords: ["app windows", "android apps", "icons"],
      control: "toggle", bind: s("phone.scrcpy.appMode.showAppIcons", true), "default": true, more: true },
    { id: "auto-unlock", section: "mirroring", label: "Unlock before opening an app",
      detail: "Show the phone's lock screen here first, so a phone app can start",
      keywords: ["unlock", "lock screen", "app windows", "pin"],
      control: "toggle", bind: s("phone.scrcpy.appMode.autoUnlock", true), "default": true, more: true },
    { id: "after-closing", section: "mirroring", label: "When a phone window closes",
      detail: "What the phone does after you close its window here",
      keywords: ["session end", "close", "home", "lock", "continue", "app windows"],
      control: "segmented", options: [
          { label: "Go home", value: "home" },
          { label: "Keep the app", value: "continue" },
          { label: "Screen off", value: "lock" }
      ],
      bind: s("phone.scrcpy.appMode.onSessionEnd", "home"), "default": "home", more: true },
    { id: "own-display", section: "mirroring", label: "Separate phone display",
      detail: "Show the phone and its apps on a display of their own, sized for this computer",
      keywords: ["flex display", "virtual display", "new display", "dex", "app windows"],
      control: "toggle", bind: s("phone.scrcpy.appMode.flexDisplay", false), "default": false, more: true },
    { id: "own-display-width", section: "mirroring", label: "Separate display width",
      keywords: ["virtual display", "width", "pixels", "size"],
      control: "stepper", min: 320, max: 7680, step: 80, unit: "px",
      bind: s("phone.scrcpy.appMode.displayWidth", 1280), "default": 1280, more: true,
      when: { bind: s("phone.scrcpy.appMode.flexDisplay", false), is: [true] } },
    { id: "own-display-height", section: "mirroring", label: "Separate display height",
      keywords: ["virtual display", "height", "pixels", "size"],
      control: "stepper", min: 320, max: 7680, step: 80, unit: "px",
      bind: s("phone.scrcpy.appMode.displayHeight", 960), "default": 960, more: true,
      when: { bind: s("phone.scrcpy.appMode.flexDisplay", false), is: [true] } },
    { id: "own-display-density", section: "mirroring", label: "Separate display density",
      detail: "Higher numbers draw the phone's apps larger",
      keywords: ["dpi", "density", "virtual display", "scale"],
      control: "stepper", min: 72, max: 640, step: 8, unit: "dpi",
      bind: s("phone.scrcpy.appMode.displayDpi", 240), "default": 240, more: true,
      when: { bind: s("phone.scrcpy.appMode.flexDisplay", false), is: [true] } },
    { id: "recording-folder", section: "mirroring", label: "Phone recordings folder", template: true, more: true,
      detail: "Where recordings of the phone's screen go; empty uses your Videos folder",
      keywords: ["record", "videos", "save to", "folder", "path"] },

    { id: "webcam-setup", section: "webcam", label: "Finish webcam setup", template: true,
      detail: "Set up the camera device video apps use",
      keywords: ["v4l2loopback", "camera module", "repair", "webcam missing"] },
    { id: "camera", section: "webcam", label: "Camera",
      detail: "Which of the phone's cameras video apps see",
      keywords: ["lens", "front camera", "back camera", "selfie", "rear"],
      control: "segmented", options: [
          { label: "Back", value: "back" },
          { label: "Front", value: "front" }
      ],
      bind: s("phone.webcam.cameraFacing", "back"), "default": "back" },
    { id: "webcam-size", section: "webcam", label: "Picture size",
      detail: "How many pixels video apps get from the phone",
      keywords: ["resolution", "720p", "1080p", "hd", "full hd"],
      control: "segmented", options: [
          { label: "720p", value: { height: 720, width: 1280 } },
          { label: "1080p", value: { height: 1080, width: 1920 } }
      ],
      bind: p("webcamSize"), "default": { height: 720, width: 1280 } },
    { id: "webcam-fps", section: "webcam", label: "Frame rate",
      detail: "Higher is smoother in video calls and uses more of the phone's battery",
      keywords: ["fps", "smoothness", "frames per second"],
      control: "segmented", options: [
          { label: "30 fps", value: 30 },
          { label: "60 fps", value: 60 }
      ],
      bind: s("phone.webcam.fps", 30), "default": 30 },
    { id: "webcam-device", section: "webcam", label: "Webcam device",
      detail: "Leave empty to use the device Ryoku sets up",
      keywords: ["video device", "/dev/video", "v4l2", "loopback"],
      control: "text", mono: true, placeholder: "/dev/video10",
      bind: s("phone.webcam.videoDevice", ""), "default": "", more: true },

    { id: "mic-level", section: "microphone", label: "Microphone level",
      detail: "How loud the phone's microphone is on this computer",
      keywords: ["gain", "volume", "input level", "boost"],
      control: "slider", min: 0, max: 200, step: 1, unit: "%",
      bind: s("phone.microphone.micGain", 100), "default": 100 },
    { id: "hear-yourself", section: "microphone", label: "Hear yourself",
      detail: "Play the phone's microphone through this computer's speakers",
      keywords: ["monitor", "loopback", "listen"],
      control: "toggle", bind: s("phone.microphone.monitor", false), "default": false },
    { id: "mic-default", section: "microphone", label: "Use as the default microphone",
      detail: "Apps use the phone's microphone while it runs, unless you pick another",
      keywords: ["default input", "default source", "pipewire"],
      control: "toggle", bind: s("phone.microphone.setDefault", false), "default": false },

    { id: "paired-list", section: "paired", label: "Paired phones", template: true,
      detail: "Phones linked to this computer; unpair one to stop it connecting",
      keywords: ["unpair", "forget", "remove phone", "devices", "battery", "connected"] },
    { id: "wireless", section: "paired", label: "Connect over Wi-Fi",
      detail: "Use wireless debugging instead of a USB cable for mirroring, the webcam and the microphone",
      keywords: ["wireless debugging", "adb", "tcpip", "network", "cable", "usb"],
      control: "toggle", bind: s("phone.scrcpy.useWireless", false), "default": false, more: true },
    { id: "wireless-auto", section: "paired", label: "Find the phone's address",
      detail: "Use the address the phone reports on your network",
      keywords: ["ip address", "auto", "discover", "mdns"],
      control: "toggle", bind: s("phone.scrcpy.autoWirelessIp", true), "default": true, more: true,
      when: { bind: s("phone.scrcpy.useWireless", false), is: [true] } },
    { id: "wireless-address", section: "paired", label: "Phone address",
      detail: "The phone's address on your network, from its wireless debugging screen",
      keywords: ["ip address", "manual", "host"],
      control: "text", mono: true, placeholder: "192.168.1.20",
      bind: s("phone.scrcpy.wirelessIp", ""), "default": "", more: true,
      when: { bind: s("phone.scrcpy.autoWirelessIp", true), is: [false] },
      also: { bind: s("phone.scrcpy.useWireless", false), is: [true] } },
    { id: "wireless-port", section: "paired", label: "Wireless debugging port",
      detail: "The port on the phone's wireless debugging screen",
      keywords: ["port", "adb port", "5555"],
      control: "stepper", min: 1, max: 65535, bind: s("phone.scrcpy.wirelessPort", 5555), "default": 5555, more: true,
      when: { bind: s("phone.scrcpy.useWireless", false), is: [true] } },
    { id: "pin-port", section: "paired", label: "Always use this port",
      detail: "Ignore other ports the phone announces",
      keywords: ["pin port", "fixed port", "adb port"],
      control: "toggle", bind: s("phone.scrcpy.pinAdbPort", false), "default": false, more: true,
      when: { bind: s("phone.scrcpy.useWireless", false), is: [true] } }
];

var related = ["bluetooth", "wifi", "notifications", "screen-capture"];

function row(id) {
    for (var i = 0; i < rows.length; i++)
        if (rows[i].id === id)
            return rows[i];
    return null;
}

// the shell keys behind each "phone" store choice, with their defaults
var composites = {
    quality: [["maxFps", "phone.scrcpy.maxFps", 60, Number], ["maxSize", "phone.scrcpy.maxSize", 0, Number],
              ["videoBitRate", "phone.scrcpy.videoBitRate", "8M", String]],
    webcamSize: [["height", "phone.webcam.height", 720, Number], ["width", "phone.webcam.width", 1280, Number]]
};

// one choice read from its keys; fields stay in alphabetical order, as rows reach the page as maps that sort them
function composite(key, get) {
    var parts = composites[key];
    if (!parts)
        return undefined;
    var out = {};
    parts.forEach(function (part) {
        var v = get(part[1]);
        out[part[0]] = part[3](v === undefined || v === null ? part[2] : v);
    });
    return out;
}

// [shell key, value] pairs that write one choice
function compositeParts(key, v) {
    var parts = composites[key];
    if (!parts || !v)
        return [];
    return parts.map(function (part) { return [part[1], v[part[0]]]; });
}
