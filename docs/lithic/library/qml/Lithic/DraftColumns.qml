import QtQuick

Item {
    id: root

    property Component form: null
    property Component aside: null
    readonly property bool wide: width >= LithicTokens.size.draftWideAt
        && aside !== null
    readonly property real asideWidth: wide
        ? Math.max(LithicTokens.size.draftAsideMin,
                   Math.min(LithicTokens.size.draftAsideMax,
                            width * LithicTokens.size.draftAsideFraction))
        : Math.min(width, LithicTokens.size.draftAsideBelow)
    readonly property real formWidth: wide
        ? Math.min(LithicTokens.size.draftFormMax,
                   width - asideWidth - LithicTokens.space.s5)
        : Math.min(width, LithicTokens.size.draftFormMax)

    implicitWidth: LithicTokens.size.draftWideAt
    implicitHeight: wide
        ? Math.max(formLoader.implicitHeight, asideLoader.implicitHeight)
        : formLoader.implicitHeight
          + (asideLoader.active
             ? LithicTokens.space.s5 + asideLoader.implicitHeight
             : 0)

    Loader {
        id: formLoader
        active: root.form !== null
        sourceComponent: root.form
        width: root.formWidth
        anchors.left: parent.left
        anchors.top: parent.top
    }

    Loader {
        id: asideLoader
        active: root.aside !== null
        sourceComponent: root.aside
        width: root.asideWidth
        anchors.left: root.wide ? formLoader.right : parent.left
        anchors.leftMargin: root.wide ? LithicTokens.space.s5 : 0
        anchors.top: root.wide ? parent.top : formLoader.bottom
        anchors.topMargin: root.wide ? 0 : LithicTokens.space.s5
    }
}
