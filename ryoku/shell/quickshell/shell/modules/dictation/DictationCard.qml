import QtQuick
import Lithic as L
import Ryoku.Ui.Singletons
import shell.services

Item {
    id: root

    property real s: 1
    property string state: "idle"
    property bool live: false
    property bool busy: false
    property bool loading: false
    property bool failed: false
    property bool offNote: false
    property var levels: []
    property real levelDb: -120
    property string modelName: ""
    property string errorText: ""
    implicitHeight: Math.ceil((root.failed ? Math.max(120, proofLabel.implicitHeight + 56) : 120) * root.s)

    readonly property string word: offNote ? I18n.tr("Dictation is off")
        : failed ? I18n.tr("Dictation failed")
        : loading ? I18n.tr("Preparing the model")
        : live ? I18n.tr("Listening")
        : state === "inserting" ? I18n.tr("Typing")
        : busy ? I18n.tr("Transcribing") : I18n.tr("Ready")
    readonly property string proof: offNote ? ""
        : failed ? (errorText || I18n.tr("Open Voice typing in Settings"))
        : loading ? modelName
        : live ? I18n.tr("Speak, then Super+` to stop")
        : busy ? I18n.tr("Turning speech into text") : ""

    // Logical coordinates keep the library's type, cuts and spacing together
    // when the shell scales this surface for a particular output.
    Item {
        width: root.width / root.s
        height: root.height / root.s
        scale: root.s
        transformOrigin: Item.TopLeft

        L.Claim {
            id: claim
            x: 16
            y: 16
            tone: root.failed ? "failed" : root.live ? "alive"
                : root.busy || root.loading ? "becoming" : "idle"
            text: root.word
        }
        L.Working {
            anchors.right: parent.right
            anchors.rightMargin: 16
            y: 16
            active: visible
            visible: !root.live && (root.busy || root.loading)
        }
        L.Label {
            id: proofLabel
            x: 16
            y: 40
            width: parent.width - 32
            visible: !root.offNote
            text: root.proof
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: root.failed ? Text.Wrap : Text.NoWrap
            elide: root.failed ? Text.ElideNone : Text.ElideRight
        }
        EngravedLevel {
            x: 16
            y: 68
            width: parent.width - 96
            height: 32
            visible: !root.offNote && !root.loading && !root.failed
            levels: root.levels
            live: root.live
        }
        L.Label {
            anchors.right: parent.right
            anchors.rightMargin: 16
            y: 76
            width: 56
            visible: root.live
            text: (root.levelDb <= -60 ? "< -60" : Math.round(root.levelDb)) + " dB"
            typeRole: "monoSm"
            inkRole: "fg3"
            numeric: true
            horizontalAlignment: Text.AlignRight
        }
    }
}
