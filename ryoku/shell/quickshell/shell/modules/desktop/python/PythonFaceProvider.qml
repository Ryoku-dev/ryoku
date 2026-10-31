pragma ComponentBehavior: Bound
import QtQuick
import "../../bar/barstyles/python/widgets/faces" as PythonFaces

// Builds one Serpantinum face for Ryoku's desktop slot. The provider itself has
// no geometry; PythonFaceWidget reparents the loaded Item into the visible slot.
Item {
    id: prov

    property string faceId: "time"
    property string variant: "digital"
    property int screenW: 1920
    property int screenH: 1080
    property string imagePath: ""
    property bool ryokuStyle: false
    property color inkOverride: "transparent"
    property color accentOverride: "transparent"
    property string sizeOverride: ""
    property real scaleOverride: 1
    property real radiusOverride: -1
    property var optionOverrides: ({})

    readonly property var item: host.item
    readonly property bool stretchWidth: prov.faceId === "visualizer" && prov.optionOverrides
        && prov.optionOverrides.stretchWidth === true
    readonly property size defaultSize: prov.stretchWidth
        ? Qt.size(prov.screenW, prov._defaultSize(prov.faceId).height)
        : prov._defaultSize(prov.faceId)
    readonly property size naturalSize: prov._naturalSize(prov.item, prov.defaultSize)
    readonly property real effectiveScale: prov.scaleOverride > 0 ? prov.scaleOverride : 1

    implicitWidth: naturalSize.width * effectiveScale
    implicitHeight: naturalSize.height * effectiveScale
    visible: false
    width: 0
    height: 0

    function _defaultSize(id) {
        switch (id) {
        case "visualizer": return Qt.size(Math.round(prov.screenW / 2), 180);
        case "time":       return Qt.size(250, 120);
        case "music":      return Qt.size(340, 120);
        case "weather":    return Qt.size(250, 120);
        case "image":      return Qt.size(300, 200);
        case "user":       return Qt.size(260, 140);
        case "cpu":
        case "ram":
        case "temp":
        case "disk":       return Qt.size(180, 130);
        case "battery":    return Qt.size(260, 90);
        case "github":     return Qt.size(540, 180);
        case "notes":      return Qt.size(360, 260);
        }
        return Qt.size(240, 160);
    }

    function _metric(face, key, fallback) {
        if (!face || face[key] === undefined)
            return fallback;
        const value = Number(face[key]);
        return Number.isFinite(value) ? value : fallback;
    }

    function _naturalSize(face, fallback) {
        const minW = Math.max(1, prov._metric(face, "minWidth", 1));
        const minH = Math.max(1, prov._metric(face, "minHeight", 1));
        const maxW = Math.max(minW, prov._metric(face, "maxWidth", 99999));
        const maxH = Math.max(minH, prov._metric(face, "maxHeight", 99999));
        const minAspect = Math.max(0, prov._metric(face, "minAspect", 0));
        const maxAspect = Math.max(minAspect, prov._metric(face, "maxAspect", 99999));
        let w = Math.max(minW, Math.min(maxW, fallback.width));
        let h = Math.max(minH, Math.min(maxH, fallback.height));

        if (minAspect > 0 && w / h < minAspect) {
            if (h * minAspect <= maxW)
                w = h * minAspect;
            else
                h = Math.max(minH, w / minAspect);
        }
        if (maxAspect > 0 && w / h > maxAspect) {
            if (h * maxAspect >= minW)
                w = h * maxAspect;
            else
                h = Math.min(maxH, w / maxAspect);
        }
        return Qt.size(w, h);
    }

    function _componentFor(id, faceVariant) {
        switch (id + ":" + faceVariant) {
        case "visualizer:bars":       return cVisualizerBars;
        case "visualizer:continuous": return cVisualizerContinuous;
        case "time:digital":          return cTimeDigital;
        case "time:analog":           return cTimeAnalog;
        case "time:minimal":          return cTimeMinimal;
        case "time:material":         return cTimeMaterial;
        case "time:materialAnalog":   return cTimeMaterialAnalog;
        case "time:lumen":            return cTimeLumen;
        case "time:stretched":        return cTimeStretched;
        case "music:full":            return cMusicFull;
        case "music:round":           return cMusicRound;
        case "music:lyrics":          return cMusicLyrics;
        case "music:simpleLyrics":    return cMusicLyricsSimple;
        case "notes:default":         return cNotes;
        case "weather:compact":       return cWeatherCompact;
        case "weather:full":          return cWeatherFull;
        case "weather:round":         return cWeatherRound;
        case "image:rect":            return cImageRect;
        case "image:rounded":         return cImageRounded;
        case "image:round":           return cImageRound;
        case "user:default":          return cUser;
        case "cpu:default":           return cCpu;
        case "cpu:circle":            return cCpuCircle;
        case "ram:default":           return cRam;
        case "ram:circle":            return cRamCircle;
        case "temp:default":          return cTemp;
        case "temp:circle":           return cTempCircle;
        case "disk:default":          return cDisk;
        case "disk:circle":           return cDiskCircle;
        case "battery:default":       return cBattery;
        case "battery:circle":        return cBatteryCircle;
        case "github:default":        return cGithub;
        }
        return null;
    }

    function _applyOptions() {
        const face = prov.item;
        const opts = prov.optionOverrides;
        if (!face || !opts || typeof opts !== "object")
            return;
        for (const key in opts) {
            if (key === "imagePath" || key === "stretchWidth" || face[key] === undefined)
                continue;
            try {
                face[key] = opts[key];
            } catch (e) {
                // Read-only face state is not a host option.
            }
        }
    }

    onOptionOverridesChanged: _applyOptions()

    Loader {
        id: host
        active: prov._componentFor(prov.faceId, prov.variant) !== null
        sourceComponent: prov._componentFor(prov.faceId, prov.variant)
        onLoaded: prov._applyOptions()
    }

    // The faces speak the same host hook the Shima faces do: ryokuBare tells
    // the face its own plate steps aside because the slot draws the backing.
    // Without it the face paints an opaque serp plate over the Ryoku card and
    // the style toggle looks dead.
    Binding { target: prov.item; property: "ryokuBare"; value: prov.ryokuStyle; when: prov.item !== null && prov.item.ryokuBare !== undefined }
    Binding { target: prov.item; property: "inkOverride"; value: prov.inkOverride; when: prov.item !== null && prov.item.inkOverride !== undefined }
    Binding { target: prov.item; property: "accentOverride"; value: prov.accentOverride; when: prov.item !== null && prov.item.accentOverride !== undefined }
    Component { id: cVisualizerBars; PythonFaces.VisualizerFace {} }
    Component { id: cVisualizerContinuous; PythonFaces.VisualizerFaceContinuous {} }
    Component { id: cTimeDigital; PythonFaces.ClockFaceDigital {} }
    Component { id: cTimeAnalog; PythonFaces.ClockFaceAnalog {} }
    Component { id: cTimeMinimal; PythonFaces.ClockFaceMinimal {} }
    Component { id: cTimeMaterial; PythonFaces.ClockFaceMaterial {} }
    Component { id: cTimeMaterialAnalog; PythonFaces.ClockFaceMaterialAnalog {} }
    Component { id: cTimeLumen; PythonFaces.ClockFaceMaterialLumen {} }
    Component { id: cTimeStretched; PythonFaces.ClockFaceStretched {} }
    Component { id: cMusicFull; PythonFaces.MusicFace {} }
    Component { id: cMusicRound; PythonFaces.MusicFaceRound {} }
    Component { id: cMusicLyrics; PythonFaces.MusicFaceLyrics {} }
    Component { id: cMusicLyricsSimple; PythonFaces.MusicFaceLyricsSimple {} }
    Component { id: cNotes; PythonFaces.NotesFace {} }
    Component { id: cWeatherCompact; PythonFaces.WeatherFaceCompact {} }
    Component { id: cWeatherFull; PythonFaces.WeatherFaceFull {} }
    Component { id: cWeatherRound; PythonFaces.WeatherFaceRound {} }
    Component { id: cImageRect; PythonFaces.ImageFaceRect { imagePath: prov.imagePath } }
    Component { id: cImageRounded; PythonFaces.ImageFaceRounded { imagePath: prov.imagePath } }
    Component { id: cImageRound; PythonFaces.ImageFaceRound { imagePath: prov.imagePath } }
    Component { id: cUser; PythonFaces.UserFace {} }
    Component { id: cCpu; PythonFaces.CpuFace {} }
    Component { id: cCpuCircle; PythonFaces.CpuFaceCircle {} }
    Component { id: cRam; PythonFaces.RamFace {} }
    Component { id: cRamCircle; PythonFaces.RamFaceCircle {} }
    Component { id: cTemp; PythonFaces.TempFace {} }
    Component { id: cTempCircle; PythonFaces.TempFaceCircle {} }
    Component { id: cDisk; PythonFaces.DiskFace {} }
    Component { id: cDiskCircle; PythonFaces.DiskFaceCircle {} }
    Component { id: cBattery; PythonFaces.BatteryFace {} }
    Component { id: cBatteryCircle; PythonFaces.BatteryFaceCircle {} }
    Component { id: cGithub; PythonFaces.GithubFace {} }
}
