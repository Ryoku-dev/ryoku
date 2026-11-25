import QtQuick

// A per-output hold map: the key is the output name, the value says whether
// something on that output is held open (the pointer is on it, or it is still
// open). The map is replaced, never mutated, so the bindings that read it
// re-evaluate. One instance per preview, so every preview keeps its own state
// while the map logic lives in one place.
QtObject {
    id: root

    property var entries: ({})

    function set(monitor, value) {
        const name = String(monitor ?? "");
        const wanted = value === true;
        if (name === "" || (root.entries[name] === true) === wanted)
            return;
        const next = Object.assign({}, root.entries);
        if (wanted)
            next[name] = true;
        else
            delete next[name];
        root.entries = next;
    }

    function has(monitor) {
        return root.entries[String(monitor ?? "")] === true;
    }
}
