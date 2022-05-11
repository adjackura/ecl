set -ex

go mod download

cd services/caaos
mkdir -p pkgroot/p3/bin

CGO_ENABLED=0 go build -tags 'netgo,osusergo,static_build' -ldflags '-s -w -extldflags=-static' -o pkgroot/p3/bin/caaos

cp -r etc pkgroot/p3/etc
mkdir -p /workspace/packages
tar -czvf /workspace/packages/caaos.tar.gz -C pkgroot .