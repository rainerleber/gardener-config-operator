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

	if input.S.Spec.DesiredOutput == "ArgoCD" {
		caData, clusterAddress, certData, keyData := sg.r.GetConfig(input.S.Spec.Project, input.S.Spec.Shoot, int(frequency), input.S.Spec.DesiredOutput)

		argoConfig := fmt.Sprintf(`{"tlsClientConfig": {"caData": %s, "certData": %s, "keyData": %s}}`, caData, certData, keyData)

		byteConfig := []byte(argoConfig)
		byteClusterAddress := []byte(clusterAddress)
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
	} else {
		shootKubeconfig, _, _, _ := sg.r.GetConfig(input.S.Spec.Project, input.S.Spec.Shoot, int(frequency), input.S.Spec.DesiredOutput)
		decodedKubeConfig, _ := base64.StdEncoding.DecodeString(shootKubeconfig)
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
