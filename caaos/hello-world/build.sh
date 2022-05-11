set -ex

cd caaos/hello-world
mkdir -p pkgroot/p3
mkdir -p pkgroot/p4/opt/caaos/hello-world

CGO_ENABLED=0 go build -ldflags="-s -w" -o hello_world

cp ./hello_world pkgroot/p4/opt/caaos/hello-world/hello_world
cp -r etc/. pkgroot/p3/etc
mkdir -p /workspace/packages
tar -czvf /workspace/packages/hello-world.tar.gz -C pkgroot .