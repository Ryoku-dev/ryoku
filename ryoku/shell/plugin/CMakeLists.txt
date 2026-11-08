# Ryoku.Blobs: the SDF metaball plugin behind the screen frame (see
# docs/frame.md). Ryoku.Spatial3D: the depth-displaced wallpaper mesh behind the
# Stage scene (docs/stage.md). Ryoku.Bus is the ryoku-shell daemon bus client,
# and Ryoku.Text handles native text processing.
# All are standard Qt6 QML modules: qt_add_qml_module
# + AUTOMOC + qmltyperegistrar register the QML_ELEMENT types, and qt_add_shaders
# bakes the metaball shaders into the Blobs module as :/shaders/*.qsb.
#
# They produce dynamically loadable modules under <build>/qml/Ryoku.
# build.sh copies the modules onto a QML import path; they ship prebuilt (the target
# has no build toolchain).
cmake_minimum_required(VERSION 3.28)

project(ryoku-blobs LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Concurrent Network Qml Quick ShaderTools Multimedia Quick3D)
find_package(Threads REQUIRED)

qt_standard_project_setup(REQUIRES 6.5)

qt_add_qml_module(ryoku-blobs
    URI Ryoku.Blobs
    VERSION 1.0
    OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/Blobs"
    SOURCES
        blobgroup.cpp blobgroup.hpp
        blobshape.cpp blobshape.hpp
        blobrect.cpp blobrect.hpp
        blobwave.cpp blobwave.hpp
        blobinvertedrect.cpp blobinvertedrect.hpp
        blobmaterial.cpp blobmaterial.hpp
        borderbeam.cpp borderbeam.hpp
        bordermaterial.cpp bordermaterial.hpp
        camerafeed.cpp camerafeed.hpp
        cavafeed.cpp cavafeed.hpp
        cavaspectrum.cpp cavaspectrum.hpp
        wavecapture.cpp wavecapture.hpp
        systemmonitor.cpp systemmonitor.hpp
        systemgraph.cpp systemgraph.hpp
        thinkingorb.cpp thinkingorb.hpp
        playbackclock.cpp playbackclock.hpp
)

# QQuickItems use scene-graph materials, CameraFeed uses Multimedia, and
# SystemMonitor keeps blocking kernel/driver reads on a worker thread.
target_link_libraries(ryoku-blobs PRIVATE Qt6::Quick Qt6::Multimedia Threads::Threads)

# Ship both the dynamic plugin and its backing library in the module directory,
# and give the plugin an $ORIGIN rpath so it resolves the backing from its own
# folder. The module is then relocatable: copy the directory onto any QML import
# path and it loads, with no shared lib to find elsewhere.
set_target_properties(ryoku-blobs ryoku-blobsplugin PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/Blobs")
set_target_properties(ryoku-blobsplugin PROPERTIES
    INSTALL_RPATH "$ORIGIN"
    BUILD_WITH_INSTALL_RPATH ON)

qt_add_qml_module(ryoku-spatial3d
    URI Ryoku.Spatial3D
    VERSION 1.0
    OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/Spatial3D"
    DEPENDENCIES
        QtQuick3D
    SOURCES
        depthmeshgeometry.cpp depthmeshgeometry.hpp
)

target_link_libraries(ryoku-spatial3d PRIVATE Qt6::Concurrent Qt6::Quick Qt6::Quick3D)

set_target_properties(ryoku-spatial3d ryoku-spatial3dplugin PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/Spatial3D")
set_target_properties(ryoku-spatial3dplugin PROPERTIES
    INSTALL_RPATH "$ORIGIN"
    BUILD_WITH_INSTALL_RPATH ON)

qt_add_qml_module(ryoku-bus
    URI Ryoku.Bus
    VERSION 1.0
    OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/Bus"
    SOURCES
        busframer.cpp busframer.hpp
        buspath.cpp buspath.hpp
        buspatch.cpp buspatch.hpp
        busworker.cpp busworker.hpp
        busdispatch.cpp busdispatch.hpp
        busclient.cpp busclient.hpp
        busvalue.cpp busvalue.hpp
        buslistdiff.cpp buslistdiff.hpp
        buslistmodel.cpp buslistmodel.hpp
)

# public so the tests linking the backing library get Qt's headers
target_link_libraries(ryoku-bus PUBLIC Qt6::Qml Qt6::Network)

set_target_properties(ryoku-bus ryoku-busplugin PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/Bus")
set_target_properties(ryoku-busplugin PROPERTIES
    INSTALL_RPATH "$ORIGIN"
    BUILD_WITH_INSTALL_RPATH ON)

qt_add_qml_module(ryoku-text
    URI Ryoku.Text
    VERSION 1.0
    OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/Text"
    SOURCES
        textcatalog.cpp textcatalog.hpp
        fuzzyranker.cpp fuzzyranker.hpp
        hubsearch.cpp hubsearch.hpp
)

target_link_libraries(ryoku-text PUBLIC Qt6::Core Qt6::Qml)

set_target_properties(ryoku-text ryoku-textplugin PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/Text")
set_target_properties(ryoku-textplugin PROPERTIES
    INSTALL_RPATH "$ORIGIN"
    BUILD_WITH_INSTALL_RPATH ON)

option(RYOKU_PLUGIN_TESTS "Build the plugin test suites" OFF)
if(RYOKU_PLUGIN_TESTS)
    enable_testing()
    add_subdirectory(tests)
endif()

qt_add_shaders(ryoku-blobs "blob_shaders"
    BATCHABLE
    GLSL "300es,330"
    PREFIX "/"
    FILES
        shaders/blob.vert
        shaders/blob.frag
        shaders/beam.vert
        shaders/beam.frag
)
