package consul

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/api/admissionregistration/v1beta1"

	"github.com/pkg/errors"
	kubemeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/solo-io/supergloo/pkg/api/v1"
)

const (
	CrbName          = "consul-crb"
	defaultNamespace = "consul"
	WebhookCfg       = "consul-connect-injector-cfg"
)

type ConsulInstaller struct{}

func (c *ConsulInstaller) GetDefaultNamespace() string {
	return defaultNamespace
}

func (c *ConsulInstaller) GetCrbName() string {
	return CrbName
}

func (c *ConsulInstaller) GetOverridesYaml(install *v1.Install) string {
	return getOverrides(install.Encryption)
}

func (c *ConsulInstaller) DoPreHelmInstall() error {
	return nil
}

func (c *ConsulInstaller) DoPostHelmInstall(install *v1.Install, kube *kubernetes.Clientset, releaseName string) error {
	if install.Encryption.TlsEnabled {
		err := updateMutatingWebhookAdapter(kube, releaseName)
		if err != nil {
			return errors.Wrap(err, "Error setting up webhook")
		}
	}
	return nil
}

func getOverrides(encryption *v1.Encryption) string {
	updatedOverrides := overridesYaml
	if encryption != nil {
		strBool := strconv.FormatBool(encryption.TlsEnabled)
		updatedOverrides = strings.Replace(overridesYaml, "@@MTLS_ENABLED@@", strBool, -1)
	}
	return updatedOverrides
}

var overridesYaml = `
global:
  # Change this to specify a version of consul.
  # soloio/consul:latest was just published to provide a 1.4 container
  # consul:1.3.0 is the latest container on docker hub from consul
  image: "soloio/consul:latest"
  imageK8S: "hashicorp/consul-k8s:0.2.1"

server:
  replicas: 1
  bootstrapExpect: 1
  connect: @@MTLS_ENABLED@@
  disruptionBudget:
    enabled: false
    maxUnavailable: null

connectInject:
  enabled: @@MTLS_ENABLED@@
`

// The webhook config is created with the wrong name by the chart
// Grab it, recreate with correct name, and delete the old one
func updateMutatingWebhookAdapter(kube *kubernetes.Clientset, releaseName string) error {
	name := fmt.Sprintf("%s-%s", releaseName, WebhookCfg)
	cfg, err := kube.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Get(name, kubemeta.GetOptions{})
	if err != nil {
		return err
	}
	fixedCfg := getFixedWebhookAdapter(cfg)
	_, err = kube.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Create(fixedCfg)
	if err != nil {
		return err
	}
	err = kube.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Delete(name, &kubemeta.DeleteOptions{})
	return err
}

func getFixedWebhookAdapter(input *v1beta1.MutatingWebhookConfiguration) *v1beta1.MutatingWebhookConfiguration {
	fixed := input.DeepCopy()
	fixed.Name = WebhookCfg
	fixed.ResourceVersion = ""
	return fixed
}
