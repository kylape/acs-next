# Pre-built builder image with all compiled dependencies.
# Build once, push to registry, then collector builds only compile source.
#
# Includes: 17 third-party libs from source + cnats (NATS C client)
#
# Build context must be the collector repo root.

FROM quay.io/centos/centos:stream10

ARG NPROCS=4

RUN dnf -y update \
    && dnf -y install --nobest \
        autoconf \
        automake \
        binutils-devel \
        bison \
        ca-certificates \
        clang \
        llvm \
        cmake \
        diffutils \
        elfutils-libelf-devel \
        file \
        flex \
        gcc \
        gcc-c++ \
        gettext \
        git \
        glibc-devel \
        libcap-ng-devel \
        libcurl-devel \
        libtool \
        libuuid-devel \
        make \
        openssl-devel \
        patchutils \
        pkgconfig \
        rsync \
        tar \
        unzip \
        wget \
        which \
        systemtap-sdt-devel \
    && dnf clean all

WORKDIR /install-tmp

COPY builder/install builder/install
COPY builder/third_party third_party

RUN builder/install/install-dependencies.sh && rm -rf /install-tmp
RUN echo -e '/usr/local/lib\n/usr/local/lib64' > /etc/ld.so.conf.d/usrlocallib.conf && ldconfig

# Build cnats (NATS C client)
RUN cd /tmp && \
    git clone --depth 1 --branch v3.9.1 https://github.com/nats-io/nats.c.git && \
    cd nats.c && mkdir build && cd build && \
    cmake .. -DCMAKE_INSTALL_PREFIX=/usr/local \
             -DNATS_BUILD_WITH_TLS=ON -DNATS_BUILD_STREAMING=OFF \
             -DBUILD_TESTING=OFF && \
    cmake --build . --target install -- -j ${NPROCS} && \
    ldconfig && rm -rf /tmp/nats.c

RUN mkdir /src && chmod a+rwx /src
WORKDIR /src
