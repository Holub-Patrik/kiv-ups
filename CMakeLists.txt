cmake_minimum_required(VERSION 4.0)

project(poker)

if(CMAKE_CXX_COMPILER_ID STREQUAL "Clang")
    message("-- Clang detected, using libc++")
    set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -stdlib=libc++")
    set(CMAKE_EXE_LINKER_FLAGS "${CMAKE_EXE_LINKER_FLAGS} -stdlib=libc++ -lc++abi")
endif()

if(UNIX)
    message("-- Found unix environment, trying if Wayland present")
    find_package(ECM REQUIRED NO_MODULE QUIET)
    list(APPEND CMAKE_MODULE_PATH "${ECM_FIND_MODULE_DIR}")

    find_package(Wayland QUIET)
    if(Wayland_FOUND)
        message("-- Wayland found, setting GLFW_BUILD_WAYLAND to ON")
        set(CUSTOMIZE_BUILD ON)
        set(GLFW_BUILD_WAYLAND ON)
    else()
        message("-- Wayland not found, moving on ...")
    endif()
endif()

function(apply_common_settings TARGET)
    target_compile_features(${TARGET} PRIVATE cxx_std_26)

    target_compile_options(${TARGET} PRIVATE 
        # $<$<CONFIG:Debug>:-fsanitize=address -gdwarf-5>
        $<$<CONFIG:Debug>:-gdwarf-5>
        $<$<CONFIG:Release>:-march=native>
        $<$<CONFIG:RelWithDebInfo>:-gdwarf-5>
    )

    target_link_libraries(${TARGET} PRIVATE
        raylib
        raylib_cpp
    )

    target_link_options(${TARGET} PRIVATE
        # $<$<CONFIG:Debug>:-fsanitize=address>
    )   
endfunction()

message("-- Trying to find raylib dependency")
find_package(raylib QUIET)
if (NOT raylib_FOUND)
    message("-- Raylib not found, downloading ...")
    include(FetchContent)

    FetchContent_Declare(
        raylib
        GIT_REPOSITORY https://github.com/raysan5/raylib.git
        GIT_TAG 5.5
        GIT_SHALLOW 1
    )
    FetchContent_MakeAvailable(raylib)
else()
    message("-- Raylib found")
endif()

message("-- Trying to find raylib_cpp dependency")
find_package(raylib_cpp QUIET)
if (NOT raylib_cpp_FOUND)
    message("-- Raylib_cpp not found, downloading ...")
    include(FetchContent)

    FetchContent_Declare(
        raylib_cpp
        GIT_REPOSITORY https://github.com/RobLoach/raylib-cpp.git
        GIT_TAG v5.5.0
    )
    FetchContent_MakeAvailable(raylib_cpp)
else()
    message("-- Raylib_cpp found")
endif()

file(GLOB_RECURSE sources 
	src/*.c 
	src/*.cpp 
	src/*.h 
	src/*.hpp
)

add_executable(poker ${sources})
apply_common_settings(poker)

file(GLOB_RECURSE exp_sources
    exp/*.c
	exp/*.cpp 
	exp/*.h 
	exp/*.hpp
)

if(exp_sources)
    add_executable(test ${exp_sources})
    apply_common_settings(test)
endif()



