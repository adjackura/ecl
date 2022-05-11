set -ex

go mod download

cd init
mkdir -p pkgroot/p3/sbin

CGO_ENABLED=0 go build -ldflags '-s -w' -o pkgroot/p3/sbin/init

cp -r etc pkgroot/p3/etc
mkdir -p /workspace/packages
tar -czvf /workspace/packages/init.tar.gz -C pkgroot .