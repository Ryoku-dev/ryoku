# Cava audio visualizer: in process through libcava when it is installed (AUR only),
# otherwise through the cava program from extra/cava.
set(CAVA_SOURCES cavaprocess.cpp cavaprocess.hpp)
set(CAVA_LIBRARIES "")
if(Cava_FOUND)
    set(CAVA_SOURCES cavaprovider.cpp cavaprovider.hpp)
    list(APPEND CAVA_LIBRARIES PkgConfig::Cava)
endif()

qml_module(caelestia-services
    URI Caelestia.Services
    SOURCES
        service.cpp
        serviceref.cpp
        beattracker.cpp
        audiocollector.cpp
        audioprovider.cpp
        ${CAVA_SOURCES}
        tickingservice.cpp
        sensorslib.cpp
        networkusage.cpp
        networkusage.hpp
        cpu.cpp
        gpu.cpp
        memory.cpp
        diskinfo.cpp
        storage.cpp
        weather.cpp
        weather.hpp
        lyriccandidate.cpp
        lyrics.cpp
        sessionmanager.cpp
        sessionmanager.hpp
        emojidb.cpp
        emojidb.hpp
        clipboardmanager.cpp
        clipboardmanager.hpp
        schemeloader.cpp
        schemeloader.hpp
        palettemanager.cpp
        palettemanager.hpp
        launcherentry.cpp
        launcherentry.hpp
    LIBRARIES
        Qt::Concurrent
        Qt::DBus
        Qt::Network
        Qt::Gui
        Qt::Quick
        PkgConfig::Pipewire
        PkgConfig::Aubio
        ${CAVA_LIBRARIES}
        Sensors::Sensors
        caelestia-core
        caelestia-config
)
