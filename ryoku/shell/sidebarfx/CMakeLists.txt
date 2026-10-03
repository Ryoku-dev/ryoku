cmake_minimum_required(VERSION 3.28)

project(ryoku-sidebarfx LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Quick ShaderTools)

qt_standard_project_setup(REQUIRES 6.5)

qt_add_qml_module(ryoku-sidebarfx
    URI Ryoku.SidebarFx
    VERSION 1.0
    OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/SidebarFx"
    SOURCES
        depthedge.cpp depthedge.hpp
)

target_link_libraries(ryoku-sidebarfx PRIVATE Qt6::Quick)

set_target_properties(ryoku-sidebarfx ryoku-sidebarfxplugin PROPERTIES
    LIBRARY_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml/Ryoku/SidebarFx")
set_target_properties(ryoku-sidebarfxplugin PROPERTIES
    INSTALL_RPATH "$ORIGIN"
    BUILD_WITH_INSTALL_RPATH ON)

qt_add_shaders(ryoku-sidebarfx "depth_shaders"
    BATCHABLE
    GLSL "300es,330"
    PREFIX "/"
    FILES
        shaders/depth.vert
        shaders/depth.frag
)
