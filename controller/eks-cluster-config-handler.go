package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/blang/semver"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	awsservices "github.com/rancher/eks-operator/pkg/eks"
	"github.com/rancher/eks-operator/pkg/eks/services"
	ekscontrollers "github.com/rancher/eks-operator/pkg/generated/controllers/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/templates"
	"github.com/rancher/eks-operator/utils"
	wranglerv1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/retry"
)

const (
	controllerName           = "eks-controller"
	controllerRemoveName     = "eks-controller-remove"
	eksConfigCreatingPhase   = "creating"
	eksConfigNotCreatedPhase = ""
	eksConfigActivePhase     = "active"
	eksConfigUpdatingPhase   = "updating"
	eksConfigImportingPhase  = "importing"
	eksClusterConfigKind     = "EKSClusterConfig"
)

type Handler struct {
	eksCC           ekscontrollers.EKSClusterConfigClient
	eksEnqueueAfter func(namespace, name string, duration time.Duration)
	eksEnqueue      func(namespace, name string)
	secrets         wranglerv1.SecretClient
	secretsCache    wranglerv1.SecretCache
}

type awsServices struct {
	cloudformation services.CloudFormationServiceInterface
	eks            services.EKSServiceInterface
	ec2            services.EC2ServiceInterface
	iam            services.IAMServiceInterface
}

func Register(
	ctx context.Context,
	secrets wranglerv1.SecretController,
	eks ekscontrollers.EKSClusterConfigController) {
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

func (h *Handler) OnEksConfigChanged(_ string, config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error) {
	if config == nil {
		return nil, nil
	}

	if config.DeletionTimestamp != nil {
		return nil, nil
	}

	awsSVCs, err := newAWSServices(h.secretsCache, config.Spec)
	if err != nil {
		return config, fmt.Errorf("error creating new AWS services: %w", err)
	}

	switch config.Status.Phase {
	case eksConfigImportingPhase:
		return h.importCluster(config, awsSVCs)
	case eksConfigNotCreatedPhase:
		return h.create(config, awsSVCs)
	case eksConfigCreatingPhase:
		return h.waitForCreationComplete(config, awsSVCs)
	case eksConfigActivePhase, eksConfigUpdatingPhase:
		return h.checkAndUpdate(config, awsSVCs)
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
				// The update is valid in that the controller should retry but there is no actionable resolution as far
				// as a user is concerned. An update has either been initiated by the eks-operator or another source
				// is already in progress. It is possible an update is not being immediately reflected in the upstream
				// cluster state. The config object will reenter the controller and then the controller will wait for
				// the update to finish.
				message = err.Error()
			}

			messageNoMeta, err := removeErrorMetadata(message)
			if err != nil {
				logrus.Errorf("Error removing metadata from failure message: %s", err.Error())
			} else {
				message = messageNoMeta
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

func removeErrorMetadata(message string) (string, error) {
	// failure message
	type RespMetadata struct {
		StatusCode int    `json:"statusCode"`
		RequestID  string `json:"requestID"`
	}

	type Message struct {
		RespMetadata  RespMetadata `json:"respMetadata"`
		ClusterName   string       `json:"clusterName"`
		Message_      string       `json:"message_"` // nolint:revive
		NodegroupName string       `json:"nodegroupName"`
	}

	// failure message with no meta
	type FailureMessage struct {
		ClusterName   string `json:"clusterName"`
		Message_      string `json:"message_"` // nolint:revive
		NodegroupName string `json:"nodegroupName"`
	}

	// Remove the first line of the message because it usually contains the name of an Amazon EKS error type that
	// implements Serializable (ex: ResourceInUseException). That name is unpredictable depending on the error. We
	// only need cluster name, message, and node group.
	index := strings.Index(message, "{")
	if index == -1 {
		return "", fmt.Errorf("message body not formatted as expected")
	}
	message = message[index:]

	// unmarshal json error to an object
	in := []byte(message)
	failureMessage := Message{}
	err := yaml.Unmarshal(in, &failureMessage)
	if err != nil {
		return "", err
	}

	// add error message fields without metadata to new object
	failureMessageNoMeta := FailureMessage{
		ClusterName:   failureMessage.ClusterName,
		Message_:      failureMessage.Message_,
		NodegroupName: failureMessage.NodegroupName,
	}

	str := fmt.Sprintf("%#v", failureMessageNoMeta)
	return str, nil
}

func (h *Handler) OnEksConfigRemoved(_ string, config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error) {
	awsSVCs, err := newAWSServices(h.secretsCache, config.Spec)
	if err != nil {
		return config, fmt.Errorf("error creating new AWS services: %w", err)
	}

	if config.Spec.Imported {
		logrus.Infof("cluster [%s] is imported, will not delete EKS cluster", config.Name)
		return config, nil
	}
	if config.Status.Phase == eksConfigNotCreatedPhase {
		// The most likely context here is that the cluster already existed in EKS, so we shouldn't delete it
		logrus.Warnf("cluster [%s] never advanced to creating status, will not delete EKS cluster", config.Name)
		return config, nil
	}

	logrus.Infof("deleting cluster [%s]", config.Name)

	logrus.Infof("starting node group deletion for config [%s]", config.Spec.DisplayName)
	waitingForNodegroupDeletion := true
	for waitingForNodegroupDeletion {
		waitingForNodegroupDeletion, err = deleteNodeGroups(config, config.Spec.NodeGroups, awsSVCs.eks)
		if err != nil {
			return config, fmt.Errorf("error deleting nodegroups for config [%s]", config.Spec.DisplayName)
		}
		time.Sleep(10 * time.Second)
		logrus.Infof("waiting for config [%s] node groups to delete", config.Name)
	}

	if config.Status.ManagedLaunchTemplateID != "" {
		logrus.Infof("deleting common launch template for config [%s]", config.Name)
		deleteLaunchTemplate(config.Status.ManagedLaunchTemplateID, awsSVCs.ec2)
	}

	logrus.Infof("starting control plane deletion for config [%s]", config.Name)
	_, err = awsSVCs.eks.DeleteCluster(&eks.DeleteClusterInput{
		Name: aws.String(config.Spec.DisplayName),
	})
	if err != nil {
		if notFound(err) {
			_, err = awsSVCs.eks.DeleteCluster(&eks.DeleteClusterInput{
				Name: aws.String(config.Spec.DisplayName),
			})
		}

		if err != nil && !notFound(err) {
			return config, fmt.Errorf("error deleting cluster: %v", err)
		}
	}

	if aws.BoolValue(config.Spec.EBSCSIDriver) {
		logrus.Infof("deleting ebs csi driver role for config [%s]", config.Name)
		if err := deleteStack(awsSVCs.cloudformation, getEBSCSIDriverRoleStackName(config.Spec.DisplayName), getEBSCSIDriverRoleStackName(config.Spec.DisplayName)); err != nil {
			return config, fmt.Errorf("error ebs csi driver role stack: %v", err)
		}
	}

	if aws.StringValue(config.Spec.ServiceRole) == "" {
		logrus.Infof("deleting service role for config [%s]", config.Name)
		if err := deleteStack(awsSVCs.cloudformation, getServiceRoleName(config.Spec.DisplayName), getServiceRoleName(config.Spec.DisplayName)); err != nil {
			return config, fmt.Errorf("error deleting service role stack: %v", err)
		}
	}

	if len(config.Spec.Subnets) == 0 {
		logrus.Infof("deleting vpc, subnets, and security groups for config [%s]", config.Name)
		if err := deleteStack(awsSVCs.cloudformation, getVPCStackName(config.Spec.DisplayName), getVPCStackName(config.Spec.DisplayName)); err != nil {
			return config, fmt.Errorf("error deleting vpc stack: %v", err)
		}
	}

	logrus.Infof("deleting node instance role for config [%s]", config.Name)
	if err := deleteStack(awsSVCs.cloudformation, fmt.Sprintf("%s-node-instance-role", config.Spec.DisplayName), fmt.Sprintf("%s-node-instance-role", config.Spec.DisplayName)); err != nil {
		return config, fmt.Errorf("error deleting worker node stack: %v", err)
	}

	return config, err
}

func (h *Handler) checkAndUpdate(config *eksv1.EKSClusterConfig, awsSVCs *awsServices) (*eksv1.EKSClusterConfig, error) {
	if awsSVCs == nil {
		return config, fmt.Errorf("aws services not initialized")
	}

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

	clusterState, err := awsservices.GetClusterState(&awsservices.GetClusterStatusOpts{
		EKSService: awsSVCs.eks,
		Config:     config,
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

	ngs, err := awsSVCs.eks.ListNodegroups(
		&eks.ListNodegroupsInput{
			ClusterName: aws.String(config.Spec.DisplayName),
		})
	if err != nil {
		return config, err
	}

	// gather upstream node groups states
	nodeGroupStates := make([]*eks.DescribeNodegroupOutput, 0, len(ngs.Nodegroups))
	nodegroupARNs := make(map[string]string)
	for _, ngName := range ngs.Nodegroups {
		ng, err := awsSVCs.eks.DescribeNodegroup(
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

	if config.Status.Phase == eksConfigActivePhase && len(config.Status.TemplateVersionsToDelete) != 0 {
		// If there are any launch template versions that need to be cleaned up, we do it now.
		awsservices.DeleteLaunchTemplateVersions(awsSVCs.ec2, config.Status.ManagedLaunchTemplateID, aws.StringSlice(config.Status.TemplateVersionsToDelete))
		config = config.DeepCopy()
		config.Status.TemplateVersionsToDelete = nil
		return h.eksCC.UpdateStatus(config)
	}

	upstreamSpec, clusterARN, err := BuildUpstreamClusterState(config.Spec.DisplayName, config.Status.ManagedLaunchTemplateID, clusterState, nodeGroupStates, awsSVCs.ec2, true)
	if err != nil {
		return config, err
	}

	return h.updateUpstreamClusterState(upstreamSpec, config, awsSVCs, clusterARN, nodegroupARNs)
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

	errs := make([]string, 0)
	// validate nodegroup versions
	for _, ng := range config.Spec.NodeGroups {
		if ng.Version == nil {
			continue
		}
		version, err := semver.New(fmt.Sprintf("%s.0", aws.StringValue(ng.Version)))
		if err != nil {
			errs = append(errs, fmt.Sprintf("improper version format for nodegroup [%s]: %s", aws.StringValue(ng.NodegroupName), aws.StringValue(ng.Version)))
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
		errs = append(errs, fmt.Sprintf("versions for cluster [%s] and nodegroup [%s] not compatible: all nodegroup kubernetes versions"+
			"must be equal to or one minor version lower than the cluster kubernetes version", aws.StringValue(config.Spec.KubernetesVersion), aws.StringValue(ng.Version)))
	}
	if len(errs) != 0 {
		return fmt.Errorf(strings.Join(errs, ";"))
	}
	return nil
}

func (h *Handler) create(config *eksv1.EKSClusterConfig, awsSVCs *awsServices) (*eksv1.EKSClusterConfig, error) {
	if awsSVCs == nil {
		return config, fmt.Errorf("aws services not initialized")
	}

	if err := h.validateCreate(config, awsSVCs); err != nil {
		return config, err
	}

	if config.Spec.Imported {
		config = config.DeepCopy()
		config.Status.Phase = eksConfigImportingPhase
		return h.eksCC.UpdateStatus(config)
	}

	config, err := h.generateAndSetNetworking(config, awsSVCs)
	if err != nil {
		return config, fmt.Errorf("error generating and setting networking: %w", err)
	}

	roleARN, err := h.createOrGetServiceRole(config, awsSVCs)
	if err != nil {
		return config, fmt.Errorf("error creating or getting service role: %w", err)
	}

	if err := awsservices.CreateCluster(&awsservices.CreateClusterOptions{
		EKSService: awsSVCs.eks,
		Config:     config,
		RoleARN:    roleARN,
	}); err != nil {
		if !isClusterConflict(err) {
			return config, fmt.Errorf("error creating cluster: %w", err)
		}
	}

	// If a user edits a cluster at the exact right (or wrong) time, then the
	// `UpdateStatus` call may produce a conflict and will error. When the
	// controller re-enters the create function, it will try to verify that a
	// cluster with the same name in EKS does not exist (in the `validateCreate`
	// call above). It will find the one that was created and error.
	// Therefore, the `RetryOnConflict` will successfully update the status in this situation.
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		config, err = h.eksCC.Get(config.Namespace, config.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		config.Status.Phase = eksConfigCreatingPhase
		config.Status.FailureMessage = ""
		config, err = h.eksCC.UpdateStatus(config)
		return err
	})
	return config, err
}

func (h *Handler) validateCreate(config *eksv1.EKSClusterConfig, awsSVCs *awsServices) error {
	if awsSVCs == nil {
		return fmt.Errorf("aws services not initialized")
	}

	// Check for existing eksclusterconfigs with the same display name
	eksConfigs, err := h.eksCC.List(config.Namespace, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("cannot list eksclusterconfigs for display name check")
	}
	for _, c := range eksConfigs.Items {
		if c.Spec.DisplayName == config.Spec.DisplayName && c.Name != config.Name {
			return fmt.Errorf("cannot create cluster [%s] because an eksclusterconfig exists with the same name", config.Spec.DisplayName)
		}
	}

	// validate nodegroup version
	nodeP := map[string]bool{}
	if !config.Spec.Imported {
		// Check for existing clusters in EKS with the same display name
		listOutput, err := awsSVCs.eks.ListClusters(&eks.ListClustersInput{})
		if err != nil {
			return fmt.Errorf("error listing clusters: %v", err)
		}
		for _, cluster := range listOutput.Clusters {
			if aws.StringValue(cluster) == config.Spec.DisplayName {
				return fmt.Errorf("cannot create cluster [%s] because a cluster in EKS exists with the same name", config.Spec.DisplayName)
			}
		}
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
			if ng.LaunchTemplate != nil {
				if ng.LaunchTemplate.ID == nil {
					return fmt.Errorf(cannotBeNilError, "launchTemplate.ID", *ng.NodegroupName, config.Name)
				}
				if ng.LaunchTemplate.Version == nil {
					return fmt.Errorf(cannotBeNilError, "launchTemplate.Version", *ng.NodegroupName, config.Name)
				}
			} else {
				if ng.Ec2SshKey == nil {
					return fmt.Errorf(cannotBeNilError, "ec2SshKey", *ng.NodegroupName, config.Name)
				}
				if ng.ResourceTags == nil {
					return fmt.Errorf(cannotBeNilError, "resourceTags", *ng.NodegroupName, config.Name)
				}
				if ng.DiskSize == nil {
					return fmt.Errorf(cannotBeNilError, "diskSize", *ng.NodegroupName, config.Name)
				}
				if !aws.BoolValue(ng.RequestSpotInstances) && ng.InstanceType == nil {
					return fmt.Errorf(cannotBeNilError, "instanceType", *ng.NodegroupName, config.Name)
				}
			}
			if ng.NodegroupName == nil {
				return fmt.Errorf(cannotBeNilError, "name", *ng.NodegroupName, config.Name)
			}
			if nodeP[*ng.NodegroupName] {
				return fmt.Errorf("NodePool names must be unique within the [%s] cluster to avoid duplication", config.Name)
			}
			nodeP[*ng.NodegroupName] = true
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
			if ng.Gpu == nil {
				return fmt.Errorf(cannotBeNilError, "gpu", *ng.NodegroupName, config.Name)
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
			if ng.RequestSpotInstances == nil {
				return fmt.Errorf(cannotBeNilError, "requestSpotInstances", *ng.NodegroupName, config.Name)
			}
			if ng.NodeRole == nil {
				logrus.Warnf("nodeRole is not specified for nodegroup [%s] in cluster [%s], the controller will generate it", *ng.NodegroupName, config.Name)
			}
			if aws.BoolValue(ng.RequestSpotInstances) {
				if len(ng.SpotInstanceTypes) == 0 {
					return fmt.Errorf("nodegroup [%s] in cluster [%s]: spotInstanceTypes must be specified when requesting spot instances", *ng.NodegroupName, config.Name)
				}
				if aws.StringValue(ng.InstanceType) != "" {
					return fmt.Errorf("nodegroup [%s] in cluster [%s]: instance type should not be specified when requestSpotInstances is specified, use spotInstanceTypes instead",
						*ng.NodegroupName, config.Name)
				}
			}
		}
		if aws.StringValue(ng.Version) != *config.Spec.KubernetesVersion {
			return fmt.Errorf("nodegroup [%s] version must match cluster [%s] version on create", aws.StringValue(ng.NodegroupName), config.Name)
		}
	}
	return nil
}

func (h *Handler) generateAndSetNetworking(config *eksv1.EKSClusterConfig, awsSVCs *awsServices) (*eksv1.EKSClusterConfig, error) {
	if awsSVCs == nil {
		return nil, fmt.Errorf("aws services not initialized")
	}

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
		stack, err := awsservices.CreateStack(&awsservices.CreateStackOptions{
			CloudFormationService: awsSVCs.cloudformation,
			StackName:             getVPCStackName(config.Spec.DisplayName),
			DisplayName:           config.Spec.DisplayName,
			TemplateBody:          templates.VpcTemplate,
			Capabilities:          []string{},
			Parameters:            []*cloudformation.Parameter{},
		})
		if err != nil {
			return config, fmt.Errorf("error creating stack with VPC template: %v", err)
		}

		virtualNetworkString := getParameterValueFromOutput("VpcId", stack.Stacks[0].Outputs)
		subnetIdsString := getParameterValueFromOutput("SubnetIds", stack.Stacks[0].Outputs)

		if subnetIdsString == "" {
			return config, fmt.Errorf("no subnet ids were returned")
		}

		config = config.DeepCopy()
		// copy generated field to status
		config.Status.VirtualNetwork = virtualNetworkString
		config.Status.Subnets = strings.Split(subnetIdsString, ",")
		config.Status.NetworkFieldsSource = "generated"
	}

	return h.eksCC.UpdateStatus(config)
}

func (h *Handler) createOrGetServiceRole(config *eksv1.EKSClusterConfig, awsSVCs *awsServices) (string, error) {
	var roleARN string
	if aws.StringValue(config.Spec.ServiceRole) == "" {
		logrus.Infof("Creating service role")

		stack, err := awsservices.CreateStack(&awsservices.CreateStackOptions{
			CloudFormationService: awsSVCs.cloudformation,
			StackName:             getServiceRoleName(config.Spec.DisplayName),
			DisplayName:           config.Spec.DisplayName,
			TemplateBody:          templates.ServiceRoleTemplate,
			Capabilities:          []string{cloudformation.CapabilityCapabilityIam},
			Parameters:            nil,
		})
		if err != nil {
			return "", fmt.Errorf("error creating stack with service role template: %v", err)
		}

		roleARN = getParameterValueFromOutput("RoleArn", stack.Stacks[0].Outputs)
		if roleARN == "" {
			return "", fmt.Errorf("no RoleARN was returned")
		}
	} else {
		logrus.Infof("Retrieving existing service role")
		role, err := awsSVCs.iam.GetRole(&iam.GetRoleInput{
			RoleName: config.Spec.ServiceRole,
		})
		if err != nil {
			return "", fmt.Errorf("error getting role: %w", err)
		}

		roleARN = *role.Role.Arn
	}

	return roleARN, nil
}

func newAWSServices(secretsCache wranglerv1.SecretCache, spec eksv1.EKSClusterConfigSpec) (*awsServices, error) {
	sess, err := newAWSSession(secretsCache, spec)
	if err != nil {
		return nil, err
	}

	return &awsServices{
		eks:            services.NewEKSService(sess),
		cloudformation: services.NewCloudFormationService(sess),
		iam:            services.NewIAMService(sess),
		ec2:            services.NewEC2Service(sess),
	}, nil
}

func newAWSSession(secretsCache wranglerv1.SecretCache, spec eksv1.EKSClusterConfigSpec) (*session.Session, error) {
	awsConfig := &aws.Config{}

	if region := spec.Region; region != "" {
		awsConfig.Region = aws.String(region)
	}

	ns, id := utils.Parse(spec.AmazonCredentialSecret)
	if amazonCredentialSecret := spec.AmazonCredentialSecret; amazonCredentialSecret != "" {
		secret, err := secretsCache.Get(ns, id)
		if err != nil {
			return nil, fmt.Errorf("error getting secret %s/%s: %w", ns, id, err)
		}

		accessKeyBytes := secret.Data["amazonec2credentialConfig-accessKey"]
		secretKeyBytes := secret.Data["amazonec2credentialConfig-secretKey"]
		if accessKeyBytes == nil || secretKeyBytes == nil {
			return nil, fmt.Errorf("invalid aws cloud credential")
		}

		accessKey := string(accessKeyBytes)
		secretKey := string(secretKeyBytes)

		awsConfig.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("error getting new aws session: %v", err)
	}

	return sess, nil
}

func (h *Handler) waitForCreationComplete(config *eksv1.EKSClusterConfig, awsSVCs *awsServices) (*eksv1.EKSClusterConfig, error) {
	if awsSVCs == nil {
		return config, fmt.Errorf("aws services not initialized")
	}

	var err error

	state, err := awsservices.GetClusterState(&awsservices.GetClusterStatusOpts{
		EKSService: awsSVCs.eks,
		Config:     config,
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
func BuildUpstreamClusterState(name, managedTemplateID string, clusterState *eks.DescribeClusterOutput, nodeGroupStates []*eks.DescribeNodegroupOutput, ec2Service services.EC2ServiceInterface, includeManagedLaunchTemplate bool) (*eksv1.EKSClusterConfigSpec, string, error) {
	upstreamSpec := &eksv1.EKSClusterConfigSpec{}

	upstreamSpec.Imported = true
	upstreamSpec.DisplayName = name

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
			NodegroupName:        ng.Nodegroup.NodegroupName,
			DiskSize:             ng.Nodegroup.DiskSize,
			Labels:               ng.Nodegroup.Labels,
			DesiredSize:          ng.Nodegroup.ScalingConfig.DesiredSize,
			MaxSize:              ng.Nodegroup.ScalingConfig.MaxSize,
			MinSize:              ng.Nodegroup.ScalingConfig.MinSize,
			NodeRole:             ng.Nodegroup.NodeRole,
			Subnets:              aws.StringValueSlice(ng.Nodegroup.Subnets),
			Tags:                 ng.Nodegroup.Tags,
			RequestSpotInstances: aws.Bool(aws.StringValue(ng.Nodegroup.CapacityType) == eks.CapacityTypesSpot),
		}

		if clusterState.Cluster.Version == ng.Nodegroup.Version ||
			aws.StringValue(ng.Nodegroup.Status) != eks.NodegroupStatusUpdating {
			ngToAdd.Version = ng.Nodegroup.Version
		}

		if aws.BoolValue(ngToAdd.RequestSpotInstances) {
			ngToAdd.SpotInstanceTypes = ng.Nodegroup.InstanceTypes
		}

		if ng.Nodegroup.LaunchTemplate != nil {
			var version *int64
			versionNumber, err := strconv.ParseInt(aws.StringValue(ng.Nodegroup.LaunchTemplate.Version), 10, 64)
			if err == nil {
				version = aws.Int64(versionNumber)
			}

			ngToAdd.LaunchTemplate = &eksv1.LaunchTemplate{
				ID:      ng.Nodegroup.LaunchTemplate.Id,
				Name:    ng.Nodegroup.LaunchTemplate.Name,
				Version: version,
			}

			if managedTemplateID == aws.StringValue(ngToAdd.LaunchTemplate.ID) {
				// If this is a rancher-managed launch template, then we move the data from the launch template to the node group.
				launchTemplateRequestOutput, err := awsservices.GetLaunchTemplateVersions(&awsservices.GetLaunchTemplateVersionsOpts{
					EC2Service:       ec2Service,
					LaunchTemplateID: ngToAdd.LaunchTemplate.ID,
					Versions:         []*string{ng.Nodegroup.LaunchTemplate.Version},
				})
				if err != nil || len(launchTemplateRequestOutput.LaunchTemplateVersions) == 0 {
					if doesNotExist(err) || notFound(err) {
						if includeManagedLaunchTemplate {
							// In this case, we need to continue rather than error so that we can update the launch template for the nodegroup.
							ngToAdd.LaunchTemplate.ID = nil
							upstreamSpec.NodeGroups = append(upstreamSpec.NodeGroups, ngToAdd)
							continue
						}

						return nil, "", fmt.Errorf("rancher-managed launch template for node group [%s] in cluster [%s] not found, must create new node group and destroy existing",
							aws.StringValue(ngToAdd.NodegroupName),
							upstreamSpec.DisplayName,
						)
					}
					return nil, "", fmt.Errorf("error getting launch template info for node group [%s] in cluster [%s]", aws.StringValue(ngToAdd.NodegroupName), upstreamSpec.DisplayName)
				}
				launchTemplateData := launchTemplateRequestOutput.LaunchTemplateVersions[0].LaunchTemplateData

				if len(launchTemplateData.BlockDeviceMappings) == 0 {
					return nil, "", fmt.Errorf("launch template for node group [%s] in cluster [%s] is malformed", aws.StringValue(ngToAdd.NodegroupName), upstreamSpec.DisplayName)
				}
				ngToAdd.DiskSize = launchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize
				ngToAdd.Ec2SshKey = launchTemplateData.KeyName
				ngToAdd.ImageID = launchTemplateData.ImageId
				ngToAdd.InstanceType = launchTemplateData.InstanceType
				ngToAdd.ResourceTags = utils.GetInstanceTags(launchTemplateData.TagSpecifications)

				userData := aws.StringValue(launchTemplateData.UserData)
				if userData != "" {
					decodedUserdata, err := base64.StdEncoding.DecodeString(userData)
					if err == nil {
						ngToAdd.UserData = aws.String(string(decodedUserdata))
					} else {
						logrus.Warnf("Could not decode userdata for nodegroup [%s] in cluster[%s]", aws.StringValue(ngToAdd.NodegroupName), name)
					}
				}

				if !includeManagedLaunchTemplate {
					ngToAdd.LaunchTemplate = nil
				}
			}
		} else {
			// If the node group does not have a launch template, then the following must be pulled from the node group config.
			if !aws.BoolValue(ngToAdd.RequestSpotInstances) && len(ng.Nodegroup.InstanceTypes) > 0 {
				ngToAdd.InstanceType = ng.Nodegroup.InstanceTypes[0]
			}
			if ng.Nodegroup.RemoteAccess != nil {
				ngToAdd.Ec2SshKey = ng.Nodegroup.RemoteAccess.Ec2SshKey
			}
		}
		if aws.StringValue(ng.Nodegroup.AmiType) == eks.AMITypesAl2X8664Gpu {
			ngToAdd.Gpu = aws.Bool(true)
		} else if aws.StringValue(ng.Nodegroup.AmiType) == eks.AMITypesAl2X8664 {
			ngToAdd.Gpu = aws.Bool(false)
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
func (h *Handler) updateUpstreamClusterState(upstreamSpec *eksv1.EKSClusterConfigSpec, config *eksv1.EKSClusterConfig, awsSVCs *awsServices, clusterARN string, ngARNs map[string]string) (*eksv1.EKSClusterConfig, error) {
	if awsSVCs == nil {
		return config, fmt.Errorf("aws services not initialized")
	}

	// check kubernetes version for update
	if config.Spec.KubernetesVersion != nil {
		updated, err := awsservices.UpdateClusterVersion(&awsservices.UpdateClusterVersionOpts{
			EKSService:          awsSVCs.eks,
			Config:              config,
			UpstreamClusterSpec: upstreamSpec,
		})
		if err != nil {
			return config, fmt.Errorf("error updating cluster version: %w", err)
		}
		if updated {
			return h.enqueueUpdate(config)
		}
	}

	// check tags for update
	if config.Spec.Tags != nil {
		updated, err := awsservices.UpdateResourceTags(&awsservices.UpdateResourceTagsOpts{
			EKSService:   awsSVCs.eks,
			Tags:         config.Spec.Tags,
			UpstreamTags: upstreamSpec.Tags,
			ResourceARN:  clusterARN,
		})
		if err != nil {
			return config, fmt.Errorf("error updating cluster tags: %w", err)
		}
		if updated {
			return h.enqueueUpdate(config)
		}
	}

	if config.Spec.LoggingTypes != nil {
		// check logging for update
		updated, err := awsservices.UpdateClusterLoggingTypes(&awsservices.UpdateLoggingTypesOpts{
			EKSService:          awsSVCs.eks,
			Config:              config,
			UpstreamClusterSpec: upstreamSpec,
		})
		if err != nil {
			return config, fmt.Errorf("error updating logging types: %w", err)
		}
		if updated {
			return h.enqueueUpdate(config)
		}
	}

	updated, err := awsservices.UpdateClusterAccess(&awsservices.UpdateClusterAccessOpts{
		EKSService:          awsSVCs.eks,
		Config:              config,
		UpstreamClusterSpec: upstreamSpec,
	})
	if err != nil {
		return config, fmt.Errorf("error updating cluster access config: %w", err)
	}
	if updated {
		return h.enqueueUpdate(config)
	}

	if config.Spec.PublicAccessSources != nil {
		updated, err := awsservices.UpdateClusterPublicAccessSources(&awsservices.UpdateClusterPublicAccessSourcesOpts{
			EKSService:          awsSVCs.eks,
			Config:              config,
			UpstreamClusterSpec: upstreamSpec,
		})
		if err != nil {
			return config, fmt.Errorf("error updating cluster public access sources: %w", err)
		}
		if updated {
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

	// Deep copy the config object here, so it's not copied multiple times for each
	// nodegroup create/delete.
	config = config.DeepCopy()

	// check if node groups need to be created
	var updatingNodegroups bool
	templateVersionsToAdd := make(map[string]string)
	for _, ng := range config.Spec.NodeGroups {
		if _, ok := upstreamNgs[aws.StringValue(ng.NodegroupName)]; ok {
			continue
		}
		if err := awsservices.CreateLaunchTemplate(&awsservices.CreateLaunchTemplateOptions{
			EC2Service: awsSVCs.ec2,
			Config:     config,
		}); err != nil {
			return config, fmt.Errorf("error getting or creating launch template: %w", err)
		}
		// in this case update is set right away because creating the
		// nodegroup may not be immediate
		if config.Status.Phase != eksConfigUpdatingPhase {
			config.Status.Phase = eksConfigUpdatingPhase
			var err error
			config, err = h.eksCC.UpdateStatus(config)
			if err != nil {
				return config, err
			}
		}

		ltVersion, generatedNodeRole, err := awsservices.CreateNodeGroup(&awsservices.CreateNodeGroupOptions{
			EC2Service:            awsSVCs.ec2,
			CloudFormationService: awsSVCs.cloudformation,
			EKSService:            awsSVCs.eks,
			Config:                config,
			NodeGroup:             ng,
		})

		if err != nil {
			return config, fmt.Errorf("error creating nodegroup: %w", err)
		}

		// if a generated node role has not been set on the Status yet and it
		// was just generated, set it
		if config.Status.GeneratedNodeRole == "" && generatedNodeRole != "" {
			config.Status.GeneratedNodeRole = generatedNodeRole
		}
		if err != nil {
			return config, err
		}
		templateVersionsToAdd[aws.StringValue(ng.NodegroupName)] = ltVersion
		updatingNodegroups = true
	}

	// check for node groups need to be deleted
	templateVersionsToDelete := make(map[string]string)
	for _, ng := range upstreamSpec.NodeGroups {
		if _, ok := ngs[aws.StringValue(ng.NodegroupName)]; ok {
			continue
		}
		templateVersionToDelete, _, err := deleteNodeGroup(config, ng, awsSVCs.eks)
		if err != nil {
			return config, err
		}
		updatingNodegroups = true
		if templateVersionToDelete != nil {
			templateVersionsToDelete[aws.StringValue(ng.NodegroupName)] = *templateVersionToDelete
		}
	}

	if updatingNodegroups {
		if len(templateVersionsToDelete) != 0 || len(templateVersionsToAdd) != 0 {
			config.Status.Phase = eksConfigUpdatingPhase
			config.Status.TemplateVersionsToDelete = append(config.Status.TemplateVersionsToDelete, utils.ValuesFromMap(templateVersionsToDelete)...)
			config.Status.ManagedLaunchTemplateVersions = utils.SubtractMaps(config.Status.ManagedLaunchTemplateVersions, templateVersionsToDelete)
			config.Status.ManagedLaunchTemplateVersions = utils.MergeMaps(config.Status.ManagedLaunchTemplateVersions, templateVersionsToAdd)
			return h.eksCC.UpdateStatus(config)
		}
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

	var updateNodegroupProperties bool
	templateVersionsToDelete = make(map[string]string)
	for _, upstreamNg := range upstreamSpec.NodeGroups {
		// if continue is used after an update, it means that update
		// must finish before others for that nodegroup can take place.
		// Some updates such as minSize, maxSize, and desiredSize can
		// happen together

		ng := ngs[aws.StringValue(upstreamNg.NodegroupName)]
		ngVersionInput := &eks.UpdateNodegroupVersionInput{
			NodegroupName: aws.String(aws.StringValue(ng.NodegroupName)),
			ClusterName:   aws.String(config.Spec.DisplayName),
		}

		// rancherManagedLaunchTemplate is true if user did not specify a custom launch template
		rancherManagedLaunchTemplate := false
		if upstreamNg.LaunchTemplate != nil {
			upstreamTemplateVersion := aws.Int64Value(upstreamNg.LaunchTemplate.Version)
			var err error
			lt := ng.LaunchTemplate

			if lt == nil && config.Status.ManagedLaunchTemplateID == aws.StringValue(upstreamNg.LaunchTemplate.ID) {
				rancherManagedLaunchTemplate = true
				// In this case, Rancher is managing the launch template, so we check to see if we need a new version.
				lt, err = newLaunchTemplateVersionIfNeeded(config, upstreamNg, ng, awsSVCs.ec2)
				if err != nil {
					return config, err
				}

				if lt != nil {
					if upstreamTemplateVersion > 0 {
						templateVersionsToDelete[aws.StringValue(upstreamNg.NodegroupName)] = strconv.FormatInt(upstreamTemplateVersion, 10)
					}
					templateVersionsToAdd[aws.StringValue(ng.NodegroupName)] = strconv.FormatInt(*lt.Version, 10)
				}
			}

			if lt != nil && aws.Int64Value(lt.Version) != upstreamTemplateVersion {
				ngVersionInput.LaunchTemplate = &eks.LaunchTemplateSpecification{
					Id:      lt.ID,
					Version: aws.String(strconv.FormatInt(*lt.Version, 10)),
				}
			}
		}

		// a node group created from a custom launch template can only be updated with a new version of the launch template
		// that uses an AMI with the desired kubernetes version, hence, only update on version mismatch if the node group was created with a rancher-managed launch template
		if ng.Version != nil && rancherManagedLaunchTemplate {
			if aws.StringValue(upstreamNg.Version) != desiredNgVersions[aws.StringValue(ng.NodegroupName)] {
				ngVersionInput.Version = aws.String(desiredNgVersions[aws.StringValue(ng.NodegroupName)])
			}
		}

		if ngVersionInput.Version != nil || ngVersionInput.LaunchTemplate != nil {
			updateNodegroupProperties = true
			if err := awsservices.UpdateNodegroupVersion(&awsservices.UpdateNodegroupVersionOpts{
				EKSService:     awsSVCs.eks,
				EC2Service:     awsSVCs.ec2,
				Config:         config,
				NodeGroup:      &ng,
				NGVersionInput: ngVersionInput,
				LTVersions:     templateVersionsToAdd,
			}); err != nil {
				return config, err
			}
			continue
		}
		updateNodegroupConfig, sendUpdateNodegroupConfig := getNodegroupConfigUpdate(config.Spec.DisplayName, ng, upstreamNg)

		if sendUpdateNodegroupConfig {
			updateNodegroupProperties = true
			_, err := awsSVCs.eks.UpdateNodegroupConfig(&updateNodegroupConfig)
			if err != nil {
				return config, err
			}
			continue
		}

		if ng.Tags != nil {
			var err error // initialize error here because we assign returned value to updateNodegroupProperties
			updateNodegroupProperties, err = awsservices.UpdateResourceTags(&awsservices.UpdateResourceTagsOpts{
				EKSService:   awsSVCs.eks,
				Tags:         aws.StringValueMap(ng.Tags),
				UpstreamTags: aws.StringValueMap(upstreamNg.Tags),
				ResourceARN:  ngARNs[aws.StringValue(ng.NodegroupName)],
			})
			if err != nil {
				return config, fmt.Errorf("error updating cluster tags: %w", err)
			}
		}
	}

	if updateNodegroupProperties {
		// if any updates are taking place on nodegroups, the config's phase needs
		// to be set to "updating" and the controller will wait for the updates to
		// finish before proceeding
		if len(templateVersionsToDelete) != 0 || len(templateVersionsToAdd) != 0 {
			config = config.DeepCopy()
			config.Status.TemplateVersionsToDelete = append(config.Status.TemplateVersionsToDelete, utils.ValuesFromMap(templateVersionsToDelete)...)
			config.Status.ManagedLaunchTemplateVersions = utils.SubtractMaps(config.Status.ManagedLaunchTemplateVersions, templateVersionsToAdd)
			config.Status.ManagedLaunchTemplateVersions = utils.MergeMaps(config.Status.ManagedLaunchTemplateVersions, templateVersionsToAdd)
			config.Status.Phase = eksConfigUpdatingPhase
			return h.eksCC.UpdateStatus(config)
		}
		return h.enqueueUpdate(config)
	}

	// check if ebs csi driver needs to be enabled
	if aws.BoolValue(config.Spec.EBSCSIDriver) {
		installedArn, err := awsservices.CheckEBSAddon(awsSVCs.eks, config)
		if err != nil {
			return nil, fmt.Errorf("error checking if ebs csi driver addon is installed: %w", err)
		}
		if installedArn == "" {
			logrus.Infof("enabling [ebs csi driver add-on] for cluster [%s]", config.Spec.DisplayName)
			ebsCSIDriverInput := awsservices.EnableEBSCSIDriverInput{
				EKSService:   awsSVCs.eks,
				IAMService:   awsSVCs.iam,
				CFService:    awsSVCs.cloudformation,
				Config:       config,
				AddonVersion: "latest",
			}
			if err := awsservices.EnableEBSCSIDriver(&ebsCSIDriverInput); err != nil {
				return config, fmt.Errorf("error enabling ebs csi driver addon: %w", err)
			}
		}
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
func (h *Handler) importCluster(config *eksv1.EKSClusterConfig, awsSVCs *awsServices) (*eksv1.EKSClusterConfig, error) {
	if awsSVCs == nil {
		return config, fmt.Errorf("aws services not initialized")
	}

	clusterState, err := awsservices.GetClusterState(&awsservices.GetClusterStatusOpts{
		EKSService: awsSVCs.eks,
		Config:     config,
	})
	if err != nil {
		return config, err
	}

	if err := h.createCASecret(config, clusterState); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return config, err
		}
	}

	launchTemplatesOutput, err := awsSVCs.ec2.DescribeLaunchTemplates(&ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []*string{aws.String(fmt.Sprintf(awsservices.LaunchTemplateNameFormat, config.Spec.DisplayName))},
	})
	if err == nil && len(launchTemplatesOutput.LaunchTemplates) > 0 {
		config.Status.ManagedLaunchTemplateID = aws.StringValue(launchTemplatesOutput.LaunchTemplates[0].LaunchTemplateId)
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
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      config.Name,
				Namespace: config.Namespace,
				OwnerReferences: []metav1.OwnerReference{
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

func getEBSCSIDriverRoleStackName(name string) string {
	return name + "-ebs-csi-driver-role"
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

func deleteStack(svc services.CloudFormationServiceInterface, newStyleName, oldStyleName string) error {
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
	if err != nil && !doesNotExist(err) {
		return fmt.Errorf("error deleting stack: %v", err)
	}

	return nil
}
