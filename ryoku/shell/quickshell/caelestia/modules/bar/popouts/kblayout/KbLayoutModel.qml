pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import Caelestia
import Caelestia.Config
import caelestia.services

// TODO: handle this better later

Item {
    id: model

    property alias visibleModel: visibleModel
    property string activeLabel: ""
    property int activeIndex: -1
    property var _xkbMap: ({})
    property bool _notifiedLimit: false

    function start() {
        xkbXmlBase.running = true;
        getKbLayoutOpt.running = true;
    }

    function refresh() {
        _notifiedLimit = false;
        getKbLayoutOpt.running = true;
    }

    function switchTo(idx) {
        switchProc.command = ["qdbus6", "org.kde.keyboard", "/Layouts", "org.kde.KeyboardLayouts.setLayout", String(idx)];
        switchProc.running = true;
    }

    function _buildXmlMap(xml) {
        const map = {};

        const re = /<name>\s*([^<]+?)\s*<\/name>[\s\S]*?<description>\s*([^<]+?)\s*<\/description>/g;

        let m;
        while ((m = re.exec(xml)) !== null) {
            const code = (m[1] || "").trim();
            const desc = (m[2] || "").trim();
            if (!code || !desc)
                continue;
            map[code] = _short(desc);
        }

        if (Object.keys(map).length === 0)
            return;

        _xkbMap = map;

        if (layoutsModel.count > 0) {
            const tmp = [];
            for (let i = 0; i < layoutsModel.count; i++) {
                const it = layoutsModel.get(i);
                tmp.push({
                    layoutIndex: it.layoutIndex,
                    token: it.token,
                    label: _pretty(it.token)
                });
            }
            layoutsModel.clear();
            tmp.forEach(t => layoutsModel.append(t));
            fetchActiveLayouts.running = true;
        }
    }

    function _short(desc) {
        const m = desc.match(/^(.*)\((.*)\)$/);
        if (!m)
            return desc;
        const lang = m[1].trim();
        const region = m[2].trim();
        const code = (region.split(/[,\s-]/)[0] || region).slice(0, 2).toUpperCase();
        return `${lang} (${code})`;
    }

    function _setLayouts(rawParts) {
        layoutsModel.clear();
        let idx = 0;
        for (const p of rawParts) {
            layoutsModel.append({
                layoutIndex: idx,
                token: p,
                label: p
            });
            idx++;
        }
    }

    function _rebuildVisible() {
        visibleModel.clear();

        let arr = [];
        for (let i = 0; i < layoutsModel.count; i++)
            arr.push(layoutsModel.get(i));

        arr = arr.filter(i => i.layoutIndex !== activeIndex);
        arr.forEach(i => visibleModel.append(i));

        if (!GlobalConfig.utilities.toasts.kbLimit)
            return;

        if (layoutsModel.count > 4) {
            Toaster.toast(qsTr("Keyboard layout limit"), qsTr("XKB supports only 4 layouts at a time"), "warning");
        }
    }

    function _pretty(token) {
        const code = token.replace(/\(.*\)$/, "").trim();
        if (_xkbMap[code])
            return code.toUpperCase() + " - " + _xkbMap[code];
        return code.toUpperCase() + " - " + code;
    }

    visible: false

    ListModel {
        id: visibleModel
    }

    ListModel {
        id: layoutsModel
    }

    Connections {
        function onActiveIndexChanged() {
            if (!fetchActiveLayouts.running && layoutsModel.count > 0)
                fetchActiveLayouts.running = true;
        }

        target: KbLayout
    }

    Process {
        id: xkbXmlBase

        command: ["xmllint", "--xpath", "//layout/configItem[name and description]", "/usr/share/X11/xkb/rules/base.xml"]
        stdout: StdioCollector {
            onStreamFinished: model._buildXmlMap(text)
        }
        onRunningChanged: if (!running && (typeof xkbXmlBase.exitCode !== "undefined") && xkbXmlBase.exitCode !== 0) // qmllint disable missing-property
            xkbXmlEvdev.running = true
    }

    Process {
        id: xkbXmlEvdev

        command: ["xmllint", "--xpath", "//layout/configItem[name and description]", "/usr/share/X11/xkb/rules/evdev.xml"]
        stdout: StdioCollector {
            onStreamFinished: model._buildXmlMap(text)
        }
    }

    Process {
        id: getKbLayoutOpt

        command: ["qdbus6", "--literal", "org.kde.keyboard", "/Layouts", "org.kde.KeyboardLayouts.getLayoutsList"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const re = /\[Argument: \(sss\) "[^"]*", "[^"]*", "([^"]+)"\]/g;
                    let m;
                    let rawList = [];
                    while ((m = re.exec(text)) !== null) {
                        rawList.push(m[1]);
                    }
                    if (rawList.length) {
                        model._setLayouts(rawList);
                    }
                } catch (e) {}
                fetchActiveLayouts.running = true;
            }
        }
    }

    Process { id: fetchLayoutsFromDevices }

    Process {
        id: fetchActiveLayouts

        command: ["qdbus6", "org.kde.keyboard", "/Layouts", "org.kde.KeyboardLayouts.getLayout"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const idx = parseInt(text.trim());
                    model.activeIndex = idx >= 0 ? idx : -1;
                    model.activeLabel = (idx >= 0 && idx < layoutsModel.count) ? layoutsModel.get(idx).label : "";
                } catch (e) {
                    model.activeIndex = -1;
                    model.activeLabel = "";
                }
                model._rebuildVisible();
            }
        }
    }

    Process {
        id: switchProc

        onRunningChanged: if (!running)
            fetchActiveLayouts.running = true
    }
}
