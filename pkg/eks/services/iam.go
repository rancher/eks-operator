package services

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

type IAMServiceInterface interface {
	GetRole(ctx context.Context, input *iam.GetRoleInput) (*iam.GetRoleOutput, error)
	ListOIDCProviders(ctx context.Context, input *iam.ListOpenIDConnectProvidersInput) (*iam.ListOpenIDConnectProvidersOutput, error)
	CreateOIDCProvider(ctx context.Context, input *iam.CreateOpenIDConnectProviderInput) (*iam.CreateOpenIDConnectProviderOutput, error)
}

type iamService struct {
	svc *iam.Client
}

func NewIAMService(cfg aws.Config) IAMServiceInterface {
	return &iamService{
		svc: iam.NewFromConfig(cfg),
	}
}

func (c *iamService) GetRole(ctx context.Context, input *iam.GetRoleInput) (*iam.GetRoleOutput, error) {
	return c.svc.GetRole(ctx, input)
}

func (c *iamService) ListOIDCProviders(ctx context.Context, input *iam.ListOpenIDConnectProvidersInput) (*iam.ListOpenIDConnectProvidersOutput, error) {
	return c.svc.ListOpenIDConnectProviders(ctx, input)
}

func (c *iamService) CreateOIDCProvider(ctx context.Context, input *iam.CreateOpenIDConnectProviderInput) (*iam.CreateOpenIDConnectProviderOutput, error) {
	return c.svc.CreateOpenIDConnectProvider(ctx, input)
}
