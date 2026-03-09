cmake_minimum_required(VERSION 3.16)
project(collector-nats CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_library(NATS_LIBRARY NAMES nats REQUIRED)
find_path(NATS_INCLUDE_DIR nats/nats.h REQUIRED)

# Library
add_library(collector-nats
  src/NatsClient.cpp
)
target_include_directories(collector-nats PUBLIC include ${NATS_INCLUDE_DIR})
target_link_libraries(collector-nats PUBLIC ${NATS_LIBRARY} pthread ssl crypto)

# Tests
add_executable(nats_client_test test/nats_client_test.cpp)
target_link_libraries(nats_client_test PRIVATE collector-nats)
