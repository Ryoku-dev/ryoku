.pragma library

// RecordingPage as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy. The recording.json rows name the store the
// recorder daemon owns and writes; the page edits them over the shell socket.

var rows = [
    {
        "tab": "",
        "group": "KEY PRESSES",
        "key": "keypressTheme",
        "label": "Keycap style",
        "desc": "Dark keycaps use white type; Light keycaps use black type",
        "ctl": "seg",
        "src": "keypresses.json",
        "opts": [
            "dark",
            "light"
        ]
    },
    {
        "tab": "",
        "group": "KEY PRESSES",
        "key": "keypressMode",
        "label": "Visible keys",
        "desc": "Show every key, or hide ordinary typing and keep shortcuts.",
        "ctl": "seg",
        "src": "keypresses.json",
        "opts": [
            "all",
            "shortcuts"
        ]
    },
    {
        "tab": "",
        "group": "QUALITY",
        "key": "fps",
        "label": "Framerate",
        "desc": "Frames per second; higher is smoother but larger.",
        "ctl": "step",
        "src": "recording.json",
        "unit": "fps"
    },
    {
        "tab": "",
        "group": "QUALITY",
        "key": "framerateMode",
        "label": "Framerate mode",
        "desc": "Constant plays everywhere; variable is smaller but choppier.",
        "ctl": "seg",
        "src": "recording.json",
        "opts": [
            "cfr",
            "vfr"
        ]
    },
    {
        "tab": "",
        "group": "QUALITY",
        "key": "quality",
        "label": "Quality",
        "desc": "Higher settings look crisper but make larger files.",
        "ctl": "seg",
        "src": "recording.json",
        "opts": [
            "medium",
            "high",
            "very_high",
            "ultra"
        ]
    },
    {
        "tab": "",
        "group": "QUALITY",
        "key": "bitrateMode",
        "label": "Rate control",
        "desc": "Quality targets a look; Constant pins a fixed bitrate.",
        "ctl": "seg",
        "src": "recording.json",
        "opts": [
            "quality",
            "cbr"
        ]
    },
    {
        "tab": "",
        "group": "QUALITY",
        "key": "bitrate",
        "label": "Bitrate",
        "desc": "The fixed data rate used in Constant mode.",
        "ctl": "step",
        "src": "recording.json",
        "unit": "kbps"
    },
    {
        "tab": "",
        "group": "QUALITY",
        "key": "maxResolution",
        "label": "Maximum resolution",
        "desc": "Scale the capture down to save space; Native keeps full size.",
        "ctl": "seg",
        "src": "recording.json",
        "opts": [
            "native",
            "1080p",
            "1440p",
            "2160p"
        ]
    },
    {
        "tab": "",
        "group": "FILE",
        "key": "container",
        "label": "Container",
        "desc": "MP4 with H.264 plays in browsers and Discord; MKV and WebM are pickier.",
        "ctl": "seg",
        "src": "recording.json",
        "opts": [
            "mp4",
            "mkv",
            "webm"
        ]
    },
    {
        "tab": "",
        "group": "FILE",
        "key": "directory",
        "label": "Save recordings to",
        "desc": "Leave empty to follow your Videos folder.",
        "ctl": "text",
        "src": "recording.json"
    },
    {
        "tab": "",
        "group": "ENCODER",
        "key": "codec",
        "label": "Codec",
        "desc": "H.264 plays anywhere; HEVC and AV1 are smaller but need a newer GPU.",
        "ctl": "seg",
        "src": "recording.json",
        "opts": [
            "h264",
            "hevc",
            "av1"
        ]
    },
    {
        "tab": "",
        "group": "ENCODER",
        "key": "encoder",
        "label": "Encoder",
        "desc": "GPU barely loads the CPU; pick CPU if GPU encoding fails.",
        "ctl": "seg",
        "src": "recording.json",
        "opts": [
            "gpu",
            "cpu"
        ]
    },
    {
        "tab": "",
        "group": "ENCODER",
        "key": "cursor",
        "label": "Show the cursor",
        "desc": "Draws the mouse pointer into the video.",
        "ctl": "sw",
        "src": "recording.json"
    },
    {
        "tab": "",
        "group": "ENCODER",
        "key": "colorRange",
        "label": "Color range",
        "desc": "Limited matches most players; Full is richer but can look washed out.",
        "ctl": "seg",
        "src": "recording.json",
        "opts": [
            "limited",
            "full"
        ]
    },
    {
        "tab": "",
        "group": "ENCODER",
        "key": "keyint",
        "label": "Keyframe interval",
        "desc": "Seconds between keyframes; lower seeks smoother but grows the file.",
        "ctl": "step",
        "src": "recording.json",
        "unit": "s"
    },
    {
        "tab": "",
        "group": "AUDIO",
        "key": "audioCodec",
        "label": "Audio codec",
        "desc": "Opus sounds better at low bitrates; AAC plays in more editors.",
        "ctl": "seg",
        "src": "recording.json",
        "opts": [
            "opus",
            "aac"
        ]
    },
    {
        "tab": "",
        "group": "AUDIO",
        "key": "audioBitrate",
        "label": "Audio bitrate",
        "desc": "Leave at 0 to let the codec choose.",
        "ctl": "step",
        "src": "recording.json",
        "unit": "kbps"
    }
];
