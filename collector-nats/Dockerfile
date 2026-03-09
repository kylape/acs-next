FROM registry.access.redhat.com/ubi9/ubi:latest AS builder

RUN dnf install -y gcc-c++ cmake make pkg-config git openssl-devel && dnf clean all

# Build cnats from source (not available as RPM)
RUN git clone --depth 1 --branch v3.9.1 https://github.com/nats-io/nats.c.git /tmp/nats.c && \
    cd /tmp/nats.c && \
    mkdir build && cd build && \
    cmake .. -DCMAKE_INSTALL_PREFIX=/usr -DNATS_BUILD_STREAMING=OFF && \
    make -j$(nproc) && make install && \
    rm -rf /tmp/nats.c

WORKDIR /app
COPY . .

RUN mkdir build && cd build && \
    cmake .. && \
    make -j$(nproc)

# Run unit tests during build
RUN cd build && ./nats_client_test

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
COPY --from=builder /app/build/nats_client_test /usr/local/bin/nats_client_test
COPY --from=builder /usr/lib64/libnats.so* /usr/lib64/
ENTRYPOINT ["/usr/local/bin/nats_client_test"]
