# Builds a statically linked `cp` from GNU coreutils and packages it as a
# minimal base image for the auto-instrumentation images. When instrumenting a
# pod, the operator creates an init container that runs `cp` to copy the
# instrumentation artifacts into the application's container, so `cp` is the
# only program the auto-instrumentation images need at runtime.
FROM alpine:latest@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS builder

ARG version

RUN apk add --no-cache build-base curl linux-headers xz

RUN mkdir /coreutils \
    && curl -fsSL "https://ftp.gnu.org/gnu/coreutils/coreutils-${version}.tar.xz" \
    | tar -xJ -C /coreutils --strip-components=1

WORKDIR /coreutils

# Generated headers are only built by `make all`, so build everything and take
# the standalone cp binary. FORCE_UNSAFE_CONFIGURE allows running configure as
# root inside the build container.
RUN FORCE_UNSAFE_CONFIGURE=1 ./configure --disable-nls LDFLAGS=-static \
    && make -j"$(nproc)" \
    && strip src/cp \
    && ./src/cp --version

FROM scratch

# Container runtimes apply a default PATH when the image doesn't set one, but
# be explicit so the operator's `cp` init container command always resolves.
ENV PATH=/bin

COPY --from=builder /coreutils/src/cp /bin/cp
