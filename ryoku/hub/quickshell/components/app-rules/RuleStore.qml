import QtQuick
import "../../Singletons"
import "rules.js" as Rules

// app rules as one Changes store, so adding, editing or removing a rule is one
// change with one undo while its records live in two desktop keys (Rules.tagged)
QtObject {
    id: store

    readonly property var rulesBind: ({ store: "desktop", key: "windowRules" })
    readonly property var looksBind: ({ store: "desktop", key: "appOverrides" })

    function read(key) {
        if (key !== "rules")
            return undefined;
        return Rules.tagged(Changes.value(store.rulesBind), Changes.value(store.looksBind));
    }

    // both halves go out every time, so Retry reapplies; the rule took once the save did
    function write(key, v, done) {
        if (key !== "rules" || !Array.isArray(v)) {
            done(false, "");
            return;
        }
        Changes.write(store.rulesBind, Rules.untagged(v, "windowRules"), "App rules", "app-rules");
        Changes.write(store.looksBind, Rules.untagged(v, "appOverrides"), "App rules", "app-rules");
        // the halves' own errors fold into the rule's, whose Retry writes both again
        Changes.afterDesktopSave(function (ok, error) {
            [store.rulesBind, store.looksBind].forEach(b => {
                if (Changes.error(b) !== "")
                    Changes.setError(b, "");
            });
            done(ok, error);
        });
    }

    Component.onCompleted: Changes.registerStore("app-rules", store)
}
