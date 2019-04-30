#!/usr/bin/env sh

set -e

add_replace_files () {
    local src_dir=$1
    local dst_dir=$2
    for f in $(ls ${src_dir}); do
        if [[ -f ${src_dir}/${f} ]]; then
            if [[ -f ${dst_dir}/${f} ]]; then
                if cmp -s ${src_dir}/${f} ${dst_dir}/${f}; then
                    echo "skip replace equal file ${f}"
                    continue
                fi
                echo "replace file ${f}"
                cp ${src_dir}/${f} ${dst_dir}/${f}
                continue
            fi
            echo "add file ${f}"
            cp ${src_dir}/${f} ${dst_dir}/${f}
            continue
        fi
    done
}

add_replace_file () {
    local src_dir=$1
    local dst_dir=$2
    local f=$3
    if [[ -f ${dst_dir}/${f} ]]; then
        if cmp -s ${src_dir}/${f} ${dst_dir}/${f}; then
            echo "skip replace equal file ${f}"
            return
        fi
    fi
    cp ${src_dir}/${f} ${dst_dir}/${f}
}

dst_dir=$1
if [[ -z "${dst_dir}" ]]; then
    dst_dir="/host/etc/cni/net.d/multus"
fi


mkdir -p ${dst_dir}

echo "=====Starting install multus conf ==========="
add_replace_file /etc/tke-cni-agent-conf ${dst_dir} 00-multus.conf

echo "=====Starting install other cni conf"
add_replace_files /etc/tke-cni-agent-conf ${dst_dir}

echo "=====Starting installing cni ========="
add_replace_files /opt/cni/bin /host/opt/cni/bin

echo "=====Done==========="

while sleep 3600; do :; done
