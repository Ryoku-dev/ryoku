pragma ComponentBehavior: Bound

import QtQuick

Item {
    id: sounds

    required property var store
    required property var packs
    readonly property bool soundEnabled: store.soundsEnabled
    readonly property bool tabCurrent: parent && parent.tabCurrent !== undefined
        ? Boolean(parent.tabCurrent) : false
    readonly property real volume: Math.max(0, Math.min(100, store.soundVolume)) / 100
    readonly property var clickPack: packs.clickPack(store.soundTheme)
    readonly property var errorPack: packs.errorPack(store.errorTheme)
    readonly property int poolSize: Math.max(1, ((clickPack && clickPack.files) ? clickPack.files.length : 1) * 2)
    readonly property string backendQml:
        "import QtQuick\n" +
        "import QtMultimedia\n" +
        "Item {\n" +
        "    id: root\n" +
        "    property var host: null\n" +
        "    Connections {\n" +
        "        target: root.host\n" +
        "        enabled: root.host !== null\n" +
        "        function onErrorRequested() { errorEffect.play(); }\n" +
        "    }\n" +
        "    Instantiator {\n" +
        "        model: root.host ? root.host.poolSize : 0\n" +
        "        delegate: QtObject {\n" +
        "            id: keyPlayer\n" +
        "            required property int index\n" +
        "            property SoundEffect effect: SoundEffect {\n" +
        "                source: root.host ? root.host.packs.variantUrl(root.host.clickPack, keyPlayer.index) : \"\"\n" +
        "                volume: root.host ? root.host.volume : 0\n" +
        "            }\n" +
        "            property Connections listener: Connections {\n" +
        "                target: root.host\n" +
        "                enabled: root.host !== null\n" +
        "                function onKeyRequested(slot) {\n" +
        "                    if (slot === keyPlayer.index) keyPlayer.effect.play();\n" +
        "                }\n" +
        "            }\n" +
        "        }\n" +
        "    }\n" +
        "    SoundEffect {\n" +
        "        id: errorEffect\n" +
        "        source: root.host ? root.host.packs.variantUrl(root.host.errorPack, 0) : \"\"\n" +
        "        volume: root.host ? root.host.volume : 0\n" +
        "    }\n" +
        "}\n"
    property var backend: null
    property int nextPlayer: 0
    property bool queuedKey: false
    property bool queuedError: false
    signal keyRequested(int slot)
    signal errorRequested()

    function ensureBackend() {
        if (!soundEnabled || !packs.loaded || backend)
            return;
        try {
            const created = Qt.createQmlObject(backendQml, sounds, "TypingSoundPool.qml");
            created.host = sounds;
            backend = created;
            Qt.callLater(flushQueued);
        } catch (error) {
            console.warn("Typing sound pool could not load:", error);
        }
    }

    function flushQueued() {
        if (!backend)
            return;
        if (queuedKey) {
            queuedKey = false;
            keyRequested(nextPlayer);
            nextPlayer = (nextPlayer + 1) % poolSize;
        }
        if (queuedError) {
            queuedError = false;
            errorRequested();
        }
    }

    function playKey() {
        if (!soundEnabled)
            return;
        if (!backend) {
            queuedKey = true;
            ensureBackend();
            return;
        }
        keyRequested(nextPlayer);
        nextPlayer = (nextPlayer + 1) % poolSize;
    }

    function playError() {
        if (!soundEnabled || !store.errorSound)
            return;
        if (!backend) {
            queuedError = true;
            ensureBackend();
            return;
        }
        errorRequested();
    }

    // The pool is built on the first key that sounds, not on a visit: it opens an
    // audio connection the shell then keeps, and Kit is passed through far more
    // often than it is typed in.

    onSoundEnabledChanged: {
        if (soundEnabled)
            return;
        queuedKey = false;
        queuedError = false;
        if (backend) {
            const oldBackend = backend;
            backend = null;
            oldBackend.destroy();
        }
    }
}
