package controller

import (
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/stretchr/testify/assert"
)

func TestGetNodegroupConfigUpdate(t *testing.T) {
	type nodegroupUpdateTestCase struct {
		clusterName           string
		ng1                   eksv1.NodeGroup
		ng2                   eksv1.NodeGroup
		expectedNgUpdateInput eks.UpdateNodegroupConfigInput
		expectedNgNeedsUpdate bool
	}
	asserts := assert.New(t)
	testCases := []nodegroupUpdateTestCase{
		{
			// test case where there should be no update
			clusterName: "testcluster1",
			ng1:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b"}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			ng2:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b"}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			expectedNgUpdateInput: eks.UpdateNodegroupConfigInput{
				ClusterName: aws.String("testcluster1"),
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize: aws.Int32(1),
					MaxSize: aws.Int32(1),
				},
			},
			expectedNgNeedsUpdate: false,
		},
		{
			// test the case where upstream doesn't have scaling fields MinSize or MaxSize size but desired does
			clusterName: "testcluster2",
			ng1:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b"}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			ng2:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b"})},
			expectedNgUpdateInput: eks.UpdateNodegroupConfigInput{
				ClusterName: aws.String("testcluster2"),
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize: aws.Int32(1),
					MaxSize: aws.Int32(1),
				}},
			expectedNgNeedsUpdate: true,
		},
		{
			// test case where scaling field, DesiredSize, should be updated
			clusterName: "testcluster3",
			ng1:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b"}), DesiredSize: aws.Int32(1)},
			ng2:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b"}), DesiredSize: aws.Int32(3)},
			expectedNgUpdateInput: eks.UpdateNodegroupConfigInput{
				ClusterName: aws.String("testcluster3"),
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					DesiredSize: aws.Int32(1),
				}},
			expectedNgNeedsUpdate: true,
		},
		{
			// test case where label should be deleted
			clusterName: "testcluster4",
			ng1:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			ng2:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b"}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			expectedNgUpdateInput: eks.UpdateNodegroupConfigInput{
				ClusterName: aws.String("testcluster4"),
				Labels: &ekstypes.UpdateLabelsPayload{
					RemoveLabels: []string{"a"},
				},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize: aws.Int32(1),
					MaxSize: aws.Int32(1),
				}},
			expectedNgNeedsUpdate: true,
		},
		{
			// test case where label should be added
			clusterName: "testcluster5",
			ng1:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b"}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			ng2:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			expectedNgUpdateInput: eks.UpdateNodegroupConfigInput{
				ClusterName: aws.String("testcluster5"),
				Labels: &ekstypes.UpdateLabelsPayload{
					AddOrUpdateLabels: map[string]string{"a": "b"},
				},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize: aws.Int32(1),
					MaxSize: aws.Int32(1),
				}},
			expectedNgNeedsUpdate: true,
		},
		{
			// test case where labels should be removed and added
			clusterName: "testcluster6",
			ng1:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b", "g": "h"}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			ng2:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"c": "d", "e": "f", "g": "h"}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			expectedNgUpdateInput: eks.UpdateNodegroupConfigInput{
				ClusterName: aws.String("testcluster6"),
				Labels: &ekstypes.UpdateLabelsPayload{
					RemoveLabels:      []string{"c", "e"},
					AddOrUpdateLabels: map[string]string{"a": "b"},
				},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize: aws.Int32(1),
					MaxSize: aws.Int32(1),
				}},
			expectedNgNeedsUpdate: true,
		},
		{
			// test case where label should be updated
			clusterName: "testcluster7",
			ng1:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b", "g": "h"}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			ng2:         eksv1.NodeGroup{Labels: aws.StringMap(map[string]string{"a": "b", "g": "i"}), MinSize: aws.Int32(1), MaxSize: aws.Int32(1)},
			expectedNgUpdateInput: eks.UpdateNodegroupConfigInput{
				ClusterName: aws.String("testcluster7"),
				Labels: &ekstypes.UpdateLabelsPayload{
					AddOrUpdateLabels: map[string]string{"g": "h"},
				},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					MinSize: aws.Int32(1),
					MaxSize: aws.Int32(1),
				}},
			expectedNgNeedsUpdate: true,
		},
	}
	for _, testCase := range testCases {
		ngUpdateInput, ngNeedsUpdate := getNodegroupConfigUpdate(testCase.clusterName, testCase.ng1, testCase.ng2)
		if ngUpdateInput.Labels != nil && len(ngUpdateInput.Labels.RemoveLabels) > 0 {
			sortedRemovedLabels := ngUpdateInput.Labels.RemoveLabels
			sort.Strings(sortedRemovedLabels)
			ngUpdateInput.Labels.RemoveLabels = sortedRemovedLabels
		}
		asserts.Equal(testCase.expectedNgUpdateInput, ngUpdateInput)
		asserts.Equal(testCase.expectedNgNeedsUpdate, ngNeedsUpdate)
	}
}
