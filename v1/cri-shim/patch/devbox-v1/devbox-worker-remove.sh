#!/bin/bash

systemctl stop cri-shim >/dev/null 2>&1
systemctl disable cri-shim >/dev/null 2>&1

rm -rf /etc/systemd/system/kubelet.service.d/12-cri-shim.conf
rm -rf /etc/systemd/system/cri-shim.service
rm -rf /usr/bin/cri-shim
systemctl daemon-reload