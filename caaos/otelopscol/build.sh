set -ex

cd caaos/otelopscol
mkdir -p pkgroot/p2
mkdir -p pkgroot/p3/opt/caaos/otelopscol

git clone https://github.com/adjackura/opentelemetry-operations-collector.git
cd opentelemetry-operations-collector
#git -c advice.detachedHead=false checkout d2c97eb
CGO_ENABLED=0 go build -ldflags '-s -w' -o otelopscol ./cmd/otelopscol 
cd ..

cp ./opentelemetry-operations-collector/otelopscol pkgroot/p3/opt/caaos/otelopscol/otelopscol
cp -r etc/. pkgroot/p2/etc
cp -r rootfs/. pkgroot/p3/opt/caaos/otelopscol
tar -czvf otelopscol.tar.gz -C pkgroot .