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

package k8sclient

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"

	"github.com/qyzhaoxun/multus-cni/pkg/conf"
	"github.com/qyzhaoxun/multus-cni/pkg/logging"
	"github.com/qyzhaoxun/multus-cni/pkg/types"
	"github.com/qyzhaoxun/multus-cni/pkg/utils"
)

const (
	GroupName                   = "tke.cloud.tencent.com"
	CNINetworksAnnotation       = "tke.cloud.tencent.com/networks"
	CNINetworksStatusAnnotation = "tke.cloud.tencent.com/networks-status"
)

// NoK8sNetworkError indicates error, no network in kubernetes
type NoK8sNetworkError struct {
	message string
}

type clientInfo struct {
	Client       KubeClient
	Podnamespace string
	Podname      string
}

func (e *NoK8sNetworkError) Error() string { return string(e.message) }

type defaultKubeClient struct {
	client kubernetes.Interface
}

// defaultKubeClient implements KubeClient
var _ KubeClient = &defaultKubeClient{}

func (d *defaultKubeClient) GetPod(namespace, name string) (*v1.Pod, error) {
	return d.client.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
}

func (d *defaultKubeClient) UpdatePodStatus(pod *v1.Pod) (*v1.Pod, error) {
	return d.client.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
}

func setKubeClientInfo(c *clientInfo, client KubeClient, k8sArgs *types.K8sArgs) {
	c.Client = client
	c.Podnamespace = string(k8sArgs.K8S_POD_NAMESPACE)
	c.Podname = string(k8sArgs.K8S_POD_NAME)
}

func SetNetworkStatus(k *clientInfo, netStatus []*types.NetworkStatus) error {
	pod, err := k.Client.GetPod(k.Podnamespace, k.Podname)
	if err != nil {
		return logging.Errorf("SetNetworkStatus: failed to query the pod %s in out of cluster comm: %v", k.Podname, err)
	}

	var ns string
	if netStatus != nil {
		var networkStatus []string
		for _, nets := range netStatus {
			data, err := json.MarshalIndent(nets, "", "    ")
			if err != nil {
				return logging.Errorf("SetNetworkStatus: error with Marshal Indent: %v", err)
			}
			networkStatus = append(networkStatus, string(data))
		}

		ns = fmt.Sprintf("[%s]", strings.Join(networkStatus, ","))
	}
	_, err = setPodNetworkAnnotation(k.Client, k.Podnamespace, pod, ns)
	if err != nil {
		return logging.Errorf("SetNetworkStatus: failed to update the pod %s in out of cluster comm: %v", k.Podname, err)
	}

	return nil
}

func setPodNetworkAnnotation(client KubeClient, namespace string, pod *v1.Pod, networkstatus string) (*v1.Pod, error) {
	logging.Infof("setPodNetworkAnnotation: %s/%s, %s", namespace, pod.Name, networkstatus)
	//if pod annotations is empty, make sure it allocatable
	if len(pod.Annotations) == 0 {
		pod.Annotations = make(map[string]string)
	}

	pod.Annotations[CNINetworksStatusAnnotation] = networkstatus

	pod = pod.DeepCopy()
	var err error
	if resultErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err != nil {
			// Re-get the pod unless it's the first attempt to update
			pod, err = client.GetPod(pod.Namespace, pod.Name)
			if err != nil {
				return err
			}
		}

		pod, err = client.UpdatePodStatus(pod)
		return err
	}); resultErr != nil {
		return nil, logging.Errorf("status update failed for pod %s/%s: %v", pod.Namespace, pod.Name, resultErr)
	}
	return pod, nil
}

func getPodNetworkAnnotation(client KubeClient, k8sArgs *types.K8sArgs) (string, string, error) {
	var err error

	pod, err := client.GetPod(string(k8sArgs.K8S_POD_NAMESPACE), string(k8sArgs.K8S_POD_NAME))
	if err != nil {
		return "", "", logging.Errorf("getPodNetworkAnnotation: failed to query the pod %v in out of cluster comm: %v", string(k8sArgs.K8S_POD_NAME), err)
	}

	logging.Infof("getPodNetworkAnnotation: %s/%s, %s", pod.Namespace, pod.Name, pod.Annotations[CNINetworksAnnotation])

	return pod.Annotations[CNINetworksAnnotation], pod.ObjectMeta.Namespace, nil
}

func getKubernetesDelegate(client KubeClient, net *types.NetworkSelectionElement, confdir string) (*types.DelegateNetConf, error) {
	logging.Debugf("getKubernetesDelegate: %+v, %s", net, confdir)
	delegate, err := conf.GetDelegateFromFile(net, confdir)
	if err != nil {
		return nil, err
	}

	return delegate, nil
}

type KubeClient interface {
	GetPod(namespace, name string) (*v1.Pod, error)
	UpdatePodStatus(pod *v1.Pod) (*v1.Pod, error)
}

func GetK8sArgs(args *skel.CmdArgs) (*types.K8sArgs, error) {
	k8sArgs := &types.K8sArgs{}

	logging.Debugf("GetK8sArgs: %s", args.Args)
	err := cnitypes.LoadArgs(args.Args, k8sArgs)
	if err != nil {
		return nil, err
	}

	return k8sArgs, nil
}

// Attempts to load Kubernetes-defined delegates and add them to the Multus config.
// Returns the number of Kubernetes-defined delegates added or an error.
func TryLoadK8sDelegates(k8sArgs *types.K8sArgs, conf *types.NetConf, kubeClient KubeClient) (int, *clientInfo, error) {
	var err error
	clientInfo := &clientInfo{}

	logging.Debugf("TryLoadK8sDelegates: %v, %v, %v", k8sArgs, conf, kubeClient)
	kubeClient, err = GetK8sClient(conf.Kubeconfig, kubeClient)
	if err != nil {
		return 0, nil, err
	}

	if kubeClient == nil {
		if len(conf.Delegates) == 0 {
			// No available kube client and no delegates, we can't do anything
			return 0, nil, logging.Errorf("must have either Kubernetes config or delegates, refer Multus README.md for the usage guide")
		}
		return 0, nil, nil
	}

	setKubeClientInfo(clientInfo, kubeClient, k8sArgs)
	delegates, err := GetK8sNetwork(kubeClient, k8sArgs, conf.ConfDir)
	if err != nil {
		if _, ok := err.(*NoK8sNetworkError); ok {
			return 0, clientInfo, nil
		}
		return 0, nil, logging.Errorf("TryLoadK8sDelegates: Err in getting k8s network from pod: %v", err)
	}

	if err = conf.SetDelegates(delegates); err != nil {
		return 0, nil, err
	}

	conf.Delegates[0].MasterPlugin = true

	return len(delegates), clientInfo, nil
}

func GetK8sClient(kubeconfig string, kubeClient KubeClient) (KubeClient, error) {
	// If we get a valid kubeClient (eg from testcases) just return that
	// one.
	if kubeClient != nil {
		return kubeClient, nil
	}

	var err error
	var config *rest.Config

	// Otherwise try to create a kubeClient from a given kubeConfig
	if kubeconfig != "" {
		// uses the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, logging.Errorf("GetK8sClient: failed to get context for the kubeconfig %v, refer Multus README.md for the usage guide: %v", kubeconfig, err)
		}
	} else if os.Getenv("KUBERNETES_SERVICE_HOST") != "" && os.Getenv("KUBERNETES_SERVICE_PORT") != "" {
		// Try in-cluster config where multus might be running in a kubernetes pod
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, logging.Errorf("createK8sClient: failed to get context for in-cluster kube config, refer Multus README.md for the usage guide: %v", err)
		}
	} else {
		// No kubernetes config; assume we shouldn't talk to Kube at all
		return nil, nil
	}

	// creates the clientset
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &defaultKubeClient{client: client}, nil
}

func GetK8sNetwork(k8sclient KubeClient, k8sArgs *types.K8sArgs, confdir string) ([]*types.DelegateNetConf, error) {
	logging.Debugf("GetK8sNetwork: %v, %v, %v", k8sclient, k8sArgs, confdir)

	netAnnot, defaultNamespace, err := getPodNetworkAnnotation(k8sclient, k8sArgs)
	if err != nil {
		return nil, err
	}

	if len(netAnnot) == 0 {
		return nil, &NoK8sNetworkError{"no kubernetes network found"}
	}

	networks, err := utils.ParsePodNetworkAnnotation(netAnnot, defaultNamespace)
	if err != nil {
		return nil, err
	}

	// Read all network objects referenced by 'networks'
	var delegates []*types.DelegateNetConf
	for _, net := range networks {
		delegate, err := getKubernetesDelegate(k8sclient, net, confdir)
		if err != nil {
			return nil, logging.Errorf("GetK8sNetwork: failed getting the delegate: %v", err)
		}
		delegates = append(delegates, delegate)
	}

	return delegates, nil
}
