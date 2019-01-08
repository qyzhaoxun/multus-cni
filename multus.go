// Copyright (c) 2017 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This is a "Multi-plugin".The delegate concept refered from CNI project
// It reads other plugin netconf, and then invoke them, e.g.
// flannel or sriov plugin.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"

	"github.com/qyzhaoxun/multus-cni/pkg/backend"
	"github.com/qyzhaoxun/multus-cni/pkg/conf"
	k8s "github.com/qyzhaoxun/multus-cni/pkg/k8sclient"
	"github.com/qyzhaoxun/multus-cni/pkg/logging"
	"github.com/qyzhaoxun/multus-cni/pkg/types"
)

const (
	IfNamePrefix = "eth"
)

func saveDelegates(containerID string, delegates []*types.DelegateNetConf, store backend.CNIStore) error {
	delegatesBytes, err := json.Marshal(delegates)
	if err != nil {
		return logging.Errorf("error serializing delegate netconf: %v", err)
	}

	if err = store.Save(containerID, delegatesBytes); err != nil {
		return logging.Errorf("error in saving delegates : %v", err)
	}

	return err
}

func validateIfName(nsname string, ifname string) error {
	logging.Debugf("validateIfName: %s, %s", nsname, ifname)
	podNs, err := ns.GetNS(nsname)
	if err != nil {
		return logging.Errorf("no netns: %v", err)
	}

	err = podNs.Do(func(_ ns.NetNS) error {
		_, err := netlink.LinkByName(ifname)
		if err != nil {
			if err.Error() == "Link not found" {
				return nil
			}
			return err
		}
		return logging.Errorf("ifname %s is already exist", ifname)
	})

	return err
}

func delegateAdd(exec invoke.Exec, ifName string, delegate *types.DelegateNetConf, rt *libcni.RuntimeConf, binDir string) (cnitypes.Result, error) {
	logging.Debugf("delegateAdd: %v, %s, %s, %v, %s", exec, ifName, delegate, rt, binDir)
	if os.Setenv("CNI_IFNAME", ifName) != nil {
		return nil, logging.Errorf("delegateAdd: error in setting CNI_IFNAME")
	}

	if err := validateIfName(os.Getenv("CNI_NETNS"), ifName); err != nil {
		return nil, logging.Errorf("delegateAdd: cannot set %q ifname to %q: %v", delegate.Conf.Type, ifName, err)
	}

	if delegate.ConfListPlugin != false {
		result, err := conf.ConflistAdd(rt, delegate.Bytes, binDir, exec)
		if err != nil {
			return nil, logging.Errorf("delegateAdd: error in invoke Conflist add - %q: %v", delegate.ConfList.Name, err)
		}

		return result, nil
	}

	result, err := invoke.DelegateAdd(delegate.Conf.Type, delegate.Bytes, exec)
	if err != nil {
		return nil, logging.Errorf("delegateAdd: error in invoke Delegate add - %q: %v", delegate.Conf.Type, err)
	}

	return result, nil
}

func delegateDel(exec invoke.Exec, ifName string, delegateConf *types.DelegateNetConf, rt *libcni.RuntimeConf, binDir string) error {
	logging.Debugf("delegateDel: %v, %s, %s, %v, %s", exec, ifName, delegateConf, rt, binDir)
	if os.Setenv("CNI_IFNAME", ifName) != nil {
		return logging.Errorf("delegateDel: error in setting CNI_IFNAME")
	}

	if delegateConf.ConfListPlugin != false {
		err := conf.ConflistDel(rt, delegateConf.Bytes, binDir)
		if err != nil {
			return logging.Errorf("delegateDel: error in invoke Conflist Del - %q: %v", delegateConf.ConfList.Name, err)
		}

		return err
	}

	if err := invoke.DelegateDel(delegateConf.Conf.Type, delegateConf.Bytes, exec); err != nil {
		return logging.Errorf("delegateDel: error in invoke Delegate del - %q: %v", delegateConf.Conf.Type, err)
	}

	return nil
}

func delPlugins(exec invoke.Exec, delegates []*types.DelegateNetConf, lastIdx int, rt *libcni.RuntimeConf, binDir string) ([]*types.DelegateNetConf, error) {
	logging.Debugf("delPlugins: %v, %d", exec, lastIdx)
	if os.Setenv("CNI_COMMAND", "DEL") != nil {
		return delegates, logging.Errorf("delPlugins: error in setting CNI_COMMAND to DEL")
	}

	var errstr []string
	var eDelegates []*types.DelegateNetConf
	for idx := lastIdx; idx >= 0; idx-- {
		ifName := delegates[idx].IfnameRequest
		rt.IfName = ifName
		if err := delegateDel(exec, ifName, delegates[idx], rt, binDir); err != nil {
			errstr = append(errstr, err.Error())
			eDelegates = append([]*types.DelegateNetConf{delegates[idx]}, eDelegates...)
		}
	}

	if len(eDelegates) > 0 {
		return eDelegates, fmt.Errorf(strings.Join(errstr, ";"))
	}

	return nil, nil
}

func setDelegatesIfname(delegates []*types.DelegateNetConf, argsIfname string) error {
	// set delegates ifname
	// get delegate which holds args.Ifname
	firstIndex := -1
	ifs := make(map[string]int)
	for i, delegate := range delegates {
		if delegate.IfnameRequest != "" {
			if _, ok := ifs[delegate.IfnameRequest]; ok {
				return logging.Errorf("Failed to set delegates ifname, conflict ifname %s request", delegate.IfnameRequest)
			}
			ifs[delegate.IfnameRequest] = i
		} else {
			// get first empty ifnameRequest index
			if firstIndex == -1 {
				firstIndex = i
			}
		}
	}

	if _, ok := ifs[argsIfname]; !ok {
		if firstIndex == -1 {
			return logging.Errorf("Failed to set delegates ifname, all delegates set specific ifname other than %s for k8s", argsIfname)
		} else {
			delegates[firstIndex].IfnameRequest = argsIfname
			ifs[argsIfname] = firstIndex
		}
	}

	// set master plugin
	mIndex := ifs[argsIfname]
	delegates[mIndex].MasterPlugin = true

	// get ifName lastIdx
	lastIdx := 0
	prefLen := len(IfNamePrefix)
	for ifName := range ifs {
		s := ifName[prefLen:]
		i, err := strconv.Atoi(s)
		if err != nil {
			logging.Infof("Ignore ifname %s, not started with %s", ifName, IfNamePrefix)
			continue
		}
		if i > lastIdx {
			lastIdx = i
		}
	}

	logging.Infof("Get last index ifname %s%d", IfNamePrefix, lastIdx)

	// set other ifName index
	for _, delegate := range delegates {
		if delegate.IfnameRequest == "" {
			lastIdx++
			delegate.IfnameRequest = fmt.Sprintf("%s%d", IfNamePrefix, lastIdx)
		}
	}
	return nil
}

func cmdAdd(args *skel.CmdArgs, exec invoke.Exec, kubeClient k8s.KubeClient) (cnitypes.Result, error) {
	logging.Infof("cmdAdd: {containerId %s, netNs %s, ifName %s, args %s, path %s, stdinData %s}, %v, %v",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path, string(args.StdinData), exec, kubeClient)

	n, err := conf.LoadNetConf(args.StdinData)
	if err != nil {
		return nil, logging.Errorf("cmdAdd: err in loading netconf: %v", err)
	}

	k8sArgs, err := k8s.GetK8sArgs(args)
	if err != nil {
		return nil, logging.Errorf("cmdAdd: Err in getting k8s args: %v", err)
	}

	_, kc, err := k8s.TryLoadK8sDelegates(k8sArgs, n, kubeClient)
	if err != nil {
		return nil, logging.Errorf("cmdAdd: Err in loading K8s Delegates k8s args: %v", err)
	}

	err = setDelegatesIfname(n.Delegates, args.IfName)
	if err != nil {
		return nil, err
	}

	store, err := backend.NewStore(n.CNIDir)
	if err != nil {
		return nil, logging.Errorf("cmdAdd: Err in new store: %v", err)
	}

	// cache the multus config if we have only Multus delegates
	if err := saveDelegates(args.ContainerID, n.Delegates, store); err != nil {
		return nil, logging.Errorf("cmdAdd: Err in saving the delegates: %v", err)
	}

	var result, tmpResult cnitypes.Result
	var netStatus []*types.NetworkStatus
	var rt *libcni.RuntimeConf

	var delegate *types.DelegateNetConf
	var idx int
	for idx, delegate = range n.Delegates {
		rt, _ = conf.LoadCNIRuntimeConf(args, k8sArgs, delegate.IfnameRequest, n.RuntimeConfig)
		tmpResult, err = delegateAdd(exec, delegate.IfnameRequest, delegate, rt, n.BinDir)
		if err != nil {
			logging.Errorf("cmdAdd: Err in %d delegate exec cni add", idx)
			break
		}

		// Master plugin result is always used if present
		if delegate.MasterPlugin || result == nil {
			result = tmpResult
		}

		//create the network status, only in case Multus as kubeconfig
		if n.Kubeconfig != "" && kc != nil {
			delegateNetStatus, err := conf.LoadNetworkStatus(tmpResult, delegate.Conf.Name, delegate.MasterPlugin)
			if err != nil {
				return nil, logging.Errorf("cmdAdd: Err in setting networks status: %v", err)
			}

			netStatus = append(netStatus, delegateNetStatus)
		}
	}

	if err != nil {
		// Ignore errors; DEL must be idempotent anyway
		_, err1 := delPlugins(exec, n.Delegates, idx, rt, n.BinDir)
		if err1 != nil {
			// TODO cache the multus config if we have only Multus delegates, kubelet would not retry cmd del
			//if err2 := saveDelegates(args.ContainerID, n.Delegates[:rIdx+1], store); err2 != nil {
			//	logging.Errorf("cmdAdd: Err in saving failed delegates: %v", err2)
			//}
			logging.Errorf("cmdAdd: Err in tearing down failed plugins: %v", err1)
		}

		// ignore error
		err3 := store.Remove(args.ContainerID)
		if err3 != nil {
			logging.Errorf("cmdAdd: Err in clean net conf: %v", err3)
		}
		return nil, logging.Errorf("cmdAdd: Err in setup plugins: %v", err)
	}

	//set the network status annotation in apiserver, only in case Multus as kubeconfig
	if n.Kubeconfig != "" && kc != nil {
		err = k8s.SetNetworkStatus(kc, netStatus)
		if err != nil {
			// ignore error
			logging.Errorf("cmdAdd: Err set the networks status: %v", err)
		}
	}

	return result, nil
}

func cmdGet(args *skel.CmdArgs, exec invoke.Exec, kubeClient k8s.KubeClient) (cnitypes.Result, error) {
	logging.Infof("cmdGet: {containerId %s, netNs %s, ifName %s, args %s, path %s, stdinData %s}, %v, %v",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path, string(args.StdinData), exec, kubeClient)

	in, err := conf.LoadNetConf(args.StdinData)
	if err != nil {
		return nil, err
	}

	// FIXME: call all delegates

	return in.PrevResult, nil
}

func cmdDel(args *skel.CmdArgs, exec invoke.Exec, kubeClient k8s.KubeClient) error {
	logging.Infof("cmdDel: {containerId %s, netNs %s, ifName %s, args %s, path %s, stdinData %s}, %v, %v",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path, string(args.StdinData), exec, kubeClient)

	n, err := conf.LoadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	k8sArgs, err := k8s.GetK8sArgs(args)
	if err != nil {
		return logging.Errorf("cmdDel: Err in getting k8s args: %v", err)
	}

	store, err := backend.NewStore(n.CNIDir)
	if err != nil {
		return logging.Errorf("cmdDel: Err in new store: %v", err)
	}

	// re-read the scratch multus config if we have only Multus delegates
	netconfBytes, err := store.Load(args.ContainerID)
	if err != nil {
		if os.IsNotExist(err) {
			// Per spec should ignore error if resources are missing / already removed
			return nil
		}
		return logging.Errorf("cmdDel: Err in reading the delegates: %v", err)
	}

	if err := json.Unmarshal(netconfBytes, &n.Delegates); err != nil {
		return logging.Errorf("cmdDel: failed to load netconf: %v", err)
	}

	rt, _ := conf.LoadCNIRuntimeConf(args, k8sArgs, "", n.RuntimeConfig)

	// Ignore errors; DEL must be idempotent anyway
	eDelegates, err := delPlugins(exec, n.Delegates, len(n.Delegates)-1, rt, n.BinDir)
	if err != nil {
		// cache the multus config, kubelet wil retry cmdDel
		if err1 := saveDelegates(args.ContainerID, eDelegates, store); err1 != nil {
			// ignore error
			logging.Errorf("cmdDel: Err in saving failed delegates: %v", err1)
		}
		return logging.Errorf("cmdDel: Err in tearing down plugins: %v", err)
	}

	// ignore error
	err = store.Remove(args.ContainerID)
	if err != nil {
		logging.Errorf("cmdDel: Err in clean net conf: %v", err)
	}

	return nil
}

func main() {
	skel.PluginMain(
		func(args *skel.CmdArgs) error {
			result, err := cmdAdd(args, nil, nil)
			if err != nil {
				return err
			}
			return result.Print()
		},
		func(args *skel.CmdArgs) error {
			result, err := cmdGet(args, nil, nil)
			if err != nil {
				return err
			}
			return result.Print()
		},
		func(args *skel.CmdArgs) error { return cmdDel(args, nil, nil) },
		version.All, "meta-plugin that delegates to other CNI plugins")
}
