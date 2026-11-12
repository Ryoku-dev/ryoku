.pragma library

var title = "Screen capture";
var lead = "How screen recordings and screenshots are made and where they are saved, the key press overlay for demos, and whether apps can share your screen.";

var sections = [
    { id: "recording", title: "Recording" },
    { id: "screenshots", title: "Screenshots" },
    { id: "keypress", title: "Key press overlay" },
    { id: "privacy", title: "Screen sharing" }
];

function rec(key) { return { store: "recording", key: key }; }
function shot(key) { return { store: "ryoshot", key: key }; }
function keys(key) { return { store: "keypress", key: key }; }

// custom rows draw a control the page owns; their store reads live state
var rows = [
    { id: "rec-fps", section: "recording", label: "Frame rate",
      detail: "Frames per second; higher is smoother but makes larger files",
      keywords: ["fps", "framerate", "frames per second", "smooth", "60 fps"],
      control: "stepper", min: 1, max: 360, unit: "fps", bind: rec("fps"), "default": 60 },
    { id: "rec-quality", section: "recording", label: "Quality",
      detail: "Higher looks crisper but makes larger files",
      keywords: ["video quality", "crisp", "file size"], control: "segmented", options: [
          { label: "Medium", value: "medium" }, { label: "High", value: "high" },
          { label: "Very high", value: "very_high" }, { label: "Ultra", value: "ultra" }
      ], bind: rec("quality"), "default": "very_high" },
    { id: "rec-resolution", section: "recording", label: "Largest size",
      detail: "Scale big screens down to save space; Full size keeps every pixel",
      keywords: ["resolution", "maximum resolution", "4k", "1080p", "1440p", "native", "scale"], control: "segmented", options: [
          { label: "Full size", value: "native" }, { label: "1080p", value: "1080p" },
          { label: "1440p", value: "1440p" }, { label: "4K", value: "2160p" }
      ], bind: rec("maxResolution"), "default": "native" },
    { id: "rec-folder", section: "recording", label: "Save recordings to",
      detail: "Leave it empty to use the Recordings folder in your Videos folder",
      keywords: ["folder", "directory", "path", "where", "videos", "location"],
      control: "custom", bind: rec("directory"), "default": "" },
    { id: "rec-cursor", section: "recording", label: "Show the pointer",
      detail: "Draw the mouse pointer into the video",
      keywords: ["cursor", "mouse", "pointer"], control: "toggle", bind: rec("cursor"), "default": true },
    { id: "rec-mic", section: "recording", label: "Record the microphone",
      detail: "Add your voice when you record from the screenshot tool",
      keywords: ["microphone", "mic", "voice", "audio", "narration", "ryoshot"],
      control: "toggle", bind: shot("recordMic"), "default": false },
    { id: "rec-engine", section: "recording", label: "Recorder",
      detail: "Which recorder and video encoder this computer uses",
      keywords: ["gpu screen recorder", "gsr", "backend", "encoder", "hardware"], more: true },

    { id: "rec-framerate-mode", section: "recording", label: "Frame rate mode",
      detail: "Constant plays smoothly everywhere; variable makes smaller files that can stutter",
      keywords: ["cfr", "vfr", "constant", "variable", "framerate mode"], control: "segmented", options: [
          { label: "Constant", value: "cfr" }, { label: "Variable", value: "vfr" }
      ], bind: rec("framerateMode"), "default": "cfr", more: true },
    { id: "rec-rate-control", section: "recording", label: "Size control",
      detail: "Aim for a look, or hold the video to a fixed data rate",
      keywords: ["rate control", "bitrate mode", "cbr", "constant bitrate", "quality"], control: "segmented", options: [
          { label: "By quality", value: "quality" }, { label: "Fixed bitrate", value: "cbr" }
      ], bind: rec("bitrateMode"), "default": "quality", more: true },
    { id: "rec-bitrate", section: "recording", label: "Bitrate",
      detail: "The fixed data rate of the video",
      keywords: ["bitrate", "kbps", "data rate"], control: "stepper", min: 500, max: 200000, step: 1000, unit: "kbps",
      bind: rec("bitrate"), "default": 20000, more: true,
      when: { bind: rec("bitrateMode"), is: ["cbr"] } },
    { id: "rec-container", section: "recording", label: "File type",
      detail: "MP4 plays in browsers and chat apps; MKV and WebM suit editors and the web",
      keywords: ["container", "format", "mp4", "mkv", "webm", "extension"], control: "segmented", options: [
          { label: "MP4", value: "mp4" }, { label: "MKV", value: "mkv" }, { label: "WebM", value: "webm" }
      ], bind: rec("container"), "default": "mp4", more: true },
    { id: "rec-codec", section: "recording", label: "Video codec",
      detail: "H.264 plays anywhere; HEVC and AV1 make smaller files but need a newer graphics card",
      keywords: ["codec", "h264", "h.264", "hevc", "h265", "av1"], control: "segmented", options: [
          { label: "H.264", value: "h264" }, { label: "HEVC", value: "hevc" }, { label: "AV1", value: "av1" }
      ], bind: rec("codec"), "default": "h264", more: true },
    { id: "rec-encoder", section: "recording", label: "Encode with",
      detail: "The graphics card barely loads the processor; use the processor if recording fails",
      keywords: ["encoder", "gpu", "cpu", "hardware encoding", "software encoding"], control: "segmented", options: [
          { label: "Graphics card", value: "gpu" }, { label: "Processor", value: "cpu" }
      ], bind: rec("encoder"), "default": "gpu", more: true },
    { id: "rec-colour-range", section: "recording", label: "Colour range",
      detail: "Limited matches most players; full is richer but can look washed out in some",
      keywords: ["color range", "full range", "limited range"], control: "segmented", options: [
          { label: "Limited", value: "limited" }, { label: "Full", value: "full" }
      ], bind: rec("colorRange"), "default": "limited", more: true },
    { id: "rec-keyint", section: "recording", label: "Keyframe interval",
      detail: "Seconds between keyframes; shorter seeks more smoothly but makes larger files",
      keywords: ["keyframe", "gop", "keyint", "seeking"], control: "stepper", min: 1, max: 600, unit: "s",
      bind: rec("keyint"), "default": 2, more: true },
    { id: "rec-audio-codec", section: "recording", label: "Audio codec",
      detail: "Opus sounds better at low bitrates; AAC opens in more editors",
      keywords: ["audio", "opus", "aac", "sound"], control: "segmented", options: [
          { label: "Opus", value: "opus" }, { label: "AAC", value: "aac" }
      ], bind: rec("audioCodec"), "default": "opus", more: true },
    { id: "rec-audio-bitrate", section: "recording", label: "Audio bitrate",
      detail: "Automatic lets the codec choose",
      keywords: ["audio bitrate", "kbps", "sound quality"], control: "picker", options: [
          { label: "Automatic", value: 0 }, { label: "64 kbps", value: 64 }, { label: "96 kbps", value: 96 },
          { label: "128 kbps", value: 128 }, { label: "160 kbps", value: 160 }, { label: "192 kbps", value: 192 },
          { label: "256 kbps", value: 256 }, { label: "320 kbps", value: 320 }
      ], bind: rec("audioBitrate"), "default": 0, more: true },

    { id: "shot-folder", section: "screenshots", label: "Save screenshots to",
      detail: "Leave it empty to use the Screenshots folder in your Pictures folder",
      keywords: ["folder", "directory", "path", "where", "pictures", "location", "ryoshot"],
      control: "custom", bind: shot("saveDir"), "default": "" },
    { id: "shot-copy", section: "screenshots", label: "Copy when saving",
      detail: "Saving a screenshot also copies it to the clipboard",
      keywords: ["clipboard", "copy on save", "paste"], control: "toggle", bind: shot("copyOnSave"), "default": false },
    { id: "shot-delay", section: "screenshots", label: "Wait before the shot",
      detail: "Time to open a menu or hover something first",
      keywords: ["delay", "timer", "countdown", "seconds"], control: "segmented", options: [
          { label: "None", value: 0 }, { label: "1 s", value: 1 }, { label: "3 s", value: 3 },
          { label: "5 s", value: 5 }, { label: "10 s", value: 10 }
      ], bind: shot("delaySeconds"), "default": 0 },
    { id: "shot-shortcut", section: "screenshots", label: "Screenshot shortcut",
      detail: "Change the keys that open the screenshot tool",
      keywords: ["print", "print screen", "hotkey", "keybind", "shortcut"], control: "link", link: "shortcuts" },
    { id: "shot-blur", section: "screenshots", label: "Blur strength",
      detail: "How strongly the blur tool hides what is under it",
      keywords: ["blur", "redact", "hide", "privacy"], control: "slider", min: 16, max: 128, step: 4, unit: "px",
      bind: shot("blurRadius"), "default": 64, more: true },
    { id: "shot-pixelate", section: "screenshots", label: "Pixelate block size",
      detail: "How coarse the pixelate tool makes what it covers",
      keywords: ["mosaic", "pixelate", "censor", "redact"], control: "slider", min: 6, max: 32, step: 1, unit: "px",
      bind: shot("mosaicBlock"), "default": 12, more: true },
    { id: "shot-zoom", section: "screenshots", label: "Magnifier zoom",
      detail: "How far the magnifier enlarges what is under it",
      keywords: ["zoom", "magnify", "loupe"], control: "slider", min: 1.5, max: 4, step: 0.1, unit: "×",
      bind: shot("zoomFactor"), "default": 2, more: true },

    { id: "key-style", section: "keypress", label: "Keycap style",
      detail: "Dark keycaps with white letters, or light ones with black letters",
      keywords: ["keycaps", "theme", "dark", "light", "screencast keys", "show keys"], control: "segmented", options: [
          { label: "Dark", value: "dark" }, { label: "Light", value: "light" }
      ], bind: keys("theme"), "default": "dark" },
    { id: "key-visible", section: "keypress", label: "Keys shown",
      detail: "Show every key, or only shortcuts so typed passwords stay off screen",
      keywords: ["shortcuts only", "all keys", "passwords", "typing"], control: "segmented", options: [
          { label: "All keys", value: "all" }, { label: "Shortcuts only", value: "shortcuts" }
      ], bind: keys("mode"), "default": "all" },
    { id: "key-sample", section: "keypress", label: "Place the overlay",
      detail: "Show a sample on the desktop and drag it where it should appear",
      keywords: ["position", "placement", "move", "drag", "preview", "reset position"],
      control: "custom", activatable: true, bind: keys("placement") },

    { id: "share-picker", section: "privacy", label: "Screen picker",
      detail: "Whether apps can ask which screen or window to share",
      keywords: ["screen sharing", "screencast", "portal", "share screen", "discord", "zoom", "meet", "obs", "repair", "doctor"],
      control: "custom" }
];

var related = ["shortcuts", "voice-typing", "performance"];
