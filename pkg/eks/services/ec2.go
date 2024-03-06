package services

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

type EC2ServiceInterface interface {
	CreateLaunchTemplate(ctx context.Context, input *ec2.CreateLaunchTemplateInput) (*ec2.CreateLaunchTemplateOutput, error)
	DeleteLaunchTemplate(ctx context.Context, input *ec2.DeleteLaunchTemplateInput) (*ec2.DeleteLaunchTemplateOutput, error)
	DescribeLaunchTemplates(ctx context.Context, input *ec2.DescribeLaunchTemplatesInput) (*ec2.DescribeLaunchTemplatesOutput, error)
	CreateLaunchTemplateVersion(ctx context.Context, input *ec2.CreateLaunchTemplateVersionInput) (*ec2.CreateLaunchTemplateVersionOutput, error)
	DeleteLaunchTemplateVersions(ctx context.Context, input *ec2.DeleteLaunchTemplateVersionsInput) (*ec2.DeleteLaunchTemplateVersionsOutput, error)
	DescribeLaunchTemplateVersions(ctx context.Context, input *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error)
	DescribeImages(ctx context.Context, input *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error)
}

type ec2Service struct {
	svc *ec2.Client
}

func NewEC2Service(cfg aws.Config) EC2ServiceInterface {
	return &ec2Service{
		svc: ec2.NewFromConfig(cfg),
	}
}

func (c *ec2Service) CreateLaunchTemplate(ctx context.Context, input *ec2.CreateLaunchTemplateInput) (*ec2.CreateLaunchTemplateOutput, error) {
	return c.svc.CreateLaunchTemplate(ctx, input)
}

func (c *ec2Service) DeleteLaunchTemplate(ctx context.Context, input *ec2.DeleteLaunchTemplateInput) (*ec2.DeleteLaunchTemplateOutput, error) {
	return c.svc.DeleteLaunchTemplate(ctx, input)
}

func (c *ec2Service) DescribeLaunchTemplateVersions(ctx context.Context, input *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	return c.svc.DescribeLaunchTemplateVersions(ctx, input)
}

func (c *ec2Service) DescribeLaunchTemplates(ctx context.Context, input *ec2.DescribeLaunchTemplatesInput) (*ec2.DescribeLaunchTemplatesOutput, error) {
	return c.svc.DescribeLaunchTemplates(ctx, input)
}

func (c *ec2Service) CreateLaunchTemplateVersion(ctx context.Context, input *ec2.CreateLaunchTemplateVersionInput) (*ec2.CreateLaunchTemplateVersionOutput, error) {
	return c.svc.CreateLaunchTemplateVersion(ctx, input)
}

func (c *ec2Service) DeleteLaunchTemplateVersions(ctx context.Context, input *ec2.DeleteLaunchTemplateVersionsInput) (*ec2.DeleteLaunchTemplateVersionsOutput, error) {
	return c.svc.DeleteLaunchTemplateVersions(ctx, input)
}

func (c *ec2Service) DescribeImages(ctx context.Context, input *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error) {
	return c.svc.DescribeImages(ctx, input)
}
