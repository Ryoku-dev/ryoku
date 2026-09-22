import QtQuick
import ".."
import Ryoku.Ui.Singletons

// A slider laid out as a settings row: the title and description sit left, the
// track and its signed value ride right. onChange fires while dragging for a
// live preview, onCommit on release. It carries the same value/min/max/suffix
// surface as RowInput so a control can move between a typed field and a slider
// without the call sites changing.
SettingsRow {
  id: row
  property real value: 0
  property real min: 0
  property real max: 100
  property int decimals: 0
  property string suffix: ""
  property bool enabled: true
  property var onCommit
  property var onChange

  opacity: enabled ? 1.0 : 0.45

  readonly property color _ink: colors ? colors.surfaceText : "#e0e2e8"
  readonly property color _accent: colors ? colors.primary : "#e2342a"
  readonly property color _line: colors ? colors.outline : Qt.rgba(1, 1, 1, 0.22)

  function _fmt(v) {
    var s = row.decimals === 0 ? Math.round(v).toString() : v.toFixed(row.decimals)
    return s + (row.suffix ? " " + row.suffix : "")
  }

  Row {
    spacing: 10 * Config.uiScale

    Item {
      id: track
      width: 132 * Config.uiScale
      height: 14 * Config.uiScale
      anchors.verticalCenter: parent.verticalCenter

      readonly property real _range: Math.max(0.0001, row.max - row.min)
      readonly property real _ratio: Math.max(0, Math.min(1, (row.value - row.min) / _range))
      readonly property real _fillW: Math.round(width * _ratio)

      Rectangle {
        anchors.fill: parent
        radius: Style.radiusSmall
        color: Qt.rgba(row._ink.r, row._ink.g, row._ink.b, 0.10)
        border.width: 1
        border.color: row._line
      }
      Rectangle {
        anchors.left: parent.left; anchors.top: parent.top; anchors.bottom: parent.bottom
        width: track._fillW
        radius: Style.radiusSmall
        color: row._accent
        visible: track._ratio > 0
      }
      Rectangle {
        width: 3
        height: parent.height + 6
        anchors.verticalCenter: parent.verticalCenter
        x: Math.max(0, Math.min(track.width - width, track._fillW - width / 2))
        radius: 1
        color: row._ink
      }

      MouseArea {
        anchors.fill: parent
        anchors.margins: -6
        cursorShape: row.enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
        enabled: row.enabled
        preventStealing: true
        function _set(mx, commit) {
          var c = Math.max(0, Math.min(track.width, mx))
          var r = track.width > 0 ? c / track.width : 0
          var v = row.min + r * (row.max - row.min)
          if (row.decimals === 0) v = Math.round(v)
          else v = Math.round(v * Math.pow(10, row.decimals)) / Math.pow(10, row.decimals)
          if (v !== row.value || commit) {
            row.value = v
            if (row.onChange) row.onChange(v)
            if (commit && row.onCommit) row.onCommit(v)
          }
        }
        onPressed: function(ev) { _set(ev.x + 6, false) }
        onPositionChanged: function(ev) { if (pressed) _set(ev.x + 6, false) }
        onReleased: function(ev) { _set(ev.x + 6, true) }
        onWheel: function(ev) {
          var step = (ev.angleDelta.y > 0 ? 1 : -1) * Math.max(row.decimals === 0 ? 1 : Math.pow(10, -row.decimals), (row.max - row.min) / 50)
          var v = Math.max(row.min, Math.min(row.max, row.value + step))
          if (row.decimals === 0) v = Math.round(v)
          else v = Math.round(v * Math.pow(10, row.decimals)) / Math.pow(10, row.decimals)
          if (v !== row.value) { row.value = v; if (row.onChange) row.onChange(v); if (row.onCommit) row.onCommit(v) }
        }
      }
    }

    Text {
      anchors.verticalCenter: parent.verticalCenter
      width: 52 * Config.uiScale
      horizontalAlignment: Text.AlignRight
      text: row._fmt(row.value)
      font.family: Style.fontFamilyCode; font.pixelSize: 11 * Config.uiScale; font.weight: Font.Bold
      color: row._ink
    }
  }
}
