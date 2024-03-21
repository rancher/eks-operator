package e2e

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kubectl "github.com/rancher-sandbox/ele-testhelpers/kubectl"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	e2eConfig "github.com/rancher/eks-operator/test/e2e/config"
	managementv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	runtimeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(managementv3.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(eksv1.AddToScheme(clientgoscheme.Scheme))
}

const (
	operatorDeploymentName    = "eks-config-operator"
	operatorReleaseName       = "rancher-eks-operator"
	operatorCrdReleaseName    = "rancher-eks-operator-crd"
	certManagerNamespace      = "cert-manager"
	certManagerName           = "cert-manager"
	certManagerCAInjectorName = "cert-manager-cainjector"
	awsCredentialsSecretName  = "aws-credentials"
	cattleSystemNamespace     = "cattle-system"
	rancherName               = "rancher"
	eksClusterConfigNamespace = "cattle-global-data"
)

// Test configuration
var (
	e2eCfg   *e2eConfig.E2EConfig
	cl       runtimeclient.Client
	ctx      = context.Background()
	crdNames = []string{
		"eksclusterconfigs.eks.cattle.io",
	}

	pollInterval = 10 * time.Second
	waitLong     = 25 * time.Minute
)

// Cluster Templates
var (
	//go:embed templates/*
	templates embed.FS

	clusterTemplates         = map[string]*eksv1.EKSClusterConfig{}
	basicClusterTemplateName = "basic-cluster"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "eks-operator e2e test Suite")
}

var _ = BeforeSuite(func() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		Fail("config path can't be empty")
	}

	var err error
	e2eCfg, err = e2eConfig.ReadE2EConfig(configPath)
	Expect(err).ToNot(HaveOccurred())

	cfg, err := runtimeconfig.GetConfig()
	Expect(err).ToNot(HaveOccurred())

	cl, err = runtimeclient.New(cfg, runtimeclient.Options{})
	Expect(err).ToNot(HaveOccurred())

	By("Deploying rancher and cert-manager", func() {
		By("Installing cert-manager", func() {
			if isDeploymentReady(certManagerNamespace, certManagerName) {
				By("already installed")
			} else {
				Expect(kubectl.RunHelmBinaryWithCustomErr(
					"-n",
					certManagerNamespace,
					"install",
					"--set",
					"installCRDs=true",
					"--create-namespace",
					certManagerNamespace,
					e2eCfg.CertManagerChartURL,
				)).To(Succeed())
				Eventually(func() bool {
					return isDeploymentReady(certManagerNamespace, certManagerName)
				}, 5*time.Minute, 2*time.Second).Should(BeTrue())
				Eventually(func() bool {
					return isDeploymentReady(certManagerNamespace, certManagerCAInjectorName)
				}, 5*time.Minute, 2*time.Second).Should(BeTrue())
			}
		})

		By("Adding rancher helm chart repository", func() {
			Expect(kubectl.RunHelmBinaryWithCustomErr(
				"repo",
				"add",
				"--force-update",
				"rancher-latest",
				fmt.Sprintf(e2eCfg.RancherChartURL),
			)).To(Succeed())
		})

		By("Update helm repositories", func() {
			Expect(kubectl.RunHelmBinaryWithCustomErr(
				"repo",
				"update",
			)).To(Succeed())
		})

		By("Installing rancher", func() {
			if isDeploymentReady(cattleSystemNamespace, rancherName) {
				By("already installed")
			} else {
				Expect(kubectl.RunHelmBinaryWithCustomErr(
					"-n",
					cattleSystemNamespace,
					"install",
					"--set",
					"bootstrapPassword=admin",
					"--set",
					"replicas=1",
					"--set",
					"extraEnv[0].name=CATTLE_SKIP_HOSTED_CLUSTER_CHART_INSTALLATION",
					"--set-string",
					"extraEnv[0].value=true",
					"--set", fmt.Sprintf("hostname=%s.%s", e2eCfg.ExternalIP, e2eCfg.MagicDNS),
					"--create-namespace",
					"--devel",
					"--set", fmt.Sprintf("rancherImageTag=%s", e2eCfg.RancherVersion),
					rancherName,
					"rancher-latest/rancher",
				)).To(Succeed())
				Eventually(func() bool {
					return isDeploymentReady(cattleSystemNamespace, rancherName)
				}, 5*time.Minute, 2*time.Second).Should(BeTrue())
			}
		})
	})

	By("Deploying eks operator CRD chart", func() {
		if isDeploymentReady(cattleSystemNamespace, operatorCrdReleaseName) {
			By("already installed")
		} else {
			Expect(kubectl.RunHelmBinaryWithCustomErr(
				"-n",
				cattleSystemNamespace,
				"install",
				"--create-namespace",
				"--set", "debug=true",
				operatorCrdReleaseName,
				e2eCfg.CRDChart,
			)).To(Succeed())

			By("Waiting for CRDs to be created")
			Eventually(func() bool {
				for _, crdName := range crdNames {
					crd := &apiextensionsv1.CustomResourceDefinition{}
					if err := cl.Get(ctx,
						runtimeclient.ObjectKey{
							Name: crdName,
						},
						crd,
					); err != nil {
						return false
					}
				}
				return true
			}, 5*time.Minute, 2*time.Second).Should(BeTrue())
		}
	})

	By("Deploying eks operator chart", func() {
		if isDeploymentReady(cattleSystemNamespace, operatorReleaseName) {
			By("already installed")
		} else {
			Expect(kubectl.RunHelmBinaryWithCustomErr(
				"-n",
				cattleSystemNamespace,
				"install",
				"--create-namespace",
				"--set", "debug=true",
				operatorReleaseName,
				e2eCfg.OperatorChart,
			)).To(Succeed())

			By("Waiting for eks operator deployment to be available")
			Eventually(func() bool {
				return isDeploymentReady(cattleSystemNamespace, operatorDeploymentName)
			}, 5*time.Minute, 2*time.Second).Should(BeTrue())
		}
		// As we are not bootstrapping rancher in the tests (going to the first login page, setting new password and rancher-url)
		// We need to manually set this value, which is the same value you would get from doing the bootstrap
		setting := &managementv3.Setting{}
		Expect(cl.Get(ctx,
			runtimeclient.ObjectKey{
				Name: "server-url",
			},
			setting,
		)).To(Succeed())

		setting.Source = "env"
		setting.Value = fmt.Sprintf("https://%s.%s", e2eCfg.ExternalIP, e2eCfg.MagicDNS)

		Expect(cl.Update(ctx, setting)).To(Succeed())

	})

	By("Creating aws credentials secret", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      awsCredentialsSecretName,
				Namespace: "default",
			},
			Data: map[string][]byte{
				"amazonec2credentialConfig-accessKey": []byte(e2eCfg.AWSAccessKey),
				"amazonec2credentialConfig-secretKey": []byte(e2eCfg.AWSSecretAccessKey),
			},
		}

		err := cl.Create(ctx, secret)
		if err != nil {
			fmt.Println(err)
			Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
		}
	})

	By("Reading cluster templates", func() {
		assets, err := templates.ReadDir("templates")
		Expect(err).ToNot(HaveOccurred())

		for _, asset := range assets {
			b, err := templates.ReadFile(path.Join("templates", asset.Name()))
			Expect(err).ToNot(HaveOccurred())

			eksCluster := &eksv1.EKSClusterConfig{}
			Expect(yaml.Unmarshal(b, eksCluster)).To(Succeed())

			name := strings.TrimSuffix(asset.Name(), ".yaml")
			generatedName := names.SimpleNameGenerator.GenerateName(name + "-")
			eksCluster.Name = generatedName
			eksCluster.Spec.DisplayName = generatedName
			eksCluster.Spec.Region = e2eCfg.AWSRegion
			Expect(eksCluster.Spec.NodeGroups).To(HaveLen(1))
			eksCluster.Spec.NodeGroups[0].NodeRole = aws.String("")

			clusterTemplates[name] = eksCluster
		}
	})
})

var _ = AfterSuite(func() {
	By("Creating artifact directory")

	if _, err := os.Stat(e2eCfg.ArtifactsDir); os.IsNotExist(err) {
		Expect(os.Mkdir(e2eCfg.ArtifactsDir, os.ModePerm)).To(Succeed())
	}

	By("Getting eks operator logs")

	podList := &corev1.PodList{}
	Expect(cl.List(ctx, podList, runtimeclient.MatchingLabels{
		"ke.cattle.io/operator": "eks",
	}, runtimeclient.InNamespace(cattleSystemNamespace),
	)).To(Succeed())

	for _, pod := range podList.Items {
		for _, container := range pod.Spec.Containers {
			output, err := kubectl.Run("logs", pod.Name, "-c", container.Name, "-n", pod.Namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(e2eCfg.ArtifactsDir, pod.Name+"-"+container.Name+".log"), redactSensitiveData([]byte(output)), 0644)).To(Succeed())
		}
	}

	By("Getting eks Clusters")

	eksClusterList := &eksv1.EKSClusterConfigList{}
	Expect(cl.List(ctx, eksClusterList, &runtimeclient.ListOptions{})).To(Succeed())

	for _, eksCluster := range eksClusterList.Items {
		output, err := yaml.Marshal(eksCluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(e2eCfg.ArtifactsDir, "eks-cluster-config-"+eksCluster.Name+".yaml"), redactSensitiveData([]byte(output)), 0644)).To(Succeed())
	}

	By("Getting Rancher Clusters")

	rancherClusterList := &managementv3.ClusterList{}
	Expect(cl.List(ctx, rancherClusterList, &runtimeclient.ListOptions{})).To(Succeed())

	for _, rancherCluster := range rancherClusterList.Items {
		output, err := yaml.Marshal(rancherCluster)
		Expect(err).ToNot(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(e2eCfg.ArtifactsDir, "rancher-cluster-"+rancherCluster.Name+".yaml"), redactSensitiveData([]byte(output)), 0644)).To(Succeed())
	}

	By("Cleaning up Rancher Clusters")

	for _, rancherCluster := range rancherClusterList.Items {
		if rancherCluster.Name == "local" {
			continue
		}

		Expect(cl.Delete(ctx, &rancherCluster)).To(Succeed())
		Eventually(func() bool {
			if err := cl.Get(ctx, runtimeclient.ObjectKey{
				Name:      rancherCluster.Name,
				Namespace: eksClusterConfigNamespace,
			}, &eksv1.EKSClusterConfig{}); err != nil {
				return apierrors.IsNotFound(err)
			}

			return false
		}, waitLong, pollInterval).Should(BeTrue())
	}
})

func isDeploymentReady(namespace, name string) bool {
	deployment := &appsv1.Deployment{}
	if err := cl.Get(ctx,
		runtimeclient.ObjectKey{
			Namespace: namespace,
			Name:      name,
		},
		deployment,
	); err != nil {
		return false
	}

	if deployment.Status.AvailableReplicas == *deployment.Spec.Replicas {
		return true
	}

	return false
}

func redactSensitiveData(input []byte) []byte {
	output := bytes.Replace(input, []byte(e2eCfg.AWSAccessKey), []byte("***"), -1)
	output = bytes.Replace(output, []byte(e2eCfg.AWSSecretAccessKey), []byte("***"), -1)
	output = bytes.Replace(output, []byte(e2eCfg.AWSRegion), []byte("***"), -1)
	return output
}
