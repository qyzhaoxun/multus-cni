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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/pkg/errors"

	"github.com/qyzhaoxun/multus-cni/conf"
	k8s "github.com/qyzhaoxun/multus-cni/k8sclient"
	"github.com/qyzhaoxun/multus-cni/logging"
	"github.com/qyzhaoxun/multus-cni/types"
	"github.com/vishvananda/netlink"
)

const (
	IfNamePrefix = "eth"
)

func saveScratchNetConf(containerID, dataDir string, netconf []byte) error {
	logging.Debugf("saveScratchNetConf: %s, %s, %s", containerID, dataDir, string(netconf))
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return logging.Errorf("failed to create the multus data directory(%q): %v", dataDir, err)
	}

	path := filepath.Join(dataDir, containerID)

	err := ioutil.WriteFile(path, netconf, 0600)
	if err != nil {
		return logging.Errorf("failed to write container data in the path(%q): %v", path, err)
	}

	return err
}

func consumeScratchNetConf(containerID, dataDir string) ([]byte, error) {
	logging.Debugf("consumeScratchNetConf: %s, %s", containerID, dataDir)
	path := filepath.Join(dataDir, containerID)
	defer os.Remove(path)

	return ioutil.ReadFile(path)
}

func cleanNetconf(containerID, dataDir string) error {
	path := filepath.Join(dataDir, containerID)
	return os.Remove(path)
}

//func getIfname(delegate *types.DelegateNetConf, argif string, idx int) string {
//	logging.Debugf("getIfname: %v, %s, %d", delegate, argif, idx)
//	if delegate.IfnameRequest != "" {
//		return delegate.IfnameRequest
//	}
//	if delegate.MasterPlugin {
//		// master plugin always uses the CNI-provided interface name
//		return argif
//	}
//
//	// Otherwise construct a unique interface name from the delegate's
//	// position in the delegate list
//	return fmt.Sprintf("eth%d", idx)
//}

func saveDelegates(containerID, dataDir string, delegates []*types.DelegateNetConf) error {
	logging.Debugf("saveDelegates: %s, %s, %v", containerID, dataDir, delegates)
	delegatesBytes, err := json.Marshal(delegates)
	if err != nil {
		return logging.Errorf("error serializing delegate netconf: %v", err)
	}

	if err = saveScratchNetConf(containerID, dataDir, delegatesBytes); err != nil {
		return logging.Errorf("error in saving the  delegates : %v", err)
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

func conflistAdd(rt *libcni.RuntimeConf, rawnetconflist []byte, binDir string, exec invoke.Exec) (cnitypes.Result, error) {
	logging.Debugf("conflistAdd: %v, %s, %s", rt, string(rawnetconflist), binDir)
	// In part, adapted from K8s pkg/kubelet/dockershim/network/cni/cni.go
	binDirs := filepath.SplitList(os.Getenv("CNI_PATH"))
	binDirs = append(binDirs, binDir)
	cniNet := libcni.NewCNIConfig(binDirs, exec)

	confList, err := libcni.ConfListFromBytes(rawnetconflist)
	if err != nil {
		return nil, logging.Errorf("error in converting the raw bytes to conflist: %v", err)
	}

	result, err := cniNet.AddNetworkList(confList, rt)
	if err != nil {
		return nil, logging.Errorf("error in getting result from AddNetworkList: %v", err)
	}

	return result, nil
}

func conflistDel(rt *libcni.RuntimeConf, rawnetconflist []byte, binDir string) error {
	logging.Debugf("conflistDel: %v, %s, %s", rt, string(rawnetconflist), binDir)
	// In part, adapted from K8s pkg/kubelet/dockershim/network/cni/cni.go
	binDirs := []string{binDir}
	cniNet := libcni.CNIConfig{Path: binDirs}

	confList, err := libcni.ConfListFromBytes(rawnetconflist)
	if err != nil {
		return logging.Errorf("error in converting the raw bytes to conflist: %v", err)
	}

	err = cniNet.DelNetworkList(confList, rt)
	if err != nil {
		return logging.Errorf("error in getting result from DelNetworkList: %v", err)
	}

	return err
}

func delegateAdd(exec invoke.Exec, ifName string, delegate *types.DelegateNetConf, rt *libcni.RuntimeConf, binDir string) (cnitypes.Result, error) {
	logging.Debugf("delegateAdd: %v, %s, %v, %v, %s", exec, ifName, delegate, rt, binDir)
	if os.Setenv("CNI_IFNAME", ifName) != nil {
		return nil, logging.Errorf("Multus: error in setting CNI_IFNAME")
	}

	if err := validateIfName(os.Getenv("CNI_NETNS"), ifName); err != nil {
		return nil, logging.Errorf("cannot set %q ifname to %q: %v", delegate.Conf.Type, ifName, err)
	}

	if delegate.ConfListPlugin != false {
		result, err := conflistAdd(rt, delegate.Bytes, binDir, exec)
		if err != nil {
			return nil, logging.Errorf("Multus: error in invoke Conflist add - %q: %v", delegate.ConfList.Name, err)
		}

		return result, nil
	}

	result, err := invoke.DelegateAdd(delegate.Conf.Type, delegate.Bytes, exec)
	if err != nil {
		return nil, logging.Errorf("Multus: error in invoke Delegate add - %q: %v", delegate.Conf.Type, err)
	}

	return result, nil
}

func delegateDel(exec invoke.Exec, ifName string, delegateConf *types.DelegateNetConf, rt *libcni.RuntimeConf, binDir string) error {
	logging.Debugf("delegateDel: %v, %s, %v, %v, %s", exec, ifName, delegateConf, rt, binDir)
	if os.Setenv("CNI_IFNAME", ifName) != nil {
		return logging.Errorf("Multus: error in setting CNI_IFNAME")
	}

	if delegateConf.ConfListPlugin != false {
		err := conflistDel(rt, delegateConf.Bytes, binDir)
		if err != nil {
			return logging.Errorf("Multus: error in invoke Conflist Del - %q: %v", delegateConf.ConfList.Name, err)
		}

		return err
	}

	if err := invoke.DelegateDel(delegateConf.Conf.Type, delegateConf.Bytes, exec); err != nil {
		return logging.Errorf("Multus: error in invoke Delegate del - %q: %v", delegateConf.Conf.Type, err)
	}

	return nil
}

func delPlugins(exec invoke.Exec, argsIfname string, delegates []*types.DelegateNetConf, lastIdx int, rt *libcni.RuntimeConf, binDir string) error {
	logging.Debugf("delPlugins: %v, %s, %v, %d, %v, %s", exec, argsIfname, delegates, lastIdx, rt, binDir)
	if os.Setenv("CNI_COMMAND", "DEL") != nil {
		return logging.Errorf("Multus: error in setting CNI_COMMAND to DEL")
	}

	var errStr string
	for idx := lastIdx; idx >= 0; idx-- {
		ifName := delegates[idx].IfnameRequest
		if ifName == "" {
			ifName = argsIfname
		}
		rt.IfName = ifName
		if err := delegateDel(exec, ifName, delegates[idx], rt, binDir); err != nil {
			logging.Errorf("delPlugins: error %v in del interface %s with delegate %v", err, ifName, *delegates[idx])
			errStr = errStr + fmt.Sprintf("error %v in del interface %s with delegate %v;", err, ifName, *delegates[idx])
		}
	}

	if errStr != "" {
		return errors.New(errStr)
	}
	return nil
}

func cmdAdd(args *skel.CmdArgs, exec invoke.Exec, kubeClient k8s.KubeClient) (cnitypes.Result, error) {
	logging.Debugf("cmdAdd: %v, %v, %v", args, exec, kubeClient)
	n, err := conf.LoadNetConf(args.StdinData)
	if err != nil {
		return nil, logging.Errorf("cmdAdd: err in loading netconf: %v", err)
	}

	k8sArgs, err := k8s.GetK8sArgs(args)
	if err != nil {
		return nil, logging.Errorf("Multus: Err in getting k8s args: %v", err)
	}

	_, kc, err := k8s.TryLoadK8sDelegates(k8sArgs, n, kubeClient)
	if err != nil {
		return nil, logging.Errorf("Multus: Err in loading K8s Delegates k8s args: %v", err)
	}

	// set delegates ifname
	// get delegate which holds args.Ifname
	firstIndex := -1
	ifs := make(map[string]int)
	for i, delegate := range n.Delegates {
		if delegate.IfnameRequest != "" {
			if _, ok := ifs[delegate.IfnameRequest]; ok {
				return nil, logging.Errorf("Multus: Err in set delegates, conflict delegate ifname %s request", delegate.IfnameRequest)
			}
			ifs[delegate.IfnameRequest] = i
		} else {
			// get first empty ifnameRequest index
			if firstIndex == -1 {
				firstIndex = i
			}
		}
	}

	if _, ok := ifs[args.IfName]; !ok {
		if firstIndex == -1 {
			return nil, logging.Errorf("Multus: Err in set delegates : all delegates set specific ifname other than %s for k8s", args.IfName)
		} else {
			n.Delegates[firstIndex].IfnameRequest = args.IfName
			ifs[args.IfName] = firstIndex
		}
	}

	// set master plugin
	mIndex := ifs[args.IfName]
	n.Delegates[mIndex].MasterPlugin = true

	// get ifName lastIdx
	lastIdx := 0
	prefLen := len(IfNamePrefix)
	for ifName := range ifs {
		s := ifName[prefLen:]
		i, err := strconv.Atoi(s)
		if err != nil {
			logging.Debugf("cmdAdd: ignore ifname %s, not started with %s", ifName, IfNamePrefix)
			continue
		}
		if i > lastIdx {
			lastIdx = i
		}
	}

	logging.Debugf("cmdAdd: get last index ifname %s%d", IfNamePrefix, lastIdx)

	// set other ifName index
	for _, delegate := range n.Delegates {
		if delegate.IfnameRequest == "" {
			lastIdx++
			delegate.IfnameRequest = fmt.Sprintf("%s%d", IfNamePrefix, lastIdx)
		}
	}

	// cache the multus config if we have only Multus delegates
	if err := saveDelegates(args.ContainerID, n.CNIDir, n.Delegates); err != nil {
		return nil, logging.Errorf("Multus: Err in saving the delegates: %v", err)
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
				return nil, logging.Errorf("Multus: Err in setting  networks status: %v", err)
			}

			netStatus = append(netStatus, delegateNetStatus)
		}
	}

	if err != nil {
		// Ignore errors; DEL must be idempotent anyway
		_ = delPlugins(exec, "", n.Delegates, idx, rt, n.BinDir)

		// ignore error
		_ = cleanNetconf(args.ContainerID, n.CNIDir)
		return nil, logging.Errorf("Multus: Err in tearing down failed plugins: %v", err)
	}

	//set the network status annotation in apiserver, only in case Multus as kubeconfig
	if n.Kubeconfig != "" && kc != nil {
		err = k8s.SetNetworkStatus(kc, netStatus)
		if err != nil {
			return nil, logging.Errorf("Multus: Err set the networks status: %v", err)
		}
	}

	return result, nil
}

func cmdGet(args *skel.CmdArgs, exec invoke.Exec, kubeClient k8s.KubeClient) (cnitypes.Result, error) {
	logging.Debugf("cmdGet: %v, %v, %v", args, exec, kubeClient)
	in, err := conf.LoadNetConf(args.StdinData)
	if err != nil {
		return nil, err
	}

	// FIXME: call all delegates

	return in.PrevResult, nil
}

func cmdDel(args *skel.CmdArgs, exec invoke.Exec, kubeClient k8s.KubeClient) error {
	logging.Debugf("cmdDel: %v, %v, %v", args, exec, kubeClient)
	in, err := conf.LoadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	k8sArgs, err := k8s.GetK8sArgs(args)
	if err != nil {
		return logging.Errorf("Multus: Err in getting k8s args: %v", err)
	}

	// re-read the scratch multus config if we have only Multus delegates
	netconfBytes, err := consumeScratchNetConf(args.ContainerID, in.CNIDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Per spec should ignore error if resources are missing / already removed
			return nil
		}
		return logging.Errorf("Multus: Err in  reading the delegates: %v", err)
	}

	if err := json.Unmarshal(netconfBytes, &in.Delegates); err != nil {
		return logging.Errorf("Multus: failed to load netconf: %v", err)
	}

	rt, _ := conf.LoadCNIRuntimeConf(args, k8sArgs, "", in.RuntimeConfig)
	return delPlugins(exec, args.IfName, in.Delegates, len(in.Delegates)-1, rt, in.BinDir)
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
