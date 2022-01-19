go mod download

cd init
mkdir -p pkgroot/p2/sbin

CGO_ENABLED=0 go build -ldflags '-s -w' -o pkgroot/p2/sbin/init

cp -r etc pkgroot/p2/etc
tar -czvf init.tar.gz -C pkgroot .