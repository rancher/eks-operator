package e2e

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
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

	It("Successfully adds and removes a node group", func() {
		initialNodeGroups := eksConfig.DeepCopy().Spec.NodeGroups

		Expect(cl.Get(ctx, runtimeclient.ObjectKey{Name: cluster.Name}, cluster)).Should(Succeed())
		patch := runtimeclient.MergeFrom(cluster.DeepCopy())

		nodeGroup := eksv1.NodeGroup{
			NodegroupName:        aws.String("ng1"),
			DiskSize:             aws.Int64(20),
			InstanceType:         aws.String("t3.medium"),
			DesiredSize:          aws.Int64(1),
			MaxSize:              aws.Int64(10),
			MinSize:              aws.Int64(1),
			RequestSpotInstances: aws.Bool(false),
		}

		cluster.Spec.EKSConfig.NodeGroups = append(cluster.Spec.EKSConfig.NodeGroups, nodeGroup)

		Expect(cl.Patch(ctx, cluster, patch)).Should(Succeed())

		By("Waiting for cluster to start adding node group")
		Eventually(func() error {
			currentCluster := &eksv1.EKSClusterConfig{}

			if err := cl.Get(ctx, runtimeclient.ObjectKey{
				Name:      cluster.Name,
				Namespace: eksClusterConfigNamespace,
			}, currentCluster); err != nil {
				return err
			}

			if currentCluster.Status.Phase == "updating" && len(currentCluster.Spec.NodeGroups) == 2 {
				return nil
			}

			return fmt.Errorf("cluster didn't create new new node group. Current phase: %s", currentCluster.Status.Phase)
		}, waitLong, pollInterval).ShouldNot(HaveOccurred())

		By("Waiting for cluster to finish adding node group")
		Eventually(func() error {
			currentCluster := &eksv1.EKSClusterConfig{}

			if err := cl.Get(ctx, runtimeclient.ObjectKey{
				Name:      cluster.Name,
				Namespace: eksClusterConfigNamespace,
			}, currentCluster); err != nil {
				return err
			}

			if currentCluster.Status.Phase == "active" && len(currentCluster.Spec.NodeGroups) == 2 {
				return nil
			}

			return fmt.Errorf("cluster didn't finish adding node group. Current phase: %s, node group count %d", currentCluster.Status.Phase, len(currentCluster.Spec.NodeGroups))
		}, waitLong, pollInterval).ShouldNot(HaveOccurred())

		By("Restoring initial node groups")

		Expect(cl.Get(ctx, runtimeclient.ObjectKey{Name: cluster.Name}, cluster)).Should(Succeed())
		patch = runtimeclient.MergeFrom(cluster.DeepCopy())

		cluster.Spec.EKSConfig.NodeGroups = initialNodeGroups

		Expect(cl.Patch(ctx, cluster, patch)).Should(Succeed())

		By("Waiting for cluster to start removing node group")
		Eventually(func() error {
			currentCluster := &eksv1.EKSClusterConfig{}

			if err := cl.Get(ctx, runtimeclient.ObjectKey{
				Name:      cluster.Name,
				Namespace: eksClusterConfigNamespace,
			}, currentCluster); err != nil {
				return err
			}

			if currentCluster.Status.Phase == "updating" && len(currentCluster.Spec.NodeGroups) == 1 {
				return nil
			}

			return fmt.Errorf("cluster didn't start removing node group. Current phase: %s, node group count %d", currentCluster.Status.Phase, len(currentCluster.Spec.NodeGroups))
		}, waitLong, pollInterval).ShouldNot(HaveOccurred())

		By("Waiting for cluster to finish removing node group")
		Eventually(func() error {
			currentCluster := &eksv1.EKSClusterConfig{}

			if err := cl.Get(ctx, runtimeclient.ObjectKey{
				Name:      cluster.Name,
				Namespace: eksClusterConfigNamespace,
			}, currentCluster); err != nil {
				return err
			}

			if currentCluster.Status.Phase == "active" && len(currentCluster.Spec.NodeGroups) == 1 {
				return nil
			}

			return fmt.Errorf("cluster didn't finish removing node group. Current phase: %s, node group count %d", currentCluster.Status.Phase, len(currentCluster.Spec.NodeGroups))
		}, waitLong, pollInterval).ShouldNot(HaveOccurred())

		By("Done waiting for cluster to finish removing node group")
	})
})
