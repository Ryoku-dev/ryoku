pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Bluetooth
import Quickshell.Services.Pipewire
import Ryoku.Ui.Singletons
import shell.services

// audio graph for the mixer: classifies Pipewire nodes into output devices,
// input devices, and per-app playback streams; switches the default sink/source
// through the writable preferred-default properties; resolves a stream's app
// name + icon; and reads/sets a Bluetooth sink's codec + profile. Devices.qml
// stays the owner of display (brightness/vibrance); this owns sound. tracks the
// sink, source, and every node it lists so volumes and metadata read live.
Singleton {
    id: root

    readonly property var sink: Pipewire.defaultAudioSink
    readonly property var source: Pipewire.defaultAudioSource
    readonly property var nodes: (Pipewire.nodes && Pipewire.ready) ? Pipewire.nodes.values : []

    // node type flags as a string, e.g. "AudioSink" / "AudioSource" /
    // "AudioOutStream". constant, so it reads without tracking, unlike
    // properties and live audio which only populate once a node is tracked.
    function typeOf(n) {
        return (n && typeof PwNodeType !== "undefined") ? PwNodeType.toString(n.type) : "";
    }

    // The equalizer's filter pair (see ryoku-eq). WirePlumber splices it in front
    // of the real device and never treats it as a default node, so it is plumbing
    // rather than a device or an app: listing it would show the speakers twice in
    // the mixer and offer a "Ryoku Equalizer" output nobody should pick. Matched on
    // node.name, a constant, so this never reads an untracked property.
    function isShellFilter(n) {
        return ((n && n.name) + "").indexOf("ryoku_equalizer") === 0;
    }

    // a real, switchable output/input device (not a stream).
    function isOutput(n) { return !!(n && n.isSink && !n.isStream && n.audio && !root.isShellFilter(n)); }
    function isInput(n) { return !!(n && !n.isSink && !n.isStream && n.audio && !root.isShellFilter(n)); }
    // an application feeding the graph (playback, not capture). per-app here.
    function isPlayStream(n) {
        return !!(n && n.isStream && n.audio && root.typeOf(n).indexOf("In") < 0
            && !root.isShellFilter(n));
    }

    // an application capturing from the graph -- a screen recorder, a call, a
    // browser tab on the mic. per-app input, surfaced to streamers. shell and
    // plumbing capture is filtered by node.name (a constant, so this never
    // deadlocks on untracked properties): our own VU peak monitors ("quickshell"),
    // the cava visualisers, and Bluetooth's internal capture leg.
    function isCaptureStream(n) {
        if (!(n && n.isStream && n.audio && root.typeOf(n).indexOf("In") >= 0))
            return false;
        var nm = (n.name + "");
        return nm !== "quickshell" && nm !== "cava"
            && nm.indexOf("bluez_capture_internal") !== 0;
    }

    // Live, instantaneous classifications. They read only the nodes' constant
    // flags, so computing them synchronously is safe -- but views must NOT bind
    // to these: rebuilding a Repeater from the live list while Pipewire is
    // mid-dispatch of a node removal has crashed Quickshell's Pipewire service.
    // Every consumer binds to the settled snapshots below instead.
    // Dedup devices by node.name: some graphs (seen on multi-card boxes) surface
    // the same sink/source node more than once, which listed a device several
    // times and lit every copy as "default". Streams are left alone -- two
    // instances of one app share an app name but are distinct nodes to mix.
    readonly property var liveOutputs: root.dedupByName(root.nodes.filter(root.isOutput))
    readonly property var liveInputs: root.dedupByName(root.nodes.filter(root.isInput))
    readonly property var liveStreams: root.nodes.filter(root.isPlayStream)
    readonly property var liveCaptureStreams: root.nodes.filter(root.isCaptureStream)

    // Collapse nodes sharing a node.name, keeping the first. node.name is a
    // device's stable identity (alsa_output.pci-..., bluez_output.<mac>...), so
    // this removes true duplicates without merging two distinct devices.
    function dedupByName(list) {
        var seen = ({});
        var out = [];
        for (var i = 0; i < list.length; i++) {
            var n = list[i];
            var key = (n && n.name) ? ("" + n.name) : ("__i" + i);
            if (seen[key])
                continue;
            seen[key] = true;
            out.push(n);
        }
        return out;
    }

    // Settled snapshots the whole shell binds to (the bar audio widget, the
    // volume panel, the framebar menus, the popout, the visualiser). A short
    // debounce defers the reassignment past the Pipewire mutation, so no Repeater
    // anywhere rebuilds inside a removal dispatch. This is the single, shell-wide
    // crash-safety; consumers need no per-view snapshotting of their own.
    property var outputs: []
    property var inputs: []
    property var streams: []
    property var captureStreams: []

    // A settled snapshot can still hold a wrapper whose node died between two
    // syncs (PipeWire destroys the monitor's DP audio sink on unplug; the bar
    // rebuilds for the new screen before the debounce fires). Reading a
    // property of that wrapper is safe -- it reads undefined -- but handing it
    // to a Repeater with required properties crashes Quickshell while it
    // builds the delegate's meta-object. Every view binding reads its model
    // through this: name is a constant, so the check never deadlocks on an
    // untracked property, and a live node always has a string name.
    function alive(n) {
        return !!(n && typeof n.name === "string");
    }

    function syncAudioLists() {
        root.outputs = root.liveOutputs.slice();
        root.inputs = root.liveInputs.slice();
        root.streams = root.liveStreams.slice();
        root.captureStreams = root.liveCaptureStreams.slice();
    }

    Timer {
        id: listSettle
        interval: 75
        repeat: false
        onTriggered: root.syncAudioLists()
    }

    onLiveOutputsChanged: listSettle.restart()
    onLiveInputsChanged: listSettle.restart()
    onLiveStreamsChanged: listSettle.restart()
    onLiveCaptureStreamsChanged: listSettle.restart()

    Component.onCompleted: root.syncAudioLists()

    function setOutput(n) { if (n) Pipewire.preferredDefaultAudioSink = n; }
    function setInput(n) { if (n) Pipewire.preferredDefaultAudioSource = n; }

    // track every node we show so its properties (media/app/codec metadata) and
    // live audio (volume, mute) populate. classification above reads only the
    // node's constant flags, so this never deadlocks on untracked properties.
    //
    // Track the SETTLED lists, not the live ones. Rewriting a PwObjectTracker's
    // object set inside Pipewire's node-removal dispatch mutates Quickshell's
    // Pipewire structures mid-teardown -- the same hazard the view snapshots above
    // avoid -- and a mass reset (a device flap, "Device or resource busy") churns
    // the live lists on every single removal. The settled lists already exclude a
    // node by the time the debounce fires, so the tracker never re-tracks across a
    // dying node. The two default devices stay live so the bar volume/mute read
    // immediately; they are single objects, not the per-removal churn, and no view
    // shows a node before it reaches the settled list, so this costs no immediacy.
    PwObjectTracker {
        objects: [root.sink, root.source].filter(Boolean)
            .concat(root.outputs).concat(root.inputs).concat(root.streams).concat(root.captureStreams)
    }

    // --- device presentation ------------------------------------------------

    function nodeLabel(n) {
        if (!n)
            return "";
        var p = n.properties || ({});
        return n.description || n.nickname || p["node.description"] || n.name || I18n.tr("Audio device");
    }

    // a GlyphIcon name for a device, from its bluez-ness / icon hint / port.
    function nodeIcon(n) {
        if (!n)
            return "speaker";
        // an input device is a microphone -- a built-in jack, a USB mic, or a
        // bluetooth headset in its HSP/HFP mode -- so it always reads as a mic.
        if (!n.isSink)
            return "mic";
        if (isBluez(n))
            return "headphones";
        var p = n.properties || ({});
        var hint = ((p["device.icon-name"] || "") + " " + (n.name || "")).toLowerCase();
        if (hint.indexOf("headphone") >= 0 || hint.indexOf("headset") >= 0)
            return "headphones";
        if (hint.indexOf("hdmi") >= 0 || hint.indexOf("displayport") >= 0 || hint.indexOf("dp-") >= 0)
            return "monitor";
        return "speaker";
    }

    // --- bluetooth sink: codec, profile, matching device --------------------

    function isBluez(n) {
        if (!n)
            return false;
        var p = n.properties || ({});
        return (p["device.api"] || "") === "bluez5"
            || ((p["factory.name"] || "").indexOf("bluez5") >= 0)
            || ((n.name || "").indexOf("bluez") >= 0);
    }

    // bluez sinks: codec + active profile live on the bluez CARD, not the sink
    // node, so they are scanned from `pactl list cards` whenever the default
    // sink becomes bluez or a profile is toggled. all degrade to empty (chip
    // hidden) without a bluez card.
    property string btCard: ""
    property string btProfile: ""
    property string btCodec: ""

    readonly property bool sinkIsBluez: root.isBluez(root.sink)
    onSinkIsBluezChanged: root.refreshBtCard()
    onSinkChanged: if (root.sinkIsBluez) root.refreshBtCard()

    // MAC from a bluez node name (bluez_output.<MAC>.<profile>), colon-formatted.
    function btMac(n) {
        var m = ((n && n.name) ? n.name : "").match(/bluez_(?:output|input)\.([0-9A-Fa-f_]+)/);
        return m ? m[1].toUpperCase().replace(/_/g, ":") : "";
    }

    function refreshBtCard() {
        if (!root.sinkIsBluez) {
            root.btCard = ""; root.btProfile = ""; root.btCodec = "";
            return;
        }
        cardScan.running = false;
        cardScan.running = true;
    }

    function parseCards(text) {
        var blocks = ("\n" + text).split(/\nCard #\d+/);
        var mac = root.btMac(root.sink).replace(/:/g, "_");
        var card = "", prof = "", codec = "";
        for (var i = 0; i < blocks.length; i++) {
            var nm = /Name:\s*(bluez_card\.\S+)/.exec(blocks[i]);
            if (!nm)
                continue;
            var match = mac.length > 0 && nm[1].toUpperCase().indexOf(mac) >= 0;
            if (match || card.length === 0) {
                var pr = /Active Profile:\s*(\S+)/.exec(blocks[i]);
                var cd = /api\.bluez5\.codec\s*=\s*"?([A-Za-z0-9._-]+)"?/.exec(blocks[i]);
                card = nm[1];
                prof = pr ? pr[1] : "";
                codec = cd ? cd[1] : "";
                if (match)
                    break;
            }
        }
        root.btCard = card;
        root.btProfile = prof.toLowerCase();
        root.btCodec = codec.toUpperCase();
    }

    function isHeadset() {
        var p = root.btProfile;
        return p.indexOf("headset") >= 0 || p.indexOf("hfp") >= 0 || p.indexOf("hsp") >= 0;
    }

    // headset (HSP/HFP) trades fidelity for a mic; a2dp is high-fidelity playback.
    function profileLabel() {
        if (!root.btProfile.length)
            return "";
        if (root.isHeadset())
            return I18n.tr("Headset");
        return root.btProfile.indexOf("a2dp") >= 0 ? I18n.tr("Hi-Fi") : root.btProfile;
    }

    // flip the active bluez card between a2dp playback and headset mode.
    function toggleProfile() {
        if (!root.btCard.length)
            return;
        var target = root.isHeadset() ? "a2dp-sink" : "headset-head-unit";
        profileProc.command = ["pactl", "set-card-profile", root.btCard, target];
        profileProc.running = false;
        profileProc.running = true;
    }

    // the BlueZ device backing the active bluez sink, for its battery: matched
    // by MAC from the node name, else the first connected audio device.
    function btDeviceFor(n) {
        if (!root.isBluez(n) || typeof Bluetooth === "undefined" || !Bluetooth || !Bluetooth.devices)
            return null;
        var devs = Bluetooth.devices.values;
        var mac = root.btMac(n);
        for (var i = 0; i < devs.length; i++)
            if (devs[i] && ((devs[i].address || "") + "").toUpperCase() === mac)
                return devs[i];
        for (var j = 0; j < devs.length; j++)
            if (devs[j] && devs[j].connected && devs[j].batteryAvailable)
                return devs[j];
        return null;
    }

    function batteryOf(n) {
        var d = btDeviceFor(n);
        if (!d || !d.batteryAvailable)
            return -1;
        var b = d.battery;
        if (b === undefined || b === null || b <= 0)
            return -1;
        if (b <= 1)
            b = b * 100;
        return Math.round(b);
    }

    // --- per-app stream presentation ----------------------------------------

    function streamName(n) {
        var p = (n && n.properties) ? n.properties : ({});
        return p["application.name"] || p["media.name"] || (n ? n.description : "") || I18n.tr("Application");
    }

    function streamIcon(n) {
        var p = (n && n.properties) ? n.properties : ({});
        var named = (p["application.icon-name"] || "") + "";
        if (named.length) {
            var direct = Icons.path(named, true);
            if (direct.length)
                return direct;
        }
        var bin = ((p["application.process.binary"] || p["application.name"] || "") + "").toLowerCase();
        if (bin.length) {
            var e = (typeof DesktopEntries !== "undefined" && DesktopEntries.heuristicLookup)
                ? DesktopEntries.heuristicLookup(bin) : null;
            if (e && e.icon)
                return Icons.path(e.icon, "application-x-executable");
            var byBin = Icons.path(bin, true);
            if (byBin.length)
                return byBin;
        }
        return Icons.path("application-x-executable", true);
    }

    Process {
        id: profileProc
        stdout: StdioCollector {}
        stderr: StdioCollector {}
        onExited: root.refreshBtCard()
    }

    Process {
        id: cardScan
        command: ["pactl", "list", "cards"]
        stdout: StdioCollector { onStreamFinished: root.parseCards(this.text) }
    }
}
