package controller

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"

	awssdkeks "github.com/aws/aws-sdk-go-v2/service/eks"
	awssdksts "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/pkg/eks"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/eks-operator/pkg/eks/services/mock_services"
	"github.com/rancher/eks-operator/pkg/test"
)

var _ = Describe("importCluster", func() {
	var (
		eksConfig               *eksv1.EKSClusterConfig
		getClusterStatusOptions *eks.GetClusterStatusOpts
		mockController          *gomock.Controller
		eksServiceMock          *mock_services.MockEKSServiceInterface
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		eksServiceMock = mock_services.NewMockEKSServiceInterface(mockController)
		eksConfig = &eksv1.EKSClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
			Spec: eksv1.EKSClusterConfigSpec{
				DisplayName:         "test",
				Region:              "test",
				PrivateAccess:       aws.Bool(true),
				PublicAccess:        aws.Bool(true),
				PublicAccessSources: []string{"test"},
				Tags:                map[string]string{"test": "test"},
				LoggingTypes:        []string{"test"},
				KubernetesVersion:   aws.String("test"),
				SecretsEncryption:   aws.Bool(true),
				KmsKey:              aws.String("test"),
			},
		}
		getClusterStatusOptions = &eks.GetClusterStatusOpts{
			EKSService: eksServiceMock,
			Config: &eksv1.EKSClusterConfig{
				Spec: eksv1.EKSClusterConfigSpec{
					DisplayName: "test",
				},
			},
		}

		Expect(cl.Create(ctx, eksConfig)).To(Succeed())
	})

	AfterEach(func() {
		Expect(test.CleanupAndWait(ctx, cl, eksConfig)).To(Succeed())
	})

	It("should get cluster state", func() {
		eksServiceMock.EXPECT().DescribeCluster(ctx,
			&awssdkeks.DescribeClusterInput{
				Name: aws.String(getClusterStatusOptions.Config.Spec.DisplayName),
			},
		).Return(&awssdkeks.DescribeClusterOutput{}, nil).AnyTimes()
		clusterState, err := eks.GetClusterState(ctx, getClusterStatusOptions)
		Expect(err).ToNot(HaveOccurred())
		Expect(clusterState).ToNot(BeNil())
	})
})

var _ = Describe("delete stack", func() {
	var (
		mockController            *gomock.Controller
		mockCloudformationService *mock_services.MockCloudFormationServiceInterface
		newStyleName              string
		oldStyleName              string
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		newStyleName = "test"
		mockCloudformationService = mock_services.NewMockCloudFormationServiceInterface(mockController)
	})

	AfterEach(func() {
		mockController.Finish()
	})

	It("should successfully delete a stack", func() {
		mockCloudformationService.EXPECT().DescribeStacks(ctx,
			&cloudformation.DescribeStacksInput{
				StackName: &newStyleName,
			},
		).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []cftypes.Stack{
					{
						StackName: &newStyleName,
					},
				},
			}, nil)

		mockCloudformationService.EXPECT().DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: &newStyleName,
		}).Return(nil, nil)

		newerr := deleteStack(ctx, mockCloudformationService, newStyleName, "")
		Expect(newerr).ToNot(HaveOccurred())
	})
	It("should successfully delete a stack with old style name", func() {
		mockCloudformationService.EXPECT().DescribeStacks(ctx,
			&cloudformation.DescribeStacksInput{
				StackName: &oldStyleName,
			},
		).Return(
			&cloudformation.DescribeStacksOutput{
				Stacks: []cftypes.Stack{
					{
						StackName: &oldStyleName,
					},
				},
			}, nil)

		mockCloudformationService.EXPECT().DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: &oldStyleName,
		}).Return(nil, nil)

		newerr := deleteStack(ctx, mockCloudformationService, "", oldStyleName)
		Expect(newerr).ToNot(HaveOccurred())
	})

	It("should fail to delete a stack if DescribeStacks returns no stacks", func() {
		mockCloudformationService.EXPECT().DescribeStacks(ctx,
			&cloudformation.DescribeStacksInput{
				StackName: &newStyleName,
			},
		).Return(&cloudformation.DescribeStacksOutput{}, nil)

		mockCloudformationService.EXPECT().DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: &newStyleName,
		}).Return(nil, errors.New("error"))
		newerr := deleteStack(ctx, mockCloudformationService, newStyleName, "")
		Expect(newerr).To(HaveOccurred())
	})
})

var _ = Describe("updateCluster", func() {
	var (
		eksConfig      *eksv1.EKSClusterConfig
		handler        *Handler
		eksServiceMock *mock_services.MockEKSServiceInterface
	)

	BeforeEach(func() {
		eksServiceMock = mock_services.NewMockEKSServiceInterface(gomock.NewController(GinkgoT()))
		handler = &Handler{
			eksCC:        eksFactory.Eks().V1().EKSClusterConfig(),
			secrets:      coreFactory.Core().V1().Secret(),
			secretsCache: coreFactory.Core().V1().Secret().Cache(),
		}

		eksConfig = &eksv1.EKSClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
			Spec: eksv1.EKSClusterConfigSpec{
				DisplayName:         "test",
				Region:              "test",
				PrivateAccess:       aws.Bool(true),
				PublicAccess:        aws.Bool(true),
				PublicAccessSources: []string{"test"},
				Tags:                map[string]string{"test": "test"},
				LoggingTypes:        []string{"test"},
				KubernetesVersion:   aws.String("1.25"),
				SecretsEncryption:   aws.Bool(true),
				KmsKey:              aws.String("test"),
				NodeGroups: []eksv1.NodeGroup{
					{
						NodegroupName: aws.String("ng1"),
					},
				},
			},
			Status: eksv1.EKSClusterConfigStatus{
				Phase: "active",
			},
		}

		Expect(cl.Create(ctx, eksConfig)).To(Succeed())
	})

	AfterEach(func() {
		Expect(test.CleanupAndWait(ctx, cl, eksConfig)).To(Succeed())
	})

	It("should not allow duplicate node group names", func() {
		eksConfig.Status.Phase = "active"
		eksConfig.Spec.NodeGroups = append(eksConfig.Spec.NodeGroups, eksConfig.Spec.NodeGroups...)
		_, err := handler.OnEksConfigChanged("", eksConfig)
		Expect(err).To(MatchError("node group name [ng1] is not unique within the cluster [test (id: test)] to avoid duplication"))
	})

	It("should not allow node group versions outside version skew", func() {
		eksConfig.Status.Phase = "active"
		eksConfig.Spec.KubernetesVersion = aws.String("1.25")
		eksConfig.Spec.NodeGroups = append(eksConfig.Spec.NodeGroups, eksv1.NodeGroup{
			NodegroupName: aws.String("ng2"),
			Version:       aws.String("1.21"),
		})
		_, err := handler.OnEksConfigChanged("", eksConfig)
		Expect(err).To(MatchError("versions for cluster [1.25] and node group [1.21] are not compatible: " +
			"the node group version may only be up to three minor versions older than the cluster version"))
	})

	It("should set the config status to updating if there are updates in progress on the cluster", func() {
		eksServiceMock.EXPECT().DescribeCluster(ctx,
			&awssdkeks.DescribeClusterInput{
				Name: aws.String(eksConfig.Spec.DisplayName),
			},
		).Return(&awssdkeks.DescribeClusterOutput{
			Cluster: &types.Cluster{
				Status: types.ClusterStatusActive,
			},
		}, nil).AnyTimes()

		eksServiceMock.EXPECT().DescribeUpdates(ctx,
			&awssdkeks.ListUpdatesInput{
				Name: aws.String(eksConfig.Spec.DisplayName),
			}, map[string]bool{},
		).Return([]*awssdkeks.DescribeUpdateOutput{
			{
				Update: &types.Update{
					Status: types.UpdateStatusInProgress,
				},
			},
		}, nil).AnyTimes()

		config, err := handler.checkAndUpdate(ctx, eksConfig, &awsServices{
			eks: eksServiceMock,
		})
		Expect(err).To(BeNil())
		Expect(config.Status.Phase).To(Equal(eksConfigUpdatingPhase))
	})

	It("should set the config status to updating and set new completed updates in config status", func() {
		eksServiceMock.EXPECT().DescribeCluster(ctx,
			&awssdkeks.DescribeClusterInput{
				Name: aws.String(eksConfig.Spec.DisplayName),
			},
		).Return(&awssdkeks.DescribeClusterOutput{
			Cluster: &types.Cluster{
				Status: types.ClusterStatusActive,
			},
		}, nil).AnyTimes()

		eksServiceMock.EXPECT().DescribeUpdates(ctx,
			&awssdkeks.ListUpdatesInput{
				Name: aws.String(eksConfig.Spec.DisplayName),
			}, map[string]bool{},
		).Return([]*awssdkeks.DescribeUpdateOutput{
			{
				Update: &types.Update{
					Id:     aws.String("1"),
					Status: types.UpdateStatusCancelled,
				},
			},
			{
				Update: &types.Update{
					Id:     aws.String("2"),
					Status: types.UpdateStatusFailed,
				},
			},
			{
				Update: &types.Update{
					Id:     aws.String("3"),
					Status: types.UpdateStatusSuccessful,
				},
			},
			{
				Update: &types.Update{
					Id:     aws.String("4"),
					Status: types.UpdateStatusInProgress,
				},
			},
		}, nil).AnyTimes()

		config, err := handler.checkAndUpdate(ctx, eksConfig, &awsServices{
			eks: eksServiceMock,
		})
		Expect(err).To(BeNil())
		Expect(config.Status.Phase).To(Equal(eksConfigUpdatingPhase))
		Expect(config.Status.CompletedUpdateIDs).To(Equal([]string{"1", "2", "3"}))
	})
})

var _ = Describe("validateCreate display name uniqueness", func() {
	var (
		handler        *Handler
		existingConfig *eksv1.EKSClusterConfig
		mockController *gomock.Controller
		stsServiceMock *mock_services.MockSTSServiceInterface
	)

	BeforeEach(func() {
		mockController = gomock.NewController(GinkgoT())
		stsServiceMock = mock_services.NewMockSTSServiceInterface(mockController)
		handler = &Handler{
			eksCC:        eksFactory.Eks().V1().EKSClusterConfig(),
			secrets:      coreFactory.Core().V1().Secret(),
			secretsCache: coreFactory.Core().V1().Secret().Cache(),
		}
	})

	AfterEach(func() {
		if existingConfig != nil {
			Expect(test.CleanupAndWait(ctx, cl, existingConfig)).To(Succeed())
			existingConfig = nil
		}
		mockController.Finish()
	})

	newImportedConfig := func(name, displayName, region, credential string) *eksv1.EKSClusterConfig {
		return &eksv1.EKSClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: eksv1.EKSClusterConfigSpec{
				DisplayName:            displayName,
				Region:                 region,
				AmazonCredentialSecret: credential,
				Imported:               true,
			},
		}
	}

	It("should reject a cluster with the same name, region and credentials", func() {
		existingConfig = newImportedConfig("existing-dup", "dup", "us-east-1", "default:cred")
		Expect(cl.Create(ctx, existingConfig)).To(Succeed())

		newConfig := newImportedConfig("new-dup", "dup", "us-east-1", "default:cred")
		err := handler.validateCreate(ctx, newConfig, &awsServices{sts: stsServiceMock})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("an eksclusterconfig exists with the same name"))
	})

	It("should allow a cluster with the same name in a different region", func() {
		existingConfig = newImportedConfig("existing-region", "dup-region", "us-east-1", "default:cred")
		Expect(cl.Create(ctx, existingConfig)).To(Succeed())

		newConfig := newImportedConfig("new-region", "dup-region", "us-west-2", "default:cred")
		err := handler.validateCreate(ctx, newConfig, &awsServices{sts: stsServiceMock})
		Expect(err).ToNot(HaveOccurred())
	})

	It("should allow a cluster with the same name and region but a different AWS account", func() {
		existingConfig = newImportedConfig("existing-account", "dup-account", "us-east-1", "default:cred-a")
		Expect(cl.Create(ctx, existingConfig)).To(Succeed())

		// The current cluster resolves to a concrete account ID, while the
		// existing cluster's credentials cannot be resolved (its secret does
		// not exist), so the two are treated as different accounts.
		stsServiceMock.EXPECT().GetCallerIdentity(gomock.Any(), gomock.Any()).
			Return(&awssdksts.GetCallerIdentityOutput{Account: aws.String("111111111111")}, nil)

		newConfig := newImportedConfig("new-account", "dup-account", "us-east-1", "default:cred-b")
		err := handler.validateCreate(ctx, newConfig, &awsServices{sts: stsServiceMock})
		Expect(err).ToNot(HaveOccurred())
	})

	It("should allow a cluster when both credentials resolve to different AWS accounts", func() {
		existingConfig = newImportedConfig("existing-diff", "dup-diff", "us-east-1", "default:cred-a")
		Expect(cl.Create(ctx, existingConfig)).To(Succeed())

		// The current cluster resolves to one concrete account ID and the
		// existing cluster resolves to a different concrete account ID, so
		// they are treated as different clusters.
		stsServiceMock.EXPECT().GetCallerIdentity(gomock.Any(), gomock.Any()).
			Return(&awssdksts.GetCallerIdentityOutput{Account: aws.String("111111111111")}, nil)
		handler.accountIDForSpec = func(_ context.Context, _ eksv1.EKSClusterConfigSpec) string {
			return "222222222222"
		}

		newConfig := newImportedConfig("new-diff", "dup-diff", "us-east-1", "default:cred-b")
		err := handler.validateCreate(ctx, newConfig, &awsServices{sts: stsServiceMock})
		Expect(err).ToNot(HaveOccurred())
	})

	It("should reject a cluster when different credentials resolve to the same AWS account", func() {
		existingConfig = newImportedConfig("existing-same", "dup-same", "us-east-1", "default:cred-a")
		Expect(cl.Create(ctx, existingConfig)).To(Succeed())

		// Different credential secrets that both resolve to the same concrete
		// account ID identify the same cluster and must be rejected.
		stsServiceMock.EXPECT().GetCallerIdentity(gomock.Any(), gomock.Any()).
			Return(&awssdksts.GetCallerIdentityOutput{Account: aws.String("333333333333")}, nil)
		handler.accountIDForSpec = func(_ context.Context, _ eksv1.EKSClusterConfigSpec) string {
			return "333333333333"
		}

		newConfig := newImportedConfig("new-same", "dup-same", "us-east-1", "default:cred-b")
		err := handler.validateCreate(ctx, newConfig, &awsServices{sts: stsServiceMock})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("an eksclusterconfig exists with the same name"))
	})
})

var _ = Describe("recordError", func() {
	var (
		eksConfig *eksv1.EKSClusterConfig
		handler   *Handler
	)

	BeforeEach(func() {
		eksConfig = &eksv1.EKSClusterConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testrecorderror",
				Namespace: "default",
			},
			Spec: eksv1.EKSClusterConfigSpec{
				DisplayName:         "test",
				Region:              "test",
				PrivateAccess:       aws.Bool(true),
				PublicAccess:        aws.Bool(true),
				PublicAccessSources: []string{"test"},
				Tags:                map[string]string{"test": "test"},
				LoggingTypes:        []string{"test"},
				KubernetesVersion:   aws.String("test"),
				SecretsEncryption:   aws.Bool(true),
				KmsKey:              aws.String("test"),
			},
		}

		Expect(cl.Create(ctx, eksConfig)).To(Succeed())
	})

	AfterEach(func() {
		Expect(test.CleanupAndWait(ctx, cl, eksConfig)).To(Succeed())
	})

	It("should return same conflict error when onChange returns a conflict error", func() {
		oldOutput := logrus.StandardLogger().Out
		buf := bytes.Buffer{}
		logrus.SetOutput(&buf)

		eksConfigUpdated := eksConfig.DeepCopy()
		Expect(cl.Update(ctx, eksConfigUpdated)).To(Succeed())

		var expectedErr error
		expectedConfig := &eksv1.EKSClusterConfig{}
		onChange := func(key string, config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error) {
			expectedErr = cl.Update(ctx, config)
			return expectedConfig, expectedErr
		}

		eksConfig.ResourceVersion = "1"
		handleFunction := handler.recordError(onChange)
		config, err := handleFunction("", eksConfig)

		Expect(config).To(Equal(expectedConfig))
		Expect(err).To(Equal(expectedErr))
		Expect("").To(Equal(string(buf.Bytes())))
		logrus.SetOutput(oldOutput)
	})

	It("should return same conflict error when onChange returns a conflict error and print a debug log for the error", func() {
		oldOutput := logrus.StandardLogger().Out
		buf := bytes.Buffer{}
		logrus.SetOutput(&buf)
		logrus.SetLevel(logrus.DebugLevel)

		eksConfigUpdated := eksConfig.DeepCopy()
		Expect(cl.Update(ctx, eksConfigUpdated)).To(Succeed())

		var expectedErr error
		expectedConfig := &eksv1.EKSClusterConfig{}
		onChange := func(key string, config *eksv1.EKSClusterConfig) (*eksv1.EKSClusterConfig, error) {
			expectedErr = cl.Update(ctx, config)
			return expectedConfig, expectedErr
		}

		eksConfig.ResourceVersion = "1"
		handleFunction := handler.recordError(onChange)
		config, err := handleFunction("", eksConfig)

		Expect(config).To(Equal(expectedConfig))
		Expect(err).To(MatchError(expectedErr))

		cleanLogOutput := strings.Replace(string(buf.Bytes()), `\"`, `"`, -1)
		Expect(strings.Contains(cleanLogOutput, err.Error())).To(BeTrue())
		logrus.SetOutput(oldOutput)
	})
})
