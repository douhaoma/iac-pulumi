package database

import (
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/dynamodb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func CreateDynamoDB(ctx *pulumi.Context) (*dynamodb.Table, error) {
	// 创建 DynamoDB 表
	table, err := dynamodb.NewTable(ctx, "emailStatusTable", &dynamodb.TableArgs{
		Attributes: dynamodb.TableAttributeArray{
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("sentId"),
				Type: pulumi.String("S"),
			},
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("toEmailAddress"),
				Type: pulumi.String("S"),
			},
		},

		BillingMode: pulumi.String("PAY_PER_REQUEST"),
		HashKey:     pulumi.String("sentId"),         // 分区键
		RangeKey:    pulumi.String("toEmailAddress"), // 排序键
	})
	if err != nil {
		return nil, err
	}

	return table, nil
}
