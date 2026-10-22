qml_module(caelestia-models
    URI Caelestia.Models
    SOURCES
        filesystemmodel.hpp filesystemmodel.cpp
        appdb.cpp
    LIBRARIES
        Qt::Gui
        Qt::Concurrent
        Qt::Sql
)
