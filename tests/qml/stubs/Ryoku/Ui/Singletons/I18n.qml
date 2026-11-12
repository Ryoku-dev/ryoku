pragma Singleton
import QtQml

// the translation lookup without Quickshell, which qmltestrunner cannot load: English as written
QtObject {
    function tr(text: string): string {
        return text;
    }
}
