#!/usr/bin/env sh

set -e

is_kernel_gt_3_12 () {
    ret1=$(uname --kernel-release)
    echo "kernel release: ${ret1}"
    mav=$(echo ${ret1} | awk -F"[.-]" '{print $1}')
    miv=$(echo ${ret1} | awk -F"[.-]" '{print $2}')
    if [[ "${mav}" -gt 3 ]]; then
        return 0
    elif [[ "${mav}" -eq 3 && "${miv}" -gt 12 ]]; then
        return 0
    fi
    return 1
}

add_replace_file () {
    local sf=$1
    local df=$2
    if [[ -f "${sf}" ]]; then
        if [[ -f "${df}" ]]; then
            if cmp -s ${sf} ${df}; then
                echo "skip replace ${df} with ${sf}, equal file"
                return
            fi
            echo "replace ${df} with ${sf}"
            cp ${sf} ${df}
            return
        fi
        echo "add ${sf}"
        cp ${sf} ${df}
        return
    fi
}

add_replace_cni_configs () {
    local sd=/etc/tke-cni-agent-conf
    for f in $(ls ${sd}); do
        if [[ -f ${sd}/${f} ]]; then
            if [[ "${f}" == "00-multus.conf" ]]; then
                continue
            fi
            add_replace_file ${sd}/${f} ${dst_dir}/${f}
        fi
    done
}

add_replace_cni_plugins () {
    local sd=/opt/cni/bin
    local dd=/host/opt/cni/bin
    for f in $(ls ${sd}); do
        if [[ -f ${sd}/${f} ]]; then
            if [[ "${f}" == "bandwidth_3_12" ]]; then
                if ! is_kernel_gt_3_12; then
                    add_replace_file ${sd}/${f} ${dd}/bandwidth
                fi
                continue
            fi
            if [[ "${f}" == "bandwidth" ]]; then
                if is_kernel_gt_3_12; then
                    add_replace_file ${sd}/${f} ${dd}/bandwidth
                fi
                continue
            fi
            add_replace_file ${sd}/${f} ${dd}/${f}
        fi
    done
}

add_cni_kubeconfig () {
    local ca=$(cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt | base64 | xargs | sed 's/ //g')
    local token=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
    local server=https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT
    local tmpPath=/etc/kubernetes/tke-cni-kubeconfig
    local configPath=/host/etc/kubernetes/tke-cni-kubeconfig
    echo "apiVersion: v1
clusters:
- name: local
  cluster:
    certificate-authority-data: ${ca}
    server: ${server}
contexts:
- name: tke-cni
  context:
    cluster: local
    user: tke-cni
current-context: tke-cni
kind: Config
preferences: {}
users:
- name: tke-cni
  user:
    token: ${token}" > ${tmpPath}
    add_replace_file ${tmpPath} ${configPath}
}

dst_dir=$1
if [[ -z "${dst_dir}" ]]; then
    dst_dir="/host/etc/cni/net.d/multus"
fi

mkdir -p ${dst_dir}

echo "=====Starting install tke-cni-kubeconfig ==========="
add_cni_kubeconfig

echo "=====Starting install multus conf ==========="
add_replace_file /etc/tke-cni-agent-conf/00-multus.conf /host/etc/cni/net.d/00-multus.conf

echo "=====Starting install other cni configs ========="
add_replace_cni_configs

echo "=====Starting installing cni plugins ========="
add_replace_cni_plugins

echo "=====Done==========="

if [ -z ${RESTART_INTERVAL} ]; then
    RESTART_INTERVAL=600
fi

echo "Reinstall tke-cni-kubeconfig with interval ${RESTART_INTERVAL}s"

while sleep ${RESTART_INTERVAL} 
do 
    echo "=====Reinstall tke-cni-kubeconfig ==========="
    add_cni_kubeconfig 
done
