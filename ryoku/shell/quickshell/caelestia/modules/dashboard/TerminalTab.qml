pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import Caelestia.Config
import caelestia.components
import caelestia.components.containers
import caelestia.components.controls
import caelestia.services
import caelestia.utils

Item {
    id: root

    readonly property string shellName: "fish"

    property string outputBuffer: ""
    property string currentDirectory: Paths.home
    property string hostname: ""
    property int activeProcessesCount: 0
    property bool isRunning: activeProcessesCount > 0
    property bool renderPending: false
    property int maxOutputLines: 2000
    property var activeShellProcess: null

    readonly property var ansiColors: ({
            30: Colours.palette.m3outline,
            31: Colours.palette.m3error,
            32: Colours.palette.m3tertiary,
            33: Colours.palette.m3secondary,
            34: Colours.palette.m3primary,
            35: Colours.palette.m3tertiary,
            36: Colours.palette.m3onPrimaryContainer,
            37: Colours.palette.m3onSurface
        })
    readonly property var ansiBrightColors: ({
            90: Colours.palette.m3onSurfaceVariant,
            91: Colours.palette.m3onErrorContainer,
            92: Colours.palette.m3onTertiaryContainer,
            93: Colours.palette.m3onSecondaryContainer,
            94: Colours.palette.m3onPrimaryContainer,
            95: Colours.palette.m3onTertiaryContainer,
            96: Colours.palette.m3onSecondaryContainer,
            97: Colours.palette.m3onSurfaceVariant
        })

    readonly property string prompt: {
        const user = Quickshell.env("USER") || "user";
        return user + "@" + (root.hostname !== "" ? root.hostname : "caelestia");
    }

    function get256Color(n: int): string {
        if (n >= 0 && n <= 7)
            return ansiColors[30 + n] || Colours.palette.m3onSurface;
        if (n >= 8 && n <= 15)
            return ansiBrightColors[90 + (n - 8)] || Colours.palette.m3onSurfaceVariant;
        if (n >= 16 && n <= 231) {
            const idx = n - 16;
            const b = (idx % 6) * 51;
            const g = (Math.floor(idx / 6) % 6) * 51;
            const r = Math.floor(idx / 36) * 51;
            return `rgb(${r},${g},${b})`;
        }
        if (n >= 232 && n <= 255) {
            const val = 8 + (n - 232) * 10;
            return `rgb(${val},${val},${val})`;
        }
        return Colours.palette.m3onSurface;
    }

    function ansiToHtml(ansiStr: string): string {
        const escaped = ansiStr.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

        let result = "";
        const regex = /\x1b\[([0-9;]*)m/g;
        let lastIndex = 0;
        let activeSpans = 0;
        let match;

        while ((match = regex.exec(escaped)) !== null) {
            result += escaped.substring(lastIndex, match.index);
            const rawCodes = match[1].split(";").map(Number);

            if (rawCodes.includes(0) || match[1] === "") {
                while (activeSpans > 0) {
                    result += "</span>";
                    activeSpans--;
                }
            }

            const styles = [];
            for (let i = 0; i < rawCodes.length; i++) {
                const code = rawCodes[i];
                if (code === 0) {
                } else if (code === 1) {
                    styles.push("font-weight: bold;");
                } else if (code === 2) {
                    styles.push("opacity: 0.75;");
                } else if (code === 3) {
                    styles.push("font-style: italic;");
                } else if (code === 4) {
                    styles.push("text-decoration: underline;");
                } else if (code >= 30 && code <= 37) {
                    styles.push("color: " + (ansiColors[code] || Colours.palette.m3onSurface) + ";");
                } else if (code === 38) {
                    if (rawCodes[i + 1] === 5 && i + 2 < rawCodes.length) {
                        styles.push("color: " + get256Color(rawCodes[i + 2]) + ";");
                        i += 2;
                    } else if (rawCodes[i + 1] === 2 && i + 4 < rawCodes.length) {
                        styles.push(`color: rgb(${rawCodes[i + 2]},${rawCodes[i + 3]},${rawCodes[i + 4]});`);
                        i += 4;
                    }
                } else if (code === 39) {
                    styles.push("color: " + Colours.palette.m3onSurface + ";");
                } else if (code >= 40 && code <= 47) {
                    styles.push("background-color: " + (ansiColors[code - 10] || Colours.palette.m3surface) + ";");
                } else if (code === 48) {
                    if (rawCodes[i + 1] === 5 && i + 2 < rawCodes.length) {
                        styles.push("background-color: " + get256Color(rawCodes[i + 2]) + ";");
                        i += 2;
                    } else if (rawCodes[i + 1] === 2 && i + 4 < rawCodes.length) {
                        styles.push(`background-color: rgb(${rawCodes[i + 2]},${rawCodes[i + 3]},${rawCodes[i + 4]});`);
                        i += 4;
                    }
                } else if (code === 49) {
                    styles.push("background-color: transparent;");
                } else if (code >= 90 && code <= 97) {
                    styles.push("color: " + (ansiBrightColors[code] || Colours.palette.m3onSurfaceVariant) + ";");
                } else if (code >= 100 && code <= 107) {
                    styles.push("background-color: " + (ansiBrightColors[code - 10] || Colours.palette.m3surfaceVariant) + ";");
                }
            }

            if (styles.length > 0) {
                result += "<span style=\"" + styles.join(" ") + "\">";
                activeSpans++;
            }

            lastIndex = regex.lastIndex;
        }

        result += escaped.substring(lastIndex);

        while (activeSpans > 0) {
            result += "</span>";
            activeSpans--;
        }

        return "<pre style=\"font-family: 'JetBrains Mono', Consolas, monospace; margin: 0;\">" + result.replace(/\n/g, "<br>") + "</pre>";
    }

    function trimOutputBuffer(): void {
        const lines = outputBuffer.split("\n");
        if (lines.length <= maxOutputLines)
            return;

        outputBuffer = lines.slice(lines.length - maxOutputLines).join("\n");
    }

    function queueRenderOutput(): void {
        if (renderPending)
            return;
        renderPending = true;
        renderTimer.restart();
    }

    function appendOutput(text: string, isError: bool): void {
        outputBuffer += (isError ? "\x1b[31m" + text + "\x1b[0m" : text) + "\n";
        trimOutputBuffer();
        queueRenderOutput();
    }

    function scrollToBottom(): void {
        Qt.callLater(() => {
            if (outputFlickable) {
                outputFlickable.contentY = Math.max(0, outputFlickable.contentHeight - outputFlickable.height);
            }
        });
    }

    function startShell(): void {
        outputBuffer = "";
        outputArea.text = "";
    }

    function sendCommand(text: string): void {
        const trimmed = text.trim();
        if (trimmed === "")
            return;

        appendOutput((outputBuffer === "" ? "" : "\n") + "\x1b[36m" + prompt + "\x1b[0m\n\x1b[32m❯\x1b[0m " + trimmed, false);

        if (trimmed === "clear") {
            clearOutput();
            return;
        }

        if (activeShellProcess !== null && activeShellProcess.running) {
            activeShellProcess.write(trimmed + "\n");
            return;
        }

        if (trimmed.startsWith("cd ")) {
            const path = trimmed.substring(3).trim();
            changeDirectory(path);
            return;
        } else if (trimmed === "cd") {
            currentDirectory = Paths.home;
            return;
        }

        activeShellProcess = shellProcessComp.createObject(root, {
            command: ["fish", "-c", trimmed],
            workingDirectory: currentDirectory,
            running: true
        });
    }

    function changeDirectory(path: string): void {
        let targetPath = path;
        if (targetPath.startsWith("~")) {
            targetPath = Paths.home + targetPath.substring(1);
        }
        pwdResolverComp.createObject(root, {
            command: ["fish", "-c", "cd " + targetPath + " && pwd"],
            workingDirectory: currentDirectory,
            running: true
        });
    }

    function clearOutput(): void {
        outputBuffer = "";
        outputArea.text = "";
    }

    implicitWidth: 840

    implicitHeight: 500

    Component.onCompleted: {
        startShell();
        hostnameResolverComp.createObject(root, {
            command: ["cat", "/etc/hostname"],
            running: true
        });
    }

    Timer {
        id: renderTimer

        interval: 33
        repeat: false
        onTriggered: {
            renderPending = false;
            outputArea.text = ansiToHtml(outputBuffer);
            scrollToBottom();
        }
    }

    Connections {
        function onCurrentLightChanged(): void {
            root.queueRenderOutput();
        }

        function onSchemeChanged(): void {
            root.queueRenderOutput();
        }

        target: Colours
    }

    Component {
        id: shellProcessComp

        Process {
            running: false
            stdout: SplitParser {
                onRead: text => {
                    appendOutput(text, false);
                }
            }
            stderr: SplitParser {
                onRead: text => {
                    appendOutput(text, true);
                }
            }
            Component.onCompleted: {
                activeProcessesCount++;
            }
            onExited: code => {
                activeProcessesCount--;
                destroy();
            }
        }
    }

    Component {
        id: pwdResolverComp

        Process {
            running: false
            stdout: StdioCollector {
                onStreamFinished: {
                    let resolved = this.text.trim();
                    if (resolved && resolved !== "") {
                        currentDirectory = resolved;
                    }
                    destroy();
                }
            }
            stderr: StdioCollector {
                onStreamFinished: {
                    outputBuffer += "\x1b[31m" + this.text + "\x1b[0m\n";
                    outputArea.text = ansiToHtml(outputBuffer);
                    scrollToBottom();
                    destroy();
                }
            }
        }
    }

    Component {
        id: autocompleterComp

        Process {
            running: false
            stdout: StdioCollector {
                onStreamFinished: {
                    let suggestions = this.text.split("\n").map(s => s.trim()).filter(s => s !== "");
                    if (suggestions.length > 0) {
                        let suggestion = suggestions[0].split("\t")[0];
                        let words = commandInput.text.split(" ");
                        words[words.length - 1] = suggestion;
                        commandInput.text = words.join(" ");
                        commandInput.cursorPosition = commandInput.text.length;
                    }
                    destroy();
                }
            }
            stderr: StdioCollector {
                onStreamFinished: {
                    destroy();
                }
            }
        }
    }

    Component {
        id: hostnameResolverComp

        Process {
            running: false
            stdout: StdioCollector {
                onStreamFinished: {
                    let resolved = this.text.trim();
                    if (resolved && resolved !== "") {
                        root.hostname = resolved;
                    }
                    destroy();
                }
            }
        }
    }

    StyledRect {
        anchors.fill: parent

        radius: Tokens.rounding.large
        color: Colours.tPalette.m3surfaceContainerHigh

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: Tokens.padding.medium
            spacing: Tokens.spacing.medium

            StyledFlickable {
                id: outputFlickable

                Layout.fillWidth: true
                Layout.fillHeight: true

                contentWidth: width
                contentHeight: outputArea.implicitHeight + Tokens.padding.small * 2
                flickableDirection: Flickable.VerticalFlick

                StyledScrollBar.vertical: StyledScrollBar {
                    flickable: outputFlickable
                }

                TextEdit {
                    id: outputArea

                    width: outputFlickable.width - Tokens.padding.small * 2
                    x: Tokens.padding.small
                    y: Tokens.padding.small

                    readOnly: true
                    selectByMouse: true
                    cursorVisible: false
                    textFormat: TextEdit.RichText
                    wrapMode: TextEdit.Wrap
                    font: Tokens.font.mono.small
                    color: Colours.palette.m3onSurface
                    selectedTextColor: Colours.palette.m3onSecondaryContainer
                    selectionColor: Colours.palette.m3secondaryContainer
                }
            }

            Rectangle {
                id: inputBoxRect

                Layout.fillWidth: true
                Layout.preferredHeight: 36

                radius: Tokens.rounding.medium
                color: Colours.palette.m3surfaceContainer
                border.width: 1
                border.color: commandInput.activeFocus ? Colours.palette.m3primary : Qt.alpha(Colours.palette.m3outlineVariant, 0.4)

                RowLayout {
                    id: inputRow

                    anchors.fill: parent
                    anchors.leftMargin: Tokens.padding.medium
                    anchors.rightMargin: Tokens.padding.medium
                    spacing: Tokens.spacing.small

                    StyledText {
                        text: root.prompt + " ❯"
                        font: Tokens.font.mono.small
                        color: Colours.palette.m3tertiary
                    }

                    // Text fields container to overlay ghost autocomplete text behind typing text
                    Item {
                        Layout.fillWidth: true
                        Layout.fillHeight: true

                        StyledTextField {
                            id: commandInput

                            property var commandHistory: []
                            property int historyIndex: -1
                            property string tempTypedText: ""

                            anchors.fill: parent
                            leftPadding: 0
                            rightPadding: 0
                            topPadding: 0
                            bottomPadding: 0

                            font: Tokens.font.mono.small
                            color: Colours.palette.m3onSurface
                            selectedTextColor: Colours.palette.m3onSecondaryContainer
                            selectionColor: Colours.palette.m3secondaryContainer
                            placeholderTextColor: Colours.palette.m3onSurfaceVariant

                            background: null
                            focus: true

                            onVisibleChanged: {
                                if (visible) {
                                    forceActiveFocus();
                                }
                            }

                            Keys.onUpPressed: event => {
                                if (commandHistory.length === 0)
                                    return;
                                if (historyIndex === -1) {
                                    tempTypedText = text;
                                    historyIndex = commandHistory.length - 1;
                                } else if (historyIndex > 0) {
                                    historyIndex--;
                                }
                                text = commandHistory[historyIndex];
                                cursorPosition = text.length;
                                event.accepted = true;
                            }

                            Keys.onDownPressed: event => {
                                if (historyIndex === -1)
                                    return;
                                if (historyIndex < commandHistory.length - 1) {
                                    historyIndex++;
                                    text = commandHistory[historyIndex];
                                } else {
                                    historyIndex = -1;
                                    text = tempTypedText;
                                }
                                cursorPosition = text.length;
                                event.accepted = true;
                            }

                            Keys.onTabPressed: event => {
                                let typed = text;
                                if (typed.trim() === "")
                                    return;

                                autocompleterComp.createObject(root, {
                                    command: ["fish", "-c", "complete -C\"" + typed.replace(/"/g, "\\\"") + "\""],
                                    workingDirectory: currentDirectory,
                                    running: true
                                });
                                event.accepted = true;
                            }

                            onAccepted: {
                                const cmd = text;
                                text = "";
                                if (cmd.trim() !== "") {
                                    if (commandHistory.length === 0 || commandHistory[commandHistory.length - 1] !== cmd) {
                                        commandHistory.push(cmd);
                                        commandHistory = commandHistory;
                                    }
                                }
                                historyIndex = -1;
                                tempTypedText = "";
                                root.sendCommand(cmd);
                            }
                        }
                    }
                }
            }
        }
    }
}
