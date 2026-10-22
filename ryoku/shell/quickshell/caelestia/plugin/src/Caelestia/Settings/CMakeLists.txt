qml_module(caelestia-settings
    URI Caelestia.Settings
    SOURCES
        common.cpp
        schema.cpp
        codecs.cpp
        quarantine.cpp
        node.cpp
        objectnode.cpp
        listnode.cpp
        rootnode.cpp
        settingsfile.cpp
        changebatcher.cpp
    LIBRARIES
        caelestia-util
)
