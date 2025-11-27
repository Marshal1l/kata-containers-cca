export CC=aarch64-linux-musl-gcc
export GOARCH=arm64 GOARM="" CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc && make
cp -v kata-runtime ~/OPCCA/SFTP_folder/kata-bins/
cp -v containerd-shim-kata-v2 ~/OPCCA/SFTP_folder/kata-bins/
cp -v kata-monitor ~/OPCCA/SFTP_folder/kata-bins/
