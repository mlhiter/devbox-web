#!/bin/bash
TAINT=${1:-"true"}
larch=$(arch)
if [[ $larch == "x86_64" ]]; then
  larch="amd64"
elif [[ $larch == "aarch64" ]]; then
  larch="arm64"
fi
systemctl stop cri-shim.service >/dev/null 2>&1
cp bin/cri-shim_linux-"${larch}" /usr/bin/cri-shim
registryDevboxConfigPwd=$(kubectl get cm -n sealos-system registry-config -o jsonpath='{.data.ADMIN_PASSWORD}')
registryDevboxConfigUsr=$(kubectl get cm -n sealos-system registry-config -o jsonpath='{.data.ADMIN_USER}')
registryDevboxConfigAddr=$(kubectl get cm -n sealos-system registry-config -o jsonpath='{.data.REGISTRY_ADDR}')
containerdRoot=$(containerd config dump | awk -F' = ' '/^root *=/ {gsub(/"/,"",$2); print $2; exit}')
cat << EOF > /etc/systemd/system/cri-shim.service
[Unit]
Description=sealos cri shim

[Service]
ExecStart= /bin/bash -c 'mkdir -p /var/run/sealos  && /usr/bin/cri-shim server --containerd-root=${containerdRoot} --cri-socket=unix:///var/run/containerd/containerd.sock --shim-socket=/var/run/sealos/cri-shim.sock --global-registry-addr=${registryDevboxConfigAddr}  --global-registry-user=${registryDevboxConfigUsr} --global-registry-password=${registryDevboxConfigPwd}'
ExecStop=rm /var/run/sealos/cri-shim.sock
Restart=always
StartLimitInterval=0
RestartSec=10
LimitNOFILE=1048576
# Having non-zero Limit*s causes performance problems due to accounting overhead
# in the kernel. We recommend using cgroups to do container-local accounting.
LimitNPROC=infinity
LimitCORE=infinity
LimitNOFILE=1048576
# Comment TasksMax if your systemd version does not supports it.
# Only systemd 226 and above support this version.
TasksMax=infinity
[Install]
WantedBy=multi-user.target

EOF
cp -rf ./etc/12-cri-shim.conf /etc/systemd/system/kubelet.service.d/
cp -rf ./etc/10-kubeadm.conf /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
chmod a+x /usr/bin/cri-shim
systemctl enable cri-shim.service
systemctl daemon-reload
systemctl restart cri-shim.service
systemctl restart kubelet
echo "init cri shim success"


hn=$(hostname);
until
  #to lower case
  hn=${hn,,};
  kubectl get node "$hn" >/dev/null 2>&1 &&
  if [ "$TAINT" == "true" ]; then
    kubectl taint --overwrite node "$hn"  devbox.sealos.io/node=:NoSchedule
  else
    kubectl taint node "$hn" devbox.sealos.io/node- || true
  fi
  kubectl label --overwrite node "$hn" devbox.sealos.io/node= ;
  do sleep 3;
done
