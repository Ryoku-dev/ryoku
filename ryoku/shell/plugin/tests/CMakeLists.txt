find_package(Qt6 REQUIRED COMPONENTS Test)

add_library(bus-fakedaemon STATIC fakedaemon.cpp fakedaemon.hpp)
target_link_libraries(bus-fakedaemon PUBLIC Qt6::Network)

foreach(suite busframer buspatch buslistdiff busqueue busclient busvalue buslistmodel)
    add_executable(tst_${suite} tst_${suite}.cpp)
    target_include_directories(tst_${suite} PRIVATE "${CMAKE_CURRENT_SOURCE_DIR}/..")
    target_link_libraries(tst_${suite} PRIVATE ryoku-bus bus-fakedaemon Qt6::Test Qt6::Qml)
    # load Ryoku.Bus from the build tree, never an installed copy
    target_compile_definitions(tst_${suite} PRIVATE RYOKU_BUS_QML_DIR="${CMAKE_BINARY_DIR}/qml")
    add_dependencies(tst_${suite} ryoku-busplugin)
    add_test(NAME ${suite} COMMAND tst_${suite})
    set_tests_properties(${suite} PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=offscreen;LANG=C.UTF-8")
endforeach()
