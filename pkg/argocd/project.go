package argocd

import (
	"fmt"
	"strings"

	customergardenerv1 "customer.gardener/config/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
type ArgoProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `yaml:"metadata,omitempty"`
	Spec              Spec `yaml:"spec"`
}

// +kubebuilder:object:generate=true
type ClusterResourceWhitelist struct {
	Group string `yaml:"group"`
	Kind  string `yaml:"kind"`
}

// +kubebuilder:object:generate=true
type Destinations struct {
	Namespace string `yaml:"namespace"`
	Server    string `yaml:"server"`
}

// +kubebuilder:object:generate=true
type NamespaceResourceWhitelist struct {
	Group string `yaml:"group"`
	Kind  string `yaml:"kind"`
}

// +kubebuilder:object:generate=true
type Roles struct {
	Name     string   `yaml:"name"`
	Policies []string `yaml:"policies"`
}

// +kubebuilder:object:generate=true
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

func ArgoCDProject(input *Input, api string) *ArgoProject {

	cid := strings.Split(input.S.Spec.Shoot, "-")[1][0:3]

	return &ArgoProject{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "AppProject",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: input.S.ObjectMeta.Namespace,
			Name:      cid,
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-wave": "0",
			},
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
