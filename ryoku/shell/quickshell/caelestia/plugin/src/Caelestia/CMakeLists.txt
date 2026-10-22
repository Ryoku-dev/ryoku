qml_module(caelestia-core
    URI Caelestia
    SOURCES
        appidentifiers.cpp
        cutils.cpp
        qalculator.cpp
        circularbuffer.cpp
        requests.cpp
        toaster.cpp
        translations.cpp
    LIBRARIES
        Qt::Gui
        Qt::Quick
        Qt::Concurrent
        Qt::Sql
        PkgConfig::Qalculate
        caelestia-util
)

# Bake the installed translations dir in so Translations can find catalogues
# even when the shell is launched from outside its install tree.
if(IS_ABSOLUTE "${INSTALL_QSCONFDIR}")
    set(_CAELESTIA_QSCONFDIR "${INSTALL_QSCONFDIR}")
else()
    set(_CAELESTIA_QSCONFDIR "${CMAKE_INSTALL_PREFIX}/${INSTALL_QSCONFDIR}")
endif()
target_compile_definitions(caelestia-core PRIVATE CAELESTIA_TRANSLATIONS_DIR="${_CAELESTIA_QSCONFDIR}/translations")

# Bake the shell version in so CUtils can expose it without spawning a process
target_compile_definitions(caelestia-core PRIVATE CAELESTIA_VERSION="${DISPLAY_VERSION}")

# Node macros expand to Q_OBJECT, automoc can't see it otherwise
list(APPEND CMAKE_AUTOMOC_MACRO_NAMES
    SETTINGS_NODE SETTINGS_NODE_NO_CTOR SETTINGS_LIST_TYPE
    CONFIG_NODE CONFIG_NODE_NO_CTOR CONFIG_LIST_TYPE
)

add_subdirectory(Components)
add_subdirectory(Config)
add_subdirectory(Settings)
add_subdirectory(Models)
add_subdirectory(Services)
add_subdirectory(Blobs)
add_subdirectory(Images)
add_subdirectory(Layouts)
