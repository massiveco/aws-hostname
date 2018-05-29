package main

import (
	"flag"
	"io/ioutil"
	"log"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const hostnamePath = "/etc/hostname"

var prefixLookupName, ec2TagName string
var setEc2Tag, writeToDisk, setHostname bool

func main() {
	flag.StringVar(&prefixLookupName, "prefix-lookup-name", "HostnamePrefix", "Set the Ec2 tag name to read the prefix from")
	flag.BoolVar(&writeToDisk, "write-disk", true, "Instruct aws-hostname to write the generated hostname to disk")
	flag.BoolVar(&setEc2Tag, "write-tag", true, "Instruct aws-hostname to write the generated hostname to an Ec2 instance tag")
	flag.BoolVar(&setHostname, "apply", true, "Instruct aws-hostname to syscall.Sethostname")
	flag.StringVar(&ec2TagName, "tag", "Name", "Which tag to write the hostname to")
	flag.Parse()

	meta := ec2metadata.New(session.New())

	identity, err := meta.GetInstanceIdentityDocument()
	if err != nil {
		log.Fatal(err)
	}

	hostname, err := GenerateHostname(identity)
	if err != nil {
		log.Fatal(err)
	}

	if writeToDisk {
		err = ioutil.WriteFile(hostnamePath, []byte(*hostname), 0600)
		if err != nil {
			log.Fatal(err)
		}
	}

	if setEc2Tag {
		writeTag(identity.InstanceID, "Name", *hostname)
	}

	if setHostname {
		args := []string{"hostname", "-F", "/etc/hostname"}
		err = syscall.Exec("/bin/hostname", args, []string{})
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("Set hostname to: %s", *hostname)
}

// GenerateHostname Generates a hostname from the instances identity and tags
func GenerateHostname(identity ec2metadata.EC2InstanceIdentityDocument) (*string, error) {

	instanceID := identity.InstanceID
	privateIP := identity.PrivateIP
	hashedPrivateIP := strings.Replace(privateIP, ".", "-", 10)

	svc := ec2.New(session.New())
	input := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("resource-id"),
				Values: []*string{
					aws.String(instanceID),
				},
			},
		},
	}

	result, err := svc.DescribeTags(input)
	if err != nil {
		return nil, err
	}

	tags := tagsToMap(*result)

	hostname := tags[prefixLookupName] + hashedPrivateIP

	return &hostname, nil
}

func tagsToMap(tags ec2.DescribeTagsOutput) map[string]string {

	tagMap := make(map[string]string)

	for _, element := range tags.Tags {
		var el = *element
		tagMap[*el.Key] = *el.Value
	}

	return tagMap
}
