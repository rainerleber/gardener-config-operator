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
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Annotations map[string]string `yaml:"annotations"`
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
}

type ClusterResourceWhitelist struct {
	Group string `yaml:"group"`
	Kind  string `yaml:"kind"`
}

type Destinations struct {
	Namespace string `yaml:"namespace"`
	Server    string `yaml:"server"`
}
type NamespaceResourceWhitelist struct {
	Group string `yaml:"group"`
	Kind  string `yaml:"kind"`
}

type Roles struct {
	Name     string   `yaml:"name"`
	Policies []string `yaml:"policies"`
}
type Spec struct {
	ClusterResourceWhitelist   []ClusterResourceWhitelist   `yaml:"clusterResourceWhitelist"`
	Description                string                       `yaml:"description"`
	Destinations               []Destinations               `yaml:"destinations"`
	NamespaceResourceWhitelist []NamespaceResourceWhitelist `yaml:"namespaceResourceWhitelist"`
	Roles                      []Roles                      `yaml:"roles"`
	SourceRepos                []string                     `yaml:"sourceRepos"`
}

type Input struct {
	S *customergardenerv1.Config
}

func CreateProject(project ArgoProject, namespace string) string {

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
	resp, _ := clientset.RESTClient().
		Post().
		AbsPath(fmt.Sprintf("apis/argoproj.io/v1alpha1/namespaces/%s/appprojects", namespace)).
		Body(json).
		DoRaw(context.TODO())
	return string(resp)
}

func ArgoCDProject(input *Input, api string) ArgoProject {

	cid := strings.Split(input.S.Spec.Shoot, "-")[1][0:3]

	return ArgoProject{
		APIVersion: "argoproj.io/v1alpha1",
		Kind:       "AppProject",
		Metadata: Metadata{
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "0",
			},
			Name:      cid,
			Namespace: input.S.ObjectMeta.Namespace,
		},
		Spec: Spec{
			ClusterResourceWhitelist: []ClusterResourceWhitelist{
				{
					Group: "*",
					Kind:  "*",
				},
			},
			Description: fmt.Sprintf("%s customer landscape", input.S.Spec.Shoot),
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
