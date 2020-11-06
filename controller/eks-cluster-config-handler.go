package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/blang/semver"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	v12 "github.com/rancher/eks-operator/pkg/generated/controllers/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/templates"
	"github.com/rancher/eks-operator/utils"
	wranglerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v15 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	controllerName           = "eks-controller"
	controllerRemoveName     = "eks-controller-remove"
	eksConfigCreatingPhase   = "creating"
	eksConfigNotCreatedPhase = ""
	eksConfigActivePhase     = "active"
	eksConfigUpdatingPhase   = "updating"
	eksConfigImportingPhase  = "importing"
	allOpen                  = "0.0.0.0/0"
	eksClusterConfigKind     = "EKSClusterConfig"
)

type Handler struct {
	eksCC           v12.EKSClusterConfigClient
	eksEnqueueAfter func(namespace, name string, duration time.Duration)
	eksEnqueue      func(namespace, name string)
	secrets         wranglerv1.SecretClient
	secretsCache    wranglerv1.SecretCache
}

func Register(
	ctx context.Context,
	secrets wranglerv1.SecretController,
	eks v12.EKSClusterConfigController) {

	controller := &Handler{
		eksCC:           eks,
		eksEnqueue:      eks.Enqueue,
		eksEnqueueAfter: eks.EnqueueAfter,
		secretsCache:    secrets.Cache(),
		secrets:         secrets,
	}

	// Register handlers
	eks.OnChange(ctx, controllerName, controller.recordError(controller.OnEksConfigChanged))
	eks.OnRemove(ctx, controllerRemoveName, controller.OnEksConfigRemoved)
}

func (h *Handler) OnEksConfigChanged(key string, config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error) {
	if config == nil {
		return nil, nil
	}

	if config.DeletionTimestamp != nil {
		return nil, nil
	}

	sess, eksService, err := StartAWSSessions(h.secretsCache, config.Spec)
	if err != nil {
		return config, err
	}

	svc := cloudformation.New(sess)

	switch config.Status.Phase {
	case eksConfigImportingPhase:
		return h.importCluster(config, eksService)
	case eksConfigNotCreatedPhase:
		return h.create(config, sess, eksService)
	case eksConfigCreatingPhase:
		return h.waitForCreationComplete(config, eksService, svc)
	case eksConfigActivePhase:
		return h.checkAndUpdate(config, eksService, svc)
	case eksConfigUpdatingPhase:
		return h.checkAndUpdate(config, eksService, svc)
	}

	return config, nil
}

// recordError writes the error return by onChange to the failureMessage field on status. If there is no error, then
// empty string will be written to status
func (h *Handler) recordError(onChange func(key string, config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error)) func(key string, config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error) {
	return func(key string, config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error) {
		var err error
		var message string
		config, err = onChange(key, config)
		if config == nil {
			// EKS config is likely deleting
			return config, err
		}
		if err != nil {
			if !strings.Contains(err.Error(), "currently has update") {
				// the update is valid in that the controller should retry but there is
				// no actionable resolution as far as a user is concerned. An update
				// that has either been initiated by eks-operator or another source is
				// already in progress. It is possible an update is not being immediately
				// reflected in upstream cluster state. The config object will reenter the
				// controller and then the controller will wait for the update to finish.
				message = err.Error()
			}
		}

		if config.Status.FailureMessage == message {
			return config, err
		}

		if message != "" {
			config = config.DeepCopy()
			if config.Status.Phase == eksConfigActivePhase {
				// can assume an update is failing
				config.Status.Phase = eksConfigUpdatingPhase
			}
		}
		config.Status.FailureMessage = message

		var recordErr error
		config, recordErr = h.eksCC.UpdateStatus(config)
		if recordErr != nil {
			logrus.Errorf("Error recording ekscc [%s] failure message: %s", config.Name, recordErr.Error())
		}
		return config, err
	}
}

func (h *Handler) OnEksConfigRemoved(key string, config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error) {
	if config.Spec.Imported {
		logrus.Infof("cluster [%s] is imported, will not delete EKS cluster", config.Name)
		return config, nil
	}

	logrus.Infof("deleting cluster [%s]", config.Name)

	sess, eksService, err := StartAWSSessions(h.secretsCache, config.Spec)
	if err != nil {
		if errors.IsNotFound(err) {
			// if AmazonCredentialSecret cannot be used then skip cleanup
			logrus.Infof("AmazonCredentialSecret [%s] not found for EKS Config [%s], AWS cleanup skipped", config.Spec.AmazonCredentialSecret, config.Name)
			return config, nil
		}
		return config, err
	}

	svc := cloudformation.New(sess)

	logrus.Infof("starting node group deletion for config [%s]", config.Spec.DisplayName)
	waitingForNodegroupDeletion := true
	for waitingForNodegroupDeletion {
		waitingForNodegroupDeletion, err = deleteNodeGroups(config.Spec.DisplayName, eksService, config.Spec.NodeGroups)
		if err != nil {
			return config, fmt.Errorf("error deleting nodegroups for config [%s]", config.Spec.DisplayName)
		}
		time.Sleep(10 * time.Second)
		logrus.Infof("waiting for config [%s] node groups to delete", config.Name)
	}

	logrus.Infof("starting control plane deletion for config [%s]", config.Name)
	_, err = eksService.DeleteCluster(&eks.DeleteClusterInput{
		Name: aws.String(config.Spec.DisplayName),
	})
	if err != nil {
		if notFound(err) {
			_, err = eksService.DeleteCluster(&eks.DeleteClusterInput{
				Name: aws.String(config.Spec.DisplayName),
			})
		}

		if err != nil && !notFound(err) {
			return config, fmt.Errorf("error deleting cluster: %v", err)
		}
	}

	if aws.StringValue(config.Spec.ServiceRole) == "" {
		logrus.Infof("deleting service role for config [%s]", config.Name)
		err = deleteStack(svc, getServiceRoleName(config.Spec.DisplayName), getServiceRoleName(config.Spec.DisplayName))
		if err != nil {
			return config, fmt.Errorf("error deleting service role stack: %v", err)
		}
	}

	if len(config.Spec.Subnets) == 0 {
		logrus.Infof("deleting vpc, subnets, and security groups for config [%s]", config.Name)
		err = deleteStack(svc, getVPCStackName(config.Spec.DisplayName), getVPCStackName(config.Spec.DisplayName))
		if err != nil {
			return config, fmt.Errorf("error deleting vpc stack: %v", err)
		}
	}

	logrus.Infof("deleting node instance role for config [%s]", config.Name)
	err = deleteStack(svc, fmt.Sprintf("%s-node-instance-role", config.Spec.DisplayName), fmt.Sprintf("%s-node-instance-role", config.Spec.DisplayName))
	if err != nil {
		return config, fmt.Errorf("error deleting worker node stack: %v", err)
	}

	return config, err
}

func deleteNodeGroups(clusterName string, eksService *eks.EKS, nodeGroups []eksv1.NodeGroup) (bool, error) {
	var waitingForNodegroupDeletion bool
	for _, ng := range nodeGroups {
		ngState, err := eksService.DescribeNodegroup(
			&eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: ng.NodegroupName,
			})
		if err != nil {
			if notFound(err) {
				continue
			}
			return false, err
		}

		waitingForNodegroupDeletion = true

		if aws.StringValue(ngState.Nodegroup.Status) == eks.NodegroupStatusDeleting {
			continue
		}

		_, err = eksService.DeleteNodegroup(
			&eks.DeleteNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: ng.NodegroupName,
			})
		if err != nil {
			return false, err
		}
	}
	return waitingForNodegroupDeletion, nil
}

func (h *Handler) checkAndUpdate(config *eksv1.EKSClusterConfig, eksService *eks.EKS, svc *cloudformation.CloudFormation) (*eksv1.EKSClusterConfig, error) {
	if err := validateUpdate(config); err != nil {
		// validation failed, will be considered a failing update until resolved
		config = config.DeepCopy()
		config.Status.Phase = eksConfigUpdatingPhase
		var updateErr error
		config, updateErr = h.eksCC.UpdateStatus(config)
		if updateErr != nil {
			return config, updateErr
		}
		return config, err
	}

	clusterState, err := eksService.DescribeCluster(
		&eks.DescribeClusterInput{
			Name: aws.String(config.Spec.DisplayName),
		})
	if err != nil {
		return config, err
	}

	if aws.StringValue(clusterState.Cluster.Status) == eks.ClusterStatusUpdating {
		// upstream cluster is already updating, must wait until sending next update
		logrus.Infof("waiting for cluster [%s] to finish updating", config.Name)
		if config.Status.Phase != eksConfigUpdatingPhase {
			config = config.DeepCopy()
			config.Status.Phase = eksConfigUpdatingPhase
			return h.eksCC.UpdateStatus(config)
		}
		h.eksEnqueueAfter(config.Namespace, config.Name, 30*time.Second)
		return config, nil
	}

	ngs, err := eksService.ListNodegroups(
		&eks.ListNodegroupsInput{
			ClusterName: aws.String(config.Spec.DisplayName),
		})
	if err != nil {
		return config, err
	}

	// gather upstream node groups states
	var nodeGroupStates []*eks.DescribeNodegroupOutput
	nodegroupARNs := make(map[string]string)
	for _, ngName := range ngs.Nodegroups {
		ng, err := eksService.DescribeNodegroup(
			&eks.DescribeNodegroupInput{
				ClusterName:   aws.String(config.Spec.DisplayName),
				NodegroupName: ngName,
			})
		if err != nil {
			return config, err
		}
		if status := aws.StringValue(ng.Nodegroup.Status); status == eks.NodegroupStatusUpdating || status == eks.NodegroupStatusDeleting ||
			status == eks.NodegroupStatusCreating {
			if config.Status.Phase != eksConfigUpdatingPhase {
				config = config.DeepCopy()
				config.Status.Phase = eksConfigUpdatingPhase
				config, err = h.eksCC.UpdateStatus(config)
				if err != nil {
					return config, err
				}
			}
			logrus.Infof("waiting for cluster [%s] to update nodegroups [%s]", config.Name, aws.StringValue(ngName))
			h.eksEnqueueAfter(config.Namespace, config.Name, 30*time.Second)
			return config, nil
		}

		nodeGroupStates = append(nodeGroupStates, ng)
		nodegroupARNs[aws.StringValue(ngName)] = aws.StringValue(ng.Nodegroup.NodegroupArn)
	}

	upstreamSpec, clusterARN, err := BuildUpstreamClusterState(config.Spec.DisplayName, clusterState, nodeGroupStates, eksService)
	if err != nil {
		return config, err
	}

	return h.updateUpstreamClusterState(upstreamSpec, config, clusterARN, nodegroupARNs, eksService, svc)
}

func createStack(svc *cloudformation.CloudFormation, name string, displayName string,
	templateBody string, capabilities []string, parameters []*cloudformation.Parameter) (*cloudformation.DescribeStacksOutput, error) {
	_, err := svc.CreateStack(&cloudformation.CreateStackInput{
		StackName:    aws.String(name),
		TemplateBody: aws.String(templateBody),
		Capabilities: aws.StringSlice(capabilities),
		Parameters:   parameters,
		Tags: []*cloudformation.Tag{
			{Key: aws.String("displayName"), Value: aws.String(displayName)},
		},
	})
	if err != nil && !alreadyExistsInCloudFormationError(err) {
		return nil, fmt.Errorf("error creating master: %v", err)
	}

	var stack *cloudformation.DescribeStacksOutput
	status := "CREATE_IN_PROGRESS"

	for status == "CREATE_IN_PROGRESS" {
		time.Sleep(time.Second * 5)
		stack, err = svc.DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String(name),
		})
		if err != nil {
			return nil, fmt.Errorf("error polling stack info: %v", err)
		}

		status = *stack.Stacks[0].StackStatus
	}

	if len(stack.Stacks) == 0 {
		return nil, fmt.Errorf("stack did not have output: %v", err)
	}

	if status != "CREATE_COMPLETE" {
		reason := "reason unknown"
		events, err := svc.DescribeStackEvents(&cloudformation.DescribeStackEventsInput{
			StackName: aws.String(name),
		})
		if err == nil {
			for _, event := range events.StackEvents {
				// guard against nil pointer dereference
				if event.ResourceStatus == nil || event.LogicalResourceId == nil || event.ResourceStatusReason == nil {
					continue
				}

				if *event.ResourceStatus == "CREATE_FAILED" {
					reason = *event.ResourceStatusReason
					break
				}

				if *event.ResourceStatus == "ROLLBACK_IN_PROGRESS" {
					reason = *event.ResourceStatusReason
					// do not break so that CREATE_FAILED takes priority
				}
			}
		}
		return nil, fmt.Errorf("stack failed to create: %v", reason)
	}

	return stack, nil
}

func validateUpdate(config *eksv1.EKSClusterConfig) error {
	var clusterVersion *semver.Version
	if config.Spec.KubernetesVersion != nil {
		var err error
		clusterVersion, err = semver.New(fmt.Sprintf("%s.0", aws.StringValue(config.Spec.KubernetesVersion)))
		if err != nil {
			return fmt.Errorf("improper version format for cluster [%s]: %s", config.Name, aws.StringValue(config.Spec.KubernetesVersion))
		}
	}

	var errors []string
	// validate nodegroup versions
	for _, ng := range config.Spec.NodeGroups {
		if ng.Version == nil {
			continue
		}
		version, err := semver.New(fmt.Sprintf("%s.0", aws.StringValue(ng.Version)))
		if err != nil {
			errors = append(errors, fmt.Sprintf("improper version format for nodegroup [%s]: %s", aws.StringValue(ng.NodegroupName), aws.StringValue(ng.Version)))
			continue
		}
		if clusterVersion == nil {
			continue
		}
		if clusterVersion.EQ(*version) {
			continue
		}
		if clusterVersion.Minor-version.Minor == 1 {
			continue
		}
		errors = append(errors, fmt.Sprintf("versions for cluster [%s] and nodegroup [%s] not compatible: all nodegroup kubernetes versions"+
			"must be equal to or one minor version lower than the cluster kubernetes version", aws.StringValue(config.Spec.KubernetesVersion), aws.StringValue(ng.Version)))
	}
	if len(errors) != 0 {
		return fmt.Errorf(strings.Join(errors, ";"))
	}
	return nil
}

func (h *Handler) create(config *eksv1.EKSClusterConfig, sess *session.Session, eksService *eks.EKS) (*eksv1.EKSClusterConfig, error) {
	if err := validateCreate(config); err != nil {
		return config, err
	}

	if config.Spec.Imported {
		config = config.DeepCopy()
		config.Status.Phase = eksConfigImportingPhase
		return h.eksCC.UpdateStatus(config)
	}

	svc := cloudformation.New(sess)

	displayName := config.Spec.DisplayName

	var err error
	config, err = h.generateAndSetNetworking(svc, config)
	if err != nil {
		return config, err
	}

	securityGroups := aws.StringSlice(config.Status.SecurityGroups)
	subnetIds := aws.StringSlice(config.Status.Subnets)

	var roleARN string
	if aws.StringValue(config.Spec.ServiceRole) == "" {
		logrus.Infof("Creating service role")

		stack, err := createStack(svc, getServiceRoleName(config.Spec.DisplayName), displayName, templates.ServiceRoleTemplate,
			[]string{cloudformation.CapabilityCapabilityIam}, nil)
		if err != nil {
			return config, fmt.Errorf("error creating stack with service role template: %v", err)
		}

		roleARN = getParameterValueFromOutput("RoleArn", stack.Stacks[0].Outputs)
		if roleARN == "" {
			return config, fmt.Errorf("no RoleARN was returned")
		}
	} else {
		logrus.Infof("Retrieving existing service role")
		iamClient := iam.New(sess, aws.NewConfig().WithRegion(config.Spec.Region))
		role, err := iamClient.GetRole(&iam.GetRoleInput{
			RoleName: config.Spec.ServiceRole,
		})
		if err != nil {
			return config, fmt.Errorf("error getting role: %v", err)
		}

		roleARN = *role.Role.Arn
	}

	createClusterInput := &eks.CreateClusterInput{
		Name:    aws.String(config.Spec.DisplayName),
		RoleArn: aws.String(roleARN),
		ResourcesVpcConfig: &eks.VpcConfigRequest{
			EndpointPrivateAccess: config.Spec.PrivateAccess,
			EndpointPublicAccess:  config.Spec.PublicAccess,
			SecurityGroupIds:      securityGroups,
			SubnetIds:             subnetIds,
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

	if _, err := eksService.CreateCluster(createClusterInput); err != nil {
		if !isClusterConflict(err) {
			return config, fmt.Errorf("error creating cluster: %v", err)
		}
	}

	config = config.DeepCopy()
	config.Status.Phase = eksConfigCreatingPhase
	return h.eksCC.UpdateStatus(config)
}

func validateCreate(config *eksv1.EKSClusterConfig) error {
	// validate nodegroup version
	if !config.Spec.Imported {
		cannotBeNilError := "field [%s] cannot be nil for non-import cluster [%s]"
		if config.Spec.KubernetesVersion == nil {
			return fmt.Errorf(cannotBeNilError, "kubernetesVersion", config.Name)
		}
		if config.Spec.PrivateAccess == nil {
			return fmt.Errorf(cannotBeNilError, "privateAccess", config.Name)
		}
		if config.Spec.PublicAccess == nil {
			return fmt.Errorf(cannotBeNilError, "publicAccess", config.Name)
		}
		if config.Spec.SecretsEncryption == nil {
			return fmt.Errorf(cannotBeNilError, "secretsEncryption", config.Name)
		}
		if config.Spec.Tags == nil {
			return fmt.Errorf(cannotBeNilError, "tags", config.Name)
		}
		if config.Spec.Subnets == nil {
			return fmt.Errorf(cannotBeNilError, "subnets", config.Name)
		}
		if config.Spec.SecurityGroups == nil {
			return fmt.Errorf(cannotBeNilError, "securityGroups", config.Name)
		}
		if config.Spec.LoggingTypes == nil {
			return fmt.Errorf(cannotBeNilError, "loggingTypes", config.Name)
		}
		if config.Spec.PublicAccessSources == nil {
			return fmt.Errorf(cannotBeNilError, "publicAccessSources", config.Name)
		}
	}
	for _, ng := range config.Spec.NodeGroups {
		cannotBeNilError := "field [%s] cannot be nil for nodegroup [%s] in non-nil cluster [%s]"
		if !config.Spec.Imported {
			if ng.Version == nil {
				return fmt.Errorf(cannotBeNilError, "version", *ng.NodegroupName, config.Name)
			}
			if ng.MinSize == nil {
				return fmt.Errorf(cannotBeNilError, "minSize", *ng.NodegroupName, config.Name)
			}
			if ng.MaxSize == nil {
				return fmt.Errorf(cannotBeNilError, "maxSize", *ng.NodegroupName, config.Name)
			}
			if ng.DesiredSize == nil {
				return fmt.Errorf(cannotBeNilError, "desiredSize", *ng.NodegroupName, config.Name)
			}
			if ng.Version == nil {
				return fmt.Errorf(cannotBeNilError, "desiredSize", *ng.NodegroupName, config.Name)
			}
			if ng.DiskSize == nil {
				return fmt.Errorf(cannotBeNilError, "diskSize", *ng.NodegroupName, config.Name)
			}
			if ng.Gpu == nil {
				return fmt.Errorf(cannotBeNilError, "gpu", *ng.NodegroupName, config.Name)
			}
			if ng.Ec2SshKey == nil {
				return fmt.Errorf(cannotBeNilError, "ec2SshKey", *ng.NodegroupName, config.Name)
			}
			if ng.InstanceType == nil {
				return fmt.Errorf(cannotBeNilError, "instanceType", *ng.NodegroupName, config.Name)
			}
			if ng.Subnets == nil {
				return fmt.Errorf(cannotBeNilError, "subnets", *ng.NodegroupName, config.Name)
			}
			if ng.Tags == nil {
				return fmt.Errorf(cannotBeNilError, "tags", *ng.NodegroupName, config.Name)
			}
			if ng.Labels == nil {
				return fmt.Errorf(cannotBeNilError, "labels", *ng.NodegroupName, config.Name)
			}
		}
		if aws.StringValue(ng.Version) != *config.Spec.KubernetesVersion {
			return fmt.Errorf("nodegroup [%s] version must match cluster [%s] version on create", aws.StringValue(ng.NodegroupName), config.Name)
		}
	}
	return nil
}

func (h *Handler) generateAndSetNetworking(svc *cloudformation.CloudFormation, config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error) {
	if len(config.Status.Subnets) != 0 {
		// networking fields have already been set
		return config, nil
	}

	if len(config.Spec.Subnets) != 0 {
		logrus.Infof("VPC info provided, skipping vpc/subnet/securitygroup creation")
		config = config.DeepCopy()
		// copy networking fields to status
		config.Status.Subnets = config.Spec.Subnets
		config.Status.SecurityGroups = config.Spec.SecurityGroups
		config.Status.NetworkFieldsSource = "provided"
	} else {
		logrus.Infof("Bringing up vpc")

		stack, err := createStack(svc, getVPCStackName(config.Spec.DisplayName), config.Spec.DisplayName, templates.VpcTemplate, []string{},
			[]*cloudformation.Parameter{})
		if err != nil {
			return config, fmt.Errorf("error creating stack with VPC template: %v", err)
		}

		virtualNetworkString := getParameterValueFromOutput("VpcId", stack.Stacks[0].Outputs)
		securityGroupsString := getParameterValueFromOutput("SecurityGroups", stack.Stacks[0].Outputs)
		subnetIdsString := getParameterValueFromOutput("SubnetIds", stack.Stacks[0].Outputs)

		if securityGroupsString == "" || subnetIdsString == "" {
			return config, fmt.Errorf("no security groups or subnet ids were returned")
		}

		config = config.DeepCopy()
		// copy generated field to status
		config.Status.VirtualNetwork = virtualNetworkString
		config.Status.SecurityGroups = strings.Split(securityGroupsString, ",")
		config.Status.Subnets = strings.Split(subnetIdsString, ",")
		config.Status.NetworkFieldsSource = "generated"
	}

	return h.eksCC.UpdateStatus(config)
}

func StartAWSSessions(secretsCache wranglerv1.SecretCache, spec eksv1.EKSClusterConfigSpec) (*session.Session, *eks.EKS, error) {
	awsConfig := &aws.Config{}

	if region := spec.Region; region != "" {
		awsConfig.Region = aws.String(region)
	}

	ns, id := utils.Parse(spec.AmazonCredentialSecret)
	if amazonCredentialSecret := spec.AmazonCredentialSecret; amazonCredentialSecret != "" {
		secret, err := secretsCache.Get(ns, id)
		if err != nil {
			return nil, nil, err
		}

		accessKeyBytes := secret.Data["amazonec2credentialConfig-accessKey"]
		secretKeyBytes := secret.Data["amazonec2credentialConfig-secretKey"]
		if accessKeyBytes == nil || secretKeyBytes == nil {
			return nil, nil, fmt.Errorf("invalid aws cloud credential")
		}

		accessKey := string(accessKeyBytes)
		secretKey := string(secretKeyBytes)

		awsConfig.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting new aws session: %v", err)
	}
	return sess, eks.New(sess), nil
}

func (h *Handler) waitForCreationComplete(config *eksv1.EKSClusterConfig, eksService *eks.EKS, svc *cloudformation.CloudFormation) (*eksv1.EKSClusterConfig, error) {
	var err error

	state, err := eksService.DescribeCluster(
		&eks.DescribeClusterInput{
			Name: aws.String(config.Spec.DisplayName),
		})
	if err != nil {
		return config, err
	}

	if state.Cluster == nil {
		return config, fmt.Errorf("no cluster data was returned")
	}

	if state.Cluster.Status == nil {
		return config, fmt.Errorf("no cluster status was returned")
	}

	status := *state.Cluster.Status
	if status == eks.ClusterStatusFailed {
		return config, fmt.Errorf("creation failed for cluster named %q with ARN %q",
			aws.StringValue(state.Cluster.Name),
			aws.StringValue(state.Cluster.Arn))
	}

	if status == eks.ClusterStatusActive {
		if err := h.createCASecret(config, state); err != nil {
			return config, err
		}
		logrus.Infof("cluster [%s] created successfully", config.Name)
		config = config.DeepCopy()
		config.Status.Phase = eksConfigActivePhase
		return h.eksCC.UpdateStatus(config)
	}

	logrus.Infof("waiting for cluster [%s] to finish creating", config.Name)
	h.eksEnqueueAfter(config.Namespace, config.Name, 30*time.Second)

	return config, nil
}

// buildUpstreamClusterState
func BuildUpstreamClusterState(name string, clusterState *eks.DescribeClusterOutput, nodeGroupStates []*eks.DescribeNodegroupOutput, eksService *eks.EKS) (*eksv1.EKSClusterConfigSpec, string, error) {
	upstreamSpec := &eksv1.EKSClusterConfigSpec{}

	upstreamSpec.Imported = true

	// set kubernetes version
	upstreamVersion := aws.StringValue(clusterState.Cluster.Version)
	if upstreamVersion == "" {
		return nil, "", fmt.Errorf("cannot detect cluster [%s] upstream kubernetes version", name)
	}
	upstreamSpec.KubernetesVersion = aws.String(upstreamVersion)

	// set  tags
	upstreamSpec.Tags = make(map[string]string)
	if len(clusterState.Cluster.Tags) != 0 {
		upstreamSpec.Tags = aws.StringValueMap(clusterState.Cluster.Tags)
	}

	// set public access
	if hasPublicAccess := clusterState.Cluster.ResourcesVpcConfig.EndpointPublicAccess; hasPublicAccess == nil || *hasPublicAccess {
		upstreamSpec.PublicAccess = aws.Bool(true)
	} else {
		upstreamSpec.PublicAccess = aws.Bool(false)
	}

	// set private access
	if hasPrivateAccess := clusterState.Cluster.ResourcesVpcConfig.EndpointPrivateAccess; hasPrivateAccess != nil && *hasPrivateAccess {
		upstreamSpec.PrivateAccess = aws.Bool(true)
	} else {
		upstreamSpec.PrivateAccess = aws.Bool(false)
	}

	// set public access sources
	upstreamSpec.PublicAccessSources = make([]string, 0)
	if publicAccessSources := aws.StringValueSlice(clusterState.Cluster.ResourcesVpcConfig.PublicAccessCidrs); len(publicAccessSources) > 0 {
		upstreamSpec.PublicAccessSources = publicAccessSources
	}

	// set logging
	upstreamSpec.LoggingTypes = make([]string, 0)
	if clusterState.Cluster.Logging != nil {
		if clusterLogging := clusterState.Cluster.Logging.ClusterLogging; len(clusterLogging) > 0 {
			setup := clusterLogging[0]
			if aws.BoolValue(setup.Enabled) {
				upstreamSpec.LoggingTypes = aws.StringValueSlice(setup.Types)
			}
		}
	}

	// set node groups
	upstreamSpec.NodeGroups = make([]eksv1.NodeGroup, 0, len(nodeGroupStates))
	for _, ng := range nodeGroupStates {
		if aws.StringValue(ng.Nodegroup.Status) == eks.NodegroupStatusDeleting {
			continue
		}
		ngToAdd := eksv1.NodeGroup{
			NodegroupName: ng.Nodegroup.NodegroupName,
			DiskSize:      ng.Nodegroup.DiskSize,
			Labels:        ng.Nodegroup.Labels,
			DesiredSize:   ng.Nodegroup.ScalingConfig.DesiredSize,
			MaxSize:       ng.Nodegroup.ScalingConfig.MaxSize,
			MinSize:       ng.Nodegroup.ScalingConfig.MinSize,
			Subnets:       aws.StringValueSlice(ng.Nodegroup.Subnets),
			Tags:          ng.Nodegroup.Tags,
			Version:       ng.Nodegroup.Version,
		}
		if len(ng.Nodegroup.InstanceTypes) > 0 {
			ngToAdd.InstanceType = ng.Nodegroup.InstanceTypes[0]
		}
		if aws.StringValue(ng.Nodegroup.AmiType) == eks.AMITypesAl2X8664Gpu {
			ngToAdd.Gpu = aws.Bool(true)
		} else if aws.StringValue(ng.Nodegroup.AmiType) == eks.AMITypesAl2X8664 {
			ngToAdd.Gpu = aws.Bool(false)
		}
		if ng.Nodegroup.RemoteAccess != nil {
			ngToAdd.Ec2SshKey = ng.Nodegroup.RemoteAccess.Ec2SshKey
		}
		upstreamSpec.NodeGroups = append(upstreamSpec.NodeGroups, ngToAdd)
	}

	// set subnets
	upstreamSpec.Subnets = aws.StringValueSlice(clusterState.Cluster.ResourcesVpcConfig.SubnetIds)
	// set security groups
	upstreamSpec.SecurityGroups = aws.StringValueSlice(clusterState.Cluster.ResourcesVpcConfig.SecurityGroupIds)

	upstreamSpec.SecretsEncryption = aws.Bool(len(clusterState.Cluster.EncryptionConfig) != 0)
	upstreamSpec.KmsKey = aws.String("")
	if len(clusterState.Cluster.EncryptionConfig) > 0 {
		upstreamSpec.KmsKey = clusterState.Cluster.EncryptionConfig[0].Provider.KeyArn
	}

	upstreamSpec.ServiceRole = clusterState.Cluster.RoleArn
	if upstreamSpec.ServiceRole == nil {
		upstreamSpec.ServiceRole = aws.String("")
	}
	return upstreamSpec, aws.StringValue(clusterState.Cluster.Arn), nil
}

// updateUpstreamClusterState compares the upstream spec with the config spec, then updates the upstream EKS cluster to
// match the config spec. Function often returns after a single update because once the cluster is in updating phase in EKS,
// no more updates will be accepted until the current update is finished.
func (h *Handler) updateUpstreamClusterState(upstreamSpec *eksv1.EKSClusterConfigSpec, config *eksv1.EKSClusterConfig, clusterARN string, ngARNs map[string]string, eksService *eks.EKS, svc *cloudformation.CloudFormation) (*eksv1.EKSClusterConfig, error) {
	// check kubernetes version for update
	if config.Spec.KubernetesVersion != nil {
		if aws.StringValue(upstreamSpec.KubernetesVersion) != aws.StringValue(config.Spec.KubernetesVersion) {
			logrus.Infof("updating kubernetes version for cluster [%s]", config.Name)
			_, err := eksService.UpdateClusterVersion(&eks.UpdateClusterVersionInput{
				Name:    aws.String(config.Spec.DisplayName),
				Version: config.Spec.KubernetesVersion,
			})
			if err != nil {
				return config, err
			}
			return h.enqueueUpdate(config)
		}
	}

	// check tags for update
	if config.Spec.Tags != nil {
		if updateTags := utils.GetKeyValuesToUpdate(config.Spec.Tags, upstreamSpec.Tags); updateTags != nil {
			_, err := eksService.TagResource(
				&eks.TagResourceInput{
					ResourceArn: aws.String(clusterARN),
					Tags:        updateTags,
				})
			if err != nil {
				return config, err
			}
		}

		if updateUntags := utils.GetKeysToDelete(config.Spec.Tags, upstreamSpec.Tags); updateUntags != nil {
			_, err := eksService.UntagResource(
				&eks.UntagResourceInput{
					ResourceArn: aws.String(clusterARN),
					TagKeys:     updateUntags,
				})
			if err != nil {
				return config, err
			}
		}
	}

	if config.Spec.LoggingTypes != nil {
		// check logging for update
		if loggingTypesUpdate := getLoggingTypesUpdate(config.Spec.LoggingTypes, upstreamSpec.LoggingTypes); loggingTypesUpdate != nil {
			_, err := eksService.UpdateClusterConfig(
				&eks.UpdateClusterConfigInput{
					Name:    aws.String(config.Spec.DisplayName),
					Logging: loggingTypesUpdate,
				},
			)
			if err != nil {
				return config, err
			}
			return h.enqueueUpdate(config)
		}
	}

	publicAccessUpdate := config.Spec.PublicAccess != nil && aws.BoolValue(upstreamSpec.PublicAccess) != aws.BoolValue(config.Spec.PublicAccess)
	privateAccessUpdate := config.Spec.PrivateAccess != nil && aws.BoolValue(upstreamSpec.PrivateAccess) != aws.BoolValue(config.Spec.PrivateAccess)
	if publicAccessUpdate || privateAccessUpdate {
		// public and private access updates need to be sent together. When they are sent one at a time
		// the request may be denied due to having both public and private access disabled.
		_, err := eksService.UpdateClusterConfig(
			&eks.UpdateClusterConfigInput{
				Name: aws.String(config.Spec.DisplayName),
				ResourcesVpcConfig: &eks.VpcConfigRequest{
					EndpointPublicAccess: config.Spec.PublicAccess,
					EndpointPrivateAccess: config.Spec.PrivateAccess,
				},
			},
		)
		if err != nil {
			return config, err
		}
		return h.enqueueUpdate(config)
	}

	if config.Spec.PublicAccessSources != nil {
		// check public access CIDRs for update (public access sources)
		filteredSpecPublicAccessSources := filterPublicAccessSources(config.Spec.PublicAccessSources)
		filteredUpstreamPublicAccessSources := filterPublicAccessSources(upstreamSpec.PublicAccessSources)
		if !utils.CompareStringSliceElements(filteredSpecPublicAccessSources, filteredUpstreamPublicAccessSources) {
			_, err := eksService.UpdateClusterConfig(
				&eks.UpdateClusterConfigInput{
					Name: aws.String(config.Spec.DisplayName),
					ResourcesVpcConfig: &eks.VpcConfigRequest{
						PublicAccessCidrs: getPublicAccessCidrs(config.Spec.PublicAccessSources),
					},
				},
			)
			if err != nil {
				return config, err
			}

			return h.enqueueUpdate(config)
		}
	}

	if config.Spec.NodeGroups == nil {
		logrus.Infof("cluster [%s] finished updating", config.Name)
		config = config.DeepCopy()
		config.Status.Phase = eksConfigActivePhase
		return h.eksCC.UpdateStatus(config)
	}

	// check nodegroups for updates

	upstreamNgs := make(map[string]eksv1.NodeGroup)
	ngs := make(map[string]eksv1.NodeGroup)

	for _, ng := range upstreamSpec.NodeGroups {
		upstreamNgs[aws.StringValue(ng.NodegroupName)] = ng
	}

	for _, ng := range config.Spec.NodeGroups {
		ngs[aws.StringValue(ng.NodegroupName)] = ng
	}

	// check if node groups need to be created
	var updatingNodegroups bool
	for _, ng := range config.Spec.NodeGroups {
		if _, ok := upstreamNgs[aws.StringValue(ng.NodegroupName)]; ok {
			continue
		}
		// in this case update is set right away because creating the
		// nodegroup may not be immediate
		if config.Status.Phase != eksConfigUpdatingPhase {
			config = config.DeepCopy()
			config.Status.Phase = eksConfigUpdatingPhase
			var err error
			config, err = h.eksCC.UpdateStatus(config)
			if err != nil {
				return config, err
			}
		}
		err := createNodeGroup(config, ng, eksService, svc)
		if err != nil {
			return config, err
		}
		updatingNodegroups = true
	}

	// check if node groups need to be deleted
	for _, ng := range upstreamSpec.NodeGroups {
		if _, ok := ngs[aws.StringValue(ng.NodegroupName)]; ok {
			continue
		}
		_, err := eksService.DeleteNodegroup(
			&eks.DeleteNodegroupInput{
				ClusterName:   aws.String(config.Spec.DisplayName),
				NodegroupName: ng.NodegroupName,
			})
		if err != nil {
			return config, err
		}
		updatingNodegroups = true
	}

	if updatingNodegroups {
		return h.enqueueUpdate(config)
	}

	// check node groups for kubernetes version updates
	desiredNgVersions := make(map[string]string)
	for _, ng := range config.Spec.NodeGroups {
		if ng.Version != nil {
			desiredVersion := aws.StringValue(ng.Version)
			if desiredVersion == "" {
				desiredVersion = aws.StringValue(config.Spec.KubernetesVersion)
			}
			desiredNgVersions[aws.StringValue(ng.NodegroupName)] = desiredVersion
		}
	}

	var attemptUpdatingNodegroups bool
	for _, upstreamNg := range upstreamSpec.NodeGroups {
		// if continue is used after an update, it means that update
		// must finish before others for that nodegroup can take place.
		// Some updates such as minSize, maxSize, and desiredSize can
		// happen together

		ng := ngs[aws.StringValue(upstreamNg.NodegroupName)]
		if ng.Version != nil {
			if aws.StringValue(upstreamNg.Version) != desiredNgVersions[aws.StringValue(ng.NodegroupName)] {
				attemptUpdatingNodegroups = true
				_, err := eksService.UpdateNodegroupVersion(
					&eks.UpdateNodegroupVersionInput{
						NodegroupName: aws.String(aws.StringValue(ng.NodegroupName)),
						ClusterName:   aws.String(config.Spec.DisplayName),
						Version:       aws.String(desiredNgVersions[aws.StringValue(ng.NodegroupName)]),
					},
				)
				if err != nil {
					return config, err
				}
				continue
			}
		}

		updateNodegroupConfig := &eks.UpdateNodegroupConfigInput{
			ClusterName:   aws.String(config.Spec.DisplayName),
			NodegroupName: ng.NodegroupName,
		}
		updateNodegroupConfig.ScalingConfig = &eks.NodegroupScalingConfig{}
		var sendUpdateNodegroupConfig bool

		if ng.Labels != nil {
			if unlabels := utils.GetKeysToDelete(aws.StringValueMap(ng.Labels), aws.StringValueMap(upstreamNg.Labels)); unlabels != nil {
				updateNodegroupConfig.Labels = &eks.UpdateLabelsPayload{
					RemoveLabels: unlabels,
				}
				sendUpdateNodegroupConfig = true
			}

			if labels := utils.GetKeyValuesToUpdate(aws.StringValueMap(ng.Labels), aws.StringValueMap(upstreamNg.Labels)); labels != nil {
				updateNodegroupConfig.Labels = &eks.UpdateLabelsPayload{
					AddOrUpdateLabels: labels,
				}
				sendUpdateNodegroupConfig = true
			}
		}

		if ng.DesiredSize != nil {
			if aws.Int64Value(upstreamNg.DesiredSize) != aws.Int64Value(ng.DesiredSize) {
				updateNodegroupConfig.ScalingConfig.DesiredSize = ng.DesiredSize
				sendUpdateNodegroupConfig = true
			}
		}

		if ng.MinSize != nil {
			if aws.Int64Value(upstreamNg.MinSize) != aws.Int64Value(ng.MinSize) {
				updateNodegroupConfig.ScalingConfig.MinSize = ng.MinSize
				sendUpdateNodegroupConfig = true
			}
		}

		if ng.MaxSize != nil {
			if aws.Int64Value(upstreamNg.MaxSize) != aws.Int64Value(ng.MaxSize) {
				updateNodegroupConfig.ScalingConfig.MaxSize = ng.MaxSize
				sendUpdateNodegroupConfig = true
			}
		}

		if sendUpdateNodegroupConfig {
			attemptUpdatingNodegroups = true
			_, err := eksService.UpdateNodegroupConfig(updateNodegroupConfig)
			if err != nil {
				return config, err
			}
			continue
		}

		if ng.Tags != nil {
			if untags := utils.GetKeysToDelete(aws.StringValueMap(ng.Tags), aws.StringValueMap(upstreamNg.Tags)); untags != nil {
				_, err := eksService.UntagResource(&eks.UntagResourceInput{
					ResourceArn: aws.String(ngARNs[aws.StringValue(ng.NodegroupName)]),
					TagKeys:     untags,
				})
				if err != nil {
					return config, err
				}
				attemptUpdatingNodegroups = true
			}

			if tags := utils.GetKeyValuesToUpdate(aws.StringValueMap(ng.Tags), aws.StringValueMap(upstreamNg.Tags)); tags != nil {
				_, err := eksService.TagResource(&eks.TagResourceInput{
					ResourceArn: aws.String(ngARNs[aws.StringValue(ng.NodegroupName)]),
					Tags:        tags,
				})
				if err != nil {
					return config, err
				}
				attemptUpdatingNodegroups = true
			}
		}
	}

	if attemptUpdatingNodegroups {
		// if any updates are taking place on nodegroups, the config's phase needs
		// to be set to "updating" and the controller will wait for the updates to
		// finish before proceeding
		return h.enqueueUpdate(config)
	}

	// no new updates, set to active
	if config.Status.Phase != eksConfigActivePhase {
		logrus.Infof("cluster [%s] finished updating", config.Name)
		config = config.DeepCopy()
		config.Status.Phase = eksConfigActivePhase
		return h.eksCC.UpdateStatus(config)
	}

	// check for node groups updates here
	return config, nil
}

// importCluster cluster returns a spec representing the upstream state of the cluster matching to the
// given config's displayName and region.
func (h *Handler) importCluster(config *eksv1.EKSClusterConfig, eksService *eks.EKS) (*eksv1.EKSClusterConfig, error) {
	clusterState, err := eksService.DescribeCluster(
		&eks.DescribeClusterInput{
			Name: aws.String(config.Spec.DisplayName),
		})
	if err != nil {
		return config, err
	}

	if err := h.createCASecret(config, clusterState); err != nil {
		if !errors.IsAlreadyExists(err) {
			return config, err
		}
	}

	config.Status.Subnets = aws.StringValueSlice(clusterState.Cluster.ResourcesVpcConfig.SubnetIds)
	config.Status.SecurityGroups = aws.StringValueSlice(clusterState.Cluster.ResourcesVpcConfig.SecurityGroupIds)
	config.Status.Phase = eksConfigActivePhase
	return h.eksCC.UpdateStatus(config)
}

// createCASecret creates a secret containing ca and endpoint. These can be used to create a kubeconfig via
// the go sdk
func (h *Handler) createCASecret(config *eksv1.EKSClusterConfig, clusterState *eks.DescribeClusterOutput) error {
	endpoint := aws.StringValue(clusterState.Cluster.Endpoint)
	ca := aws.StringValue(clusterState.Cluster.CertificateAuthority.Data)

	_, err := h.secrets.Create(
		&v1.Secret{
			ObjectMeta: v15.ObjectMeta{
				Name:      config.Name,
				Namespace: config.Namespace,
				OwnerReferences: []v15.OwnerReference{
					{
						APIVersion: eksv1.SchemeGroupVersion.String(),
						Kind:       eksClusterConfigKind,
						UID:        config.UID,
						Name:       config.Name,
					},
				},
			},
			Data: map[string][]byte{
				"endpoint": []byte(endpoint),
				"ca":       []byte(ca),
			},
		})
	return err
}

// enqueueUpdate enqueues the config if it is already in the updating phase. Otherwise, the
// phase is updated to "updating". This is important because the object needs to reenter the
// onChange handler to start waiting on the update.
func (h *Handler) enqueueUpdate(config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error) {
	if config.Status.Phase == eksConfigUpdatingPhase {
		h.eksEnqueue(config.Namespace, config.Name)
		return config, nil
	}
	config = config.DeepCopy()
	config.Status.Phase = eksConfigUpdatingPhase
	return h.eksCC.UpdateStatus(config)
}

func getVPCStackName(name string) string {
	return name + "-eks-vpc"
}

func getServiceRoleName(name string) string {
	return name + "-eks-service-role"
}

func getParameterValueFromOutput(key string, outputs []*cloudformation.Output) string {
	for _, output := range outputs {
		if *output.OutputKey == key {
			return *output.OutputValue
		}
	}

	return ""
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

func isClusterConflict(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeResourceInUseException
	}

	return false
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

func getTags(tags map[string]string) map[string]*string {
	if len(tags) == 0 {
		return nil
	}

	return aws.StringMap(tags)
}

func getPublicAccessCidrs(publicAccessCidrs []string) []*string {
	if len(publicAccessCidrs) == 0 {
		return aws.StringSlice([]string{"0.0.0.0/0"})
	}

	return aws.StringSlice(publicAccessCidrs)
}

func getLoggingTypesUpdate(loggingTypes []string, upstreamLoggingTypes []string) *eks.Logging {
	loggingUpdate := &eks.Logging{}

	if loggingTypesToDisable := getLoggingTypesToDisable(loggingTypes, upstreamLoggingTypes); loggingTypesToDisable != nil {
		loggingUpdate.ClusterLogging = append(loggingUpdate.ClusterLogging, loggingTypesToDisable)
	}

	if loggingTypesToEnable := getLoggingTypesToEnable(loggingTypes, upstreamLoggingTypes); loggingTypesToEnable != nil {
		loggingUpdate.ClusterLogging = append(loggingUpdate.ClusterLogging, loggingTypesToEnable)
	}

	if len(loggingUpdate.ClusterLogging) > 0 {
		return loggingUpdate
	}

	return nil
}

func getLoggingTypesToDisable(loggingTypes []string, upstreamLoggingTypes []string) *eks.LogSetup {
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
		return &eks.LogSetup{
			Enabled: aws.Bool(false),
			Types:   aws.StringSlice(loggingTypesToDisable),
		}
	}

	return nil
}

func getLoggingTypesToEnable(loggingTypes []string, upstreamLoggingTypes []string) *eks.LogSetup {
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
		return &eks.LogSetup{
			Enabled: aws.Bool(true),
			Types:   aws.StringSlice(loggingTypesToEnable),
		}
	}

	return nil
}

func createNodeGroup(eksConfig *eksv1.EKSClusterConfig, group eksv1.NodeGroup, eksService *eks.EKS, svc *cloudformation.CloudFormation) error {
	nodeGroupCreateInput := &eks.CreateNodegroupInput{
		ClusterName:   aws.String(eksConfig.Spec.DisplayName),
		NodegroupName: group.NodegroupName,
		DiskSize:      group.DiskSize,
		InstanceTypes: []*string{group.InstanceType},
		Labels:        group.Labels,
		ScalingConfig: &eks.NodegroupScalingConfig{
			DesiredSize: group.DesiredSize,
			MaxSize:     group.MaxSize,
			MinSize:     group.MinSize,
		},
	}

	if gpu := group.Gpu; aws.BoolValue(gpu) {
		nodeGroupCreateInput.AmiType = aws.String(eks.AMITypesAl2X8664Gpu)
	} else {
		nodeGroupCreateInput.AmiType = aws.String(eks.AMITypesAl2X8664)
	}

	if sshKey := group.Ec2SshKey; aws.StringValue(sshKey) != "" {
		nodeGroupCreateInput.RemoteAccess = &eks.RemoteAccessConfig{
			Ec2SshKey: sshKey,
		}
	}

	if len(group.Subnets) != 0 {
		nodeGroupCreateInput.Subnets = aws.StringSlice(group.Subnets)
	} else {
		nodeGroupCreateInput.Subnets = aws.StringSlice(eksConfig.Status.Subnets)
	}

	finalTemplate := fmt.Sprintf(templates.NodeInstanceRoleTemplate, getEC2ServiceEndpoint(eksConfig.Spec.Region))
	output, err := createStack(svc, fmt.Sprintf("%s-node-instance-role", eksConfig.Spec.DisplayName), eksConfig.Spec.DisplayName, finalTemplate, []string{cloudformation.CapabilityCapabilityIam}, []*cloudformation.Parameter{})
	if err != nil {
		return err
	}

	nodeGroupCreateInput.NodeRole = aws.String(getParameterValueFromOutput("NodeInstanceRole", output.Stacks[0].Outputs))
	_, err = eksService.CreateNodegroup(nodeGroupCreateInput)
	return err
}

func getEC2ServiceEndpoint(region string) string {
	if p, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region); ok {
		return fmt.Sprintf("%s.%s", ec2.ServiceName, p.DNSSuffix())
	}
	return "ec2.amazonaws.com"
}

func deleteStack(svc *cloudformation.CloudFormation, newStyleName, oldStyleName string) error {
	name := newStyleName
	_, err := svc.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(name),
	})
	if doesNotExist(err) {
		name = oldStyleName
	}

	_, err = svc.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(name),
	})
	if err != nil {
		return fmt.Errorf("error deleting stack: %v", err)
	}

	return nil
}

func doesNotExist(err error) bool {
	// There is no better way of doing this because AWS API does not distinguish between a attempt to delete a stack
	// (or key pair) that does not exist, and, for example, a malformed delete request, so we have to parse the error
	// message
	if err != nil {
		return strings.Contains(err.Error(), "does not exist")
	}

	return false
}

func notFound(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeResourceNotFoundException
	}

	return false
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
