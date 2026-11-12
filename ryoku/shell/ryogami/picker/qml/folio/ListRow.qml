import QtQuick
import Ryoku.Ui.Singletons
import "FolioActions.js" as Actions

Item {
    id: row

    // { base, itemLabel, listControl, fields[], itemActions[], addControl, daemon } from FolioLayout.buildRows.
    property var group: ({})
    property var state
    property var options
    property var host
    property real reveal: 1

    readonly property string base: row.group.base ? row.group.base : ""
    readonly property bool _resolution: row.base === "filterBar.resolutionPresets"
    readonly property var _items: Actions.asList(arrsv.value)

    implicitHeight: col.implicitHeight

    SettingValue { id: arrsv; key: row.base }

    function _itemTitle(i) {
        var lbl = row.group.itemLabel;
        if (lbl && lbl.indexOf("%1") >= 0)
            return lbl.replace("%1", String(i + 1));
        var n = i + 1;
        return (n < 10 ? "0" + n : String(n)) + (lbl ? "  " + lbl : "");
    }
    function _leaf(control) {
        var m = /\.(?:N|\{idx\})\.(.+)$/.exec(control.key || control.id || "");
        return m ? m[1] : "";
    }
    function _setField(i, field, value) { Actions.setField(Settings, row.base, i, field, value); }

    Column {
        id: col
        width: row.width
        spacing: 10 * Theme.scale

        Column {
            width: parent.width
            spacing: 3 * Theme.scale
            visible: !!row.group.listControl
            Text {
                width: parent.width
                text: row.group.listControl && row.group.listControl.help ? row.group.listControl.help : ""
                visible: text.length > 0
                font.family: Theme.sans
                font.weight: Font.Normal
                font.pixelSize: Theme.fontBase
                color: Theme.withAlpha(Theme.surfaceText, 0.56 * row.reveal)
                lineHeight: 1.38
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }
        }

        Text {
            width: parent.width
            visible: row._items.length === 0
            text: I18n.tr("Nothing here yet.")
            font.family: Theme.sans
            font.weight: Font.Normal
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.surfaceText, 0.5 * row.reveal)
            renderType: Text.NativeRendering
        }

        Repeater {
            model: row._items.length

            delegate: StackBar {
                id: item
                required property int index
                width: col.width
                expanded: row._items.length <= 3
                title: row._itemTitle(item.index)

                readonly property var _el: row._items[item.index] ? row._items[item.index] : ({})

                Column {
                    parent: item.body
                    width: item.body.width
                    spacing: 12 * Theme.scale
                    topPadding: 6 * Theme.scale
                    bottomPadding: 8 * Theme.scale

                    Repeater {
                        model: row.group.fields

                        delegate: FolioControl {
                            required property var modelData
                            width: parent.width
                            state: row.state
                            options: row.options
                            host: row.host
                            reveal: row.reveal
                            control: modelData
                            bound: false
                            visible: !(row._resolution && ["from", "to", "custom"].indexOf(row._leaf(modelData)) >= 0)
                            boundValue: item._el[row._leaf(modelData)]
                            onEdited: (v) => row._setField(item.index, row._leaf(modelData), v)
                        }
                    }

                    Details {
                        id: customDetails
                        width: parent.width
                        visible: row._resolution
                        title: I18n.tr("Custom range")

                        Column {
                            parent: customDetails.body
                            width: customDetails.body.width
                            spacing: 10 * Theme.scale
                            topPadding: 4 * Theme.scale

                            ResolutionRow {
                                width: parent.width
                                state: row.state
                                bound: false
                                control: ({ label: I18n.tr("Minimum size"),
                                            help: I18n.tr("Smallest width and height included. 0 leaves it open.") })
                                boundValue: ({ w: item._el.minWidth || 0, h: item._el.minHeight || 0 })
                                onEdited: (v) => { row._setField(item.index, "minWidth", v.w); row._setField(item.index, "minHeight", v.h); }
                            }
                            ResolutionRow {
                                width: parent.width
                                state: row.state
                                bound: false
                                control: ({ label: I18n.tr("Maximum size"),
                                            help: I18n.tr("Largest width and height included. 0 leaves it open.") })
                                boundValue: ({ w: item._el.maxWidth || 0, h: item._el.maxHeight || 0 })
                                onEdited: (v) => { row._setField(item.index, "maxWidth", v.w); row._setField(item.index, "maxHeight", v.h); }
                            }
                        }
                    }

                    Flow {
                        width: parent.width
                        spacing: 8 * Theme.scale
                        Repeater {
                            model: row.group.itemActions
                            delegate: ActionRow {
                                required property var modelData
                                width: parent.width
                                state: row.state
                                host: row.host
                                reveal: row.reveal
                                control: modelData
                                actionArgs: ({ index: item.index })
                            }
                        }
                    }
                }
            }
        }

        ActionRow {
            width: parent.width
            visible: !!row.group.addControl
            state: row.state
            options: row.options
            host: row.host
            reveal: row.reveal
            control: row.group.addControl ? row.group.addControl : ({})
        }
    }
}
