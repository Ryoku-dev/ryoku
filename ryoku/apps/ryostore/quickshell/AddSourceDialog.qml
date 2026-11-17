import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"
import "lib/apps.js" as AppsLogic

// Adding a store: first what a source can do to this computer, then its
// address. The backend reads and checks the store's index before keeping it.
Item {
    id: dialog

    property bool open: false
    property bool reducedMotion: false
    // "warn" first, "form" once the user has read it
    property string step: "warn"
    property string problem: ""

    readonly property bool adding: Sources.busyId === "+"
    readonly property string backendError: Sources.changeErrorId === "+" ? Sources.changeError : ""

    function show() {
        dialog.step = "warn";
        dialog.problem = "";
        urlField.clear();
        nameField.clear();
        Sources.clearError();
        dialog.open = true;
        Qt.callLater(() => continueBtn.forceActiveFocus());
    }

    function close() {
        if (dialog.adding)
            return;
        dialog.open = false;
    }

    function proceed() {
        dialog.step = "form";
        Qt.callLater(() => urlField.grabFocus());
    }

    function submit() {
        const kind = AppsLogic.sourceUrlProblem(urlField.text);
        if (kind === "empty")
            dialog.problem = I18n.tr("Type the store's address.");
        else if (kind === "scheme")
            dialog.problem = I18n.tr("Use an https:// address. Plain http:// works only for a store on this computer, and file:/// for a folder.");
        else
            dialog.problem = "";
        if (kind !== "")
            return;
        Sources.add(urlField.text, nameField.text);
    }

    visible: opacity > 0
    opacity: dialog.open ? 1 : 0
    Behavior on opacity { enabled: !dialog.reducedMotion; NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }

    Connections {
        target: Sources
        function onAdded(source) { dialog.open = false; }
    }

    Keys.onEscapePressed: event => {
        dialog.close();
        event.accepted = true;
    }

    // the scrim takes every click, so nothing behind the dialog acts
    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0, 0, 0, 0.45)
        TapHandler { onTapped: dialog.close() }
        HoverHandler { }
        WheelHandler { onWheel: event => event.accepted = true }
    }

    Rectangle {
        id: panel
        objectName: "ryostore-add-source-dialog"
        anchors.centerIn: parent
        width: Math.min(parent.width - Tokens.s7 * 2, 560)
        height: body.implicitHeight + Tokens.s6 * 2
        radius: Tokens.radius
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        scale: dialog.open ? 1 : 0.97
        Behavior on scale { enabled: !dialog.reducedMotion; NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }
        Behavior on height { enabled: !dialog.reducedMotion; NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }
        clip: true
        TapHandler { }

        Column {
            id: body
            anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s6 }
            spacing: Tokens.s4

            Text {
                text: UiPreferences.brandMark("力") + "  " + (dialog.step === "warn" ? I18n.tr("BEFORE YOU ADD A SOURCE") : I18n.tr("ADD A SOURCE"))
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }
            Text {
                width: parent.width
                text: dialog.step === "warn" ? I18n.tr("Only add a store you trust") : I18n.tr("Where is the store?")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.px(24)
                font.weight: Font.Medium
                wrapMode: Text.Wrap
            }

            // what a source can do, said plainly before anything is typed
            Column {
                objectName: "ryostore-add-source-warning"
                visible: dialog.step === "warn"
                width: parent.width
                spacing: Tokens.s3

                Repeater {
                    model: [
                        I18n.tr("Ryoku does not review stores you add. Their items run inside your shell with your permissions, so only add a store you trust."),
                        I18n.tr("A plugin or a bar style from it can read your files and reach the network, like anything else you run."),
                        I18n.tr("Ryostore takes only checked, hashed pieces from it (plugins, bar styles and the like), never installers, profiles or rices. A checked piece can still be harmful."),
                        I18n.tr("Every item from a store you added is marked with its name and this warning.")
                    ]
                    delegate: Row {
                        required property string modelData
                        required property int index
                        width: parent.width
                        spacing: Tokens.s3
                        Text {
                            text: String(index + 1).padStart(2, "0")
                            color: Tokens.alert
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro
                            font.letterSpacing: Tokens.trackLabel
                            topPadding: 2
                        }
                        Text {
                            width: parent.width - Tokens.s6
                            text: modelData
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.Wrap
                        }
                    }
                }
            }

            Column {
                visible: dialog.step === "form"
                width: parent.width
                spacing: Tokens.s2

                Text {
                    text: I18n.tr("ADDRESS")
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackLabel
                }
                Field {
                    id: urlField
                    objectName: "ryostore-add-source-url"
                    width: parent.width
                    toolbar: true
                    tabular: true
                    placeholder: "https://example.org/my-store"
                    readOnly: dialog.adding
                    clearOnEscape: false
                    onAccepted: dialog.submit()
                    onEdited: dialog.problem = ""
                    onEscaped: dialog.close()
                }
                Text {
                    topPadding: Tokens.s2
                    text: I18n.tr("NAME (OPTIONAL)")
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackLabel
                }
                Field {
                    id: nameField
                    width: parent.width
                    toolbar: true
                    placeholder: I18n.tr("The store's own name when empty")
                    readOnly: dialog.adding
                    clearOnEscape: false
                    onAccepted: dialog.submit()
                    onEscaped: dialog.close()
                }
                Text {
                    width: parent.width
                    visible: text !== ""
                    topPadding: Tokens.s1
                    text: dialog.problem !== "" ? dialog.problem : dialog.backendError
                    color: Tokens.alert
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.Wrap
                }
                Text {
                    width: parent.width
                    visible: dialog.adding
                    topPadding: Tokens.s1
                    text: I18n.tr("Reading the store's index and checking it.")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
            }

            Item {
                width: parent.width
                height: buttons.implicitHeight

                Btn {
                    objectName: "ryostore-add-source-guide"
                    anchors.left: parent.left
                    text: I18n.tr("HOW TO HOST A STORE ↗")
                    motionEnabled: !dialog.reducedMotion
                    onAct: Qt.openUrlExternally("https://github.com/ryoku-dev/ryostore-revamped/blob/main/docs/host-a-store.md")
                    Accessible.role: Accessible.Link
                    Accessible.name: I18n.tr("How to host a store")
                }

                Row {
                    id: buttons
                    anchors.right: parent.right
                    spacing: Tokens.s2

                    Btn {
                        text: I18n.tr("CANCEL")
                        armed: !dialog.adding
                        motionEnabled: !dialog.reducedMotion
                        onAct: dialog.close()
                    }
                    Btn {
                        id: continueBtn
                        objectName: dialog.step === "warn" ? "ryostore-add-source-continue" : "ryostore-add-source-submit"
                        primary: true
                        armed: !dialog.adding
                        motionEnabled: !dialog.reducedMotion
                        text: dialog.step === "warn" ? I18n.tr("I UNDERSTAND") : (dialog.adding ? I18n.tr("CHECKING") : I18n.tr("ADD SOURCE"))
                        onAct: dialog.step === "warn" ? dialog.proceed() : dialog.submit()
                    }
                }
            }
        }
    }
}
