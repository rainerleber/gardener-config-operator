package gardener

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// logic for the controller
func (r *DefaultConfigRetriever) GetConfig(project string, shoot string, secondsToExpiration int, output string) (string, string, string, string) {
	newConfig := getClusterConfig(project, shoot, secondsToExpiration)
	if output == "ArgoCD" {
		return yamlParse(newConfig)
	} else {
		return newConfig, "", "", ""
	}
}

type ConfigRetriever interface {
	GetConfig(project string, shoot string, secondsToExpiration int, output string) (string, string, string, string)
}

type DefaultConfigRetriever struct {
}

func NewDefaultConfigRetriever() *DefaultConfigRetriever {
	return &DefaultConfigRetriever{}
}

// constant env kubeconfig for the seed
// could not be only KUBECONFIG because then the normal controller SVC will be overridden
const (
	kubeConfigEnvName = "KUBECONFIG_REMOTE"
)

// Post Body
type ConfigSpec struct {
	ExpirationSeconds int `json:"expirationSeconds"`
}

type GenerateConfig struct {
	ApiVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Spec       ConfigSpec `json:"spec"`
}

// Cluster Response struct
type JsonResponseStatus struct {
	Kubeconfig string `json:"kubeconfig"`
}

type JsonResponse struct {
	Kind       string             `json:"kind"`
	ApiVersion string             `json:"apiVersion"`
	Status     JsonResponseStatus `json:"status"`
}

// Yaml from Response struct
type Cluster struct {
	CaData string `yaml:"certificate-authority-data"`
	Server string `yaml:"server"`
}

type Context struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type User struct {
	ClientCert string `yaml:"client-certificate-data"`
	ClientKey  string `yaml:"client-key-data"`
}

type Clusters struct {
	Name    string  `yaml:"name"`
	Cluster Cluster `yaml:"cluster"`
}

type Contexts struct {
	Context Context `yaml:"context"`
	Name    string  `yaml:"name"`
}

type Users struct {
	Name string `yaml:"name"`
	User User   `yaml:"user"`
}

type KubeConfig struct {
	Clusters       []Clusters `yaml:"clusters"`
	Contexts       []Contexts `yaml:"contexts"`
	Users          []Users    `yaml:"users"`
	CurrentContext string     `yaml:"current-context"`
}

// generate the kubeconfig out of the gardener seed cluster
func getClusterConfig(project string, shoot string, expiration int) string {
	kubeconfig := os.Getenv(kubeConfigEnvName)
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	expire := ConfigSpec{ExpirationSeconds: expiration}
	GeneratedConfig := GenerateConfig{ApiVersion: "authentication.gardener.cloud/v1alpha1", Kind: "AdminKubeconfigRequest", Spec: expire}
	json, err := json.Marshal(GeneratedConfig)
	if err != nil {
		log.Println("Error on response.\n[ERROR] -", err)
	}

	resp, err := clientset.RESTClient().
		Post().
		AbsPath(fmt.Sprintf("apis/core.gardener.cloud/v1beta1/namespaces/garden-%s/shoots/%s/adminkubeconfig", project, shoot)).
		Body(json).
		DoRaw(context.TODO())
	if err != nil {
		fmt.Println(string(resp))
		panic(err.Error())
	}

	data := JsonResponse{}
	yaml.Unmarshal(resp, &data)

	return fmt.Sprintf(data.Status.Kubeconfig)
}

// parse the returned kubeconfig
func yamlParse(encodedYaml string) (caData string, clusterAddress string, certData string, keyData string) {
	sDec, err := base64.StdEncoding.DecodeString(encodedYaml)
	if err != nil {
		panic(err.Error())
	}
	var kubeconfig KubeConfig
	err = yaml.Unmarshal(sDec, &kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	usedContext := kubeconfig.CurrentContext
	for _, e := range kubeconfig.Clusters {
		if e.Name == usedContext {
			caData = fmt.Sprintf(e.Cluster.CaData)
			clusterAddress = fmt.Sprintf(e.Cluster.Server)
		}
	}

	for _, e := range kubeconfig.Users {
		certData = fmt.Sprintf(e.User.ClientCert)
		keyData = fmt.Sprintf(e.User.ClientKey)
	}
	return
}
