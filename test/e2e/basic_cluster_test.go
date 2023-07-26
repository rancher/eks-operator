package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	managementv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("BasicCluster", func() {
	var eksConfig *eksv1.EKSClusterConfig
	var cluster *managementv3.Cluster

	BeforeEach(func() {
		var ok bool
		eksConfig, ok = clusterTemplates[basicClusterTemplateName]
		Expect(ok).To(BeTrue())
		Expect(eksConfig).NotTo(BeNil())

		cluster = &managementv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      eksConfig.Name,
				Namespace: eksClusterConfigNamespace,
			},
			Spec: managementv3.ClusterSpec{
				EKSConfig: &eksConfig.Spec,
			},
		}

	})

	It("Succesfully creates a cluster", func() {
		By("Creating a cluster")
		Expect(cl.Create(ctx, cluster)).Should(Succeed())

		By("Waiting for cluster to be ready")
		Eventually(func() error {
			currentCluster := &eksv1.EKSClusterConfig{}

			if err := cl.Get(ctx, runtimeclient.ObjectKey{
				Name:      cluster.Name,
				Namespace: eksClusterConfigNamespace,
			}, currentCluster); err != nil {
				return err
			}

			if currentCluster.Status.Phase == "active" {
				return nil
			}

			return fmt.Errorf("cluster is not ready yet. Current phase: %s", currentCluster.Status.Phase)
		}, waitLong, pollInterval).ShouldNot(HaveOccurred())
	})
})
