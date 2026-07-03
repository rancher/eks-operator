package services

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type STSServiceInterface interface {
	GetCallerIdentity(ctx context.Context, input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
}

type stsService struct {
	svc *sts.Client
}

func NewSTSService(cfg aws.Config) STSServiceInterface {
	return &stsService{
		svc: sts.NewFromConfig(cfg),
	}
}

func (c *stsService) GetCallerIdentity(ctx context.Context, input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
	return c.svc.GetCallerIdentity(ctx, input)
}
