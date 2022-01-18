go mod download

cd init
mkdir -p pkgroot/sbin

CGO_ENABLED=0 go build -ldflags '-s -w' -o pkgroot/sbin/init

cp -r etc pkgroot/etc
tar -czvf init.tar.gz -C pkgroot .