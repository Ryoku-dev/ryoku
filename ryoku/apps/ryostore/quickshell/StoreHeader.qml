import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: header

    property string view: "discover"
    property string categoryID: ""
    property var categories: []
    property string query: ""
    property int libraryCount: 0
    property int updateCount: 0
    property bool offline: false
    property bool refreshing: false
    property bool searchActive: false
    property int resultCount: 0
    property bool updateAvailable: false
    property bool reducedMotion: false

    signal routeRequested(string view, string categoryID)
    signal refreshRequested()
    signal queryEdited(string value)
    signal searchActivated()
    signal searchEscaped()

    implicitWidth: Tokens.railW

    function activateDiscover() { routeRequested("discover", ""); }
    function activateCategory(id) { routeRequested("discover", id); }
    function activateLibrary() { routeRequested("library", ""); }
    function focusSearch() { searchField.forceActiveFocus(); }
    function sealFor(id) {
        const seals = {
            "lockscreens": "施錠",
            "rices": "飯",
            "colorschemes": "色",
            "themes": "色",
            "barstyles": "棒",
            "fastfetch": "速",
            "plugins": "部品",
            "omarchy-plugins": "部品",
            "bundles": "束",
            "decors": "飾",
            "launcher-images": "起動",
            "fastfetch-emblems": "徽"
        };
        return seals[String(id)] || "品";
    }

    component RailItem: Rectangle {
        id: plate
        property string label: ""
        property string note: ""
        property string seal: ""
        property bool current: false
        property bool flagged: false
        signal chose()

        width: navColumn.width
        height: Tokens.rowH
        radius: Tokens.radius
        color: plate.current ? "transparent"
              : (plateTap.pressed ? Tokens.tint16 : (pointer.hovered ? Tokens.tint5 : "transparent"))
        activeFocusOnTab: true
        Accessible.role: Accessible.Button
        Accessible.name: note === "" ? I18n.tr(label) : I18n.tr(label) + ", " + note
        Accessible.onPressAction: chose()
        onActiveFocusChanged: if (activeFocus) navScroll.reveal(plate)
        onCurrentChanged: if (current) Qt.callLater(function() { navScroll.reveal(plate); })

        Behavior on color {
            enabled: !header.reducedMotion
            ColorAnimation { duration: Tokens.snap }
        }

        Keys.onPressed: event => {
            if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
                chose();
                event.accepted = true;
            }
        }

        Row {
            anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter }
            anchors.leftMargin: Tokens.s3
            anchors.rightMargin: Tokens.s3
            spacing: Tokens.s2

            Text {
                id: lead
                text: "//"
                visible: plate.current
                color: Tokens.inkOnBoneDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                anchors.verticalCenter: parent.verticalCenter
            }

            Column {
                width: parent.width - sealText.implicitWidth - parent.spacing
                        - (lead.visible ? lead.implicitWidth + parent.spacing : 0)
                anchors.verticalCenter: parent.verticalCenter
                spacing: 0

                Text {
                    width: parent.width
                    text: I18n.tr(plate.label)
                    color: plate.current ? Tokens.inkOnBone : (plate.activeFocus ? Tokens.ink : Tokens.inkDim)
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                    Behavior on color {
                        enabled: !header.reducedMotion
                        ColorAnimation { duration: Tokens.snap }
                    }
                }

                Text {
                    width: parent.width
                    visible: plate.note !== ""
                    text: I18n.tr(plate.note)
                    color: plate.current ? Tokens.inkOnBoneDim : Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                    elide: Text.ElideRight
                    Behavior on color {
                        enabled: !header.reducedMotion
                        ColorAnimation { duration: Tokens.snap }
                    }
                }
            }

            Text {
                id: sealText
                text: plate.seal
                color: plate.current ? Tokens.inkOnBoneDim : Tokens.inkFaint
                font.family: Tokens.jp
                font.pixelSize: Tokens.fBody
                anchors.verticalCenter: parent.verticalCenter
                Behavior on color {
                    enabled: !header.reducedMotion
                    ColorAnimation { duration: Tokens.snap }
                }
            }
        }

        Rectangle {
            visible: plate.flagged
            width: 6
            height: 6
            radius: 3
            color: Tokens.alert
            anchors { top: parent.top; right: parent.right; margins: Tokens.s2 }
        }

        HoverHandler { id: pointer; cursorShape: Qt.PointingHandCursor }
        TapHandler { id: plateTap; onTapped: plate.chose() }
    }

    Rectangle {
        objectName: "ryostore-rail"
        anchors.fill: parent
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.line
    }

    Column {
        id: masthead
        anchors { left: parent.left; right: parent.right; top: parent.top }
        anchors.margins: Tokens.s5
        spacing: Tokens.s1

        Row {
            spacing: Tokens.s3
            Text {
                text: "力"
                color: Tokens.sun
                font.family: Tokens.jp
                font.pixelSize: 24
            }
            Column {
                anchors.verticalCenter: parent.verticalCenter
                spacing: 0
                Text {
                    text: I18n.tr("RYOKU")
                    color: Tokens.ink
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fBody
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackMark
                }
                Text {
                    text: I18n.tr("STORE")
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackLabel
                }
            }
        }

        Rectangle {
            id: searchBox
            objectName: "ryostore-header-search"
            width: parent.width
            height: 36
            radius: Tokens.radius
            color: searchField.activeFocus ? Tokens.tint5 : Tokens.paper
            border.width: Tokens.border
            border.color: searchField.activeFocus ? Tokens.lineStrong : Tokens.line

            Text {
                id: searchGlyph
                anchors { left: parent.left; leftMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                text: "⌕"
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fRow
            }

            TextInput {
                id: searchField
                objectName: "ryostore-header-search-field"
                anchors {
                    left: searchGlyph.right; leftMargin: Tokens.s2
                    right: parent.right; rightMargin: Tokens.s3
                    verticalCenter: parent.verticalCenter
                }
                Component.onCompleted: text = header.query
                color: Tokens.ink
                selectionColor: Tokens.tint16
                selectedTextColor: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fBody
                clip: true
                activeFocusOnTab: true
                Accessible.role: Accessible.EditableText
                Accessible.name: header.offline ? I18n.tr("Search RyoStore (offline)") : I18n.tr("Search RyoStore")
                onTextEdited: header.queryEdited(text)
                onActiveFocusChanged: if (activeFocus) header.searchActivated()
                Keys.onEscapePressed: event => { header.searchEscaped(); event.accepted = true; }

                Connections {
                    target: header
                    function onQueryChanged() {
                        if (searchField.text !== header.query)
                            searchField.text = header.query;
                    }
                }

                Text {
                    anchors.fill: parent
                    visible: searchField.text === ""
                    text: header.offline ? I18n.tr("Search offline") : I18n.tr("Search the store")
                    color: Tokens.inkMuted
                    font: searchField.font
                    verticalAlignment: Text.AlignVCenter
                    elide: Text.ElideRight
                }
            }

            HoverHandler { cursorShape: Qt.IBeamCursor }
            TapHandler { onTapped: searchField.forceActiveFocus() }
        }

        Text {
            visible: header.searchActive && header.query !== ""
            text: header.resultCount === 1
                    ? I18n.tr("%1 RESULT").arg(header.resultCount)
                    : I18n.tr("%1 RESULTS").arg(header.resultCount)
            color: Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
            font.letterSpacing: Tokens.trackLabel
        }
    }

    Rectangle {
        anchors { left: parent.left; right: parent.right; top: masthead.bottom; topMargin: Tokens.s4 }
        height: Tokens.border
        color: Tokens.line
    }

    Flickable {
        id: navScroll
        objectName: "ryostore-header-categories"
        anchors {
            left: parent.left; right: parent.right
            top: masthead.bottom; topMargin: Tokens.s5
            bottom: footer.top; bottomMargin: Tokens.s4
        }
        anchors.leftMargin: Tokens.s4
        anchors.rightMargin: Tokens.s4
        contentWidth: width
        contentHeight: navColumn.implicitHeight > height
                ? navColumn.implicitHeight + Tokens.rowH + Tokens.s1
                : navColumn.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        function reveal(item) {
            if (navColumn.implicitHeight <= height) {
                contentY = 0;
                return;
            }
            const top = item.y;
            const bottom = top + item.height;
            var target = contentY;
            if (top < contentY)
                target = top;
            else if (bottom > contentY + height)
                target = bottom - height;
            else
                return;

            if (target > contentY) {
                const rows = navColumn.children;
                for (var i = 0; i < rows.length; i++) {
                    const row = rows[i];
                    if (row.visible !== false && row.height > 0 && row.y >= target) {
                        target = row.y;
                        break;
                    }
                }
            }
            contentY = Math.max(0, Math.min(contentHeight - height, target));
        }
        clip: true
        readonly property real selectedY: {
            header.view;
            header.categoryID;
            header.searchActive;
            header.categories;
            const rows = navColumn.children;
            for (let i = 0; i < rows.length; i++) {
                if (rows[i].current === true)
                    return rows[i].y;
            }
            return 0;
        }
        readonly property bool hasSelection: {
            header.view;
            header.categoryID;
            header.searchActive;
            const rows = navColumn.children;
            for (let i = 0; i < rows.length; i++) {
                if (rows[i].current === true)
                    return true;
            }
            return false;
        }

        Rectangle {
            id: selectionPlate
            objectName: "ryostore-category-selection"
            x: 0
            y: navScroll.selectedY
            z: 0
            width: navColumn.width
            height: Tokens.rowH
            radius: Tokens.radius
            color: Tokens.bone
            opacity: navScroll.hasSelection ? 1 : 0

            Behavior on y {
                enabled: !header.reducedMotion
                NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease }
            }
            Behavior on opacity {
                enabled: !header.reducedMotion
                NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
            }
        }


        Column {
            id: navColumn
            width: navScroll.width
            spacing: Tokens.s1
            z: 1

            Text {
                text: I18n.tr("01 BROWSE")
                color: header.view === "discover" ? Tokens.inkDim : Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
                bottomPadding: Tokens.s2
            }

            RailItem {
                objectName: "ryostore-header-discover"
                label: I18n.tr("Discover")
                note: I18n.tr("A changing edit")
                seal: "発見"
                current: header.view === "discover" && header.categoryID === "" && !header.searchActive
                onChose: header.activateDiscover()
            }

            Repeater {
                model: header.categories
                delegate: RailItem {
                    required property var modelData
                    objectName: "ryostore-category-" + String(modelData.id)
                    label: String(modelData.name || modelData.id)
                    note: I18n.tr("%1 pieces").arg(Number(modelData.count || 0))
                    seal: header.sealFor(modelData.id)
                    current: header.view === "discover" && header.categoryID === String(modelData.id) && !header.searchActive
                    onChose: header.activateCategory(String(modelData.id))
                }
            }

            Text {
                text: I18n.tr("02 YOUR STORE")
                color: header.view === "library" ? Tokens.inkDim : Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
                topPadding: Tokens.s4
                bottomPadding: Tokens.s2
            }

            RailItem {
                objectName: "ryostore-header-library"
                label: I18n.tr("Library")
                note: header.updateCount === 1
                        ? I18n.tr("%1 installed, 1 update ready").arg(header.libraryCount)
                        : (header.updateCount > 1
                           ? I18n.tr("%1 installed, %2 updates ready").arg(header.libraryCount).arg(header.updateCount)
                           : I18n.tr("%1 installed").arg(header.libraryCount))
                seal: "蔵"
                current: header.view === "library" && !header.searchActive
                flagged: header.updateCount > 0
                onChose: header.activateLibrary()
            }
        }

        WheelScroll { }
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
    }

    Item {
        id: footer
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: 68

        Rectangle {
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: Tokens.border
            color: Tokens.line
        }

        Btn {
            objectName: "ryostore-header-refresh"
            anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter }
            anchors.leftMargin: Tokens.s4
            anchors.rightMargin: Tokens.s4
            text: header.refreshing ? I18n.tr("SYNCING")
                    : (header.updateAvailable ? I18n.tr("REFRESH CATALOGUE") : I18n.tr("REFRESH"))
            armed: !header.refreshing
            primary: header.updateAvailable
            onAct: header.refreshRequested()
            Accessible.role: Accessible.Button
            Accessible.name: text
            Accessible.onPressAction: header.refreshRequested()
        }
    }
}
