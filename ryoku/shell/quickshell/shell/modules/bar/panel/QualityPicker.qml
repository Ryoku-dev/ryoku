pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../../../components"
import Ryoku.Ui.Singletons

/**
 * The quality picker for a pasted link, and the box that hosts it.
 *
 * It occupies the one place below the field that already had something to say,
 * and it swaps what that says: with nothing probed it is the sites bubble, with
 * a probe running it is a quiet line, and once the formats arrive it is the file
 * title and one row per tier. Four states in one slot beats four stacked boxes
 * that fight over the same vertical space.
 *
 * Nothing here probes on its own. A format list is a network round trip, so it
 * costs a deliberate press of Fetch, and this renders whatever `Stash.probeTiers`
 * already holds. The panel above never has to know the shape of that list, and
 * a link the user has not asked about never spends a request.
 *
 * Every mode gets one, because every mode has something to choose. Video and
 * Mute choose a resolution, Audio chooses a bitrate, and the same row carries
 * either, since a row that showed "1080p" under Audio would be a lie about what
 * the user is about to get.
 */
Item {
    id: root

    property real s: 1
    property string url: ""
    property string mode: "auto"
    // The download has been queued and the field is spent, so the panel clears
    // it rather than leaving a link that has already been taken.
    signal accepted()

    readonly property var tiers: Stash.probeTiers

    implicitHeight: body.implicitHeight
    visible: implicitHeight > 0

    function pick(tier) {
        const u = root.url.trim();
        if (u.length === 0)
            return;
        Stash.enqueueDownload(u, root.mode, tier);
        Stash.resetProbe();
        root.accepted();
    }

    // The words above a tier are ours, the rung below it is the probe's. Video
    // answers in heights, audio in bitrates, and only the label is shared, so
    // the key is the single thing that decides what the row is called.
    function tierName(key) {
        switch ("" + key) {
        case "best": return I18n.tr("Best");
        case "avg": return I18n.tr("Mid");
        case "low": return I18n.tr("Low");
        case "high": return I18n.tr("High");
        case "std": return I18n.tr("Standard");
        case "light": return I18n.tr("Light");
        case "audio": return I18n.tr("Audio");
        }
        return I18n.tr("Quality");
    }

    // What the second line says depends on what the tier carries: a rung on the
    // resolution ladder for a video, a bitrate for a track. An audio-only link
    // probed under Video or Mute still answers in bitrates, so the tier's own
    // field decides, not the chip. Neither is printed when the probe had
    // nothing to say. The unit stays a unit: "kbps" and "p" are readouts, not
    // words, so they are appended rather than wrapped.
    function detail(tier) {
        if (tier.bitrate)
            return parseInt(tier.bitrate) + "k";
        const h = tier.height ? parseInt(tier.height) : 0;
        return h > 0 ? h + "p" : "";
    }

    // Sizes are an estimate from the probe, so they never claim a decimal:
    // a tenth of a megabyte is noise, a whole gigabyte is not.
    function size(tier) {
        const mb = tier.sizeMB != null ? parseFloat(tier.sizeMB) : NaN;
        if (isNaN(mb) || mb <= 0)
            return "";
        if (mb >= 1000)
            return I18n.tr("~%1 GB").arg((mb / 1000).toFixed(1));
        return I18n.tr("~%1 MB").arg(Math.round(mb));
    }

    Column {
        id: body
        width: parent.width
        spacing: 7 * root.s

        // ── Idle ───────────────────────────────────────────────────────
        // What the slot says before anything has been asked: the reach of the
        // engine behind the field, live from cobalt when it is the one running.
        Rectangle {
            width: parent.width
            visible: Stash.probeState === "idle"
            radius: Theme.radiusWidget
            color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.04)
            border.width: Theme.borderWidth
            border.color: Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.4)
            implicitHeight: sitesCol.implicitHeight + 18 * root.s

            Column {
                id: sitesCol
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.margins: 9 * root.s
                spacing: 4 * root.s

                Row {
                    spacing: 5 * root.s
                    MaterialIcon {
                        anchors.verticalCenter: parent.verticalCenter
                        font.pixelSize: 10 * root.s
                        text: "public"
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: Stash.cobaltState === "running"
                            ? I18n.tr("WORKS WITH %1 SITES").arg(Stash.supportedSites.length)
                            : I18n.tr("POWERED BY YT-DLP")
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                        font.family: Theme.fontPrimary
                        font.pixelSize: 6.5 * root.s
                        font.weight: Font.DemiBold
                        font.letterSpacing: 1.5
                    }
                }

                Text {
                    width: parent.width
                    text: Stash.cobaltState === "running"
                        ? Stash.supportedSites.map(s => s === "twitter" ? "x" : s).join("  ·  ")
                        : I18n.tr("Works with 1000+ sites, including YouTube, Twitter/X, Reddit, TikTok, and more.")
                    wrapMode: Text.WordWrap
                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                    font.family: Theme.mono
                    font.pixelSize: 8.5 * root.s
                    lineHeight: 1.3
                }
            }
        }

        // ── Probing ────────────────────────────────────────────────────
        // A bare line, and deliberately not a box: reading a format list is a
        // second of waiting, not a state worth framing. The bar in the field's
        // own button says the same thing where the press that caused it is.
        Row {
            width: parent.width
            height: 14 * root.s
            spacing: 6 * root.s
            visible: Stash.probeState === "probing"

            MaterialIcon {
                anchors.verticalCenter: parent.verticalCenter
                font.pixelSize: 11 * root.s
                text: "progress_activity"
                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                RotationAnimation on rotation {
                    running: Stash.probeState === "probing"
                    loops: Animation.Infinite
                    from: 0; to: 360; duration: 900
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Reading formats")
                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                font.family: Theme.fontPrimary
                font.pixelSize: 8.5 * root.s
            }
        }

        // ── Ready ──────────────────────────────────────────────────────
        // The file's own title heads the list, so the rows below are read as
        // qualities of that file and not of whatever was last pasted.
        Rectangle {
            width: parent.width
            visible: Stash.probeState === "ready" && root.tiers.length > 0
            radius: Theme.radiusWidget
            color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.04)
            border.width: Theme.borderWidth
            border.color: Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.4)
            implicitHeight: readyCol.implicitHeight + 16 * root.s

            Column {
                id: readyCol
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.margins: 8 * root.s
                spacing: 5 * root.s

                // A real title, not a label: the letter-spaced small-caps this wore was for
                // a word the panel chose, and a video's own name is often two
                // lines of it. It wraps and the box grows, because a title cut
                // to one ellipsis line says nothing about what is being offered.
                Text {
                    width: parent.width
                    text: Stash.probeTitle.length > 0 ? Stash.probeTitle : I18n.tr("Quality")
                    wrapMode: Text.WordWrap
                    lineHeight: 1.25
                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                    font.family: Theme.fontPrimary
                    font.pixelSize: 9.5 * root.s
                    font.weight: Font.DemiBold
                }

                Repeater {
                    model: root.tiers
                    delegate: Rectangle {
                        id: tierRow
                        required property var modelData
                        required property int index

                        width: parent.width
                        implicitHeight: tierCol.implicitHeight + 10 * root.s
                        radius: Theme.radiusWidget
                        color: tierHover.containsMouse
                            ? Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.10)
                            : Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.05)
                        border.width: Theme.borderWidth
                        border.color: Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.30)
                        Behavior on color { ColorAnimation { duration: Motion.fast } }

                        // The list arrives as the answer to a press, so it is
                        // staggered rather than dropped in flat: the eye is still
                        // on the Fetch button that asked.
                        opacity: 0
                        SequentialAnimation on opacity {
                            PauseAnimation { duration: tierRow.index * 45 }
                            NumberAnimation {
                                to: 1
                                duration: Motion.rowReveal
                                easing.type: Motion.rowRevealCurve
                            }
                        }
                        Translate {
                            y: 5 * root.s
                            SequentialAnimation on y {
                                PauseAnimation { duration: tierRow.index * 45 }
                                NumberAnimation {
                                    to: 0
                                    duration: Motion.rowReveal
                                    easing.type: Motion.rowRevealCurve
                                }
                            }
                        }

                        // Hover only: the button on the right owns the action,
                        // so a pass over the row to read the numbers cannot
                        // start anything.
                        MouseArea {
                            id: tierHover
                            anchors.fill: parent
                            hoverEnabled: true
                            acceptedButtons: Qt.NoButton
                        }

                        Column {
                            id: tierCol
                            anchors.left: parent.left
                            anchors.right: tierBtn.left
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.leftMargin: 9 * root.s
                            anchors.rightMargin: 8 * root.s
                            spacing: 1 * root.s

                            Text {
                                width: parent.width
                                text: root.tierName(tierRow.modelData.key)
                                elide: Text.ElideRight
                                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                                font.family: Theme.fontPrimary
                                font.pixelSize: 9.5 * root.s
                                font.weight: Font.DemiBold
                            }

                            Row {
                                width: parent.width
                                spacing: 6 * root.s

                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    visible: text.length > 0
                                    text: root.detail(tierRow.modelData)
                                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                                    font.family: Theme.mono
                                    font.pixelSize: 8 * root.s
                                }
                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    visible: text.length > 0
                                    text: root.size(tierRow.modelData)
                                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                                    font.family: Theme.mono
                                    font.pixelSize: 8 * root.s
                                }
                            }
                        }

                        Rectangle {
                            id: tierBtn
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.rightMargin: 7 * root.s
                            width: dlText.implicitWidth + 16 * root.s
                            height: 18 * root.s
                            radius: Theme.radiusWidget
                            color: dlArea.containsMouse
                                ? Qt.rgba(Theme.primary.r, Theme.primary.g, Theme.primary.b, 0.28)
                                : Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.07)
                            border.width: Theme.borderWidth
                            border.color: dlArea.containsMouse
                                ? "transparent"
                                : Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.35)
                            Behavior on color { ColorAnimation { duration: Motion.fast } }
                            Behavior on border.color { ColorAnimation { duration: Motion.fast } }

                            Text {
                                id: dlText
                                anchors.centerIn: parent
                                text: I18n.tr("Download")
                                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                                font.family: Theme.fontPrimary
                                font.pixelSize: 8.5 * root.s
                                font.weight: Font.DemiBold
                            }

                            // The plate is short, so the tap target is grown past
                            // it: a label is not a target size.
                            MouseArea {
                                id: dlArea
                                anchors.fill: parent
                                anchors.margins: -4 * root.s
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: root.pick(tierRow.modelData)
                            }
                        }
                    }
                }
            }
        }

        // ── Ready, but nothing on this chip ────────────────────────────
        // One probe answers every mode at once, so a source can succeed and
        // still have nothing to offer the chip now selected: a silent video
        // has no audio rungs to draw. That is a true answer rather than a
        // failure, so it wears the same bare line the probe wore, and no box,
        // no red and no fallback button. Anything else here would claim the
        // link could not be read when it very much could.
        Row {
            width: parent.width
            height: 14 * root.s
            spacing: 6 * root.s
            visible: Stash.probeState === "ready" && root.tiers.length === 0

            // The glyph is not the probe's spinner, so nothing turns: the
            // waiting is over and this is what it found. It keeps the slot
            // width so the line reads in the same column as "Reading
            // formats" above it rather than jumping left, and it follows the
            // chip so a muted source is never answered with a broken speaker.
            MaterialIcon {
                anchors.verticalCenter: parent.verticalCenter
                font.pixelSize: 11 * root.s
                text: root.mode === "audio" ? "volume_off" : "visibility_off"
                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                // Only Audio empties in practice, and saying so is more use
                // than a generic shrug if a source ever comes back bare.
                text: root.mode === "audio" ? I18n.tr("No audio tracks")
                                           : I18n.tr("No formats for this")
                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                font.family: Theme.fontPrimary
                font.pixelSize: 8.5 * root.s
            }
        }

        // ── Failed ─────────────────────────────────────────────────────
        // A link that cannot be read is still a link the user wants, so the
        // line offers the old behaviour rather than a dead end.
        Rectangle {
            width: parent.width
            visible: Stash.probeState === "error"
            radius: Theme.radiusWidget
            color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.04)
            border.width: Theme.borderWidth
            border.color: Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.4)
            implicitHeight: errCol.implicitHeight + 14 * root.s

            Column {
                id: errCol
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.margins: 7 * root.s
                spacing: 5 * root.s

                Row {
                    width: parent.width
                    height: 18 * root.s
                    spacing: 7 * root.s

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Couldn't read formats")
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                        font.family: Theme.fontPrimary
                        font.pixelSize: 8.5 * root.s
                    }

                    Rectangle {
                        anchors.verticalCenter: parent.verticalCenter
                        width: bestText.implicitWidth + 16 * root.s
                        height: 18 * root.s
                        radius: Theme.radiusWidget
                        color: bestArea.containsMouse
                            ? Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.10)
                            : Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.05)
                        border.width: Theme.borderWidth
                        border.color: Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.35)
                        Behavior on color { ColorAnimation { duration: Motion.fast } }

                        Text {
                            id: bestText
                            anchors.centerIn: parent
                            text: I18n.tr("Download best")
                            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                            font.family: Theme.fontPrimary
                            font.pixelSize: 8.5 * root.s
                            font.weight: Font.DemiBold
                        }

                        // A small label is still a control, so the tap target is
                        // grown past the plate rather than left at 18px.
                        MouseArea {
                            id: bestArea
                            anchors.fill: parent
                            anchors.margins: -4 * root.s
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: root.pick(null)
                        }
                    }
                }

                // The helper's own words, not ours: "Unsupported URL" tells the
                // user which link failed and a private video tells them to try a
                // different one. Mono because it is the helper speaking, and
                // elided because a shell error can run long and a wrapped one
                // would push the rest of the panel around.
                Text {
                    width: parent.width
                    visible: Stash.probeMsg.length > 0
                    text: Stash.probeMsg
                    elide: Text.ElideRight
                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                    font.family: Theme.mono
                    font.pixelSize: 7.5 * root.s
                }
            }
        }
    }
}