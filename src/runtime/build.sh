export CC=aarch64-linux-musl-gcc
export GOARCH=arm64 GOARM="" CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc && make
