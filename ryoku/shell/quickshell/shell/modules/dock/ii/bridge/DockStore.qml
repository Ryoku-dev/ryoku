pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import ".."
import Quickshell
import shell.services as ShellCfg

/**
 * The shared dock store's view for the ii design.
 *
 * The ii dock keeps its knobs in shell.json's `dock` object like every other
 * design so the daemon remains the sole writer.
 */
Singleton {
    id: root

    // The shell edge the dock resolves "auto" against: the shared store's
    // edge, which the Hub and `ryoku-shell dock edge` own.
    function sharedEdge(): string {
        return ShellCfg.Dock.resolvedEdge();
    }

    // The bar geometry the window-sizing maths reads when a bar conflicts
    // with the dock's axis. Ryoku's frame bar is always horizontal at the
    // top or bottom, never vertical.
    function sharedVertical(): bool {
        return false;
    }

    function sharedBarWidth(): real {
        return ShellCfg.Config.frameThickness;
    }

    function sharedBarHeight(): real {
        return ShellCfg.Config.frameThickness;
    }

    // The shared pins in canonical form, so the design's default order can
    // be built from what the user actually pinned elsewhere.
    function sharedPinned(): var {
        return Array.from(ShellCfg.Dock.pinnedOrStarter() ?? []);
    }

    // Leaves the ii design shares with every other design: the shared store
    // owns them, so the Stage Editor's shared rows drive this dock too.
    readonly property var sharedKeys: ({
        "enable": "enabled",
        "position": "edge",
        "height": "size",
        "enableMagnification": "magnify",
        "edgeGap": "edgeGap"
    })
    // Shared keys whose meaning is the opposite of the ii leaf: a dock that
    // hides automatically is one that does not start pinned.
    readonly property var invertedSharedKeys: ({
        "pinnedOnStartup": "autohide"
    })
    // What the shared rows show while a key is unset, so this dock agrees.
    readonly property var sharedFallbacks: ({
        "magnify": true,
        "autohide": false,
        "edgeGap": 0
    })

    function frame(): var {
        const d = ShellCfg.Dock.cfg("ii", {});
        return (d && typeof d === "object") ? d : {};
    }

    // Read one option leaf: the shared keys resolve against the top-level
    // dock object (mirroring what every other design and the Hub see); the
    // pinned list prefers an explicit ii list, then the shared pins; the
    // order falls back to a list built from the shared pins; everything else
    // lives in the dock.ii frame, dotted.
    // Leaf bindings re-evaluate whenever the dock subtree is rewritten; a
    // fresh array identity on each read would look like a user change and
    // churn the store forever. Reads are memoised against a snapshot of the
    // subtree, so a binding only sees a new object when the data moved.
    // One plain object, mutated in place: never reassigned, so it stays
    // invisible to the binding engine and cannot retrigger the read.
    property var _memo: ({ state: "", data: {}, defaults: {} })

    function snapshot(): string {
        return ShellCfg.Dock.iconRev + ":" + JSON.stringify(ShellCfg.Config.dock ?? {});
    }

    function pull(path: string, fallback: var): var {
        const m = root._memo;
        const snap = root.snapshot();
        if (snap !== m.state) {
            m.state = snap;
            m.data = {};
        }
        if (path in m.data)
            return m.data[path];
        const v = root.resolve(path, fallback);
        m.data[path] = v;
        return v;
    }

    // The dock's own code assigns leaves (a remove from its menu, a dragged
    // order), and an assignment drops the leaf's binding to the store. A leaf
    // whose change commit() actually wrote hands this back to itself, so a
    // later write from the Stage Editor, the Hub or the CLI still reaches the
    // dock. A change the binding itself produced writes nothing and is not
    // rebound: list leaves come back as fresh copies, and rebinding those
    // would fire the leaf again forever.
    function rebind(path: string, fallback: var): var {
        return Qt.binding(() => root.pull(path, fallback));
    }

    function resolve(path: string, fallback: var): var {
        const shared = root.sharedKeys[path];
        if (shared !== undefined) {
            const v = ShellCfg.Dock.cfg(shared, root.sharedFallbacks[shared] ?? null);
            return (v === null || v === undefined) ? fallback : v;
        }
        const inverted = root.invertedSharedKeys[path];
        if (inverted !== undefined) {
            const v = ShellCfg.Dock.cfg(inverted, root.sharedFallbacks[inverted] ?? null);
            return (v === null || v === undefined) ? fallback : !v;
        }
        if (path === "pinnedApps")
            return Array.from(ShellCfg.Dock.pinnedOrStarter() ?? []);
        if (path === "order") {
            const own = root.frame().order;
            if (own !== undefined && own !== null)
                return Array.from(own, entry => {
                    const key = String(entry);
                    const prefix = key.startsWith("app:") ? "app:"
                        : key.startsWith("runningApp:") ? "runningApp:" : "";
                    return prefix ? prefix + ShellCfg.Dock.canonicalId(key.substring(prefix.length)) : key;
                });
            const built = ["pin"];
            const pins = root.sharedPinned();
            for (let i = 0; i < pins.length; i++)
                built.push("app:" + pins[i]);
            return built.concat(["runningApps", "media", "weather", "sports",
                "livePreview", "phone", "trash", "overview"]);
        }
        const parts = path.split(".");
        let node = root.frame();
        for (let i = 0; i < parts.length; i++) {
            if (node === undefined || node === null || typeof node !== "object")
                return fallback;
            node = node[parts[i]];
        }
        return (node === undefined || node === null) ? fallback : node;
    }

    // The first Changed a leaf fires is the binding's own default landing,
    // not a user edit; remembering it keeps defaults out of shell.json.
    function isDefaultWrite(path: string, current: var, value: var): bool {
        if (current !== undefined)
            return JSON.stringify(current) === JSON.stringify(value);
        const encoded = JSON.stringify(value);
        const d = root._memo.defaults;
        if (d[path] === undefined) {
            d[path] = encoded;
            return true;
        }
        return d[path] === encoded;
    }

    // Write one option leaf back, answering whether anything was written.
    // Shared keys go straight to the shared store; a pinned-apps write mirrors
    // there too so the other designs stay in sync. Everything else merges into
    // the dock.ii frame.
    function commit(path: string, value: var): bool {
        // Compare against the stored data, not the memo: an absent key reads
        // back the sentinel, and the first Changed a leaf fires is the
        // binding's own default landing, which must never be persisted.
        const current = root.resolve(path, root);
        if (current !== root && JSON.stringify(current) === JSON.stringify(value))
            return false;
        if (current === root && root.isDefaultWrite(path, undefined, value))
            return false;
        const shared = root.sharedKeys[path];
        if (shared !== undefined) {
            ShellCfg.Dock.setCfg(shared, value);
            return true;
        }
        const inverted = root.invertedSharedKeys[path];
        if (inverted !== undefined) {
            ShellCfg.Dock.setCfg(inverted, !value);
            return true;
        }
        if (path === "pinnedApps") {
            ShellCfg.Dock.setPinned(Array.from(value ?? []));
            return true;
        }
        const next = JSON.parse(JSON.stringify(root.frame()));
        const parts = path.split(".");
        let node = next;
        for (let i = 0; i < parts.length - 1; i++) {
            if (node[parts[i]] === undefined || typeof node[parts[i]] !== "object")
                node[parts[i]] = {};
            node = node[parts[i]];
        }
        node[parts[parts.length - 1]] = value;
        ShellCfg.Dock.setCfg("ii", next);
        return true;
    }
}
