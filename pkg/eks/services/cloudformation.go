package services

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
)

type CloudFormationServiceInterface interface {
	DescribeStacks(ctx context.Context, input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
	DeleteStack(ctx context.Context, input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error)
	CreateStack(ctx context.Context, input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
	DescribeStackEvents(ctx context.Context, input *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error)
}

type cloudFormationService struct {
	svc *cloudformation.Client
}

func NewCloudFormationService(cfg aws.Config) CloudFormationServiceInterface {
	return &cloudFormationService{
		svc: cloudformation.NewFromConfig(cfg),
	}
}

func (c *cloudFormationService) DescribeStacks(ctx context.Context, input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	return c.svc.DescribeStacks(ctx, input)
}

func (c *cloudFormationService) DeleteStack(ctx context.Context, input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	return c.svc.DeleteStack(ctx, input)
}

func (c *cloudFormationService) CreateStack(ctx context.Context, input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	return c.svc.CreateStack(ctx, input)
}

func (c *cloudFormationService) DescribeStackEvents(ctx context.Context, input *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error) {
	return c.svc.DescribeStackEvents(ctx, input)
}
