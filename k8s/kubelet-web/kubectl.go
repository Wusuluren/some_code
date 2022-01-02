package main

import (
	"flag"
	"path/filepath"

	"github.com/emicklei/go-restful/v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type kubectlHandler func(request *restful.Request, response *restful.Response, option *KubectlOption)

type KubectlOption struct {
	Namespace string
	Output    string
}

func InitKubeClient() *kubernetes.Clientset {

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	return clientset
}

type Kubectl struct {
	clientset *kubernetes.Clientset
}

func (k *Kubectl) Init() {
	k.clientset = InitKubeClient()
}
