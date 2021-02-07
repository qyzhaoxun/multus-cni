ARG cni_tag
ARG arch
ARG BASEIMAGE

FROM ccr.ccs.tencentyun.com/tkeimages/cni-plugins-${arch}:$cni_tag as CNI-Plugins

FROM ccr.ccs.tencentyun.com/tkeimages/cni-plugins-${arch}:v0.7.0-tencent as Tencent-CNI-Plugins

FROM ccr.ccs.tencentyun.com/tkeimages/multus-cni:${arch}-v0.1.0 as Multus

FROM ${BASEIMAGE}

WORKDIR /opt/cni/bin/

COPY --from=CNI-Plugins /opt/cni/bin/* ./

COPY --from=Tencent-CNI-Plugins /opt/cni/bin/bandwidth ./bandwidth_3_12

COPY --from=Multus /multus ./

COPY scripts/install-cni.sh /

ENTRYPOINT ["/install-cni.sh"]
