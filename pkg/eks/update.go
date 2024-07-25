package eks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks/services"
	"github.com/rancher/eks-operator/utils"
	"github.com/sirupsen/logrus"
)

const (
	allOpen = "0.0.0.0/0"
)

type UpdateClusterVersionOpts struct {
	EKSService          services.EKSServiceInterface
	Config              *eksv1.EKSClusterConfig
	UpstreamClusterSpec *eksv1.EKSClusterConfigSpec
}

func UpdateClusterVersion(ctx context.Context, opts *UpdateClusterVersionOpts) (bool, error) {
	updated := false
	if aws.ToString(opts.UpstreamClusterSpec.KubernetesVersion) != aws.ToString(opts.Config.Spec.KubernetesVersion) {
		logrus.Infof("updating kubernetes version for cluster [%s]", opts.Config.Name)
		_, err := opts.EKSService.UpdateClusterVersion(ctx, &eks.UpdateClusterVersionInput{
			Name:    aws.String(opts.Config.Spec.DisplayName),
			Version: opts.Config.Spec.KubernetesVersion,
		})
		if err != nil {
			return updated, fmt.Errorf("error updating cluster [%s] kubernetes version: %w", opts.Config.Name, err)
		}
		updated = true
	}

	return updated, nil
}

type UpdateResourceTagsOpts struct {
	EKSService   services.EKSServiceInterface
	Tags         map[string]string
	UpstreamTags map[string]string
	ClusterName  string
	ResourceARN  string
}

func UpdateResourceTags(ctx context.Context, opts *UpdateResourceTagsOpts) (bool, error) {
	updated := false
	if updateTags := utils.GetKeyValuesToUpdate(opts.Tags, opts.UpstreamTags); updateTags != nil {
		_, err := opts.EKSService.TagResource(ctx,
			&eks.TagResourceInput{
				ResourceArn: aws.String(opts.ResourceARN),
				Tags:        updateTags,
			})
		if err != nil {
			return false, fmt.Errorf("error tagging cluster [%s]: %w", opts.ClusterName, err)
		}
		updated = true
	}

	if updateUntags := utils.GetKeysToDelete(opts.Tags, opts.UpstreamTags); updateUntags != nil {
		_, err := opts.EKSService.UntagResource(ctx,
			&eks.UntagResourceInput{
				ResourceArn: aws.String(opts.ResourceARN),
				TagKeys:     updateUntags,
			})
		if err != nil {
			return false, fmt.Errorf("error untagging cluster [%s]: %w", opts.ClusterName, err)
		}
		updated = true
	}

	return updated, nil
}

type UpdateLoggingTypesOpts struct {
	EKSService          services.EKSServiceInterface
	Config              *eksv1.EKSClusterConfig
	UpstreamClusterSpec *eksv1.EKSClusterConfigSpec
}

func UpdateClusterLoggingTypes(ctx context.Context, opts *UpdateLoggingTypesOpts) (bool, error) {
	updated := false
	if loggingTypesUpdate := getLoggingTypesUpdate(opts.Config.Spec.LoggingTypes, opts.UpstreamClusterSpec.LoggingTypes); loggingTypesUpdate != nil {
		_, err := opts.EKSService.UpdateClusterConfig(ctx,
			&eks.UpdateClusterConfigInput{
				Name:    aws.String(opts.Config.Spec.DisplayName),
				Logging: loggingTypesUpdate,
			},
		)
		if err != nil {
			return false, fmt.Errorf("error updating cluster [%s] logging types: %w", opts.Config.Name, err)
		}
		updated = true
	}

	return updated, nil
}

type UpdateClusterAccessOpts struct {
	EKSService          services.EKSServiceInterface
	Config              *eksv1.EKSClusterConfig
	UpstreamClusterSpec *eksv1.EKSClusterConfigSpec
}

func UpdateClusterAccess(ctx context.Context, opts *UpdateClusterAccessOpts) (bool, error) {
	updated := false

	publicAccessUpdate := opts.Config.Spec.PublicAccess != nil && aws.ToBool(opts.UpstreamClusterSpec.PublicAccess) != aws.ToBool(opts.Config.Spec.PublicAccess)
	privateAccessUpdate := opts.Config.Spec.PrivateAccess != nil && aws.ToBool(opts.UpstreamClusterSpec.PrivateAccess) != aws.ToBool(opts.Config.Spec.PrivateAccess)
	if publicAccessUpdate || privateAccessUpdate {
		// public and private access updates need to be sent together. When they are sent one at a time
		// the request may be denied due to having both public and private access disabled.
		_, err := opts.EKSService.UpdateClusterConfig(ctx,
			&eks.UpdateClusterConfigInput{
				Name: aws.String(opts.Config.Spec.DisplayName),
				ResourcesVpcConfig: &ekstypes.VpcConfigRequest{
					EndpointPublicAccess:  opts.Config.Spec.PublicAccess,
					EndpointPrivateAccess: opts.Config.Spec.PrivateAccess,
				},
			},
		)
		if err != nil {
			return false, fmt.Errorf("error updating cluster [%s] public/private access: %w", opts.Config.Name, err)
		}
		updated = true
	}

	return updated, nil
}

type UpdateClusterPublicAccessSourcesOpts struct {
	EKSService          services.EKSServiceInterface
	Config              *eksv1.EKSClusterConfig
	UpstreamClusterSpec *eksv1.EKSClusterConfigSpec
}

func UpdateClusterPublicAccessSources(ctx context.Context, opts *UpdateClusterPublicAccessSourcesOpts) (bool, error) {
	updated := false
	// check public access CIDRs for update (public access sources)

	filteredSpecPublicAccessSources := filterPublicAccessSources(opts.Config.Spec.PublicAccessSources)
	filteredUpstreamPublicAccessSources := filterPublicAccessSources(opts.UpstreamClusterSpec.PublicAccessSources)
	if !utils.CompareStringSliceElements(filteredSpecPublicAccessSources, filteredUpstreamPublicAccessSources) {
		_, err := opts.EKSService.UpdateClusterConfig(ctx,
			&eks.UpdateClusterConfigInput{
				Name: aws.String(opts.Config.Spec.DisplayName),
				ResourcesVpcConfig: &ekstypes.VpcConfigRequest{
					PublicAccessCidrs: getPublicAccessCidrs(opts.Config.Spec.PublicAccessSources),
				},
			},
		)
		if err != nil {
			return false, fmt.Errorf("error updating cluster [%s] public access sources: %w", opts.Config.Name, err)
		}

		updated = true
	}

	return updated, nil
}

type UpdateNodegroupVersionOpts struct {
	EKSService     services.EKSServiceInterface
	EC2Service     services.EC2ServiceInterface
	Config         *eksv1.EKSClusterConfig
	NodeGroup      *eksv1.NodeGroup
	NGVersionInput *eks.UpdateNodegroupVersionInput
	LTVersions     map[string]string
}

func UpdateNodegroupVersion(ctx context.Context, opts *UpdateNodegroupVersionOpts) error {
	if _, err := opts.EKSService.UpdateNodegroupVersion(ctx, opts.NGVersionInput); err != nil {
		if version, ok := opts.LTVersions[aws.ToString(opts.NodeGroup.NodegroupName)]; ok {
			// If there was an error updating the node group and a Rancher-managed launch template version was created,
			// then the version that caused the issue needs to be deleted to prevent bad versions from piling up.
			DeleteLaunchTemplateVersions(ctx, opts.EC2Service, opts.Config.Status.ManagedLaunchTemplateID, []*string{aws.String(version)})
		}
		return err
	}

	return nil
}

func getLoggingTypesUpdate(loggingTypes []string, upstreamLoggingTypes []string) *ekstypes.Logging {
	loggingUpdate := &ekstypes.Logging{}

	if len(loggingTypes) >= 0 {
		loggingTypesToDisable := getLoggingTypesToDisable(loggingTypes, upstreamLoggingTypes)
		if loggingTypesToDisable.Enabled != nil {
			loggingUpdate.ClusterLogging = append(loggingUpdate.ClusterLogging, loggingTypesToDisable)
		}

		loggingTypesToEnable := getLoggingTypesToEnable(loggingTypes, upstreamLoggingTypes)
		if loggingTypesToEnable.Enabled != nil {
			loggingUpdate.ClusterLogging = append(loggingUpdate.ClusterLogging, loggingTypesToEnable)
		}
	}

	if len(loggingUpdate.ClusterLogging) > 0 {
		return loggingUpdate
	}

	return nil
}

func getLoggingTypesToDisable(loggingTypes []string, upstreamLoggingTypes []string) ekstypes.LogSetup {
	loggingTypesMap := make(map[string]bool)

	for _, val := range loggingTypes {
		loggingTypesMap[val] = true
	}

	var loggingTypesToDisable []string
	for _, val := range upstreamLoggingTypes {
		if !loggingTypesMap[val] {
			loggingTypesToDisable = append(loggingTypesToDisable, val)
		}
	}

	if len(loggingTypesToDisable) > 0 {
		return ekstypes.LogSetup{
			Enabled: aws.Bool(false),
			Types:   utils.ConvertToLogTypes(loggingTypesToDisable),
		}
	}

	return ekstypes.LogSetup{}
}

func getLoggingTypesToEnable(loggingTypes []string, upstreamLoggingTypes []string) ekstypes.LogSetup {
	upstreamLoggingTypesMap := make(map[string]bool)

	for _, val := range upstreamLoggingTypes {
		upstreamLoggingTypesMap[val] = true
	}

	var loggingTypesToEnable []string
	for _, val := range loggingTypes {
		if !upstreamLoggingTypesMap[val] {
			loggingTypesToEnable = append(loggingTypesToEnable, val)
		}
	}

	if len(loggingTypesToEnable) > 0 {
		return ekstypes.LogSetup{
			Enabled: aws.Bool(true),
			Types:   utils.ConvertToLogTypes(loggingTypesToEnable),
		}
	}

	return ekstypes.LogSetup{}
}

func filterPublicAccessSources(sources []string) []string {
	if len(sources) == 0 {
		return nil
	}
	if len(sources) == 1 && sources[0] == allOpen {
		return nil
	}
	return sources
}
