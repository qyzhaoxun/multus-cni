### default delegates options

1. multi cni in json format

```
  '[
            { "name": "flannel-conf" },
            { "name": "sriov-conf" },
            { "name": "sriov-vlanid-l2enable-conf",
              "interfaceRequest": "north" }
    ]'
```

2. multi cni names

```
  'tke-bridge,tke-eni-cni'
```

3. multi cni names with interface name

```
  'tke-bridge,tke-eni-cni@net0'
```

4. single cni name

```
    'tke-bridge'
```