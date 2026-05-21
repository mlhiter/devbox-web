#!/bin/bash

systemctl stop cri-shim
systemctl disable cri-shim

rm -rf /etc/systemd/system/kubelet.service.d/12-cri-shim.conf
rm -rf /etc/systemd/system/cri-shim.service
systemctl daemon-reload
systemctl restart kubelet
rm -rf /usr/bin/cri-shim

hn=$(hostname);
until
  #to lower case
  hn=${hn,,};
  kubectl get node "$hn" >/dev/null 2>&1 &&
  kubectl taint node "$hn" devbox.sealos.io/node- || true
  kubectl label node "$hn" devbox.sealos.io/node- || true
  do sleep 3;
done