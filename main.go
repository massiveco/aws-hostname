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

var ec2TagName string
var setEc2Tag, writeToDisk, setHostname bool

func main() {
	flag.BoolVar(&writeToDisk, "write-disk", true, "Instruct aws-hostname to write the generated hostname to disk")
	flag.BoolVar(&setEc2Tag, "write-tag", true, "Instruct aws-hostname to write the generated hostname to an Ec2 instance tag")
	flag.BoolVar(&setHostname, "apply", true, "Instruct aws-hostname to syscall.Sethostname")
	flag.StringVar(&ec2TagName, "tag", "Name", "Which tag to write the hostname to")
	flag.Parse()

	instance, err := getInstance()
	if err != nil {
		log.Fatal(err)
	}

	hostname, err := GenerateHostname(*instance)
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
		svc := ec2.New(session.New())

		input := &ec2.CreateTagsInput{
			Resources: []*string{
				aws.String(*instance.InstanceId),
			},
			Tags: []*ec2.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(*hostname),
				},
			},
		}

		_, err := svc.CreateTags(input)
		if err != nil {
			log.Fatal(err)
		}
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

func getInstance() (*ec2.Instance, error) {

	meta := ec2metadata.New(session.New())

	identity, err := meta.GetInstanceIdentityDocument()
	if err != nil {
		log.Fatal(err)
	}
	svc := ec2.New(session.New())
	describedInstances, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("resource-id"),
				Values: []*string{
					aws.String(identity.InstanceID),
				},
			},
		},
	})

	if describedInstances.Reservations[0] != nil && describedInstances.Reservations[0].Instances[0] != nil {
		return describedInstances.Reservations[0].Instances[0], nil
	}
	return nil, nil
}

// GenerateHostname Generates a hostname from the ec2.Instance and tags
func GenerateHostname(instance ec2.Instance) (*string, error) {

	privateIP := instance.PrivateIpAddress
	hashedPrivateIP := strings.Replace(*privateIP, ".", "-", 10)

	tags := tagsToMap(instance.Tags)

	hostname := tags["HostnamePrefix"] + hashedPrivateIP

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
