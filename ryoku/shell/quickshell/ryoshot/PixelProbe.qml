import QtQuick

// Reads one device pixel of the frozen frame back to a hex string. QML has no
// path from a texture to a colour value, so it grabs a one-pixel view of the
// frame and reads it through a Canvas. While tracking it follows the point at
// most once per frame interval and never queues a second grab behind the first.
Item {
    id: probe

    required property Item source
    property real dpr: 1
    property bool tracking: false
    property real cx: 0
    property real cy: 0
    property string hex: ""

    property var _waiting: []
    property bool _busy: false
    property real _at: -1

    width: 1
    height: 1
    opacity: 0

    function _key(x, y) { return Math.floor(x * probe.dpr) + "," + Math.floor(y * probe.dpr); }

    /** Reads the pixel under local (lx, ly) and calls cb with "#rrggbb", or "" on failure. */
    function sample(lx, ly, cb) {
        probe._waiting.push({ x: lx, y: ly, cb: cb });
        probe._pump();
    }

    function _pump() {
        if (probe._busy) return;
        var job = probe._waiting.length ? probe._waiting.shift()
            : (probe.tracking && probe._at !== probe._key(probe.cx, probe.cy) ? { x: probe.cx, y: probe.cy, cb: null } : null);
        if (!job) return;
        probe._busy = true;
        view.sourceRect = Qt.rect(Math.floor(job.x * probe.dpr) / probe.dpr, Math.floor(job.y * probe.dpr) / probe.dpr,
                                  1 / probe.dpr, 1 / probe.dpr);
        var key = probe._key(job.x, job.y);
        var ok = view.grabToImage(function (r) {
            reader.job = { url: r.url, key: key, cb: job.cb, keep: r };
            reader.loadImage(r.url);
            if (reader.isImageLoaded(r.url)) reader.requestPaint();
        }, Qt.size(1, 1));
        if (!ok) probe._finish("", key, job.cb);
    }

    function _finish(value, key, cb) {
        probe._busy = false;
        if (value) {
            probe._at = key;
            probe.hex = value;
        }
        if (cb) cb(value);
        probe._pump();
    }

    onCxChanged: if (tracking) pace.start()
    onCyChanged: if (tracking) pace.start()
    onTrackingChanged: if (tracking) probe._pump()

    Timer {
        id: pace
        interval: 16
        onTriggered: probe._pump()
    }

    ShaderEffectSource {
        id: view
        anchors.fill: parent
        sourceItem: probe.source
        live: true
        smooth: false
        textureSize: Qt.size(1, 1)
    }

    Canvas {
        id: reader
        property var job: null
        width: 1
        height: 1
        visible: false
        renderStrategy: Canvas.Immediate
        onImageLoaded: requestPaint()
        onPaint: {
            var j = reader.job;
            if (!j || !reader.isImageLoaded(j.url)) return;
            reader.job = null;
            var ctx = getContext("2d");
            ctx.clearRect(0, 0, 1, 1);
            ctx.drawImage(j.url, 0, 0, 1, 1);
            var d = ctx.getImageData(0, 0, 1, 1).data;
            reader.unloadImage(j.url);
            function h(n) { return (n < 16 ? "0" : "") + n.toString(16); }
            probe._finish(("#" + h(d[0]) + h(d[1]) + h(d[2])).toUpperCase(), j.key, j.cb);
        }
    }
}
