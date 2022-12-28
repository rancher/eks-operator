package services

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

type CloudFormationServiceInterface interface {
	DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
	DeleteStack(input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error)
	CreateStack(input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error)
	DescribeStackEvents(input *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error)
}

type cloudFormationService struct {
	svc *cloudformation.CloudFormation
}

func NewCloudFormationService(sess *session.Session) *cloudFormationService {
	return &cloudFormationService{
		svc: cloudformation.New(sess),
	}
}

func (c *cloudFormationService) DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	return c.svc.DescribeStacks(input)
}

func (c *cloudFormationService) DeleteStack(input *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	return c.svc.DeleteStack(input)
}

func (c *cloudFormationService) CreateStack(input *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	return c.svc.CreateStack(input)
}

func (c *cloudFormationService) DescribeStackEvents(input *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error) {
	return c.svc.DescribeStackEvents(input)
}
