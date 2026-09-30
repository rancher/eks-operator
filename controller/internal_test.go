package controller

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
)

var _ = Describe("newAWSConfigV2", func() {
	// resolveEC2Endpoint returns the endpoint an EC2 client built from the given
	// config sends its requests to, without performing any AWS API call.
	resolveEC2Endpoint := func(cfg aws.Config) string {
		options := ec2.NewFromConfig(cfg).Options()
		endpoint, err := options.EndpointResolverV2.ResolveEndpoint(ctx, ec2.EndpointParameters{
			Region:       aws.String(cfg.Region),
			UseDualStack: aws.Bool(options.EndpointOptions.UseDualStackEndpoint == aws.DualStackEndpointStateEnabled),
			UseFIPS:      aws.Bool(options.EndpointOptions.UseFIPSEndpoint == aws.FIPSEndpointStateEnabled),
		})
		Expect(err).ToNot(HaveOccurred())

		return endpoint.URI.String()
	}

	dualStackEndpointState := func(cfg aws.Config) aws.DualStackEndpointState {
		return ec2.NewFromConfig(cfg).Options().EndpointOptions.UseDualStackEndpoint
	}

	BeforeEach(func() {
		// Isolate the specs from the AWS configuration of the machine running them.
		GinkgoT().Setenv("AWS_CONFIG_FILE", "does-not-exist")
		GinkgoT().Setenv("AWS_SHARED_CREDENTIALS_FILE", "does-not-exist")
		GinkgoT().Setenv("AWS_EC2_METADATA_DISABLED", "true")
		GinkgoT().Setenv("AWS_USE_DUALSTACK_ENDPOINT", "")
		GinkgoT().Setenv("AWS_REGION", "us-east-1")
	})

	It("should use dual-stack endpoints in the standard AWS partition", func() {
		cfg, err := newAWSConfigV2(ctx, nil, eksv1.EKSClusterConfigSpec{Region: "us-east-1"})
		Expect(err).ToNot(HaveOccurred())
		Expect(dualStackEndpointState(cfg)).To(Equal(aws.DualStackEndpointStateEnabled))
		Expect(resolveEC2Endpoint(cfg)).To(Equal("https://ec2.us-east-1.api.aws"))
	})

	It("should not use dual-stack endpoints in the AWS China partition", func() {
		for region, expectedEndpoint := range map[string]string{
			"cn-north-1":     "https://ec2.cn-north-1.amazonaws.com.cn",
			"cn-northwest-1": "https://ec2.cn-northwest-1.amazonaws.com.cn",
		} {
			cfg, err := newAWSConfigV2(ctx, nil, eksv1.EKSClusterConfigSpec{Region: region})
			Expect(err).ToNot(HaveOccurred())
			Expect(dualStackEndpointState(cfg)).To(Equal(aws.DualStackEndpointStateUnset))
			Expect(resolveEC2Endpoint(cfg)).To(Equal(expectedEndpoint))
		}
	})

	It("should use dual-stack endpoints in the AWS GovCloud partition", func() {
		cfg, err := newAWSConfigV2(ctx, nil, eksv1.EKSClusterConfigSpec{Region: "us-gov-west-1"})
		Expect(err).ToNot(HaveOccurred())
		Expect(dualStackEndpointState(cfg)).To(Equal(aws.DualStackEndpointStateEnabled))
		Expect(resolveEC2Endpoint(cfg)).To(Equal("https://ec2.us-gov-west-1.api.aws"))
	})

	It("should select the endpoints from the configured region when the spec has none", func() {
		GinkgoT().Setenv("AWS_REGION", "cn-northwest-1")

		cfg, err := newAWSConfigV2(ctx, nil, eksv1.EKSClusterConfigSpec{})
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.Region).To(Equal("cn-northwest-1"))
		Expect(dualStackEndpointState(cfg)).To(Equal(aws.DualStackEndpointStateUnset))
		Expect(resolveEC2Endpoint(cfg)).To(Equal("https://ec2.cn-northwest-1.amazonaws.com.cn"))
	})

	It("should let the user disable dual-stack endpoints", func() {
		GinkgoT().Setenv("AWS_USE_DUALSTACK_ENDPOINT", "false")

		cfg, err := newAWSConfigV2(ctx, nil, eksv1.EKSClusterConfigSpec{Region: "us-east-1"})
		Expect(err).ToNot(HaveOccurred())
		Expect(dualStackEndpointState(cfg)).To(Equal(aws.DualStackEndpointStateDisabled))
		Expect(resolveEC2Endpoint(cfg)).To(Equal("https://ec2.us-east-1.amazonaws.com"))
	})

	It("should let the user enable dual-stack endpoints", func() {
		GinkgoT().Setenv("AWS_USE_DUALSTACK_ENDPOINT", "true")

		cfg, err := newAWSConfigV2(ctx, nil, eksv1.EKSClusterConfigSpec{Region: "cn-northwest-1"})
		Expect(err).ToNot(HaveOccurred())
		Expect(dualStackEndpointState(cfg)).To(Equal(aws.DualStackEndpointStateEnabled))
		Expect(resolveEC2Endpoint(cfg)).To(Equal("https://ec2.cn-northwest-1.api.amazonwebservices.com.cn"))
	})
})
