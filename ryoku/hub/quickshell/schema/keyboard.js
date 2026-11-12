.pragma library

var title = "Keyboard";
var lead = "What your keys type, how a held key repeats, and what the special keys do.";

var sections = [
    { id: "layouts", title: "Layouts" },
    { id: "typing", title: "Typing" },
    { id: "special", title: "Special keys" },
    { id: "everywhere", title: "Use these keys everywhere" }
];

function k(key) {
    return { store: "keyboard", key: key };
}

function d(key) {
    return { store: "desktop", key: key };
}

// rows bound to the "keyboard" store edit one part of the shared layout and option strings
var rows = [
    { id: "layout", section: "layouts", label: "Layout", changeLabel: "Keyboard layout",
      detail: "What your keys type, like QWERTY, AZERTY or Dvorak",
      keywords: ["xkb", "kb_layout", "keymap", "qwerty", "azerty", "qwertz", "dvorak", "language", "input language"],
      control: "custom", activatable: true, bind: k("layout") },
    { id: "variant", section: "layouts", label: "Layout style", changeLabel: "Layout style",
      detail: "A version of the layout, like international or without dead keys",
      keywords: ["variant", "kb_variant", "intl", "international", "dead keys", "colemak", "mac"],
      control: "custom", activatable: true, bind: k("variant"), "default": "" },
    { id: "second-layout", section: "layouts", label: "Second layout", changeLabel: "Second layout",
      detail: "Another layout kept ready, to switch to with a key combination",
      keywords: ["switch layout", "two layouts", "multiple layouts", "bilingual", "group", "language switch"],
      control: "custom", activatable: true, bind: k("second"), "default": "", needs: { cap: "keyboardLayoutSwitch" } },
    { id: "switch-layouts", section: "layouts", label: "Switch layouts with", changeLabel: "Switch layouts with",
      detail: "The keys that flip between your two layouts",
      keywords: ["grp", "layout toggle", "alt shift", "super space", "change language"],
      control: "segmented", options: [
          { label: "Off", value: "" },
          { label: "Alt+Shift", value: "grp:alt_shift_toggle" },
          { label: "Super+Space", value: "grp:win_space_toggle" }
      ], bind: k("switch"), "default": "", needs: { cap: "keyboardLayoutSwitch" },
      when: { bind: k("hasSecond"), is: [true] } },

    { id: "repeat-rate", section: "typing", label: "Repeat rate",
      detail: "How many times a second a held key repeats",
      keywords: ["key repeat", "autorepeat", "repeat speed", "typematic"],
      control: "stepper", min: 1, max: 100, unit: "/s", bind: d("input.repeatRate") },
    { id: "repeat-delay", section: "typing", label: "Repeat delay",
      detail: "How long to hold a key before it starts repeating",
      keywords: ["key repeat", "autorepeat", "delay", "typematic"],
      control: "stepper", min: 100, max: 2000, unit: "ms", bind: d("input.repeatDelay") },
    { id: "num-lock", section: "typing", label: "Num Lock on at sign-in",
      detail: "Start each session with the number pad typing digits",
      keywords: ["numlock", "num lock", "numpad", "number pad", "keypad"],
      control: "toggle", bind: d("input.numlockByDefault") },

    { id: "caps-lock", section: "special", label: "Caps Lock key", changeLabel: "Caps Lock key",
      detail: "Keep it, make it another Escape or Ctrl, swap it with Escape, or turn it off",
      keywords: ["caps", "capslock", "remap", "escape", "ctrl", "control", "caps:escape", "ctrl:nocaps", "vim"],
      control: "picker", options: [
          { label: "Caps Lock", value: "" },
          { label: "Escape", value: "caps:escape" },
          { label: "Ctrl", value: "ctrl:nocaps" },
          { label: "Swap with Escape", value: "caps:swapescape" },
          { label: "Off", value: "caps:none" }
      ], bind: k("caps"), "default": "" },
    { id: "compose", section: "special", label: "Compose key", changeLabel: "Compose key",
      detail: "Press it, then two keys, to type characters like é and ñ",
      keywords: ["compose", "accents", "special characters", "dead keys", "multi key", "right alt", "menu key"],
      control: "segmented", options: [
          { label: "Off", value: "" },
          { label: "Right Alt", value: "compose:ralt" },
          { label: "Menu", value: "compose:menu" }
      ], bind: k("compose"), "default": "" },
    { id: "swap-alt-super", section: "special", label: "Swap Alt and Super", changeLabel: "Swap Alt and Super",
      detail: "Trade the two keys, as on a Mac keyboard",
      keywords: ["altwin", "swap_alt_win", "mac", "command key", "windows key", "super key", "meta"],
      control: "toggle", bind: k("swap"), "default": false },
    { id: "keyboard-options", section: "special", label: "Keyboard options", changeLabel: "Keyboard options",
      detail: "More options in xkb's own words, separated by commas. The choices above add theirs on their own.",
      keywords: ["xkb", "kb_options", "raw", "advanced", "options", "terminate", "lv3"],
      control: "text", mono: true, placeholder: "None",
      bind: k("extra"), "default": "", more: true },

    { id: "everywhere", section: "everywhere", label: "Copy to sign-in, console and startup",
      detail: "Use your layout, style and key options on the sign-in screen, in the text console and at the disk unlock prompt during startup",
      keywords: ["system-wide", "system wide", "login screen", "greeter", "sddm", "tty", "console", "vconsole", "boot", "luks", "disk password", "initramfs", "localectl"],
      control: "custom", bind: k("everywhere") }
];

var related = ["shortcuts", "mouse-touchpad", "time-language"];
