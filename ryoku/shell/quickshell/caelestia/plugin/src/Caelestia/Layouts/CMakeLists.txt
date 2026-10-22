qml_module(caelestia-layouts
    URI Caelestia.Layouts
    SOURCES
        layoutgnome.cpp
        layoutgnome.hpp
        layoutkde.cpp
        layoutkde.hpp
    LIBRARIES
        Qt::Gui
        Qt::Qml
        caelestia-core
)
