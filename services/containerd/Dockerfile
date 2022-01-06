FROM gcr.io/cloud-builders/go:1.17

RUN apk add --no-cache libseccomp-static libseccomp-dev pkgconfig

WORKDIR /workspace

RUN git -c advice.detachedHead=false clone -b v1.0.3 https://github.com/opencontainers/runc
RUN make -C runc static && cp runc/runc /

RUN git -c advice.detachedHead=false clone -b v1.5.9 https://github.com/containerd/containerd
RUN make -C containerd EXTRA_FLAGS="-buildmode=pie" EXTRA_LDFLAGS='-s -w -extldflags "-fno-PIC -static"' BUILDTAGS="netgo osusergo static_build no_btrfs" bin/containerd bin/containerd-shim-runc-v2 \
    && cp containerd/bin/containerd containerd/bin/containerd-shim-runc-v2 /

FROM scratch

COPY --from=0 containerd containerd-shim-runc-v2 runc /bin/