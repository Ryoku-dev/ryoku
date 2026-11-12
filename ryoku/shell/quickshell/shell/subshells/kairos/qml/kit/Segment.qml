import QtQuick

Rectangle {
    id: segment

    property string position: "only"
    readonly property bool topOuter: segment.position === "only" || segment.position === "first"
    readonly property bool bottomOuter: segment.position === "only" || segment.position === "last"

    width: parent ? parent.width : 0
    topLeftRadius: segment.topOuter ? Md.rXl : Md.rXs
    topRightRadius: segment.topOuter ? Md.rXl : Md.rXs
    bottomLeftRadius: segment.bottomOuter ? Md.rXl : Md.rXs
    bottomRightRadius: segment.bottomOuter ? Md.rXl : Md.rXs
    color: Md.container
}
