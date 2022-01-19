set -ex

go mod download

cd services/caaos
mkdir -p pkgroot/p2/bin

CGO_ENABLED=0 go build -tags 'netgo,osusergo,static_build' -ldflags '-s -w -extldflags=-static' -o pkgroot/p2/bin/caaos

cp -r etc pkgroot/p2/etc
tar -czvf caaos.tar.gz -C pkgroot .