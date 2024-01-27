package database

import (
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/rds"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func CreateDbSecurityGroup(ctx *pulumi.Context, vpcId pulumi.IDOutput, appSecurityGroupId pulumi.IDOutput) (*ec2.SecurityGroup, error) {
	dbSecurityGroup, err := ec2.NewSecurityGroup(ctx, "DBSecurityGroup", &ec2.SecurityGroupArgs{
		Description: pulumi.String("Enable access for RDS instances"),
		VpcId:       vpcId, // 您的VPC ID
	})
	if err != nil {
		return dbSecurityGroup, err
	}

	// 允许特定端口的入站流量
	_, err = ec2.NewSecurityGroupRule(ctx, "dbIngressRule", &ec2.SecurityGroupRuleArgs{
		Type:                  pulumi.String("ingress"),
		FromPort:              pulumi.Int(3306),
		ToPort:                pulumi.Int(3306),
		Protocol:              pulumi.String("tcp"),
		SecurityGroupId:       dbSecurityGroup.ID(),
		SourceSecurityGroupId: appSecurityGroupId, // 应用程序安全组的ID
	})
	if err != nil {
		return nil, err
	}
	return dbSecurityGroup, nil
}

func CreateRDBInstance(ctx *pulumi.Context, dbSecurityGroupId pulumi.IDOutput, subnetIds []pulumi.IDOutput) (*rds.Instance, error) {
	conf := config.New(ctx, "")
	// 创建rds参数组
	paramGroup, err := rds.NewParameterGroup(ctx, "csye6225-rds-param-group", &rds.ParameterGroupArgs{
		Family:      pulumi.String("mysql8.0"), // 根据使用的 MySQL 版本进行选择
		Description: pulumi.String("Custom parameter group for csye6225 MySQL RDS"),
		Parameters: rds.ParameterGroupParameterArray{
			// 自定义的参数
			&rds.ParameterGroupParameterArgs{
				Name:  pulumi.String("character_set_server"),
				Value: pulumi.String("utf8mb4"),
			},
			&rds.ParameterGroupParameterArgs{
				Name:  pulumi.String("collation_server"),
				Value: pulumi.String("utf8mb4_unicode_ci"),
			},
			// 添加更多参数...
		},
	})
	if err != nil {
		return nil, err
	}

	// 获取数据库密码
	dbPassword := conf.RequireSecret("dbPassword") // 这将是一个加密的 Secret<T> 类型
	var subnetIDs pulumi.StringArray
	for _, id := range subnetIds {
		subnetIDs = append(subnetIDs, id)
	}
	dbSubnetGroup, err := rds.NewSubnetGroup(ctx, "rds-subnet-group", &rds.SubnetGroupArgs{
		SubnetIds: subnetIDs,
	})
	if err != nil {
		return nil, err
	}
	dbInstance, err := rds.NewInstance(ctx, "csye6225", &rds.InstanceArgs{
		Engine:              pulumi.String("mysql"),       // 或 postgres 或 mariadb，根据您的需求
		InstanceClass:       pulumi.String("db.t3.micro"), // 最便宜的类型
		MultiAz:             pulumi.Bool(false),
		Identifier:          pulumi.String("csye6225"),
		Username:            pulumi.String("csye6225"),
		Password:            dbPassword,
		DbSubnetGroupName:   dbSubnetGroup.Name,
		VpcSecurityGroupIds: pulumi.StringArray{dbSecurityGroupId},
		PubliclyAccessible:  pulumi.Bool(false),
		SkipFinalSnapshot:   pulumi.Bool(true),
		DeletionProtection:  pulumi.Bool(false),
		AllocatedStorage:    pulumi.Int(20),            // 最低配置的硬盘空间
		DbName:              pulumi.String("csye6225"), // 数据库名称
		ParameterGroupName:  paramGroup.Name,           // 关联参数组
	})
	if err != nil {
		return nil, err
	}
	return dbInstance, nil
}
