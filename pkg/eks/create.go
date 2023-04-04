package eks

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/eks"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks/services"
)

const (
	createInProgressStatus   = "CREATE_IN_PROGRESS"
	createCompleteStatus     = "CREATE_COMPLETE"
	createFailedStatus       = "CREATE_FAILED"
	rollbackInProgressStatus = "ROLLBACK_IN_PROGRESS"
)

type CreateClusterOptions struct {
	EKSService services.EKSServiceInterface
	Config     *eksv1.EKSClusterConfig
	RoleARN    string
}

func CreateCluster(opts CreateClusterOptions) error {
	createClusterInput := newClusterInput(opts.Config, opts.RoleARN)

	_, err := opts.EKSService.CreateCluster(createClusterInput)
	return err
}

func newClusterInput(config *eksv1.EKSClusterConfig, roleARN string) *eks.CreateClusterInput {
	createClusterInput := &eks.CreateClusterInput{
		Name:    aws.String(config.Spec.DisplayName),
		RoleArn: aws.String(roleARN),
		ResourcesVpcConfig: &eks.VpcConfigRequest{
			EndpointPrivateAccess: config.Spec.PrivateAccess,
			EndpointPublicAccess:  config.Spec.PublicAccess,
			SecurityGroupIds:      aws.StringSlice(config.Status.SecurityGroups),
			SubnetIds:             aws.StringSlice(config.Status.Subnets),
			PublicAccessCidrs:     getPublicAccessCidrs(config.Spec.PublicAccessSources),
		},
		Tags:    getTags(config.Spec.Tags),
		Logging: getLogging(config.Spec.LoggingTypes),
		Version: config.Spec.KubernetesVersion,
	}

	if aws.BoolValue(config.Spec.SecretsEncryption) {
		createClusterInput.EncryptionConfig = []*eks.EncryptionConfig{
			{
				Provider: &eks.Provider{
					KeyArn: config.Spec.KmsKey,
				},
				Resources: aws.StringSlice([]string{"secrets"}),
			},
		}
	}

	return createClusterInput
}

type CreateStackOptions struct {
	CloudFormationService services.CloudFormationServiceInterface
	StackName             string
	DisplayName           string
	TemplateBody          string
	Capabilities          []string
	Parameters            []*cloudformation.Parameter
}

func CreateStack(opts CreateStackOptions) (*cloudformation.DescribeStacksOutput, error) {
	_, err := opts.CloudFormationService.CreateStack(&cloudformation.CreateStackInput{
		StackName:    aws.String(opts.StackName),
		TemplateBody: aws.String(opts.StackName),
		Capabilities: aws.StringSlice(opts.Capabilities),
		Parameters:   opts.Parameters,
		Tags: []*cloudformation.Tag{
			{
				Key:   aws.String("displayName"),
				Value: aws.String(opts.DisplayName),
			},
		},
	})
	if err != nil && !alreadyExistsInCloudFormationError(err) {
		return nil, fmt.Errorf("error creating master: %v", err)
	}

	var stack *cloudformation.DescribeStacksOutput
	status := createInProgressStatus

	for status == createInProgressStatus {
		time.Sleep(time.Second * 5)
		stack, err = opts.CloudFormationService.DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String(opts.StackName),
		})
		if err != nil {
			return nil, fmt.Errorf("error polling stack info: %v", err)
		}

		status = *stack.Stacks[0].StackStatus
	}

	if len(stack.Stacks) == 0 {
		return nil, fmt.Errorf("stack did not have output: %v", err)
	}

	if status != createCompleteStatus {
		reason := "reason unknown"
		events, err := opts.CloudFormationService.DescribeStackEvents(&cloudformation.DescribeStackEventsInput{
			StackName: aws.String(opts.StackName),
		})
		if err == nil {
			for _, event := range events.StackEvents {
				// guard against nil pointer dereference
				if event.ResourceStatus == nil || event.LogicalResourceId == nil || event.ResourceStatusReason == nil {
					continue
				}

				if *event.ResourceStatus == createFailedStatus {
					reason = *event.ResourceStatusReason
					break
				}

				if *event.ResourceStatus == rollbackInProgressStatus {
					reason = *event.ResourceStatusReason
					// do not break so that CREATE_FAILED takes priority
				}
			}
		}
		return nil, fmt.Errorf("stack failed to create: %v", reason)
	}

	return stack, nil
}

func getTags(tags map[string]string) map[string]*string {
	if len(tags) == 0 {
		return nil
	}

	return aws.StringMap(tags)
}

func getLogging(loggingTypes []string) *eks.Logging {
	if len(loggingTypes) == 0 {
		return &eks.Logging{
			ClusterLogging: []*eks.LogSetup{
				{
					Enabled: aws.Bool(false),
					Types:   aws.StringSlice(loggingTypes),
				},
			},
		}
	}
	return &eks.Logging{
		ClusterLogging: []*eks.LogSetup{
			{
				Enabled: aws.Bool(true),
				Types:   aws.StringSlice(loggingTypes),
			},
		},
	}
}

func getPublicAccessCidrs(publicAccessCidrs []string) []*string {
	if len(publicAccessCidrs) == 0 {
		return aws.StringSlice([]string{"0.0.0.0/0"})
	}

	return aws.StringSlice(publicAccessCidrs)
}

func alreadyExistsInCloudFormationError(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case cloudformation.ErrCodeAlreadyExistsException:
			return true
		}
	}

	return false
}
