export CC=aarch64-linux-musl-gcc
export GOARCH=arm64 GOARM="" CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc && make
cp -v kata-runtime ~/opencca-work/SFTP_folder/kata-bins/
cp -v containerd-shim-kata-v2 ~/opencca-work/SFTP_folder/kata-bins/
cp -v kata-monitor ~/opencca-work/SFTP_folder/kata-bins/
