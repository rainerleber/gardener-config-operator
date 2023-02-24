package gardener

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func GetInfo(project string, shoot string) ([]string, error) {
	purposeRaw, provider, err := getInfo(project, shoot)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("something went wrong getting the shoot cluster info, check if cluster %s exsists", shoot))
	}
	purpose := purposeShort(purposeRaw)
	return []string{purpose, provider}, nil
}

type Spec struct {
	Provider Provider `json:"provider"`
	Purpose  string   `json:"purpose"`
}

type Provider struct {
	Type string `json:"type"`
}

type InfoJsonResponse struct {
	Spec Spec `json:"spec"`
}

func purposeShort(purpose string) string {
	if purpose == "production" {
		return "prod"
	} else {
		return "dev"
	}
}

func getInfo(project string, shoot string) (string, string, error) {
	kubeconfig := os.Getenv(kubeConfigEnvName)
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return "", "", fmt.Errorf("error on response.\n%s -", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", "", fmt.Errorf("error on clientset.\n%s -", err)
	}

	resp, err := clientset.RESTClient().
		Get().
		AbsPath(fmt.Sprintf("apis/core.gardener.cloud/v1beta1/namespaces/garden-%s/shoots/%s", project, shoot)).
		DoRaw(context.TODO())
	if err != nil {
		return "", "", fmt.Errorf("error on clientset.\n%s -", err)
	}

	data := InfoJsonResponse{}
	json.Unmarshal(resp, &data)

	return data.Spec.Purpose, data.Spec.Provider.Type, nil
}
