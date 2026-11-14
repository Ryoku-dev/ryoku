.pragma library

var title = "Voice typing";
var lead = "Type with your voice in any app: switch it on, choose the speech model it listens with, and pick the computer it runs the model on.";

var sections = [
    { id: "voice", title: "Voice typing" },
    { id: "model", title: "Engine and model" },
    { id: "microphone", title: "Microphone and behaviour" },
    { id: "engine", title: "Remove voice typing" }
];

function vt(key) { return { store: "dictation", key: key }; }

function model(key, label, detail, words) {
    return { id: "model-" + key, section: "model", label: label, detail: detail, template: true,
             keywords: ["model", "engine", "speech model", "download", "parakeet", "whisper"].concat(words) };
}

// template rows feed search; the page draws the models the catalogue offers
var rows = [
    { id: "vt-enabled", section: "voice", label: "Voice typing",
      detail: "Press its shortcut to type with your voice in the app in front; press it again to stop",
      keywords: ["dictation", "speech to text", "stt", "microphone", "dictate", "on", "off"],
      control: "toggle", bind: vt("enabled"), "default": false },
    { id: "vt-install", section: "voice", label: "Install voice typing",
      detail: "Install the Ryoku engine. Speech models are downloaded separately below",
      keywords: ["install", "engine", "package", "dictation", "ryoku-dictation"], control: "custom", activatable: true },
    { id: "vt-shortcut", section: "voice", label: "Voice typing shortcut",
      detail: "Change the keys that start and stop voice typing",
      keywords: ["hotkey", "keybind", "shortcut", "super grave"], control: "link", link: "shortcuts" },

    { id: "vt-device", section: "model", label: "Runs on",
      detail: "Automatic uses a supported GPU with a working Vulkan driver. CPU is available on every machine",
      keywords: ["gpu", "cpu", "vulkan", "cuda", "acceleration", "speed", "backend"],
      control: "segmented", options: [
          { label: "Automatic", value: "auto" }, { label: "CPU", value: "cpu" }, { label: "GPU", value: "gpu" }
      ], bind: vt("device"), "default": "auto" },

    model("parakeet-110m", "Parakeet 110M", "English only. Small and quick, 135 MB download", ["parakeet", "english", "small", "download"]),
    model("parakeet-v2", "Parakeet 0.6B", "English only. Clearly more accurate, still quick on a CPU", ["parakeet", "english", "accurate"]),
    model("parakeet-ultra", "Parakeet Ultra", "25 European languages, detected by itself", ["parakeet", "ultra", "european", "multilingual"]),
    model("whisper-turbo", "Whisper Large v3 Turbo", "About 100 languages, for anything outside Parakeet's set", ["whisper", "turbo", "multilingual", "languages"]),

    { id: "vt-source", section: "microphone", label: "Microphone",
      detail: "Leave it on the system default unless you dictate through a specific device",
      keywords: ["microphone", "input", "device", "headset", "mic"], control: "custom" },
    { id: "vt-language", section: "microphone", label: "Language hint",
      detail: "Empty lets the model decide. Parakeet 110M is English only; Ultra and Whisper cover more",
      keywords: ["language", "accent", "french", "german", "spanish"], control: "text", bind: vt("language"), "default": "" },
    { id: "vt-mode", section: "microphone", label: "How the words arrive",
      detail: "Paste is one clipboard step and never drops a character; typing sends each key",
      keywords: ["paste", "type", "clipboard", "keystrokes", "typing"],
      control: "segmented", options: [
          { label: "Paste", value: "paste" }, { label: "Type", value: "type" }
      ], bind: vt("mode"), "default": "paste" },
    { id: "vt-silence", section: "microphone", label: "Stop after silence",
      detail: "Milliseconds of quiet after you stop speaking. 0 keeps recording until you press the shortcut again",
      keywords: ["silence", "stop", "auto", "end", "quiet"],
      control: "stepper", min: 0, max: 8000, unit: "ms", bind: vt("silenceMs"), "default": 1500 },
    { id: "vt-max", section: "microphone", label: "Longest recording",
      detail: "The card's bar measures against this; recording stops on its own when it runs out",
      keywords: ["maximum", "length", "duration", "limit", "seconds"],
      control: "stepper", min: 10, max: 600, unit: "s", bind: vt("maxDurationSecs"), "default": 120 },
    { id: "vt-delay", section: "microphone", label: "Gap between keystrokes",
      detail: "Only used when words arrive by typing. A short gap keeps slow apps from dropping letters",
      keywords: ["delay", "keystroke", "speed", "typing"],
      control: "stepper", min: 0, max: 50, unit: "ms", bind: vt("typeDelayMs"), "default": 2,
      when: { bind: vt("mode"), is: ["type"] }, more: true },

    { id: "vt-remove", section: "engine", label: "Remove voice typing",
      detail: "Uninstalls the Ryoku engine. Your model choice and downloaded models stay on disk",
      keywords: ["uninstall", "remove", "engine", "package", "ryoku-dictation"], control: "custom", activatable: true }
];

var related = ["shortcuts", "screen-capture", "rashin"];
