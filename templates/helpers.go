package templates

import (
	"bytes"
	"text/template"

	"github.com/aws/aws-sdk-go/aws/endpoints"
)

type EBSCSIDriverTemplateData struct {
	AWSArnPrefix string
	Region       string
	ProviderID   string
	AWSDomain    string
}

type NodeInstanceRoleTemplateData struct {
	AWSArnPrefix string
	EC2Service   string
}

func getAWSDNSSuffix(region string) string {
	if p, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region); ok {
		return p.DNSSuffix()
	}
	return endpoints.AwsPartition().DNSSuffix()
}

func getEC2ServiceEndpoint(region string) string {
	return "ec2." + getAWSDNSSuffix(region)
}

func getArnPrefixForRegion(region string) string {
	if p, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region); ok {
		return "arn:" + p.ID()
	}
	return "arn:" + endpoints.AwsPartition().ID()
}

func GetServiceRoleTemplate(region string) (string, error) {
	tmpl, err := template.New("serviceRole").Parse(ServiceRoleTemplate)
	if err != nil {
		return "", err
	}

	// Create the data for the template
	data := struct {
		AWSArnPrefix string
	}{
		AWSArnPrefix: getArnPrefixForRegion(region),
	}

	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func GetNodeInstanceRoleTemplate(region string) (string, error) {
	tmpl, err := template.New("nodeInstanceRole").Parse(NodeInstanceRoleTemplate)
	if err != nil {
		return "", err
	}

	// Create the data for the template
	data := NodeInstanceRoleTemplateData{
		AWSArnPrefix: getArnPrefixForRegion(region),
		EC2Service:   getEC2ServiceEndpoint(region),
	}

	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func GetEBSCSIDriverTemplate(region string, providerID string) (string, error) {
	tmpl, err := template.New("ebsrole").Parse(EBSCSIDriverTemplate)
	if err != nil {
		return "", err
	}

	// Create the data for the template
	data := EBSCSIDriverTemplateData{
		AWSArnPrefix: getArnPrefixForRegion(region),
		AWSDomain:    getAWSDNSSuffix(region),
		Region:       region,
		ProviderID:   providerID,
	}

	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
