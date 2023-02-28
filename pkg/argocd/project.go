package argocd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	customergardenerv1 "customer.gardener/config/api/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ArgoProject struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`
}

type Metadata struct {
	Annotations map[string]string `json:"annotations"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
}

type ClusterResourceWhitelist struct {
	Group string `json:"group"`
	Kind  string `json:"kind"`
}

type Destinations struct {
	Namespace string `json:"namespace"`
	Server    string `json:"server"`
}
type NamespaceResourceWhitelist struct {
	Group string `json:"group"`
	Kind  string `json:"kind"`
}

type Roles struct {
	Name     string   `json:"name"`
	Policies []string `json:"policies"`
}
type Spec struct {
	ClusterResourceWhitelist   []ClusterResourceWhitelist   `json:"clusterResourceWhitelist"`
	Description                string                       `json:"description"`
	Destinations               []Destinations               `json:"destinations"`
	NamespaceResourceWhitelist []NamespaceResourceWhitelist `json:"namespaceResourceWhitelist"`
	Roles                      []Roles                      `json:"roles"`
	SourceRepos                []string                     `json:"sourceRepos"`
}

type Input struct {
	S *customergardenerv1.Config
}

func DeleteProject(namespace string, projectName string) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	clientset.RESTClient().
		Delete().
		AbsPath(fmt.Sprintf("apis/argoproj.io/v1alpha1/namespaces/%s/appprojects/%s", namespace, projectName)).
		DoRaw(context.TODO())

}

func CreateProject(input *Input, api string) error {
	cid := strings.Split(input.S.Spec.Shoot, "-")[1][0:3]
	project := ArgoCDProject(cid, input.S.ObjectMeta.Namespace, api)

	json, err := json.Marshal(project)
	if err != nil {
		panic(err.Error())
	}
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	_, err = clientset.RESTClient().
		Post().
		AbsPath(fmt.Sprintf("apis/argoproj.io/v1alpha1/namespaces/%s/appprojects", input.S.ObjectMeta.Namespace)).
		Body(json).
		DoRaw(context.TODO())
	return err
}

func ArgoCDProject(cid string, namespace string, api string) ArgoProject {

	return ArgoProject{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "AppProject",
		Metadata: Metadata{
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "0",
			},
			Name:      cid,
			Namespace: namespace,
		},
		Spec: Spec{
			ClusterResourceWhitelist: []ClusterResourceWhitelist{
				{
					Group: "*",
					Kind:  "*",
				},
			},
			Description: fmt.Sprintf("%s customer landscape", cid),
			Destinations: []Destinations{{
				Namespace: "*",
				Server:    api,
			},
			},
			NamespaceResourceWhitelist: []NamespaceResourceWhitelist{
				{
					Group: "*",
					Kind:  "*",
				},
			},
			Roles: []Roles{
				{
					Name: "default",
					Policies: []string{
						fmt.Sprintf("p, proj:%s:default, applications, *, %s/*, allow", cid, cid),
					},
				},
			},
			SourceRepos: []string{
				"*",
			},
		},
	}
}
