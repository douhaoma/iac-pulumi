package main

import (
	"fmt"
	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/route53"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"iac-pulumi/AWS-Roles"
	AWS_SNS "iac-pulumi/AWS-SNS"
	"iac-pulumi/GCP"
	"iac-pulumi/database"
	"iac-pulumi/highAvailability"
	"net"
	"strconv"
	"strings"
)

func main() {

	pulumi.Run(func(ctx *pulumi.Context) error {
		// get variables from yaml
		zones, err := aws.GetAvailabilityZones(ctx, nil, nil)
		numOfAV := len(zones.Names)
		vpcCidr, _ := ctx.GetConfig("vpc:cidrBlock")
		vpcName, _ := ctx.GetConfig("vpc:name")
		igwName, _ := ctx.GetConfig("igw:name")
		region, _ := ctx.GetConfig("aws:region")
		// 创建 VPC
		myVPC, err := ec2.NewVpc(ctx, vpcName, &ec2.VpcArgs{
			CidrBlock: pulumi.String(vpcCidr),
			Tags: pulumi.StringMap{
				"Name": pulumi.String(vpcName),
			},
		})
		myVPC.ID()
		if err != nil {
			return err
		}
		// 创建 Internet Gateway
		myIGW, err := ec2.NewInternetGateway(ctx, igwName, &ec2.InternetGatewayArgs{
			Tags: pulumi.StringMap{
				"Name": pulumi.String(igwName),
			},
		})
		if err != nil {
			return err
		}
		// 附加 Internet Gateway 到 VPC
		_, err = ec2.NewInternetGatewayAttachment(ctx, "csye6225-igw-attachment", &ec2.InternetGatewayAttachmentArgs{
			VpcId:             myVPC.ID(),
			InternetGatewayId: myIGW.ID(),
		})
		if err != nil {
			return err
		}

		// 创建公有路由表
		publicRouteTable, err := ec2.NewRouteTable(ctx, "public-route-table", &ec2.RouteTableArgs{
			VpcId: myVPC.ID(),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("public-route-table"), // 只需要在 Name 部分使用 pulumi.String
			},
		})
		if err != nil {
			return err
		}

		// 创建私有路由表
		privateRouteTable, err := ec2.NewRouteTable(ctx, "private-route-table", &ec2.RouteTableArgs{
			VpcId: myVPC.ID(),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("private-route-table"), // 只需要在 Name 部分使用 pulumi.String
			},
		})
		if err != nil {
			return err
		}

		destinationCidr, _ := ctx.GetConfig("publicRoute:destinationCIDR")
		// 创建公有路由
		_, err = ec2.NewRoute(ctx, "public-route", &ec2.RouteArgs{
			RouteTableId:         publicRouteTable.ID(),
			DestinationCidrBlock: pulumi.String(destinationCidr),
			GatewayId:            myIGW.ID(),
		})
		if err != nil {
			return err
		}

		subnetBits := 24 - getHostBits(vpcCidr) // 要扩展的位数
		_, mainNet, _ := net.ParseCIDR(vpcCidr)
		var privateSubnetIds []pulumi.IDOutput
		var publicSubnetIds []pulumi.IDOutput
		// 创建3个公共子网，并和公共路由表关联
		for i := 1; i <= numOfAV && i <= 3; i++ {
			// 使用 Subnet 函数来创建子网
			subnet, _ := cidr.Subnet(mainNet, subnetBits, i)
			publicSubnet, err := ec2.NewSubnet(ctx, "publicSubnet"+strconv.Itoa(i), &ec2.SubnetArgs{
				VpcId:               myVPC.ID(),
				CidrBlock:           pulumi.String(subnet.String()),
				AvailabilityZone:    pulumi.String(zones.Names[i-1]),
				MapPublicIpOnLaunch: pulumi.Bool(true), // 这行会启用自动分配公共IPv4地址
				Tags: pulumi.StringMap{
					"Name": pulumi.String("public-subnet" + strconv.Itoa(i)),
				},
			})
			_, err = ec2.NewRouteTableAssociation(ctx, "publicAssociation"+strconv.Itoa(i), &ec2.RouteTableAssociationArgs{
				SubnetId:     publicSubnet.ID(),
				RouteTableId: publicRouteTable.ID(),
			})
			if err != nil {
				return err
			}
			publicSubnetIds = append(publicSubnetIds, publicSubnet.ID())
		}
		// 创建3个私有子网，并和私有路由表关联
		for i := 1; i <= numOfAV && i <= 3; i++ {
			subnet, _ := cidr.Subnet(mainNet, subnetBits, i+3)
			privateSubnet, err := ec2.NewSubnet(ctx, "privateSubnet"+strconv.Itoa(i), &ec2.SubnetArgs{
				VpcId:            myVPC.ID(),
				CidrBlock:        pulumi.String(subnet.String()),
				AvailabilityZone: pulumi.String(zones.Names[i-1]),
				Tags: pulumi.StringMap{
					"Name": pulumi.String("private-subnet" + strconv.Itoa(i)),
				},
			})
			_, err = ec2.NewRouteTableAssociation(ctx, "privateAssociation"+strconv.Itoa(i), &ec2.RouteTableAssociationArgs{
				SubnetId:     privateSubnet.ID(),
				RouteTableId: privateRouteTable.ID(),
			})
			if err != nil {
				return err
			}
			privateSubnetIds = append(privateSubnetIds, privateSubnet.ID())
		}

		// 创建app安全组
		appSG, lbSG, _ := highAvailability.CreateApp_LoadBalanSecurityGroup(ctx, myVPC.ID())

		//创建数据库安全组
		DBSG, _ := database.CreateDbSecurityGroup(ctx, myVPC.ID(), appSG.ID())

		// 创建rds
		dbInstance, _ := database.CreateRDBInstance(ctx, DBSG.ID(), privateSubnetIds)

		// 获取 RDS 实例的端点
		dbEndpoint := dbInstance.Endpoint

		// 设置userdata
		conf := config.New(ctx, "")

		// 创建 SNS topic
		topic, err := AWS_SNS.CreateSNSTopic(ctx)

		// 获取数据库密码
		dbPassword := conf.RequireSecret("dbPassword")

		// 异步获取password和endpoint
		resolvedDbPassword := dbPassword.ApplyT(func(pw string) string {
			return pw
		}).(pulumi.StringOutput)

		resolvedDbEndpoint := dbEndpoint.ApplyT(func(ep string) string {
			return ep
		}).(pulumi.StringOutput)

		userDataOutput := pulumi.All(resolvedDbPassword, resolvedDbEndpoint, topic.Arn).ApplyT(
			func(args []interface{}) string {
				password := args[0].(string)
				endpoint := args[1].(string)
				topicArn := args[2].(string)

				// 配置文件和启动脚本
				cloudWatchAgentConfig := `
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
    -a fetch-config \
    -m ec2 \
    -c file:/opt/cloudwatch-config.json \
    -s`

				// 用户数据脚本
				userData := fmt.Sprintf(`#!/bin/bash
ENV_FILE="/opt/csye6225/application.properties"
echo "[mysql]" >> $ENV_FILE
echo "username=csye6225" >> $ENV_FILE
echo "password=%s" >> $ENV_FILE
echo "hostname=%s" >> $ENV_FILE
echo "database=csye6225" >> $ENV_FILE
echo "[sns]" >> $ENV_FILE
echo "topicArn=%s" >> $ENV_FILE
echo "region=%s" >> $ENV_FILE
chown csye6225:csye6225 ${ENV_FILE}
chmod 444 ${ENV_FILE}

%s`, password, endpoint, topicArn, region, cloudWatchAgentConfig)

				return userData
			}).(pulumi.StringOutput)

		// 创建 EC2 iam role
		role, err := AWS_Roles.CreateEC2Role(ctx)
		if err != nil {
			return err
		}

		// 创建 CW policy attachment
		err = AWS_Roles.CreateCWPolicyAttachment(ctx, role)
		if err != nil {
			return err
		}

		// 创建 SNS policy attachment
		err = AWS_Roles.CreateSNSPolicyAttachment(ctx, role)
		if err != nil {
			return err
		}

		// 创建目标组
		targetGroup, err := highAvailability.CreateTargetGroup(ctx, myVPC.ID())
		if err != nil {
			return err
		}

		// 创建负载均衡器
		loadBalancer, err := highAvailability.CreateLoadBalancer(ctx, targetGroup, lbSG, publicSubnetIds)
		if err != nil {
			return err
		}

		// 创建自动伸缩组启动模板
		autoScalingTemplate, err := highAvailability.CreateAutoScalingTemplate(ctx, role, appSG, userDataOutput)
		if err != nil {
			return err
		}

		// 创建自动伸缩组
		_, err = highAvailability.CreateAutoScalingGroup(ctx, targetGroup, autoScalingTemplate, publicSubnetIds)
		if err != nil {
			fmt.Println(err)
			return err
		}

		// 创建dynamoDB
		table, err := database.CreateDynamoDB(ctx)
		if err != nil {
			return err
		}

		// 创建 GCP service account 和 key
		_, key, bucket, err := GCP.CreateGCPServiceAccount(ctx)

		// 创建 lambda函数
		err = AWS_SNS.CreateLambda(ctx, topic, key, bucket, table)
		if err != nil {
			fmt.Println(err)
			return err
		}

		// 创建 A 记录的别名记录，使用 lb 的 dns，直接指向负载均衡器
		zoneId, _ := ctx.GetConfig("dns:hostedZoneId")
		domainName, _ := ctx.GetConfig("dns:domainName")
		_, err = route53.NewRecord(ctx, "csye6225-webServerRecord", &route53.RecordArgs{
			ZoneId: pulumi.String(zoneId),     // 托管区域 ID
			Name:   pulumi.String(domainName), // 子域名
			Type:   pulumi.String("A"),
			Aliases: route53.RecordAliasArray{
				&route53.RecordAliasArgs{
					Name:                 loadBalancer.DnsName,
					ZoneId:               loadBalancer.ZoneId,
					EvaluateTargetHealth: pulumi.Bool(true),
				},
			},
		})
		if err != nil {
			return err
		}

		return nil
	})
}

func getHostBits(cidr string) int {
	parts := strings.Split(cidr, "/")
	res, _ := strconv.Atoi(parts[1])
	return res
}
