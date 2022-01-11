set -ex

apk add --no-cache libseccomp-static libseccomp-dev pkgconfig

cd services/containerd
mkdir -p pkgroot/bin

git -c advice.detachedHead=false clone -b v1.0.3 https://github.com/opencontainers/runc
make -C runc static && cp runc/runc pkgroot/bin/

git -c advice.detachedHead=false clone -b v1.5.9 https://github.com/containerd/containerd
make -C containerd EXTRA_FLAGS="-buildmode=pie" EXTRA_LDFLAGS='-s -w -extldflags "-fno-PIC -static"' BUILDTAGS="netgo osusergo static_build no_btrfs" bin/containerd bin/containerd-shim-runc-v2 \
  && cp containerd/bin/containerd containerd/bin/containerd-shim-runc-v2 pkgroot/bin/

cp -r etc pkgroot/etc
tar -czvf containerd.tar.gz -C pkgroot .