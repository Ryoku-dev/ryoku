.pragma library

var title = "Voice typing";
var lead = "Type with your voice in any app: switch it on, choose the speech model it listens with, and add a key for the online model.";

var sections = [
    { id: "voice", title: "Voice typing" },
    { id: "model", title: "Engine and model" },
    { id: "key", title: "Cloud key" },
    { id: "engine", title: "Remove voice typing" }
];

function vt(key) { return { store: "voxtype", key: key }; }

function model(key, label, detail, words) {
    return { id: "model-" + key, section: "model", label: label, detail: detail, template: true,
             keywords: ["model", "engine", "speech model", "download"].concat(words) };
}

// template rows feed search; the page draws the models it finds on this computer
var rows = [
    { id: "vt-enabled", section: "voice", label: "Voice typing",
      detail: "Press its shortcut to type with your voice in the app in front; press it again to stop",
      keywords: ["dictation", "speech to text", "stt", "voxtype", "microphone", "dictate", "on", "off"],
      control: "toggle", bind: vt("enabled") },
    { id: "vt-install", section: "voice", label: "Install voice typing",
      detail: "Voice typing needs the Voxtype engine, about 600 MB before any model. A terminal opens to confirm the install",
      keywords: ["install", "voxtype", "package", "dictation"], control: "custom", activatable: true },
    { id: "vt-shortcut", section: "voice", label: "Voice typing shortcut",
      detail: "Change the keys that start and stop voice typing",
      keywords: ["hotkey", "keybind", "shortcut", "super grave"], control: "link", link: "shortcuts" },

    model("whisper-tiny-en", "Tiny English", "English only, the smallest and fastest model on this computer", ["whisper", "tiny", "english", "fast"]),
    model("whisper-fast", "Base English", "English only, quick everyday dictation", ["whisper", "base", "english"]),
    model("whisper-small-en", "Small English", "English only, the best balance of speed and accuracy", ["whisper", "small", "english", "accurate"]),
    model("whisper-base", "Base Multilingual", "Any of 99 languages, light and fast", ["whisper", "multilingual", "languages"]),
    model("parakeet-redux", "Redux Multilingual + Adapter", "25 European languages, fast everyday dictation", ["parakeet", "redux", "european", "languages"]),
    model("whisper-medium", "Medium Multilingual", "Any of 99 languages, more accurate, larger and slower", ["whisper", "medium", "multilingual"]),
    model("whisper-accurate", "Large Turbo", "Any of 99 languages, fast and accurate on capable hardware", ["whisper", "large", "turbo", "accurate"]),
    model("openai", "OpenAI API", "Online transcription; your voice is sent to OpenAI", ["cloud", "online", "openai", "api", "whisper-1"]),

    { id: "vt-key", section: "key", label: "OpenAI key",
      detail: "The online model needs your OpenAI key; it stays on this computer, in Voxtype's settings",
      keywords: ["api key", "openai", "token", "secret", "cloud"], control: "custom" },

    { id: "vt-remove", section: "engine", label: "Remove Voxtype",
      detail: "Uninstalls the voice typing engine. Your model choice and downloaded models stay on disk",
      keywords: ["uninstall", "remove", "voxtype", "package"], control: "custom", activatable: true }
];

var related = ["shortcuts", "screen-capture", "rashin"];
