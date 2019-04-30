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


echo "=====Starting install cni conf ==========="
add_replace_files /etc/tke-cni-agent-conf /host/etc/cni/net.d

echo "=====Starting installing cni ========="
add_replace_files /opt/cni/bin /host/opt/cni/bin

echo "=====Done==========="

while sleep 3600; do :; done
