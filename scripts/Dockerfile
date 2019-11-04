FROM ccr.ccs.tencentyun.com/tkeimages/cni-plugins-amd64:v0.7.1 as CNI-Plugins

FROM ccr.ccs.tencentyun.com/tkeimages/multus-cni:v0.0.5 as Multus

FROM alpine:3.6

WORKDIR /opt/cni/bin/

COPY --from=CNI-Plugins /opt/cni/bin/* ./

COPY --from=Multus /multus ./

COPY scripts/install-cni.sh /

ENTRYPOINT ["/install-cni.sh"]
