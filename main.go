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
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/massiveco/aws-hostname/identity"
)

const hostnamePath = "/etc/hostname"

var ec2TagName string
var setEc2Tag, writeToDisk, setHostname, writeToRoute53 bool

func main() {
	flag.BoolVar(&writeToDisk, "write-disk", true, "Instruct aws-hostname to write the generated hostname to disk")
	flag.BoolVar(&writeToRoute53, "write-route53", true, "Instruct aws-hostname to write the generated hostname to a Route53 A record")
	flag.BoolVar(&setEc2Tag, "write-tag", true, "Instruct aws-hostname to write the generated hostname to an Ec2 instance tag")
	flag.BoolVar(&setHostname, "apply", true, "Instruct aws-hostname to syscall.Sethostname")
	flag.StringVar(&ec2TagName, "tag", "Name", "Which tag to write the hostname to")
	flag.Parse()

	instance, err := getInstance()
	if err != nil {
		log.Fatal(err)
	}

	hostname, err := identity.GenerateHostname(*instance)
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

	if writeToRoute53 {
		zoneID := extractTag("massive:DNS-SD:Route53:zone", instance.Tags)
		r53 := route53.New(session.New())
		zone, err := r53.GetHostedZone(&route53.GetHostedZoneInput{Id: zoneID})
		if err != nil {
			log.Fatal(err)
		}
		fqdn := strings.Join([]string{*hostname, *zone.HostedZone.Name}, ".")

		_, err = r53.ChangeResourceRecordSets(&route53.ChangeResourceRecordSetsInput{
			ChangeBatch: &route53.ChangeBatch{
				Changes: []*route53.Change{{
					Action: aws.String("UPSERT"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name:            aws.String(fqdn),
						Type:            aws.String("A"),
						ResourceRecords: []*route53.ResourceRecord{&route53.ResourceRecord{Value: aws.String(*instance.PrivateIpAddress)}},
						TTL:             aws.Int64(60),
					},
				}},
			},
			HostedZoneId: aws.String(*zoneID),
		})
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

func extractTag(tagName string, tags []*ec2.Tag) *string {

	for _, tag := range tags {
		if *tag.Key == tagName {
			return tag.Value
		}
	}

	return nil
}
