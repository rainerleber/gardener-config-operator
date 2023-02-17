package gardener

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// logic for the controller
func (r *DefaultConfigRetriever) GetConfig(project string, shoot string, secondsToExpiration int, output string) ([]string, error) {
	if checkShootClusterAvailability(project, shoot) {
		newConfig := getClusterConfig(project, shoot, secondsToExpiration)
		if output == "ArgoCD" {
			parsed := yamlParse(newConfig)
			return parsed, nil
		} else {
			return []string{newConfig}, nil
		}
	} else {
		return nil, fmt.Errorf(fmt.Sprintf("something went wrong getting the shoot cluster config, check if cluster %s exsists", shoot))
	}
}

type ConfigRetriever interface {
	GetConfig(project string, shoot string, secondsToExpiration int, output string) ([]string, error)
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

func checkShootClusterAvailability(project string, shoot string) bool {
	kubeconfig := os.Getenv(kubeConfigEnvName)
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Println("Error on response.\n[ERROR] -", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println("Error on response.\n[ERROR] -", err)
	}

	secrets, err := clientset.CoreV1().Secrets(fmt.Sprintf("garden-%s", project)).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Println("Error on response.\n[ERROR] -", err)
	}
	for _, secret := range secrets.Items {
		if strings.Contains(secret.Name, shoot) {
			return true
		}
	}
	return false
}

// generate the kubeconfig out of the gardener seed cluster
func getClusterConfig(project string, shoot string, expiration int) string {
	kubeconfig := os.Getenv(kubeConfigEnvName)
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Println("Error in the current context.\n[ERROR] -", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println("Error on clientset.\n[ERROR] -", err)
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
		fmt.Println(string(resp), err)
	}

	data := JsonResponse{}
	yaml.Unmarshal(resp, &data)

	return fmt.Sprintf(data.Status.Kubeconfig)
}

// parse the returned kubeconfig
func yamlParse(encodedYaml string) []string {
	sDec, err := base64.StdEncoding.DecodeString(encodedYaml)
	if err != nil {
		log.Println("Error on YAML Encode.\n[ERROR] -", err)
	}
	var kubeconfig KubeConfig
	err = yaml.Unmarshal(sDec, &kubeconfig)
	if err != nil {
		log.Println("Error on YAML Unmarshaling.\n[ERROR] -", err)
	}

	usedContext := kubeconfig.CurrentContext
	var caData string
	var clusterAddress string
	var certData string
	var keyData string
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
	return []string{caData, clusterAddress, certData, keyData}
}
