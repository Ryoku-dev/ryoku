pragma Singleton
import QtQuick

QtObject {
    function env(name) {
        if (name === "XDG_CONFIG_HOME") return "/tmp/config";
        if (name === "HOME") return "/tmp";
        return "";
    }
}
