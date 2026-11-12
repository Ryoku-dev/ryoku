.pragma library

var title = "Terminal";
var lead = "The shell your terminals start in, the prompt it draws, and the greeting a new terminal shows.";

var sections = [
    { id: "shell", title: "Login shell" },
    { id: "prompt", title: "Prompt" },
    { id: "greeting", title: "Greeting" },
    { id: "picture", title: "Greeting picture" },
    { id: "lines", title: "Greeting lines" }
];

var related = ["default-apps", "fonts-icons", "startup-apps", "wallpaper-colours"];

function t(key) {
    return { store: "terminal", key: key };
}

var colourModes = [
    { label: "Fixed", value: "fixed" },
    { label: "Follow the wallpaper", value: "wallpaper" }
];

var ryogamiTheme = ["ryogami", "wallpaper", "settings", "theme"];

// rows the page fills in (choices, files, pictures) keep their words here so
// search and the page share them
var rows = [
    { id: "login-shell", section: "shell", label: "Login shell",
      detail: "New terminals, consoles and remote sessions start in it. Changing it asks for your password.",
      keywords: ["shell", "default shell", "chsh", "fish", "bash", "zsh", "usermod", "account"],
      control: "segmented", bind: t("shell") },
    { id: "zsh-prompt", section: "shell", label: "Zsh prompt",
      detail: "What Zsh draws before each command",
      keywords: ["oh my zsh", "omz", "starship", "zsh theme"],
      control: "segmented", bind: t("zshPrompt"),
      options: [{ label: "Starship", value: "starship" }, { label: "Oh My Zsh", value: "oh-my-zsh" }] },
    { id: "own-lines", section: "shell", label: "Your own shell settings",
      detail: "Lines you add to your shell's own file run in every new terminal",
      keywords: ["user.fish", "user.bash", "user.zsh", "rc", "aliases", "custom", "config.fish", "bashrc", "zshrc"],
      control: "action", actionText: "Open" },

    { id: "prompt-style", section: "prompt", label: "Prompt style",
      detail: "Shared by Fish, Bash and Zsh with Starship",
      keywords: ["starship", "layout", "pill", "minimal", "two line", "powerline", "lean", "ps1"],
      control: "picker", bind: t("starshipLayout"), "default": "pill" },
    { id: "prompt-colours", section: "prompt", label: "Prompt colours",
      detail: "Fixed Ryoku colours, or the colours of your wallpaper",
      keywords: ["palette", "theme", "colors", "starship palette", "wallpaper colours"],
      control: "segmented", bind: t("starshipPalette"), "default": "fixed", options: colourModes },
    { id: "prompt-edit-colours", section: "prompt", label: "Edit colours",
      detail: "The wallpaper's colours are set in Wallpaper & colours",
      keywords: ["palette", "matugen", "scheme", "ryogami"],
      control: "handoff", command: ryogamiTheme, actionText: "Open",
      when: { bind: t("starshipPalette"), is: ["wallpaper"] } },
    { id: "terminal-font", section: "prompt", label: "Terminal font",
      detail: "The monospace font is set in Fonts & icons",
      keywords: ["monospace", "font", "typeface", "nerd font", "size"],
      control: "link", link: "fonts-icons/monospace-font", actionText: "Open" },

    { id: "greeting-preview", section: "greeting", label: "Greeting preview", template: true,
      detail: "What a new terminal shows, drawn live",
      keywords: ["fastfetch", "readout", "neofetch", "system info", "preview"] },
    { id: "greeting-picture-kind", section: "greeting", label: "Picture",
      detail: "Art beside the greeting's lines. Logo is your system's own, drawn in text.",
      keywords: ["emblem", "logo", "ascii", "image", "fastfetch logo"],
      control: "segmented", bind: t("ff.logo.kind"),
      options: [{ label: "Image", value: "image" }, { label: "Text art", value: "ascii" },
                { label: "Logo", value: "builtin" }, { label: "None", value: "none" }] },
    { id: "greeting-colours", section: "greeting", label: "Greeting colours",
      detail: "Fixed colours, or the colours of your wallpaper",
      keywords: ["palette", "theme", "colors", "fastfetch palette"],
      control: "segmented", bind: t("ff.palette"), options: colourModes },
    { id: "greeting-accent", section: "greeting", label: "Accent colour",
      detail: "Tints the label column of every line",
      keywords: ["color", "accent", "keys colour", "hex"],
      control: "color", bind: t("ff.accent"), "default": "#e2342a",
      when: { bind: t("ff.palette"), is: ["fixed"] } },
    { id: "greeting-edit-colours", section: "greeting", label: "Edit colours",
      detail: "The wallpaper's colours are set in Wallpaper & colours",
      keywords: ["palette", "matugen", "scheme", "ryogami"],
      control: "handoff", command: ryogamiTheme, actionText: "Open",
      when: { bind: t("ff.palette"), is: ["wallpaper"] } },
    { id: "greeting-style", section: "greeting", label: "Greeting style from RyoStore", more: true,
      detail: "Use a greeting you installed from RyoStore. It replaces this one.",
      keywords: ["store", "ryostore", "style", "apply", "theme", "fastfetch style"],
      control: "custom" },
    { id: "greeting-remove-style", section: "greeting", label: "Remove an installed style", more: true,
      detail: "Delete a style from your RyoStore library. The greeting in use stays.",
      keywords: ["store", "uninstall", "delete style"],
      control: "custom" },
    { id: "greeting-browse", section: "greeting", label: "Find more styles", more: true,
      detail: "Greeting styles and pictures in RyoStore",
      keywords: ["store", "ryostore", "download", "emblems"],
      control: "handoff", command: ["ryostore", "open", "fastfetch"], actionText: "Open" },
    { id: "greeting-file", section: "greeting", label: "The greeting's file", more: true,
      detail: "Open the fastfetch settings this page writes, to edit them by hand",
      keywords: ["config.jsonc", "fastfetch config", "edit config", "jsonc"],
      control: "action", actionText: "Open" },
    { id: "greeting-reset", section: "greeting", label: "Reset the greeting", more: true,
      detail: "Go back to a plain greeting with the system logo and five lines",
      keywords: ["default", "restore", "reset fastfetch"],
      control: "custom" },

    { id: "greeting-image", section: "picture", label: "Choose a picture", template: true,
      detail: "Ryoku's marks, the shipped art and your own image",
      keywords: ["emblem", "logo", "image", "your image", "png", "svg", "import"] },
    { id: "greeting-text-art", section: "picture", label: "Text art file",
      detail: "A text file of characters drawn as the picture",
      keywords: ["ascii", "ascii art", "txt"],
      control: "custom", bind: t("ff.textArt"),
      when: { bind: t("ff.logo.kind"), is: ["ascii"] } },
    { id: "greeting-width", section: "picture", label: "Width",
      detail: "How many character columns the picture spans",
      keywords: ["size", "columns", "logo width"],
      control: "stepper", min: 0, max: 80, unit: "col", bind: t("ff.logo.width"), "default": 28,
      when: { bind: t("ff.logo.kind"), is: ["image", "ascii"] } },
    { id: "greeting-height", section: "picture", label: "Height",
      detail: "How many lines of text the picture covers",
      keywords: ["size", "rows", "logo height"],
      control: "stepper", min: 0, max: 60, unit: "lines", bind: t("ff.logo.height"), "default": 14,
      when: { bind: t("ff.logo.kind"), is: ["image", "ascii"] } },
    { id: "greeting-padding", section: "picture", label: "Space before it",
      detail: "Blank columns between the terminal's edge and the picture",
      keywords: ["padding", "margin", "pad", "indent"],
      control: "stepper", min: 0, max: 20, unit: "col", bind: t("ff.logo.padding"), "default": 3,
      when: { bind: t("ff.logo.kind"), is: ["image", "ascii"] } },
    { id: "greeting-fit", section: "picture", label: "Fit the height to the picture",
      detail: "Keeps the width and sets the height so the picture is not stretched",
      keywords: ["auto fit", "aspect", "ratio", "distort"],
      control: "custom", activatable: true,
      when: { bind: t("ff.logo.kind"), is: ["image"] } },
    { id: "greeting-dither", section: "picture", label: "Stipple the picture",
      detail: "Draws it as one-colour dots, like Ryoku's own art",
      keywords: ["dither", "1-bit", "bayer", "halftone"],
      control: "toggle", bind: t("ff.logo.dither"),
      when: { bind: t("ff.logo.kind"), is: ["image"] } },
    { id: "greeting-open-picture", section: "picture", label: "Open the picture's file",
      detail: "Look at or edit the file the greeting draws",
      keywords: ["open file", "emblem file", "logo file", "ascii file"],
      control: "action", actionText: "Open",
      when: { bind: t("ff.logo.kind"), is: ["image", "ascii"] } },
    { id: "greeting-remove-picture", section: "picture", label: "Remove your imported picture",
      detail: "Deletes the copy of your image the greeting uses and goes back to Ryoku's emblem",
      keywords: ["delete", "imported emblem", "logo file"],
      control: "custom" },

    { id: "greeting-lines", section: "lines", label: "Greeting lines", template: true,
      detail: "Add, order, rename, hide or remove the greeting's lines",
      keywords: ["modules", "rows", "info", "cpu", "memory", "kernel", "uptime", "header", "spacer", "colour swatches", "reorder"] }
];
