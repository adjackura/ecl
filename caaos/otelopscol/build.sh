set -ex

cd caaos/otelopscol
mkdir -p pkgroot/p3
mkdir -p pkgroot/p4/opt/caaos/otelopscol

git clone https://github.com/adjackura/opentelemetry-operations-collector.git
cd opentelemetry-operations-collector
#git -c advice.detachedHead=false checkout d2c97eb
CGO_ENABLED=0 go build -ldflags '-s -w' -o otelopscol ./cmd/otelopscol 
cd ..

cp ./opentelemetry-operations-collector/otelopscol pkgroot/p4/opt/caaos/otelopscol/otelopscol
cp -r etc/. pkgroot/p3/etc
cp -r rootfs/. pkgroot/p4/opt/caaos/otelopscol
mkdir -p /workspace/packages
tar -czvf /workspace/packages/otelopscol.tar.gz -C pkgroot .