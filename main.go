package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/rds"

	"github.com/aws/aws-sdk-go/aws"

	nuke_aws "github.com/gruntwork-io/cloud-nuke/aws"
	"github.com/gruntwork-io/cloud-nuke/externalcreds"
)

var (
//	ec2Svc *ec2.EC2
	elbSvc *elbv2.ELBV2
	rdsSvc *rds.RDS
)

func main() {

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	
	// Services
		//ec2Svc = ec2.New(sess)
		elbSvc = elbv2.New(sess)
		rdsSvc = rds.New(sess)

	// You can scan multiple regions at once, or just pass a single region for speed
	// targetRegions := []string{"us-east-1", "us-west-1", "us-west-2"}
	targetRegions := []string{"eu-west-3", "us-east-1"}

	excludeRegions := []string{}
	// You can simultaneously target multiple resource types as well
	resourceTypes := []string{"ec2", "vpc"}
	excludeResourceTypes := []string{}
	// excludeAfter is parsed identically to the --older-than flag
	excludeAfter := time.Now()

	// Any custom settings you want
	myCustomConfig := &aws.Config{}

	myCustomConfig.WithMaxRetries(3)
	myCustomConfig.WithLogLevel(aws.LogDebugWithRequestErrors)
	// Optionally, set custom credentials
	// myCustomConfig.WithCredentials()

	// Be sure to set your config prior to calling any library methods such as NewQuery
	externalcreds.Set(myCustomConfig)

	// NewQuery is a convenience method for configuring parameters you want to pass to your resource search
	query, err := nuke_aws.NewQuery(
		targetRegions,
		excludeRegions,
		resourceTypes,
		excludeResourceTypes,
		excludeAfter,
	)
	if err != nil {
		fmt.Println(err)
	}

	// InspectResources still returns *AwsAccountResources, but this struct has been extended with several
	// convenience methods for quickly determining if resources exist in a given region
	accountResources, err := nuke_aws.InspectResources(query)
	if err != nil {
		fmt.Println(err)
	}

	// You can call GetRegion to examine a single region's resources
	usWest1Resources := accountResources.GetRegion("eu-west-1")

	// Then interrogate them with the new methods:

	// Count the number of any resource type within the region
	countOfEc2InUsWest1 := usWest1Resources.CountOfResourceType("ec2")

	fmt.Printf("countOfEc2InUsWest1: %d\n", countOfEc2InUsWest1)

	fmt.Printf("usWest1Resources.ResourceTypePresent(\"ec2\"):%b\n", usWest1Resources.ResourceTypePresent("ec2"))
	// usWest1Resources.ResourceTypePresent("ec2"): true

	// Get all the resource identifiers for a given resource type
	// First: we're only looking for ec2 instances
	resourceIds := usWest1Resources.IdentifiersForResourceType("ec2")
	// fmt.Printf("resourceIds: %s", resourceIds)
	// resourceIds:  [i-0c5d16c3ef28dda24 i-09d9739e1f4d27814]
	for _, value := range resourceIds {
		fmt.Println("Stopping EC2 instance: ", value)
		//FreezeEC2(&value)
	}

    // 2-  We're releasing all the EIPs
	//ec2ReleaseAddresses()

	// 3- Load Balancers
	// StopAlb()

	// 4- Delete all the RDS instances
	StopRds()
}

func StopRds() {
	fmt.Println("Searching for RDS Instances")
	instances, err := getRDSInstances()
	fmt.Println("Stopping RDS instances: ", instances)
	if err != nil {
		panic(err)
	}

	clusters, err := getRDSClusters()
	fmt.Println("Stopping RDS clusters: ", clusters)
	if err != nil {
		panic(err)
	}

	//This command doesn't apply to RDS Custom, Aurora MySQL, and Aurora PostgreSQL. For Aurora clusters we'll use StopDBCluster instead.
	// Instances and clusters will be deleted

	// stopRDSInstances(instances)
	//stopRDSClusters(clusters)
}

func getRDSInstances() ([]*string, error) {
	var instances []*string

	result, err := rdsSvc.DescribeDBInstances(nil)

	if err != nil {
		return nil, err
	}

	for _, rds := range result.DBInstances {
		instances = append(instances, rds.DBInstanceIdentifier)
	}

	return instances, nil
}

func getRDSClusters() ([]*string, error) {
	var instances []*string

	result, err := rdsSvc.DescribeDBClusters(nil)

	if err != nil {
		return nil, err
	}

	for _, rds := range result.DBClusters {
		instances = append(instances, rds.DBClusterIdentifier)
	}

	return instances, nil
}

func stopRDSInstances(instances []*string) {
	for _, instance := range instances {

		params := &rds.DeleteDBInstanceInput{
			DBInstanceIdentifier: instance,
			SkipFinalSnapshot:    aws.Bool(true),
		}
		     
		_, err := rdsSvc.DeleteDBInstance(params)

		if err != nil {
			fmt.Printf("Failed to terminate RDS", err)
		}

	}
}

func stopRDSClusters(instances []*string) {
	for _, instance := range instances {
		params := &rds.DeleteDBClusterInput{
			DBClusterIdentifier: instance,
			SkipFinalSnapshot:   aws.Bool(true),
		}

		// Sending a termination request for DB Clusters.
		_, err := rdsSvc.DeleteDBCluster(params)

		if err != nil {
			fmt.Printf("Failed to terminate RDS Cluster", err)
		}

	}
}

func StopAlb() {
	fmt.Println("Searching for ALB / ELB / NLBs Instances")
	instances, err := getLoadBalancersInstances()

	if err != nil {
		panic(err)
	}

	terminateLoadBalancers(instances)
}

func getLoadBalancersInstances() ([]*string, error) {

	var instances []*string

	input := &elbv2.DescribeLoadBalancersInput{}
	result, err := elbSvc.DescribeLoadBalancers(input)

	if err != nil {
		return nil, err
	}

	for _, lb := range result.LoadBalancers {
		instances = append(instances, lb.LoadBalancerArn)
	}

	return instances, nil

}

func terminateLoadBalancers(instances []*string) {
	for _, instance := range instances {

		params := &elbv2.DeleteLoadBalancerInput{
			LoadBalancerArn: instance,
		}
		fmt.Println("- Terminating Load Balancer: ", params)
		_, err := elbSvc.DeleteLoadBalancer(params)

	 	if err != nil {
	 		fmt.Printf("Failed to terminate lb", err)
		}

	}
}

// StopInstance stops an Amazon EC2 instance.
// Inputs:
//
//	svc is an Amazon EC2 service client
//	instance ID is the ID of the instance
//
// Output:
//
//	If success, nil
//	Otherwise, an error from the call to StopInstances
func StopInstance(svc ec2iface.EC2API, instanceID *string) error {
	// snippet-start:[ec2.go.start_stop_instances.stop]
	input := &ec2.StopInstancesInput{
		InstanceIds: []*string{
			instanceID,
		},
		DryRun: aws.Bool(true),
	}
	_, err := svc.StopInstances(input)
	awsErr, ok := err.(awserr.Error)
	if ok && awsErr.Code() == "DryRunOperation" {
		input.DryRun = aws.Bool(false)
		_, err = svc.StopInstances(input)
		// snippet-end:[ec2.go.start_stop_instances.stop]
		if err != nil {
			return err
		}

		return nil
	}

	return err
}

func FreezeEC2(instanceID *string) {
	fmt.Printf("resourceIds: %s", instanceID)

	//	state := flag.String("s", "", "The state to put the instance in: START or STOP")
	//	flag.Parse()

	if *instanceID == "" {
		fmt.Println("You must supply an instance ID")
		return
	}
	// snippet-end:[ec2.go.start_stop_instances.args]

	// snippet-start:[ec2.go.start_stop_instances.session]
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := ec2.New(sess)

	err := StopInstance(svc, instanceID)
	if err != nil {
		fmt.Println("Got an error stopping the instance")
		fmt.Println(err)
		return
	}

	fmt.Println("Stopped instance with ID " + *instanceID)
}

func fmtAddress(addr *ec2.Address) string {
	out := fmt.Sprintf("IP: %s,  allocation id: %s",
		aws.StringValue(addr.PublicIp), aws.StringValue(addr.AllocationId))
	if addr.InstanceId != nil {
		out += fmt.Sprintf(", instance-id: %s", *addr.InstanceId)
	}
	return out
}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func ec2ReleaseAddresses() {
	// Discover all the Addresses first
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := ec2.New(sess)

	// ***************************************************************

	// Make the API request to EC2 filtering for the addresses in the
	// account's VPC.
	result, err := svc.DescribeAddresses(&ec2.DescribeAddressesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("domain"),
				Values: aws.StringSlice([]string{"vpc"}),
			},
		},
	})
	if err != nil {
		exitErrorf("Unable to elastic IP address, %v", err)
	}

	// Printout the IP addresses if there are any.
	if len(result.Addresses) == 0 {
		fmt.Printf("No elastic IPs for %s region\n", *svc.Config.Region)
	} else {
		fmt.Println("Elastic IPs Cleanup:")
		for _, addr := range result.Addresses {
			// fmt.Println("*", fmtAddress(addr))
			fmt.Println("* Releasing: ", aws.StringValue(addr.AllocationId))
			// Release the IP Addresse using its allocation ID
			//releaseAddress()
		}
	}
}
