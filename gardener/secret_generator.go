package gardener

import (
	"encoding/base64"
	"fmt"

	gardener "cluster.gardener/config/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type DefaultSecretGenerator struct {
	r ConfigRetriever
}

func NewDefaultSecretGenerator(r ConfigRetriever) *DefaultSecretGenerator {
	return &DefaultSecretGenerator{
		r: r,
	}
}

// generate a secret to define declarative a managed ArgoCD Cluster
func (sg *DefaultSecretGenerator) GenerateSecret(input *Input) (*v1.Secret, error) {
	frequency := input.S.Spec.Frequency.Duration.Seconds()
	returendData := sg.r.GetConfig(input.S.Spec.Project, input.S.Spec.Shoot, int(frequency), input.S.Spec.DesiredOutput)

	if input.S.Spec.DesiredOutput == "ArgoCD" && len(returendData) > 0 {
		// caData, clusterAddress, certData, keyData
		argoConfig := fmt.Sprintf(`{"tlsClientConfig": {"caData": %s, "certData": %s, "keyData": %s}}`, returendData[0], returendData[2], returendData[3])

		byteConfig := []byte(argoConfig)
		byteClusterAddress := []byte(returendData[1])
		byteShoot := []byte(input.S.Spec.Shoot)

		return &v1.Secret{
			TypeMeta: secretMeta,
			ObjectMeta: metav1.ObjectMeta{
				Namespace: input.S.ObjectMeta.Namespace,
				Name:      input.S.Spec.Shoot,
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
				},
			},
			Data: map[string][]byte{
				"name":   byteShoot,
				"server": byteClusterAddress,
				"config": byteConfig,
			},
		}, nil
	} else if input.S.Spec.DesiredOutput == "Plain" && len(returendData) > 0 {
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
	} else {
		return nil, fmt.Errorf("something went wrong getting the shoot cluster config, check if cluster name exsists")
	}
}
