import QtQuick
QtObject {
    property string config: ""
    property string configDirectory: ""
    property string user: ""
    property bool active: false
    property bool responseRequired: false
    property int startCount: 0
    property int abortCount: 0
    property int respondCount: 0
    property string lastResponse: ""
    signal completed(int result)
    signal error(var value)
    signal pamMessage(bool messageIsError)
    Component.onCompleted: PamRegistry.add(this)
    function start() { startCount += 1; active = true; return true }
    function abort() { abortCount += 1; active = false; responseRequired = false }
    function respond(value) { respondCount += 1; lastResponse = value; responseRequired = false }
}
