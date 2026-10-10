pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// Looking Glass: the passthrough lane. A machine here owns a real GPU, bound to
// the guest and shown only through the Looking Glass client -- no window, no
// SPICE, no headless. The host has to be capable first, so the page gates on
// Lg.ready and, when it isn't, states the blocker plainly and offers nothing to
// launch. When it is, the yard of passthrough machines fills the left, a create
// lane and a standing guest-setup checklist the right. Paper and ink; a running
// machine earns the one state colour, exactly as the quickemu yard does.
Item {
    id: pass

    property bool active: false

    // the create lane, folded until asked for.
    property bool newOpen: false
    property string selectedName: ""
    readonly property var selectedMachine: {
        for (var i = 0; i < Lg.machines.length; i++)
            if (Lg.machines[i].name === selectedName) return Lg.machines[i];
        return null;
    }
    property string vmName: ""
    property string isoPath: ""
    property string os: "windows"
    property int diskGb: 64
    property int ramGb: 16
    readonly property bool canCreate: pass.vmName.trim().length > 0 && pass.isoPath.trim().length > 0 && !Lg.busy

    function resetForm() { pass.vmName = ""; pass.isoPath = ""; pass.os = "windows"; pass.diskGb = 64; pass.ramGb = 16; }

    function running(it) { return it && it.state === "running"; }
    function stateWord(s) {
        return ({ running: I18n.tr("RUN"), paused: I18n.tr("HOLD"), shutoff: I18n.tr("OFF"), absent: I18n.tr("ABSENT") })[s] || String(s || "").toUpperCase();
    }
    function specLine(it) {
        var g = Math.round((it.ramMb || 0) / 1024);
        return (it.vcpus || 0) + "c · " + g + "G · " + (it.diskGb || 0) + "G " + I18n.tr("disk");
    }
    Connections {
        target: Lg
        function onMachinesChanged() {
            if (!pass.selectedMachine && Lg.machines.length > 0)
                pass.selectedName = Lg.machines[0].name;
        }
    }

    // ---- head --------------------------------------------------------------
    PageHead {
        id: header
        anchors { top: parent.top; left: parent.left; right: parent.right }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.topMargin: Tokens.s5
        eyebrow: I18n.tr("PASSTHROUGH")
        title: I18n.tr("Looking Glass")
        blurb: I18n.tr("Machines that own a real GPU, bound to the guest and shown through the Looking Glass client. A world apart from the quickemu yard.")
    }

    // ---- toolbar (only when the host can bind a dGPU) ----------------------
    Item {
        id: toolbar
        anchors { top: header.bottom; left: parent.left; right: parent.right }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.topMargin: Tokens.s3
        height: 40
        visible: Lg.ready

        Row {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: String(Lg.machines.length).padStart(2, "0") + I18n.tr(" MACHINES")
                color: Tokens.inkMuted
                font.family: Tokens.mono; font.pixelSize: 10; font.letterSpacing: 1.2
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: pass.newOpen ? I18n.tr("CLOSE") : I18n.tr("NEW")
                primary: !pass.newOpen
                onAct: pass.newOpen = !pass.newOpen
            }
        }
    }

    // ---- not ready: a single plate that names the blocker ------------------
    Item {
        anchors { top: header.bottom; left: parent.left; right: parent.right; bottom: faultStrip.top }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s4; anchors.bottomMargin: Tokens.s5
        visible: !Lg.ready

        Rectangle {
            anchors.centerIn: parent
            width: Math.min(parent.width, 600)
            height: blockCol.implicitHeight + 2 * Tokens.s6
            color: "transparent"
            radius: Tokens.radius
            border.width: Tokens.border
            border.color: Tokens.line
            antialiasing: false
            Ticks { color: Tokens.line }

            Column {
                id: blockCol
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                anchors.margins: Tokens.s6
                spacing: Tokens.s3

                Mark { anchors.horizontalCenter: parent.horizontalCenter; size: 72 }
                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: I18n.tr("Direct-GPU virtual machines")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: 22
                }
                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.WordWrap
                    text: I18n.tr("Give one VM control of a discrete GPU and view it through Looking Glass.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: 12
                }
                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.WordWrap
                    maximumLineCount: 2
                    elide: Text.ElideRight
                    text: Lg.blocker.length > 0 && Lg.blocker.length < 160
                        ? Lg.blocker
                        : I18n.tr("This host is not ready for GPU passthrough yet. Open Graphics & Power for the readiness checks.")
                    color: Tokens.ink
                    font.family: Tokens.mono
                    font.pixelSize: 10
                }
                Btn {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: I18n.tr("SET UP PASSTHROUGH")
                    primary: true
                    onAct: Quickshell.execDetached(["ryoku-shell", "hub", "open", "gpu"])
                }
            }
        }
    }

    // ---- ready: the yard, left; create + checklist, right ------------------
    Item {
        id: main
        anchors { top: toolbar.bottom; left: parent.left; right: parent.right; bottom: faultStrip.top }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s4; anchors.bottomMargin: Tokens.s5
        visible: Lg.ready

        readonly property real gCol: (width - (Spans.cols - 1) * Tokens.s2) / Spans.cols
        readonly property real leftW: 6 * gCol + 5 * Tokens.s2
        readonly property int seamW: Tokens.s5

        // -- left: the passthrough machines --
        Item {
            id: leftCol
            anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
            width: main.leftW

            Flickable {
                anchors.fill: parent
                contentHeight: list.implicitHeight + Tokens.s3
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }

                Column {
                    id: list
                    width: parent.width
                    spacing: Tokens.s2

                    Repeater {
                        model: Lg.machines
                        Rectangle {
                            id: mCard
                            required property var modelData
                            readonly property bool isRun: pass.running(modelData)
                            width: list.width
                            height: 104
                            radius: Tokens.radius
                            color: pass.selectedName === mCard.modelData.name ? Tokens.tint10 : (mCard.isRun ? Tokens.tint5 : "transparent")
                            border.width: Tokens.border
                            border.color: pass.selectedName === mCard.modelData.name ? Tokens.ink : (mCard.isRun ? Tokens.lineStrong : Tokens.line)
                            antialiasing: false
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }

                            // head: mark, name, state flap.
                            Item {
                                id: cHead
                                anchors { top: parent.top; left: parent.left; right: parent.right }
                                anchors.margins: Tokens.s3
                                height: 40

                                OsIcon {
                                    id: cMark
                                    anchors.left: parent.left
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 30; height: 30; size: 30
                                    slug: mCard.modelData.os || ""
                                    label: mCard.modelData.name || mCard.modelData.os || ""
                                }
                                Column {
                                    anchors.left: cMark.right
                                    anchors.leftMargin: Tokens.s3
                                    anchors.right: cFlap.left
                                    anchors.rightMargin: Tokens.s2
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: 3
                                    Text {
                                        width: parent.width
                                        elide: Text.ElideRight
                                        text: mCard.modelData.name || ""
                                        color: mCard.isRun ? Tokens.ink : Tokens.inkDim
                                        font.family: Tokens.ui; font.pixelSize: 14
                                        font.weight: mCard.isRun ? Font.DemiBold : Font.Medium
                                    }
                                    Text {
                                        width: parent.width
                                        elide: Text.ElideRight
                                        text: pass.specLine(mCard.modelData)
                                        color: Tokens.inkFaint
                                        font.family: Tokens.mono; font.pixelSize: 11
                                    }
                                }
                                FlapWord {
                                    id: cFlap
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: pass.stateWord(mCard.modelData.state)
                                    pad: 3
                                    cellW: 13; cellH: 20; fontPx: 11
                                    ink: mCard.isRun ? Tokens.ink : Tokens.inkDim
                                }
                            }

                            MetricSparkline {
                                anchors.right: parent.right
                                anchors.rightMargin: Tokens.s3
                                anchors.bottom: parent.bottom
                                anchors.bottomMargin: 38
                                width: 82
                                height: 20
                                values: Lg.series(mCard.modelData.name, "cpu")
                                fixedMax: 100
                            }

                            TapHandler { onTapped: pass.selectedName = mCard.modelData.name }

                            // foot: the actions. Launch is always the primary
                            // verb (it re-opens the viewer on a running box too);
                            // Stop only shows while running.
                            Row {
                                anchors { bottom: parent.bottom; left: parent.left }
                                anchors.margins: Tokens.s3
                                spacing: Tokens.s2
                                Btn {
                                    primary: true
                                    text: I18n.tr("LAUNCH (LOOKING GLASS)")
                                    armed: !Lg.busy
                                    onAct: Lg.launch(mCard.modelData.name)
                                }
                                Btn {
                                    visible: mCard.isRun
                                    text: I18n.tr("STOP")
                                    armed: !Lg.busy
                                    onAct: Lg.stop(mCard.modelData.name)
                                }
                            }
                            Btn {
                                anchors { bottom: parent.bottom; right: parent.right }
                                anchors.margins: Tokens.s3
                                text: I18n.tr("REMOVE")
                                armed: !Lg.busy && !mCard.isRun
                                onAct: Lg.remove(mCard.modelData.name, false)
                            }
                        }
                    }
                }
            }

            Empty {
                anchors.centerIn: parent
                width: parent.width
                visible: Lg.machines.length === 0 && !Lg.loading
                caption: I18n.tr("No passthrough machines yet. Build one with NEW: point it at an install ISO and pick the guest OS.")
            }
        }

        Rectangle {
            anchors.left: leftCol.right
            anchors.leftMargin: main.seamW / 2
            anchors { top: parent.top; bottom: parent.bottom }
            anchors.topMargin: Tokens.s2; anchors.bottomMargin: Tokens.s2
            width: 1
            color: Tokens.line
        }

        // -- right: create lane (folded) over the standing checklist --
        Item {
            id: rightCol
            anchors.left: leftCol.right
            anchors.leftMargin: main.seamW
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.bottom: parent.bottom

            Flickable {
                anchors.fill: parent
                contentHeight: rcol.implicitHeight + Tokens.s3
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }

                Column {
                    id: rcol
                    width: parent.width
                    spacing: Tokens.s4

                    // the create lane.
                    Rectangle {
                        width: parent.width
                        height: formCol.implicitHeight + 2 * Tokens.s4
                        visible: pass.newOpen
                        color: "transparent"
                        radius: Tokens.radius
                        border.width: Tokens.border
                        border.color: Tokens.line
                        antialiasing: false

                        Column {
                            id: formCol
                            anchors { left: parent.left; right: parent.right; top: parent.top }
                            anchors.margins: Tokens.s4
                            spacing: Tokens.s4

                            Text {
                                text: I18n.tr("NEW PASSTHROUGH MACHINE")
                                color: Tokens.ink
                                font.family: Tokens.ui; font.pixelSize: 10; font.weight: Font.Medium
                                font.letterSpacing: Tokens.trackLabel; font.capitalization: Font.AllUppercase
                            }

                            Column {
                                width: parent.width
                                spacing: Tokens.s2
                                FieldLabel { text: I18n.tr("Name") }
                                Field {
                                    width: parent.width
                                    text: pass.vmName
                                    placeholder: "win11"
                                    onEdited: (v) => pass.vmName = v.replace(/[\/\s]+/g, "-")
                                }
                            }

                            Column {
                                width: parent.width
                                spacing: Tokens.s2
                                FieldLabel { text: I18n.tr("Guest OS") }
                                Seg {
                                    options: ["WINDOWS", "LINUX"]
                                    current: pass.os.toUpperCase()
                                    onChose: (k) => pass.os = k.toLowerCase()
                                }
                            }

                            Column {
                                width: parent.width
                                spacing: Tokens.s2
                                FieldLabel { text: I18n.tr("Install ISO") }
                                Row {
                                    width: parent.width
                                    spacing: Tokens.s3
                                    Field {
                                        width: parent.width - browse.width - Tokens.s3
                                        tabular: true
                                        text: pass.isoPath
                                        placeholder: "/path/to/os.iso"
                                        onEdited: (v) => pass.isoPath = v
                                    }
                                    Btn {
                                        id: browse
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("BROWSE")
                                        onAct: pickProc.running = true
                                    }
                                }
                            }

                            Row {
                                width: parent.width
                                spacing: Tokens.s5
                                NumRow {
                                    label: I18n.tr("Disk")
                                    unit: "GB"
                                    value: pass.diskGb
                                    from: 16; to: 1024
                                    onModified: (v) => pass.diskGb = v
                                }
                                NumRow {
                                    label: I18n.tr("Memory")
                                    unit: "GB"
                                    value: pass.ramGb
                                    from: 2; to: 128
                                    onModified: (v) => pass.ramGb = v
                                }
                            }

                            Text {
                                width: parent.width
                                wrapMode: Text.WordWrap
                                visible: pass.os === "windows"
                                text: I18n.tr("Windows gets a VirtIO driver CD attached so setup sees the disk and the network. Fetch the install ISO from microsoft.com/software-download.")
                                color: Tokens.inkMuted
                                font.family: Tokens.ui; font.pixelSize: 11
                            }

                            Btn {
                                primary: true
                                text: I18n.tr("CREATE")
                                armed: pass.canCreate
                                onAct: {
                                    Lg.create(pass.vmName.trim(), pass.isoPath.trim(), pass.os, pass.diskGb, pass.ramGb * 1024);
                                    pass.resetForm();
                                    pass.newOpen = false;
                                }
                            }
                        }
                    }

                    Column {
                        width: parent.width
                        spacing: Tokens.s3
                        visible: !pass.newOpen && pass.selectedMachine !== null

                        Row {
                            width: parent.width
                            Text {
                                width: parent.width - stateText.width - Tokens.s3
                                text: pass.selectedMachine ? pass.selectedMachine.name : ""
                                elide: Text.ElideRight
                                color: Tokens.ink
                                font.family: Tokens.display
                                font.pixelSize: 26
                            }
                            Text {
                                id: stateText
                                anchors.verticalCenter: parent.verticalCenter
                                text: pass.selectedMachine ? pass.stateWord(pass.selectedMachine.state) : ""
                                color: Tokens.inkMuted
                                font.family: Tokens.mono
                                font.pixelSize: 10
                            }
                        }
                        MetricsPanel {
                            width: parent.width
                            height: 286
                            cpuValues: Lg.series(pass.selectedName, "cpu")
                            ramValues: Lg.series(pass.selectedName, "ram")
                            diskValues: Lg.series(pass.selectedName, "disk")
                            netValues: Lg.series(pass.selectedName, "net")
                        }
                        Row {
                            width: parent.width
                            spacing: Tokens.s4
                            Repeater {
                                model: pass.selectedMachine ? [
                                    { label: I18n.tr("OS"), value: pass.selectedMachine.os || "-" },
                                    { label: I18n.tr("CPU"), value: pass.selectedMachine.vcpus + "c" },
                                    { label: I18n.tr("RAM"), value: Math.round(pass.selectedMachine.ramMb / 1024) + " GB" },
                                    { label: I18n.tr("DISK"), value: pass.selectedMachine.diskGb + " GB" }
                                ] : []
                                Column {
                                    required property var modelData
                                    width: (parent.width - Tokens.s4 * 3) / 4
                                    spacing: 2
                                    Text { text: modelData.label; color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: 9 }
                                    Text { text: modelData.value; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 10 }
                                }
                            }
                        }
                    }

                    // the standing guest-setup checklist: what has to happen
                    // inside the guest before the viewer shows a picture. Quiet,
                    // persistent, no colour.
                    Rectangle {
                        width: parent.width
                        height: checkCol.implicitHeight + 2 * Tokens.s4
                        color: "transparent"
                        radius: Tokens.radius
                        border.width: Tokens.border
                        border.color: Tokens.line
                        antialiasing: false
                        Ticks { color: Tokens.line }

                        Column {
                            id: checkCol
                            anchors { left: parent.left; right: parent.right; top: parent.top }
                            anchors.margins: Tokens.s4
                            spacing: Tokens.s3

                            Row {
                                spacing: Tokens.s2
                                Text { text: "//"; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro }
                                Text {
                                    text: I18n.tr("INSIDE_THE_GUEST"); color: Tokens.ink
                                    font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                                    font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                                }
                                Text {
                                    text: "客"; color: Tokens.inkFaint; font.family: Tokens.jp; font.pixelSize: 12
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }

                            Repeater {
                                model: [
                                    { n: "01", t: I18n.tr("Install the OS from the ISO the usual way.") },
                                    { n: "02", t: I18n.tr("Install the VirtIO drivers from the attached CD so disk and network work.") },
                                    { n: "03", t: I18n.tr("Install the Looking Glass HOST app inside the guest, the viewer stays black until it runs.") }
                                ]
                                Row {
                                    id: chkRow
                                    required property var modelData
                                    width: checkCol.width
                                    spacing: Tokens.s3
                                    Text {
                                        text: chkRow.modelData.n
                                        color: Tokens.inkFaint
                                        font.family: Tokens.mono; font.pixelSize: 11
                                    }
                                    Text {
                                        width: checkCol.width - 30
                                        wrapMode: Text.WordWrap
                                        text: chkRow.modelData.t
                                        color: Tokens.inkMuted
                                        font.family: Tokens.ui; font.pixelSize: 12
                                    }
                                }
                            }

                            Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }

                            Text {
                                width: parent.width
                                wrapMode: Text.WordWrap
                                text: I18n.tr("A laptop or render-only dGPU has no display outputs, so the guest also needs a virtual-display driver for Looking Glass to have a frame to capture.")
                                color: Tokens.inkFaint
                                font.family: Tokens.ui; font.pixelSize: 11
                            }
                        }
                    }
                }
            }
        }
    }

    // ---- fault strip: sticky until dismissed or superseded -----------------
    Rectangle {
        id: faultStrip
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        anchors.bottomMargin: Lg.fault.length > 0 ? Tokens.s5 : 0
        height: Lg.fault.length > 0 ? faultCol.implicitHeight + 2 * Tokens.s2 : 0
        visible: Lg.fault.length > 0
        color: "transparent"
        radius: Tokens.radius
        border.width: Tokens.border
        border.color: Tokens.line
        antialiasing: false
        property bool expanded: false

        Column {
            id: faultCol
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.s2
            spacing: Tokens.s2

            Item {
                width: parent.width
                height: 22
                Row {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3
                    Annunciator { anchors.verticalCenter: parent.verticalCenter; label: I18n.tr("FAULT"); lit: true; warn: true; tileW: 52 }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        width: faultStrip.width - 240
                        elide: Text.ElideRight
                        text: Lg.fault
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: 12
                    }
                }
                Row {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3
                    Text {
                        visible: Lg.faultDetail.indexOf("\n") >= 0
                        anchors.verticalCenter: parent.verticalCenter
                        text: faultStrip.expanded ? I18n.tr("LESS") : I18n.tr("DETAIL")
                        color: fdh.hovered ? Tokens.ink : Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: 9
                        font.letterSpacing: 1.5
                        HoverHandler { id: fdh; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: faultStrip.expanded = !faultStrip.expanded }
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("DISMISS")
                        color: fxh.hovered ? Tokens.ink : Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: 9
                        font.letterSpacing: 1.5
                        HoverHandler { id: fxh; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: { faultStrip.expanded = false; Lg.clearFault(); } }
                    }
                }
            }

            Flickable {
                visible: faultStrip.expanded
                width: parent.width
                height: Math.min(faultDetailText.implicitHeight, 140)
                contentHeight: faultDetailText.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                Text {
                    id: faultDetailText
                    width: parent.width
                    wrapMode: Text.WrapAnywhere
                    text: Lg.faultDetail
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: 11
                }
            }
        }
    }

    // a bounded-integer row: the label, the live numeral, and the +/- pair.
    component NumRow: Row {
        id: nr
        property string label: ""
        property string unit: ""
        property int value: 0
        property int from: 0
        property int to: 100
        signal modified(int v)
        spacing: Tokens.s3
        Column {
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s1
            FieldLabel { text: I18n.tr(nr.label) }
            Text {
                text: nr.value + (nr.unit.length > 0 ? " " + nr.unit : "")
                color: Tokens.ink
                font.family: Tokens.mono; font.pixelSize: 13
            }
        }
        Step {
            anchors.verticalCenter: parent.verticalCenter
            value: nr.value
            from: nr.from
            to: nr.to
            onModified: (v) => nr.modified(v)
        }
    }

    component FieldLabel: Text {
        color: Tokens.inkMuted
        font.family: Tokens.ui
        font.pixelSize: 10
        font.weight: Font.Medium
        font.letterSpacing: Tokens.trackLabel
        font.capitalization: Font.AllUppercase
    }

    // the same zenity/kdialog picker the ISO lane uses, verbatim.
    Process {
        id: pickProc
        command: ["sh", "-c", "zenity --file-selection --title='Select an ISO' --file-filter='ISO images | *.iso *.ISO *.img' --file-filter='All files | *' 2>/dev/null || kdialog --getopenfilename \"$HOME\" '*.iso *.ISO *.img|ISO images' 2>/dev/null"]
        stdout: StdioCollector {
            onStreamFinished: {
                var p = this.text.trim();
                if (p.length > 0) {
                    pass.isoPath = p;
                    if (pass.vmName.length === 0) {
                        var base = p.split("/").pop().replace(/\.(iso|img|ISO|IMG)$/, "");
                        pass.vmName = base.replace(/[\/\s]+/g, "-");
                    }
                }
            }
        }
    }
}
