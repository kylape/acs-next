# Generate C++ files for Protobuf and GRPC services
#
# Based on https://github.com/faaxm/exmpl-cmake-grpc

find_package(protobuf CONFIG REQUIRED)
find_package(gRPC CONFIG REQUIRED)
find_package(Threads)

set(ROX_PROTO_FILES
    api/v1/empty.proto
    api/v1/signal.proto
    internalapi/sensor/collector.proto
    internalapi/sensor/network_connection_info.proto
    internalapi/sensor/network_connection_iservice.proto
    internalapi/sensor/network_enums.proto
    internalapi/sensor/signal_iservice.proto
    storage/network_flow.proto
    storage/process_indicator.proto
)

set(ACS_PROTO_FILES
    acs/broker/v1/common.proto
    acs/broker/v1/process.proto
    acs/broker/v1/network.proto
)

add_library(acs-proto ${ACS_PROTO_FILES})
target_link_libraries(acs-proto
    PUBLIC
    protobuf::libprotobuf
)
target_include_directories(acs-proto PUBLIC ${CMAKE_CURRENT_BINARY_DIR})
protobuf_generate(TARGET acs-proto LANGUAGE cpp)

add_library(rox-proto ${ROX_PROTO_FILES})
target_link_libraries(rox-proto
    PUBLIC
    protobuf::libprotobuf
    gRPC::grpc
    gRPC::grpc++
)
target_include_directories(rox-proto PUBLIC ${CMAKE_CURRENT_BINARY_DIR})

get_target_property(grpc_cpp_plugin_location gRPC::grpc_cpp_plugin LOCATION)
protobuf_generate(TARGET rox-proto LANGUAGE cpp)
protobuf_generate(TARGET rox-proto LANGUAGE grpc GENERATE_EXTENSIONS .grpc.pb.h .grpc.pb.cc PLUGIN "protoc-gen-grpc=${grpc_cpp_plugin_location}" PLUGIN_OPTIONS generate_mock_code=true)
