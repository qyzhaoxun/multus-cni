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
//

package conf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/qyzhaoxun/multus-cni/pkg/logging"
	mtypes "github.com/qyzhaoxun/multus-cni/pkg/types"
	"github.com/qyzhaoxun/multus-cni/pkg/utils"
)

const (
	defaultCNIDir  = "/var/lib/cni/networks/multus"
	defaultConfDir = "/etc/cni/net.d"
	defaultBinDir  = "/opt/cni/bin"
)

func LoadDelegateNetConfList(bytes []byte, delegateConf *mtypes.DelegateNetConf) error {

	logging.Debugf("LoadDelegateNetConfList: %s, %v", string(bytes), delegateConf)
	if err := json.Unmarshal(bytes, &delegateConf.ConfList); err != nil {
		return logging.Errorf("err in unmarshalling delegate conflist: %v", err)
	}

	if delegateConf.ConfList.Plugins == nil {
		return logging.Errorf("delegate must have the 'type'or 'Plugin' field")
	}
	if delegateConf.ConfList.Plugins[0].Type == "" {
		return logging.Errorf("a plugin delegate must have the 'type' field")
	}
	delegateConf.ConfListPlugin = true
	return nil
}

// Convert raw CNI JSON into a DelegateNetConf structure
func LoadDelegateNetConf(bytes []byte, ifnameRequest string) (*mtypes.DelegateNetConf, error) {
	delegateConf := &mtypes.DelegateNetConf{}
	logging.Debugf("LoadDelegateNetConf: %s, %s", string(bytes), ifnameRequest)
	if err := json.Unmarshal(bytes, &delegateConf.Conf); err != nil {
		return nil, logging.Errorf("error in LoadDelegateNetConf - unmarshalling delegate config: %v", err)
	}

	// Do some minimal validation
	if delegateConf.Conf.Type == "" {
		if err := LoadDelegateNetConfList(bytes, delegateConf); err != nil {
			return nil, logging.Errorf("error in LoadDelegateNetConf: %v", err)
		}
	}

	if ifnameRequest != "" {
		delegateConf.IfnameRequest = ifnameRequest
	}

	delegateConf.Bytes = bytes

	return delegateConf, nil
}

func LoadCNIRuntimeConf(args *skel.CmdArgs, k8sArgs *mtypes.K8sArgs, ifName string, rc map[string]interface{}) (*libcni.RuntimeConf, error) {

	logging.Debugf("LoadCNIRuntimeConf: %v, %s, %v", k8sArgs, ifName, rc)
	// In part, adapted from K8s pkg/kubelet/dockershim/network/cni/cni.go#buildCNIRuntimeConf
	// Todo
	// ingress, egress and bandwidth capability features as same as kubelet.
	rt := &libcni.RuntimeConf{
		ContainerID: args.ContainerID,
		NetNS:       args.Netns,
		IfName:      ifName,
		Args: [][2]string{
			{"IgnoreUnknown", "1"},
			{"K8S_POD_NAMESPACE", string(k8sArgs.K8S_POD_NAMESPACE)},
			{"K8S_POD_NAME", string(k8sArgs.K8S_POD_NAME)},
			{"K8S_POD_INFRA_CONTAINER_ID", string(k8sArgs.K8S_POD_INFRA_CONTAINER_ID)},
		},
	}

	if rc != nil {
		rt.CapabilityArgs = rc
	}
	return rt, nil
}

func LoadNetworkStatus(r types.Result, netName string, defaultNet bool) (*mtypes.NetworkStatus, error) {
	logging.Debugf("LoadNetworkStatus: %v, %s, %t", r, netName, defaultNet)

	// Convert whatever the IPAM result was into the current Result type
	result, err := current.NewResultFromResult(r)
	if err != nil {
		return nil, logging.Errorf("error convert the type.Result to current.Result: %v", err)
	}

	netstatus := &mtypes.NetworkStatus{}
	netstatus.Name = netName
	netstatus.Default = defaultNet

	for _, ifs := range result.Interfaces {
		//Only pod interfaces can have sandbox information
		if ifs.Sandbox != "" {
			netstatus.Interface = ifs.Name
			netstatus.Mac = ifs.Mac
		}
	}

	for _, ipconfig := range result.IPs {
		if ipconfig.Version == "4" && ipconfig.Address.IP.To4() != nil {
			netstatus.IPs = append(netstatus.IPs, ipconfig.Address.IP.String())
		}

		if ipconfig.Version == "6" && ipconfig.Address.IP.To16() != nil {
			netstatus.IPs = append(netstatus.IPs, ipconfig.Address.IP.String())
		}
	}

	netstatus.DNS = result.DNS

	return netstatus, nil

}

func LoadNetConf(bytes []byte) (*mtypes.NetConf, error) {
	netconf := &mtypes.NetConf{}

	if err := json.Unmarshal(bytes, netconf); err != nil {
		return nil, logging.Errorf("failed to load netconf: %v", err)
	}

	// Logging
	if netconf.LogFile != "" {
		logging.SetLogFile(netconf.LogFile)
	}
	if netconf.LogLevel != "" {
		logging.SetLogLevel(netconf.LogLevel)
	}

	// Parse previous result
	if netconf.RawPrevResult != nil {
		resultBytes, err := json.Marshal(netconf.RawPrevResult)
		if err != nil {
			return nil, logging.Errorf("could not serialize prevResult: %v", err)
		}
		res, err := version.NewResult(netconf.CNIVersion, resultBytes)
		if err != nil {
			return nil, logging.Errorf("could not parse prevResult: %v", err)
		}
		netconf.RawPrevResult = nil
		netconf.PrevResult, err = current.NewResultFromResult(res)
		if err != nil {
			return nil, logging.Errorf("could not convert result to current version: %v", err)
		}
	}

	if netconf.CNIDir == "" {
		netconf.CNIDir = defaultCNIDir
	}

	if netconf.ConfDir == "" {
		netconf.ConfDir = defaultConfDir
	}

	if netconf.BinDir == "" {
		netconf.BinDir = defaultBinDir
	}

	if netconf.DefaultDelegates != "" {
		delegates, err := getDefaultDelegates(netconf.DefaultDelegates, netconf.ConfDir)
		if err != nil {
			return nil, logging.Errorf("failed to load default delegates from config: %v", err)
		}
		for _, delegate := range delegates {
			netconf.Delegates = append(netconf.Delegates, delegate)
		}
	}

	return netconf, nil
}

func getDefaultDelegates(delegatesAnnot, confdir string) ([]*mtypes.DelegateNetConf, error) {
	networks, err := utils.ParsePodNetworkAnnotation(delegatesAnnot, "")
	if err != nil {
		return nil, err
	}

	// Read all network objects referenced by 'networks'
	var delegates []*mtypes.DelegateNetConf
	for _, net := range networks {
		delegate, err := GetDelegateFromFile(net, confdir)
		if err != nil {
			return nil, logging.Errorf("GetDefaultDelegates: failed getting the delegate: %v", err)
		}
		delegates = append(delegates, delegate)
	}

	return delegates, nil
}

func getCNIConfigFromFile(name string, confdir string) ([]byte, error) {
	logging.Debugf("getCNIConfigFromFile: %s, %s", name, confdir)

	// In the absence of valid keys in a Spec, the runtime (or
	// meta-plugin) should load and execute a CNI .configlist
	// or .config (in that order) file on-disk whose JSON
	// “name” key matches this Network object’s name.

	// In part, adapted from K8s pkg/kubelet/dockershim/network/cni/cni.go#getDefaultCNINetwork
	files, err := libcni.ConfFiles(confdir, []string{".conf", ".json", ".conflist"})
	switch {
	case err != nil:
		return nil, logging.Errorf("No networks found in %s", confdir)
	case len(files) == 0:
		return nil, logging.Errorf("No networks found in %s", confdir)
	}

	for _, confFile := range files {
		var confList *libcni.NetworkConfigList
		if strings.HasSuffix(confFile, ".conflist") {
			confList, err = libcni.ConfListFromFile(confFile)
			if err != nil {
				return nil, logging.Errorf("Error loading CNI conflist file %s: %v", confFile, err)
			}

			if confList.Name == name {
				return confList.Bytes, nil
			}

		} else {
			conf, err := libcni.ConfFromFile(confFile)
			if err != nil {
				return nil, logging.Errorf("Error loading CNI config file %s: %v", confFile, err)
			}

			if conf.Network.Name == name {
				// Ensure the config has a "type" so we know what plugin to run.
				// Also catches the case where somebody put a conflist into a conf file.
				if conf.Network.Type == "" {
					return nil, logging.Errorf("Error loading CNI config file %s: no 'type'; perhaps this is a .conflist?", confFile)
				}
				return conf.Bytes, nil
			}
		}
	}

	return nil, logging.Errorf("no network available in the name %s in cni dir %s", name, confdir)
}

func GetDelegateFromFile(net *mtypes.NetworkSelectionElement, confdir string) (*mtypes.DelegateNetConf, error) {
	logging.Infof("getDelegateFromFile: %+v, %s", net, confdir)
	configBytes, err := getCNIConfigFromFile(net.Name, confdir)
	if err != nil {
		return nil, logging.Errorf("cniConfigFromNetworkResource: err in getCNIConfigFromFile: %v", err)
	}

	delegate, err := LoadDelegateNetConf(configBytes, net.InterfaceRequest)
	if err != nil {
		return nil, err
	}

	return delegate, nil
}

func ConflistAdd(rt *libcni.RuntimeConf, rawnetconflist []byte, binDir string, exec invoke.Exec) (cnitypes.Result, error) {
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

func ConflistDel(rt *libcni.RuntimeConf, rawnetconflist []byte, binDir string) error {
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

func ConfAdd(rt *libcni.RuntimeConf, rawnetconf []byte, binDir string, exec invoke.Exec) (cnitypes.Result, error) {
	logging.Debugf("confAdd: %v, %s, %s", rt, string(rawnetconf), binDir)
	// In part, adapted from K8s pkg/kubelet/dockershim/network/cni/cni.go
	binDirs := filepath.SplitList(os.Getenv("CNI_PATH"))
	binDirs = append(binDirs, binDir)
	cniNet := libcni.NewCNIConfig(binDirs, exec)

	conf, err := libcni.ConfFromBytes(rawnetconf)
	if err != nil {
		return nil, logging.Errorf("error in converting the raw bytes to conf: %v", err)
	}

	result, err := cniNet.AddNetwork(conf, rt)
	if err != nil {
		return nil, logging.Errorf("error in getting result from AddNetwork: %v", err)
	}

	return result, nil
}

func ConfDel(rt *libcni.RuntimeConf, rawnetconf []byte, binDir string) error {
	logging.Debugf("confDel: %v, %s, %s", rt, string(rawnetconf), binDir)
	// In part, adapted from K8s pkg/kubelet/dockershim/network/cni/cni.go
	binDirs := []string{binDir}
	cniNet := libcni.CNIConfig{Path: binDirs}

	conf, err := libcni.ConfFromBytes(rawnetconf)
	if err != nil {
		return logging.Errorf("error in converting the raw bytes to conf: %v", err)
	}

	err = cniNet.DelNetwork(conf, rt)
	if err != nil {
		return logging.Errorf("error in getting result from DelNetwork: %v", err)
	}

	return err
}
