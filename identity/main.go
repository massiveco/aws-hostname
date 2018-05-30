package identity

import (
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
)

// GenerateHostname Generates a hostname from the ec2.Instance and tags
func GenerateHostname(instance ec2.Instance) (*string, error) {

	privateIP := instance.PrivateIpAddress
	hashedPrivateIP := strings.Replace(*privateIP, ".", "-", 10)

	tags := tagsToMap(instance.Tags)

	hostname := tags["massive:HostnamePrefix"] + hashedPrivateIP

	return &hostname, nil
}

func tagsToMap(tags []*ec2.Tag) map[string]string {

	tagMap := make(map[string]string)

	for _, element := range tags {
		var el = *element
		tagMap[*el.Key] = *el.Value
	}

	return tagMap
}
