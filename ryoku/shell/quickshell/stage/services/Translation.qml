pragma Singleton

import QtQuick
import Ryoku.Ui.Singletons as RyokuUi

QtObject {
    readonly property string languageCode: RyokuUi.I18n.lang
    function tr(text) {
        return RyokuUi.I18n.tr(text)
    }
}
