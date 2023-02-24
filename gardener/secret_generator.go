package gardener

import (
	"encoding/base64"
	"fmt"
	"time"

	gardener "cluster.gardener/config/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// constant env kubeconfig for the seed
// could not be only KUBECONFIG because then the normal controller SVC will be overridden
const (
	kubeConfigEnvName = "KUBECONFIG_REMOTE"
)

var secretMeta = metav1.TypeMeta{
	APIVersion: "v1",
	Kind:       "Secret",
}

type Input struct {
	S *gardener.Config
}

type SecretGenerator interface {
	GenerateSecret(input *Input) (*v1.Secret, error)
}

// generate a secret to define declarative a managed ArgoCD Cluster
func GenerateSecret(input *Input) (*v1.Secret, error) {
	// add 60 Seconds concurrency to prevent reconciling gaps
	frequency := input.S.Spec.Frequency.Duration.Seconds() + (time.Duration(60) * time.Second).Seconds()

	returendInfo, err := GetInfo(input.S.Spec.Project, input.S.Spec.Shoot)
	if err != nil {
		return nil, err
	}

	returendData, err := GetConfig(input.S.Spec.Project, input.S.Spec.Shoot, int(frequency), input.S.Spec.DesiredOutput)
	if err != nil {
		return nil, err
	}

	if input.S.Spec.DesiredOutput == "ArgoCD" {

		// build labels if input is not empty
		labels := map[string]string{
			"argocd.argoproj.io/secret-type": "cluster",
			"clustername":                    input.S.Spec.Shoot,
		}

		if input.S.Spec.Stage != "" {
			labels["stage"] = input.S.Spec.Stage
		} else {
			labels["stage"] = returendInfo[0]
		}
		if input.S.Spec.Stage != "" {
			labels["cloudprovider"] = input.S.Spec.CloudProvider
		} else {
			labels["cloudprovider"] = returendInfo[1]
		}

		// caData, clusterAddress, certData, keyData
		argoConfig := fmt.Sprintf(`{"tlsClientConfig": {"caData": "%s", "certData": "%s", "keyData": "%s"}}`, returendData[0], returendData[2], returendData[3])
		byteConfig := []byte(argoConfig)

		byteClusterAddress := []byte(returendData[1])
		byteShoot := []byte(input.S.Spec.Shoot)

		return &v1.Secret{
			TypeMeta: secretMeta,
			ObjectMeta: metav1.ObjectMeta{
				Namespace: input.S.ObjectMeta.Namespace,
				Name:      input.S.Spec.Shoot,
				Labels:    labels,
			},
			Data: map[string][]byte{
				"name":   byteShoot,
				"server": byteClusterAddress,
				"config": byteConfig,
			},
		}, nil
	} else {
		decodedKubeConfig, _ := base64.StdEncoding.DecodeString(returendData[0])
		return &v1.Secret{
			TypeMeta: secretMeta,
			ObjectMeta: metav1.ObjectMeta{
				Namespace: input.S.ObjectMeta.Namespace,
				Name:      fmt.Sprintf("%s-plain", input.S.Spec.Shoot),
			},
			Data: map[string][]byte{
				"kubeconfig": []byte(decodedKubeConfig),
			},
		}, nil
	}
}
