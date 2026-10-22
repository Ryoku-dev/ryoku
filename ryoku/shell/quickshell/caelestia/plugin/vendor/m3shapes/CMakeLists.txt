cmake_minimum_required(VERSION 3.16)
project(M3Shapes VERSION 1.0.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
set(CMAKE_AUTOMOC ON)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)

add_compile_options(
    -Wall -Wextra -Wpedantic -Wshadow -Wconversion
    -Wold-style-cast -Wnull-dereference -Wdouble-promotion
    -Wformat=2 -Wfloat-equal -Woverloaded-virtual
    -Wsign-conversion -Wredundant-decls -Wswitch
    -Wunreachable-code
)

if(CMAKE_CXX_COMPILER_ID MATCHES "Clang")
    add_compile_options(-Wunused-lambda-capture)
endif()

find_package(Qt6 REQUIRED COMPONENTS Core Quick Qml ShaderTools)

qt_policy(SET QTP0001 NEW)
qt_policy(SET QTP0004 NEW)

# Core library
add_library(m3shapes_core STATIC
    src/core/Point.hpp
    src/core/Utils.hpp
    src/core/CornerRounding.hpp
    src/core/Cubic.hpp
    src/core/Cubic.cpp
    src/core/Feature.hpp
    src/core/Feature.cpp
    src/core/RoundedPolygon.hpp
    src/core/RoundedPolygon.cpp
)

target_include_directories(m3shapes_core PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/src
)

target_link_libraries(m3shapes_core PUBLIC
    Qt6::Core
)

# Morph library
add_library(m3shapes_morph STATIC
    src/morph/FloatMapping.hpp
    src/morph/FloatMapping.cpp
    src/morph/PolygonMeasure.hpp
    src/morph/PolygonMeasure.cpp
    src/morph/FeatureMapping.hpp
    src/morph/FeatureMapping.cpp
    src/morph/Morph.hpp
    src/morph/Morph.cpp
)

target_include_directories(m3shapes_morph PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/src
)

target_link_libraries(m3shapes_morph PUBLIC
    m3shapes_core
)

# Shapes library
add_library(m3shapes_shapes STATIC
    src/shapes/Shapes.hpp
    src/shapes/Shapes.cpp
    src/shapes/MaterialShapes.hpp
    src/shapes/MaterialShapes.cpp
)

target_include_directories(m3shapes_shapes PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/src
)

target_link_libraries(m3shapes_shapes PUBLIC
    m3shapes_core
)

# QML Plugin
if(NOT DEFINED QT_QML_OUTPUT_DIRECTORY)
    set(QT_QML_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/qml")
endif()

qt_add_qml_module(m3shapes
    URI M3Shapes
    VERSION 1.0
    SOURCES
        src/qml/MaterialShapeItem.hpp
        src/qml/MaterialShapeItem.cpp
        src/qml/SmoothShapeMaterial.hpp
        src/qml/SmoothShapeMaterial.cpp
    OUTPUT_DIRECTORY
        ${QT_QML_OUTPUT_DIRECTORY}/M3Shapes
)

# Compile the analytic-antialiasing shaders to .qsb via Qt Shader Tools and
# embed them at :/m3shapes/shaders/*.qsb (see SmoothShapeShader).
qt_add_shaders(m3shapes "m3shapes_shaders"
    PREFIX "/m3shapes"
    BASE "${CMAKE_CURRENT_SOURCE_DIR}/src"
    FILES
        src/shaders/smoothshape.vert
        src/shaders/smoothshape.frag
)

qt_query_qml_module(m3shapes
    URI module_uri
    PLUGIN_TARGET module_plugin_target
    TARGET_PATH module_target_path
    QMLDIR module_qmldir
    TYPEINFO module_typeinfo
)

set(INSTALL_QMLDIR "usr/lib/qt6/qml" CACHE STRING "QML install dir")
set(module_dir "${INSTALL_QMLDIR}/${module_target_path}")
install(TARGETS m3shapes LIBRARY DESTINATION "${module_dir}" RUNTIME DESTINATION "${module_dir}")
install(TARGETS "${module_plugin_target}" LIBRARY DESTINATION "${module_dir}" RUNTIME DESTINATION "${module_dir}")
install(FILES "${module_qmldir}" DESTINATION "${module_dir}")
install(FILES "${module_typeinfo}" DESTINATION "${module_dir}")

target_include_directories(m3shapes PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/src
    ${CMAKE_CURRENT_SOURCE_DIR}/src/qml
)

target_link_libraries(m3shapes PUBLIC
    Qt::Quick
    Qt::Qml
    m3shapes_core
    m3shapes_morph
    m3shapes_shapes
)

# Example application
option(M3SHAPES_BUILD_EXAMPLES "Build example applications" OFF)

if(M3SHAPES_BUILD_EXAMPLES)
    qt_add_executable(shape_gallery
        examples/main.cpp
    )

    qt_add_qml_module(shape_gallery
        URI ShapeGalleryApp
        VERSION 1.0
        QML_FILES
            examples/ShapeGallery.qml
    )

    target_link_libraries(shape_gallery PRIVATE
        Qt6::Quick
        Qt6::Qml
        m3shapes
    )

    # Set up import paths
    set_target_properties(shape_gallery PROPERTIES
        QT_QML_IMPORT_PATH "${CMAKE_BINARY_DIR}"
    )
endif()
