package templates

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Templates", func() {
	Context("Helper functions", func() {
		Describe("getAWSDNSSuffix", func() {
			It("should return the correct DNS suffix for a given region", func() {
				Expect(getAWSDNSSuffix("us-east-1")).To(Equal("amazonaws.com"))
				Expect(getAWSDNSSuffix("cn-north-1")).To(Equal("amazonaws.com.cn"))
				Expect(getAWSDNSSuffix("us-gov-west-1")).To(Equal("amazonaws.com"))
			})

			It("should return default DNS suffix for unknown region", func() {
				Expect(getAWSDNSSuffix("unknown-region")).To(Equal(endpoints.AwsPartition().DNSSuffix()))
			})
		})

		Describe("getEC2ServiceEndpoint", func() {
			It("should return the correct EC2 service endpoint for a given region", func() {
				Expect(getEC2ServiceEndpoint("us-east-1")).To(Equal("ec2.amazonaws.com"))
				Expect(getEC2ServiceEndpoint("us-gov-west-1")).To(Equal("ec2.amazonaws.com"))
				Expect(getEC2ServiceEndpoint("cn-north-1")).To(Equal("ec2.amazonaws.com.cn"))
			})
		})

		Describe("getArnPrefixForRegion", func() {
			It("should return the correct ARN prefix for a given region", func() {
				Expect(getArnPrefixForRegion("us-east-1")).To(Equal("arn:aws"))
				Expect(getArnPrefixForRegion("cn-north-1")).To(Equal("arn:aws-cn"))
				Expect(getArnPrefixForRegion("us-gov-west-1")).To(Equal("arn:aws-us-gov"))
			})

			It("should return default ARN prefix for unknown region", func() {
				Expect(getArnPrefixForRegion("unknown-region")).To(Equal("arn:" + endpoints.AwsPartition().ID()))
			})
		})
	})

	Context("Template generation", func() {
		Describe("GetServiceRoleTemplate", func() {
			It("should generate a valid service role template for us-east-1", func() {
				tmpl, err := GetServiceRoleTemplate("us-east-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpl).To(ContainSubstring("AWSServiceRoleForAmazonEKS"))
				Expect(tmpl).To(ContainSubstring("arn:aws:iam::aws:policy/AmazonEKSServicePolicy"))
				Expect(tmpl).To(ContainSubstring("arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"))
			})

			It("should generate a valid service role template for cn-north-1", func() {
				tmpl, err := GetServiceRoleTemplate("cn-north-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpl).To(ContainSubstring("arn:aws-cn:iam::aws:policy/AmazonEKSServicePolicy"))
			})

			It("should generate a valid service role template for us-gov-west-1", func() {
				tmpl, err := GetServiceRoleTemplate("us-gov-west-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpl).To(ContainSubstring("arn:aws-us-gov:iam::aws:policy/AmazonEKSServicePolicy"))
			})
		})

		Describe("GetNodeInstanceRoleTemplate", func() {
			It("should generate a valid node instance role template for us-east-1", func() {
				tmpl, err := GetNodeInstanceRoleTemplate("us-east-1", aws.String("ipv4"))
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpl).To(ContainSubstring("NodeInstanceRole"))
				Expect(tmpl).To(ContainSubstring("ec2.amazonaws.com"))
				Expect(tmpl).To(ContainSubstring("arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"))
				Expect(tmpl).To(ContainSubstring("arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"))
				Expect(tmpl).To(ContainSubstring("arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"))
			})

			It("should generate a valid node instance role template for cn-north-1", func() {
				tmpl, err := GetNodeInstanceRoleTemplate("cn-north-1", aws.String("ipv4"))
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpl).To(ContainSubstring("ec2.amazonaws.com.cn"))
				Expect(tmpl).To(ContainSubstring("arn:aws-cn:iam::aws:policy/AmazonEKSWorkerNodePolicy"))
			})

			It("should generate a valid node instance role template for us-gov-west-1", func() {
				tmpl, err := GetNodeInstanceRoleTemplate("us-gov-west-1", aws.String("ipv4"))
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpl).To(ContainSubstring("ec2.amazonaws.com"))
				Expect(tmpl).To(ContainSubstring("arn:aws-us-gov:iam::aws:policy/AmazonEKSWorkerNodePolicy"))
			})
		})

		Describe("GetEBSCSIDriverTemplate", func() {
			It("should generate a valid EBS CSI driver template for us-east-1", func() {
				providerID := "ABCDEF12345678"
				tmpl, err := GetEBSCSIDriverTemplate("us-east-1", providerID)
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpl).To(ContainSubstring("AWSEBSCSIDriverRoleForAmazonEKS"))
				Expect(tmpl).To(ContainSubstring("arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"))
				Expect(tmpl).To(ContainSubstring("oidc.eks.us-east-1.amazonaws.com/id/" + providerID))
				Expect(tmpl).To(ContainSubstring("system:serviceaccount:kube-system:ebs-csi-controller-sa"))
				Expect(tmpl).To(ContainSubstring("sts.amazonaws.com"))
			})

			It("should generate a valid EBS CSI driver template for cn-north-1", func() {
				providerID := "ABCDEF12345678"
				tmpl, err := GetEBSCSIDriverTemplate("cn-north-1", providerID)
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpl).To(ContainSubstring("arn:aws-cn:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"))
				Expect(tmpl).To(ContainSubstring("oidc.eks.cn-north-1.amazonaws.com.cn/id/" + providerID))
				Expect(tmpl).To(ContainSubstring("sts.amazonaws.com.cn"))
			})

			It("should generate a valid EBS CSI driver template for us-gov-west-1", func() {
				providerID := "ABCDEF12345678"
				tmpl, err := GetEBSCSIDriverTemplate("us-gov-west-1", providerID)
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpl).To(ContainSubstring("arn:aws-us-gov:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"))
				Expect(tmpl).To(ContainSubstring("oidc.eks.us-gov-west-1.amazonaws.com/id/" + providerID))
				Expect(tmpl).To(ContainSubstring("sts.amazonaws.com"))
			})
		})
	})
})
