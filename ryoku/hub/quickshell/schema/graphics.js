.pragma library

var title = "Graphics";
var lead = "Which graphics card draws the desktop, which one the screen is wired to, and how the cards are tuned.";

var sections = [
    { id: "mode", title: "Graphics mode" },
    { id: "tuning", title: "GPU tuning" },
    { id: "presets", title: "Tuning presets" }
];

var related = ["power", "performance", "displays"];

// template rows feed search; the page draws one row per knob the cards report
var rows = [
    { id: "graphics-mode", section: "mode", label: "Graphics mode",
      detail: "Which graphics card draws the desktop. Takes effect after you log out.",
      keywords: ["gpu", "hybrid", "performance", "passthrough", "discrete", "integrated", "dgpu", "igpu", "nvidia", "amd", "prime", "render", "external monitor", "monitor port", "hdmi"],
      control: "custom", bind: { store: "graphics", key: "mode" } },
    { id: "display-wired", section: "mode", label: "Display wired to",
      detail: "Which graphics card the built-in screen is wired to. Takes effect after a restart.",
      keywords: ["mux", "gpu mode", "optimus", "advanced optimus", "discrete", "dgpu", "ultimate", "bios"],
      control: "custom", bind: { store: "graphics", key: "mux" } },
    { id: "passthrough", section: "mode", more: true, template: true, label: "GPU passthrough",
      detail: "Free the discrete graphics card for a virtual machine",
      keywords: ["vfio", "vm", "virtual machine", "looking glass", "kvmfr", "libvirt", "iommu", "gaming vm", "windows vm"] },
    { id: "passthrough-checks", section: "mode", more: true, template: true, label: "Passthrough readiness checks",
      detail: "What this computer needs before a card can go to a virtual machine",
      keywords: ["iommu", "kvm", "virtualization", "readiness"] },

    { id: "tune-power-limit", section: "tuning", template: true, label: "Power limit",
      detail: "The graphics card's power budget, in watts", keywords: ["tdp", "watts", "power cap", "nvidia-smi"] },
    { id: "tune-persistence", section: "tuning", template: true, label: "Keep the driver loaded",
      detail: "The card answers faster, and may idle a little higher", keywords: ["persistence mode", "nvidia"] },
    { id: "tune-perf-level", section: "tuning", template: true, label: "Performance level",
      detail: "Let the card choose, or hold it low or high", keywords: ["dpm", "amd", "power dpm force performance level", "auto", "low", "high"] },
    { id: "tune-clock", section: "tuning", more: true, template: true, label: "Highest graphics clock",
      detail: "Overclock or underclock the card", keywords: ["overclock", "underclock", "undervolt", "mhz", "core clock", "sclk", "overdrive"] },
    { id: "tune-fan", section: "tuning", more: true, template: true, label: "Fan speed",
      detail: "Hold the card's fan at a fixed speed", keywords: ["fan", "pwm", "cooling", "noise"] },

    { id: "tune-preset", section: "presets", label: "Apply a preset",
      detail: "Quiet, Balanced and Performance, and the presets you saved",
      keywords: ["preset", "profile", "quiet", "balanced", "performance", "bundle"],
      control: "custom", bind: { store: "graphics", key: "tune|all" } },
    { id: "tune-reset", section: "presets", label: "Reset all tuning",
      detail: "Back to the driver's own settings",
      keywords: ["reset", "defaults", "undo tuning", "stock"],
      control: "custom", bind: { store: "graphics", key: "tune|reset" } },
    { id: "your-presets", section: "presets", more: true, template: true, label: "Your presets",
      detail: "Save the current tuning under a name, rename or remove it",
      keywords: ["save preset", "custom preset", "rename", "delete preset"] }
];
