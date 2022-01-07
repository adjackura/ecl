go mod download

cd services/caaos
mkdir -p pkgroot/bin

CGO_ENABLED=0 go build -tags 'netgo,osusergo,static_build' -ldflags '-s -w -extldflags=-static' -o pkgroot/bin/caaos

cp -r etc pkgroot/etc
tar -czvf caaos.tar.gz -C pkgroot .