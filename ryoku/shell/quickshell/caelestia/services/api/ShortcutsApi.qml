import QtQuick

QtObject {
    id: root

    function register(name, description, key, callback) {
        let qml = `
            import QtQuick
            import caelestia.components.misc

            CustomShortcut {
                name: ${JSON.stringify(name)}
                description: ${JSON.stringify(description)}
                key: ${JSON.stringify(key)}
            }
        `;
        let shortcut = Qt.createQmlObject(qml, root, "dynamicShortcut_" + name);
        if (shortcut && callback) {
            shortcut.pressed.connect(callback);
        }
        return shortcut;
    }
}
