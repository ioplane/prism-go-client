package kubernetes_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	providerk8s "github.com/nutanix-cloud-native/prism-go-client/environment/providers/kubernetes"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	krand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	"github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	"github.com/nutanix-cloud-native/prism-go-client/internal/certutils"
)

const (
	fixtureValueLength  = 10
	fixtureAPIKeyLength = 32
)

func runSecretInformer(ctx context.Context, clientset kubernetes.Interface) coreinformers.SecretInformer {
	informerFactory := informers.NewSharedInformerFactory(clientset, time.Minute)
	secretInformer := informerFactory.Core().V1().Secrets()
	informer := secretInformer.Informer()

	ginkgo.By("spawning secret informer", func() {
		go informer.Run(ctx.Done())
	})
	ginkgo.By("waiting for secret informer cache to be in sync", func() {
		gomega.Expect(
			cache.WaitForCacheSync(ctx.Done(), informer.HasSynced),
		).To(gomega.BeTrue())
	})

	return secretInformer
}

func runCMInformer(ctx context.Context, clientset kubernetes.Interface) coreinformers.ConfigMapInformer {
	informerFactory := informers.NewSharedInformerFactory(clientset, time.Minute)
	cmInformer := informerFactory.Core().V1().ConfigMaps()
	informer := cmInformer.Informer()

	ginkgo.By("spawning cm informer", func() {
		go informer.Run(ctx.Done())
	})
	ginkgo.By("waiting for cm informer cache to be in sync", func() {
		gomega.Expect(
			cache.WaitForCacheSync(ctx.Done(), informer.HasSynced),
		).To(gomega.BeTrue())
	})

	return cmInformer
}

var _ = ginkgo.Describe("Kubernetes Environment Provider", ginkgo.Ordered, func() {
	expectedCACert, expectedB64CACert, err := certutils.GenerateCACertForTesting()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	var (
		cmName         = "user-ca-bundle"
		namespace      = "kube-system"
		secretName     = "nutanix-credentials"
		secretInformer coreinformers.SecretInformer
		cmInformer     coreinformers.ConfigMapInformer
		prov           types.Provider
		endpointHost   = krand.String(fixtureValueLength)
		username       = krand.String(fixtureValueLength)
		password       = krand.String(fixtureValueLength)

		fakeSecret = &corev1.Secret{
			Name:      secretName,
			Namespace: namespace,
			Data: map[string][]byte{
				"credentials": []byte(fmt.Sprintf(`
				[
					{
					  "type": "basic_auth", 
					  "data": { 
						"prismCentral":{
						  "username": "%s", 
						  "password": "%s"
						},
						"prismElements": null
					  }
					}
				]
			`, username, password)),
			},
		}
	)

	const fakeCMTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
binaryData:
  ca.crt: %s`

	fakeCMStr := fmt.Sprintf(fakeCMTemplate, cmName, namespace, expectedB64CACert)
	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder()
	fakeCM := &corev1.ConfigMap{}
	err = runtime.DecodeInto(decoder, []byte(fakeCMStr), fakeCM)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	fakeClientset := fake.NewSimpleClientset(fakeSecret, fakeCM)
	prismEndpoint := credentials.NutanixPrismEndpoint{
		Address:  endpointHost,
		Port:     9440,
		Insecure: true,
		CredentialRef: &credentials.NutanixCredentialReference{
			Kind:      credentials.SecretKind,
			Name:      secretName,
			Namespace: namespace,
		},
		AdditionalTrustBundle: &credentials.NutanixTrustBundleReference{
			Data:      "",
			Kind:      credentials.NutanixTrustBundleKindConfigMap,
			Name:      cmName,
			Namespace: namespace,
		},
	}

	ginkgo.BeforeAll(func() {
		secretInformer = runSecretInformer(context.TODO(), fakeClientset)
		cmInformer = runCMInformer(context.TODO(), fakeClientset)
		prov = providerk8s.NewProvider(prismEndpoint, secretInformer, cmInformer)
	})
	ginkgo.It("rejects missing credential reference before informer access", func() {
		invalid := providerk8s.NewProvider(credentials.NutanixPrismEndpoint{}, nil, nil)
		endpoint, err := invalid.GetManagementEndpoint(nil)
		gomega.Expect(endpoint).To(gomega.BeNil())
		gomega.Expect(err).To(gomega.MatchError("credential reference must not be nil"))
		gomega.Expect(errors.Is(err, types.ErrNotFound)).To(gomega.BeFalse())
	})
	ginkgo.It("must be able to look up secret", func() {
		_, err := fakeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		gomega.Expect(err).To(gomega.Succeed())

		_, err = secretInformer.Lister().Secrets(namespace).Get(secretName)
		gomega.Expect(err).To(gomega.Succeed())

		_, err = fakeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), cmName, metav1.GetOptions{})
		gomega.Expect(err).To(gomega.Succeed())

		_, err = cmInformer.Lister().ConfigMaps(namespace).Get(cmName)
		gomega.Expect(err).To(gomega.Succeed())
	})
	ginkgo.It("keeps malformed endpoint diagnostics safe", func() {
		const marker = "synthetic-kubernetes-url-secret-marker"
		for _, address := range []string{"fixture:" + marker + "@%41.invalid", "%41.invalid?token=" + marker} {
			invalidEndpoint := prismEndpoint
			invalidEndpoint.Address = address
			invalid := providerk8s.NewProvider(invalidEndpoint, secretInformer, cmInformer)
			endpoint, err := invalid.GetManagementEndpoint(nil)
			gomega.Expect(endpoint).To(gomega.BeNil())
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).NotTo(gomega.ContainSubstring(marker))

			var cause *url.Error
			gomega.Expect(errors.As(err, &cause)).To(gomega.BeTrue())
			gomega.Expect(cause.URL).To(gomega.Equal("https://" + address + ":9440"))
			gomega.Expect(cause.Op).To(gomega.Equal("parse"))
		}
	})
	ginkgo.It("must get management endpoint", func() {
		me, err := prov.GetManagementEndpoint(nil)
		gomega.Expect(err).To(gomega.Succeed())

		addr := &url.URL{Scheme: "https", Host: endpointHost + ":9440"}
		gomega.Expect(me).To(gomega.BeEquivalentTo(&types.ManagementEndpoint{
			Username:              username,
			Password:              password,
			Insecure:              true,
			Address:               addr,
			AdditionalTrustBundle: expectedCACert,
		}))
	})
})

var _ = ginkgo.Describe("Kubernetes Environment Provider with API key credentials", ginkgo.Ordered, func() {
	var (
		namespace      = "kube-system"
		secretName     = "nutanix-api-key-credentials"
		secretInformer coreinformers.SecretInformer
		cmInformer     coreinformers.ConfigMapInformer
		prov           types.Provider
		endpointHost   = krand.String(fixtureValueLength)
		apiKey         = krand.String(fixtureAPIKeyLength)

		fakeSecret = &corev1.Secret{
			Name:      secretName,
			Namespace: namespace,
			Data: map[string][]byte{
				"credentials": []byte(fmt.Sprintf(`
				[
					{
					  "type": "api_key",
					  "data": {
						"prismCentral":{
						  "apiKey": "%s"
						}
					  }
					}
				]
			`, apiKey)),
			},
		}
	)

	fakeClientset := fake.NewSimpleClientset(fakeSecret)
	prismEndpoint := credentials.NutanixPrismEndpoint{
		AdditionalTrustBundle: nil,
		Address:               endpointHost,
		Port:                  9440,
		Insecure:              true,
		CredentialRef: &credentials.NutanixCredentialReference{
			Kind:      credentials.SecretKind,
			Name:      secretName,
			Namespace: namespace,
		},
	}

	ginkgo.BeforeAll(func() {
		secretInformer = runSecretInformer(context.TODO(), fakeClientset)
		cmInformer = runCMInformer(context.TODO(), fakeClientset)
		prov = providerk8s.NewProvider(prismEndpoint, secretInformer, cmInformer)
	})
	ginkgo.It("must get management endpoint with api key credentials", func() {
		me, err := prov.GetManagementEndpoint(nil)
		gomega.Expect(err).To(gomega.Succeed())

		addr := &url.URL{Scheme: "https", Host: endpointHost + ":9440"}
		gomega.Expect(me).To(gomega.BeEquivalentTo(&types.ManagementEndpoint{
			APIKey:   apiKey,
			Insecure: true,
			Address:  addr,
		}))
	})
})

func TestSuite(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Kubernetes Environment Provider Suite")
}
