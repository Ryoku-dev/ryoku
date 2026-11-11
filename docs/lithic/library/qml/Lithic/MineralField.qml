import QtQuick

Item {
    id: root

    property string tone: "idle"
    property real inset: 0
    property color fieldColor: Lithic.mineralField(tone)
    default property alias content: contentItem.data
    readonly property alias bodyItem: contentItem
    property bool _animateTone: false

    readonly property real contentImplicitWidth: {
        var maximum = 0;
        for (var i = 0; i < contentItem.children.length; ++i)
            maximum = Math.max(maximum, contentItem.children[i].implicitWidth || 0);
        return maximum;
    }
    readonly property real contentImplicitHeight: {
        var maximum = 0;
        for (var i = 0; i < contentItem.children.length; ++i)
            maximum = Math.max(maximum, contentItem.children[i].implicitHeight || 0);
        return maximum;
    }

    implicitWidth: contentImplicitWidth + inset * 2
    implicitHeight: contentImplicitHeight + inset * 2

    function _snapField() {
        _animateTone = false;
        fieldColor = Lithic.mineralField(tone);
    }

    onToneChanged: {
        _animateTone = true;
        fieldColor = Lithic.mineralField(tone);
    }

    Component.onCompleted: _snapField()

    Connections {
        target: Lithic
        function onStone2Changed() { root._snapField(); }
        function onStone3Changed() { root._snapField(); }
        function onJadeChanged() { root._snapField(); }
        function onLapisChanged() { root._snapField(); }
        function onAmberChanged() { root._snapField(); }
        function onJasperChanged() { root._snapField(); }
    }

    ChiselColorBehavior on fieldColor {
        enabled: root._animateTone
        duration: LithicTokens.motion.field
    }

    CutRect {
        anchors.fill: parent
        fill: root.fieldColor
    }

    Item {
        id: contentItem
        anchors.fill: parent
        anchors.margins: root.inset
    }
}
