/*
** description("").
** copyright('tuoyun,www.tuoyun.net').
** author("fg,Gordon@tuoyun.net").
** time(2021/5/11 9:36).
 */
package kafka

import (
	"OpenIM/pkg/common/constant"
	"OpenIM/pkg/common/tracelog"
	"context"
	"github.com/Shopify/sarama"
)

type MConsumerGroup struct {
	sarama.ConsumerGroup
	groupID string
	topics  []string
}

type MConsumerGroupConfig struct {
	KafkaVersion   sarama.KafkaVersion
	OffsetsInitial int64
	IsReturnErr    bool
}

func NewMConsumerGroup(consumerConfig *MConsumerGroupConfig, topics, addrs []string, groupID string) *MConsumerGroup {
	config := sarama.NewConfig()
	config.Version = consumerConfig.KafkaVersion
	config.Consumer.Offsets.Initial = consumerConfig.OffsetsInitial
	config.Consumer.Return.Errors = consumerConfig.IsReturnErr
	//fmt.Println("init address is ", addrs, "topics is ", topics)
	consumerGroup, err := sarama.NewConsumerGroup(addrs, groupID, config)
	if err != nil {
		panic(err.Error())
	}
	return &MConsumerGroup{
		consumerGroup,
		groupID,
		topics,
	}
}

func (mc *MConsumerGroup) GetContextFromMsg(cMsg *sarama.ConsumerMessage, rootFuncName string) context.Context {
	ctx := tracelog.NewCtx(rootFuncName, "")
	var operationID string
	for _, v := range cMsg.Headers {
		if string(v.Key) == constant.OperationID {
			operationID = string(v.Value)
		}
	}
	tracelog.SetOperationID(ctx, operationID)
	return ctx
}

func (mc *MConsumerGroup) RegisterHandleAndConsumer(handler sarama.ConsumerGroupHandler) {
	ctx := context.Background()
	for {
		err := mc.ConsumerGroup.Consume(ctx, mc.topics, handler)
		if err != nil {
			panic(err.Error())
		}
	}
}
