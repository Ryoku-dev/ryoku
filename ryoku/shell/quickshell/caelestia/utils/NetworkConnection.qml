pragma Singleton

import QtQuick
import caelestia.services

/**
 * NetworkConnection
 *
 * Centralized utility for network connection logic. Provides a single source of truth
 * for connecting to wireless networks, eliminating code duplication across
 * controlcenter components and bar popouts.
 *
 * Usage:
 * ```qml
 * import qs.utils
 *
 * // With Session object (controlcenter)
 * NetworkConnection.handleConnect(network, session);
 *
 * // Without Session object (bar popouts) - provide password dialog callback
 * NetworkConnection.handleConnect(network, null, (network) => {
 *     // Show password dialog
 *     root.passwordNetwork = network;
 *     root.showPasswordDialog = true;
 * });
 * ```
 */
QtObject {
    id: root

    property var passwordNetwork: null

    function disconnectFirstIfNeeded(isTarget: bool, connect: var): void {
        if (Nmcli.active && !isTarget) {
            Nmcli.disconnectFromNetwork();
            Qt.callLater(connect);
        } else {
            connect();
        }
    }

    function handleConnect(network, session, onPasswordNeeded): void {
        if (!network) {
            return;
        }

        root.disconnectFirstIfNeeded(Nmcli.active?.ssid === network.ssid, () => {
            root.connectToNetwork(network, session, onPasswordNeeded);
        });
    }

    function connectToSavedProfile(uuid, onResult): void {
        if (!uuid) {
            return;
        }

        const isTarget = !!Nmcli.savedConnectionProfiles.find(p => p.uuid === uuid)?.active;
        root.disconnectFirstIfNeeded(isTarget, () => {
            Nmcli.connectToNetworkByUuid(uuid, onResult || null);
        });
    }

    function connectToNetwork(network, session, onPasswordNeeded): void {
        if (!network) {
            return;
        }

        if (network.isSecure) {
            const hasSavedProfile = Nmcli.hasSavedProfile(network.ssid);

            if (hasSavedProfile) {
                Nmcli.connectToNetwork(network.ssid, "", network.bssid, null);
            } else {
                Nmcli.connectToNetworkWithPasswordCheck(network.ssid, network.isSecure, result => {
                    if (result.needsPassword) {
                        if (Nmcli.pendingConnection) {
                            Nmcli.connectionCheckTimer.stop();
                            Nmcli.immediateCheckTimer.stop();
                            Nmcli.immediateCheckTimer.checkCount = 0;
                            Nmcli.pendingConnection = null;
                        }

                        // Handle password dialog - use session if available, otherwise use callback
                        if (session && session.network) {
                            session.network.showPasswordDialog = true;
                            session.network.pendingNetwork = network;
                        } else if (onPasswordNeeded) {
                            onPasswordNeeded(network);
                        }
                    }
                }, network.bssid);
            }
        } else {
            Nmcli.connectToNetwork(network.ssid, "", network.bssid, null);
        }
    }

    function connectWithPassword(network, password, onResult): void {
        if (!network) {
            return;
        }

        Nmcli.connectToNetwork(network.ssid, password || "", network.bssid || "", onResult || null);
    }
}
