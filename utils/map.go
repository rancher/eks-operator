package utils

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

func GetKeyValuesToUpdate(tags map[string]string, upstreamTags map[string]string) map[string]string {
	if len(tags) == 0 {
		return nil
	}

	if len(upstreamTags) == 0 {
		return tags
	}

	updateTags := make(map[string]string)
	for key, val := range tags {
		if upstreamTags[key] != val {
			updateTags[key] = val
		}
	}

	if len(updateTags) == 0 {
		return nil
	}
	return updateTags
}

func GetKeysToDelete(tags map[string]string, upstreamTags map[string]string) []string {
	if len(upstreamTags) == 0 {
		return nil
	}

	var updateUntags []string
	for key := range upstreamTags {
		_, ok := tags[key]
		if !ok {
			updateUntags = append(updateUntags, key)
		}
	}

	if len(updateUntags) == 0 {
		return nil
	}
	return updateUntags
}

// MergeMaps will add all keys and values from map2 to map1.
func MergeMaps(map1, map2 map[string]string) map[string]string {
	if map1 == nil {
		map1 = make(map[string]string)
	}
	for key, value := range map2 {
		map1[key] = value
	}

	return map1
}

// SubtractMaps will remove all keys and values in map2 from map1.
func SubtractMaps(map1, map2 map[string]string) map[string]string {
	if map1 == nil {
		return nil
	}
	for key := range map2 {
		delete(map1, key)
	}

	return map1
}

func ValuesFromMap(m map[string]string) []string {
	s := make([]string, len(m))
	i := 0
	for _, value := range m {
		s[i] = value
		i++
	}

	return s
}

func GetInstanceTags(templateTags []ec2types.LaunchTemplateTagSpecification) map[string]string {
	tags := make(map[string]string)

	for _, tag := range templateTags {
		if tag.ResourceType == ec2types.ResourceTypeInstance {
			for _, t := range tag.Tags {
				tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
			}
		}
	}
	return tags
}

func CreateTagSpecs(instanceTags map[string]string) []ec2types.LaunchTemplateTagSpecificationRequest {
	if len(instanceTags) == 0 {
		return nil
	}

	tags := make([]ec2types.Tag, 0)
	for key, value := range instanceTags {
		keyCopy := key
		valueCopy := value
		tags = append(tags, ec2types.Tag{Key: &keyCopy, Value: &valueCopy})
	}
	return []ec2types.LaunchTemplateTagSpecificationRequest{
		{
			ResourceType: ec2types.ResourceTypeInstance,
			Tags:         tags,
		},
		{
			ResourceType: ec2types.ResourceTypeVolume,
			Tags:         tags,
		},
		{
			ResourceType: ec2types.ResourceTypeSpotInstancesRequest,
			Tags:         tags,
		},
	}
}

func ConvertToLogTypes(loggingTypes []string) []ekstypes.LogType {
	if len(loggingTypes) == 0 {
		return []ekstypes.LogType{}
	}

	types := make([]ekstypes.LogType, len(loggingTypes))
	for i, lt := range loggingTypes {
		types[i] = ekstypes.LogType(lt)
	}
	return types
}

func ConvertFromLogTypes(logTypes []ekstypes.LogType) []string {
	if len(logTypes) == 0 {
		return []string{}
	}

	types := make([]string, len(logTypes))
	for i, lt := range logTypes {
		types[i] = string(lt)
	}
	return types
}

func CompareStringMaps(map1, map2 map[string]string) bool {
	if len(map1) != len(map2) {
		return false
	}
	for key, value1 := range map1 {
		if value2, exists := map2[key]; !exists || value1 != value2 {
			return false
		}
	}
	return true
}
