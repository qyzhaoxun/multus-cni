# MULTUS CNI 插件
- Multus 是 meta cni，该 cni 使 k8s 集群支持多种 cni 插件，并且一个 pod 中可以设置多个网络接口。
- Multus 调用其他 cni（bridge,tke-eni-cni）来完成容器的网络配置。
- Multus 复用了 flannel 中 CNI 委托的处理方式，根据 pod annotation 中指定的 cni 序列，依次读取对应的 cni 配置文件，执行相应的 cni 命令。
- Multus 委托调用的 cni 如果没有指定请求的网卡名称，则 Multus 会为该 cni 生成网卡名称，例如 "eth1"，"eth2"，"ethX" 等等，kubelet 中的默认网卡对应的 cni 为主 cni。

可以查看详细了解 [CNI](https://github.com/containernetworking/cni)

# 指引

Multus 可以以 Daemonset 的方式部署在 k8s 集群, 本例使用 tke-bridge 做示例。

首先部署委托 cni tke-bridge 的 Daemonset。

```
$ kubectl create -f https://raw.githubusercontent.com/qyzhaoxun/tke-bridge-agent/master/deploy/v0.0.3/tke-bridge-agent.yaml
```

接着部署 multus-cni 的 Daemonset。

```
$ kubectl create -f https://raw.githubusercontent.com/qyzhaoxun/multus-cni/master/deploy/v0.0.1/multus.yaml
```


然后部署一个 pod，该 pod 的 annotation 中指定使用该 tke-bridge。

```
cat <<EOF | kubectl create -f -
apiVersion: v1
kind: Pod
metadata:
  name: samplepod
  annotations:
    tke.cloud.tencent.com/networks: tke-bridge
spec:
  containers:
  - name: samplepod
    command: ["/bin/bash", "-c", "sleep 2000000000000"]
    image: busybox
EOF
```

通过如下命令查看该 pod 的网络接口配置：

```
$ kubectl exec -it samplepod -- ip a
```


## 构建

通过 docker 构建

```
$ make docker-build
```

## 工作流
<p align="center">
   <img src="doc/images/workflow.png" width="1008" />
</p>

## Multus 配置参数

- name (string, required): cni 名称
- type (string, required): &quot;multus&quot;
- kubeconfig (string, optional): Multus 使用该配置和 kube-apiserver 通信。查看示例 [kubeconfig](https://github.com/qyzhaoxun/multus-cni/blob/master/doc/node-kubeconfig.yaml)
- defaultDelegates (string,optional): 默认的委托 cni 配置。如果 pod 没有指定 annotation，Multus 会使用该 cni 配置。查看示例 [defaultDelegates](https://github.com/qyzhaoxun/multus-cni/blob/master/doc/default-delegates.md)

### 配置 kubeconfig

1. 在 Kubernetes node 创建如下 cni 配置文件：/etc/cni/net.d/multus-cni.conf。kubeconfig 文件应该使用绝对路径。CNI 二进制的默认路径我们认为是 (`/opt/cni/bin dir`) CNI 配置文件的默认路径我们认为是 (`/etc/cni/net.d dir`)

```
{
    "name": "node-cni-network",
    "type": "multus",
    "kubeconfig": "/root/.kube/config"
}
```

### 配置 kubeconfig 和默认委托 cni

1. 下面的配置中, 如果 pod 没有指定 annotation，tke-bridge 将充当默认 cni

```
{
    "name": "node-cni-network",
    "type": "multus",
    "kubeconfig": "/root/.kube/config",
    "defaultDelegates": "tke-bridge"
}
```

### 配置 Pod 使用多个 cni

1. 将下面配置保存为文件 pod-multi-network.yaml。 下面的配置中 flannel-conf 对应的网卡是 eth0 为主网卡。集群中需要部署 cni：flannel-conf，sriov-conf，sriov-vlanid-l2enable-conf
```
# cat pod-multi-network.yaml
apiVersion: v1
kind: Pod
metadata:
  name: multus-multi-net-poc
  annotations:
    tke.cloud.tencent.com/networks: '[
            { "name": "flannel-conf" },
            { "name": "sriov-conf" },
            { "name": "sriov-vlanid-l2enable-conf",
              "interfaceRequest": "north" }
    ]'
spec:  # specification of the pod's contents
  containers:
  - name: multus-multi-net-poc
    image: "busybox"
    command: ["top"]
    stdin: true
    tty: true
```

2. 创建 pod

```
# kubectl create -f ./pod-multi-network.yaml
pod "multus-multi-net-poc" created
```

3. 查看 pod

```
# kubectl get pods
NAME                   READY     STATUS    RESTARTS   AGE
multus-multi-net-poc   1/1       Running   0          30s
```

### 查看 pod 网卡信息

1. Run `ifconfig` command in Pod:

```
# kubectl exec -it multus-multi-net-poc -- ifconfig
lo        Link encap:Local Loopback
          inet addr:127.0.0.1  Mask:255.0.0.0
          inet6 addr: ::1/128 Scope:Host
          UP LOOPBACK RUNNING  MTU:65536  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:0 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1
          RX bytes:0 (0.0 B)  TX bytes:0 (0.0 B)

eth0      Link encap:Ethernet  HWaddr 06:21:91:2D:74:B9
          inet addr:192.168.42.3  Bcast:0.0.0.0  Mask:255.255.255.0
          inet6 addr: fe80::421:91ff:fe2d:74b9/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1450  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:8 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:0 (0.0 B)  TX bytes:648 (648.0 B)

eth1      Link encap:Ethernet  HWaddr D2:94:98:82:00:00
          inet addr:10.56.217.171  Bcast:0.0.0.0  Mask:255.255.255.0
          inet6 addr: fe80::d094:98ff:fe82:0/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:2 errors:0 dropped:0 overruns:0 frame:0
          TX packets:8 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000
          RX bytes:120 (120.0 B)  TX bytes:648 (648.0 B)

north     Link encap:Ethernet  HWaddr BE:F2:48:42:83:12
          inet6 addr: fe80::bcf2:48ff:fe42:8312/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:1420 errors:0 dropped:0 overruns:0 frame:0
          TX packets:1276 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000
          RX bytes:95956 (93.7 KiB)  TX bytes:82200 (80.2 KiB)
```

| Interface name | Description |
| --- | --- |
| lo | loopback |
| eth0 | Flannel tap 网卡 |
| eth1 | SR-IOV CNI 插件设置的网卡 [Intel - SR-IOV CNI](https://github.com/intel/sriov-cni)  |
| north | SR-IOV CNI 插件设置的网卡 |

2. 检查 north 网卡的 vlan ID

```
# ip link show enp2s0
20: enp2s0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 24:8a:07:e8:7d:40 brd ff:ff:ff:ff:ff:ff
    vf 0 MAC 00:00:00:00:00:00, vlan 210, spoof checking off, link-state auto
    vf 1 MAC 00:00:00:00:00:00, vlan 4095, spoof checking off, link-state auto
    vf 2 MAC 00:00:00:00:00:00, vlan 4095, spoof checking off, link-state auto
    vf 3 MAC 00:00:00:00:00:00, vlan 4095, spoof checking off, link-state auto
```

## 日志选项

Multus 会将日志输出到 `STDERR`, 该方法是 CNI 插件输出错误的标准方法，这些错误会输出到 kubelet 的日志中。

### 将日志输出到文件

设置 Multus 的日志文件：

```
    "LogFile": "/var/log/multus.log",
```

### 设置日志登记

默认的日志等级是 `info`

有下列日志等级：

* `debug`
* `info`
* `error`
* `panic`

设置 Multus 的日志等级：

```
    "LogLevel": "debug",
```

## 测试 Multus CNI

### 多 flannel 网络

Github 用户 [YYGCui](https://github.com/YYGCui) 使用了 Multus 跑多 flannel 网络。具体可查看 [closed issue](https://github.com/intel/multus-cni/issues/7) 。

确保 multus, [sriov](https://github.com/Intel-Corp/sriov-cni), [flannel](https://github.com/containernetworking/cni/blob/master/Documentation/flannel.md), and [ptp](https://github.com/containernetworking/cni/blob/master/Documentation/ptp.md) 二进制在 /opt/cni/bin 文件夹，配置在 /etc/cni/net.d 文件夹。

#### 配置 Kubernetes 使用 CNI

Kubelet 需要配置使用 CNI 网络插件。编辑 `/etc/kubernetes/kubelet` 文件，添加如下 `--network-plugin=cni`，`KUBELET\_OPTS `参数:

```
KUBELET_OPTS="...
--network-plugin-dir=/etc/cni/net.d
--network-plugin=cni
"
```

详细设置可以参考以下链接：
- [Single Node](https://kubernetes.io/docs/getting-started-guides/fedora/fedora_manual_config/)
- [Multi Node](https://kubernetes.io/docs/getting-started-guides/fedora/flannel_multi_node_cluster/)
- [Network plugin](https://kubernetes.io/docs/admin/network-plugins/)

#### 创建 Kubernetes workload

1. Create `multus-test.yaml` file containing below configuration. Created pod will consist of one `busybox` container running `top` command.

```
apiVersion: v1
kind: Pod
metadata:
  name: multus-test
spec:  # specification of the pod's contents
  restartPolicy: Never
  containers:
  - name: test1
    image: "busybox"
    command: ["top"]
    stdin: true
    tty: true

```

2. 创建 pod:

```
# kubectl create -f multus-test.yaml
pod "multus-test" created
```

3. 在容器中运行 &quot;ip link&quot; 命令:

```
# 1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue qlen 1
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
3: eth0@if41: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue
    link/ether 26:52:6b:d8:44:2d brd ff:ff:ff:ff:ff:ff
20: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq qlen 1000
    link/ether f6:fb:21:4f:1d:63 brd ff:ff:ff:ff:ff:ff
21: eth2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq qlen 1000
    link/ether 76:13:b1:60:00:00 brd ff:ff:ff:ff:ff:ff
```

| Interface name | Description |
| --- | --- |
| lo | loopback |
| eth0@if41 | Flannel tap 网卡 |
| eth1 | SR-IOV 设置为容器的 VF |
| eth2 | ptp 本地网卡 |

