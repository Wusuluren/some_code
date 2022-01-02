package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/emicklei/go-restful/v3"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	printers2 "k8s.io/cli-runtime/pkg/printers"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/printers"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
)

var getHandlerMap map[string]kubectlHandler

func (k *Kubectl) getHandler(resource string) kubectlHandler {
	if getHandlerMap == nil {
		getHandlerMap = map[string]kubectlHandler{
			"deploy":     k.getDeployment,
			"deployment": k.getDeployment,
		}
	}
	return getHandlerMap[resource]
}

func (k *Kubectl) Get(request *restful.Request, response *restful.Response) {
	resource := request.PathParameter("resource")
	name := request.PathParameter("name")

	namespace := AnyQueryParameter(request, "namespace", "n")
	if namespace == "" {
		namespace = apiv1.NamespaceDefault
	}

	output := AnyQueryParameter(request, "output", "o")

	option := &KubectlOption{
		Namespace: namespace,
		Output:    output,
	}
	klog.InfoS("Get", "resource", resource, "name", name, "option", option)

	handler := k.getHandler(resource)
	if handler == nil {
		err := fmt.Errorf("the server doesn't have a resource type %q", resource)
		HandleRespWithError(response, err)
		return
	}

	handler(request, response, option)

	response.Write([]byte("\n"))
}

func (k *Kubectl) getDeployment(request *restful.Request, response *restful.Response, option *KubectlOption) {
	clientset := k.clientset
	TableConvertor := printers.NewTableGenerator().With(printersinternal.AddHandlers)

	deploymentsClient := clientset.AppsV1().Deployments(option.Namespace)
	list, err := deploymentsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		HandleRespWithError(response, err)
		return
	}
	if option.Output == "json" {

		popt := printers2.PrintOptions{
			NoHeaders:        false,
			WithNamespace:    false,
			WithKind:         false,
			Wide:             true,
			ShowLabels:       false,
			Kind:             schema.GroupKind{},
			ColumnLabels:     nil,
			SortBy:           "",
			AllowMissingKeys: false,
		}
		err := printers2.NewTablePrinter(popt).PrintObj(list, response.ResponseWriter)
		if err != nil {
			HandleRespWithError(response, err)
			return
		}
		return

		list2 := new(apps.DeploymentList)
		listbyte, _ := json.Marshal(list)
		json.Unmarshal(listbyte, &list2)
		table, err := TableConvertor.GenerateTable(list2, printers.GenerateOptions{})
		if err != nil {
			HandleRespWithError(response, err)
			return
		}
		b, _ := json.Marshal(table)
		response.Write(b)
		return

		codec := legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Group: v1.GroupName, Version: "v1"})
		for _, d := range list.Items {
			b, err := runtime.Encode(codec, &d)
			if err != nil {
				HandleRespWithError(response, err)
				return
			}
			response.Write(b)
		}
	} else if option.Output == "yaml" {
		b, err := yaml.Marshal(list.Items)
		if err != nil {
			HandleRespWithError(response, err)
			return
		}
		response.Write(b)
	} else {
		table := NewTableWriter(response.ResponseWriter)
		table.SetHeader([]string{"NAME", "READY", "UP-TO-DATE", "AVAILABLE", "AGE"})
		for _, d := range list.Items {
			table.Append([]string{
				d.Name,
				fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, d.Status.Replicas),
				fmt.Sprintf("%d", d.Status.UpdatedReplicas),
				fmt.Sprintf("%d", d.Status.AvailableReplicas),
				fmt.Sprintf("%dm", d.CreationTimestamp.Minute()),
			})
		}
		table.Render() // Send output
		//response.Write(respBuf.Bytes())
	}
	response.Write([]byte("\n"))
}
