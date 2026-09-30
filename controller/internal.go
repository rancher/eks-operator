package controller

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks/services"
	"github.com/rancher/eks-operator/utils"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// dualStackEndpointDefault is an AWS config source holding the dual-stack
// endpoint preference the operator defaults to. The SDK resolves the first
// config source that has a value set, so appending it after the sources loaded
// by the SDK keeps an explicit user preference, set through the
// AWS_USE_DUALSTACK_ENDPOINT environment variable or the use_dualstack_endpoint
// shared configuration option, taking precedence over it.
type dualStackEndpointDefault aws.DualStackEndpointState

// GetUseDualStackEndpoint implements the dual-stack endpoint provider interface
// the AWS SDK looks for in the config sources of a client.
func (d dualStackEndpointDefault) GetUseDualStackEndpoint(context.Context) (aws.DualStackEndpointState, bool, error) {
	return aws.DualStackEndpointState(d), true, nil
}

func newAWSConfigV2(ctx context.Context, secretClient wranglerv1.SecretClient, spec eksv1.EKSClusterConfigSpec) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return cfg, fmt.Errorf("error loading default AWS config: %w", err)
	}

	if region := spec.Region; region != "" {
		cfg.Region = region
	}

	// Dual-stack endpoints are only requested in the partitions publishing them
	// for every AWS service used by the operator. Requesting them elsewhere, for
	// example in the AWS China partition where EC2 has no dual-stack endpoint,
	// makes the requests fail against a host that does not resolve.
	if utils.SupportsDualStackEndpoints(cfg.Region) {
		cfg.ConfigSources = append(cfg.ConfigSources, dualStackEndpointDefault(aws.DualStackEndpointStateEnabled))
	}

	if amazonCredentialSecret := spec.AmazonCredentialSecret; amazonCredentialSecret != "" {
		ns, id := utils.Parse(spec.AmazonCredentialSecret)
		secret, err := secretClient.Get(ns, id, metav1.GetOptions{})
		if err != nil {
			return cfg, fmt.Errorf("error getting secret %s/%s: %w", ns, id, err)
		}

		accessKeyBytes := secret.Data["amazonec2credentialConfig-accessKey"]
		secretKeyBytes := secret.Data["amazonec2credentialConfig-secretKey"]
		if accessKeyBytes == nil || secretKeyBytes == nil {
			return cfg, fmt.Errorf("invalid aws cloud credential")
		}

		accessKey := string(accessKeyBytes)
		secretKey := string(secretKeyBytes)

		cfg.Credentials = credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	}

	return cfg, nil
}

func newAWSv2Services(ctx context.Context, secretClient wranglerv1.SecretClient, spec eksv1.EKSClusterConfigSpec) (*awsServices, error) {
	cfg, err := newAWSConfigV2(ctx, secretClient, spec)
	if err != nil {
		return nil, err
	}

	return &awsServices{
		eks:            services.NewEKSService(cfg),
		cloudformation: services.NewCloudFormationService(cfg),
		iam:            services.NewIAMService(cfg),
		ec2:            services.NewEC2Service(cfg),
		sts:            services.NewSTSService(cfg),
	}, nil
}

func deleteStack(ctx context.Context, svc services.CloudFormationServiceInterface, newStyleName, oldStyleName string) error {
	name := newStyleName
	_, err := svc.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(name),
	})
	if doesNotExist(err) {
		name = oldStyleName
	}

	_, err = svc.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: aws.String(name),
	})
	if err != nil && !doesNotExist(err) {
		return fmt.Errorf("error deleting stack: %w", err)
	}

	return nil
}
