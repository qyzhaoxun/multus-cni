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

package types

import (
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
)

// NetConf for cni config file written in json
type NetConf struct {
	types.NetConf

	// support chaining for master interface and IP decisions
	// occurring prior to running ipvlan plugin
	RawPrevResult *map[string]interface{} `json:"prevResult"`
	PrevResult    *current.Result         `json:"-"`

	ConfDir string `json:"confDir"`
	CNIDir  string `json:"cniDir"`
	BinDir  string `json:"binDir"`

	Delegates        []*DelegateNetConf `json:"-"`
	NetStatus        []*NetworkStatus   `json:"-"`
	Kubeconfig       string             `json:"kubeconfig"`
	LogFile          string             `json:"logFile"`
	LogLevel         string             `json:"logLevel"`
	RuntimeConfig    *RuntimeConfig     `json:"runtimeConfig,omitempty"`
	DefaultDelegates string             `json:"defaultDelegates"`
}

// AddDelegates appends the new delegates to the delegates list
func (n *NetConf) AddDelegates(newDelegates []*DelegateNetConf) error {
	n.Delegates = append(n.Delegates, newDelegates...)
	return nil
}

// SetDelegates set the new delegates to the delegates list
func (n *NetConf) SetDelegates(newDelegates []*DelegateNetConf) error {
	n.Delegates = newDelegates
	return nil
}

type RuntimeConfig struct {
	PortMaps []PortMapEntry `json:"portMappings,omitempty"`
}

type PortMapEntry struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIP,omitempty"`
}

type NetworkStatus struct {
	Name      string    `json:"name"`
	Interface string    `json:"interface,omitempty"`
	IPs       []string  `json:"ips,omitempty"`
	Mac       string    `json:"mac,omitempty"`
	Default   bool      `json:"default,omitempty"`
	DNS       types.DNS `json:"dns,omitempty"`
}

type DelegateNetConf struct {
	Conf          types.NetConf
	ConfList      types.NetConfList
	IfnameRequest string `json:"ifnameRequest,omitempty"`
	// MasterPlugin is only used internal housekeeping
	MasterPlugin bool `json:"-"`
	// Conflist plugin is only used internal housekeeping
	ConfListPlugin bool `json:"-"`

	// Raw JSON
	Bytes []byte
}

func (d *DelegateNetConf) String() string {
	if d.ConfListPlugin {
		return fmt.Sprintf("{conf: %#v, ifnameRequest: %s, master: %t}", d.ConfList, d.IfnameRequest, d.MasterPlugin)
	}
	return fmt.Sprintf("{conf: %#v, ifnameRequest: %s, master: %t}", d.Conf, d.IfnameRequest, d.MasterPlugin)
}

// NetworkSelectionElement represents one element of the JSON format
// Network Attachment Selection Annotation as described in section 4.1.2
// of the CRD specification.
type NetworkSelectionElement struct {
	// Name contains the name of the Network object this element selects
	Name string `json:"name"`
	// Namespace contains the optional namespace that the network referenced
	// by Name exists in
	Namespace string `json:"namespace,omitempty"`
	// IPRequest contains an optional requested IP address for this network
	// attachment
	IPRequest string `json:"ipRequest,omitempty"`
	// MacRequest contains an optional requested MAC address for this
	// network attachment
	MacRequest string `json:"macRequest,omitempty"`
	// InterfaceRequest contains an optional requested name for the
	// network interface this attachment will create in the container
	InterfaceRequest string `json:"interfaceRequest,omitempty"`
}

// K8sArgs is the valid CNI_ARGS used for Kubernetes
type K8sArgs struct {
	types.CommonArgs
	IP                         net.IP
	K8S_POD_NAME               types.UnmarshallableString
	K8S_POD_NAMESPACE          types.UnmarshallableString
	K8S_POD_INFRA_CONTAINER_ID types.UnmarshallableString
}
