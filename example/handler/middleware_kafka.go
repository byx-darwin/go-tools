package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/byx-darwin/go-tools/go-middleware/kafka"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// kafkaWriter Kafka 生产者实例（由 main 通过 SetKafkaWriter 注入）。
var kafkaWriter *kafka.Writer

// kafkaConsumer Kafka 消费者实例（由 main 通过 SetKafkaConsumer 注入）。
var kafkaConsumer *kafka.Consumer

// SetKafkaWriter 注入 Kafka 生产者（在 main 中调用）。
func SetKafkaWriter(w *kafka.Writer) {
	kafkaWriter = w
}

// SetKafkaConsumer 注入 Kafka 消费者（在 main 中调用）。
func SetKafkaConsumer(c *kafka.Consumer) {
	kafkaConsumer = c
}

// RegisterKafkaRoutes 注册 kafka 示例路由。
func RegisterKafkaRoutes(h *server.Hertz) {
	h.POST("/middleware/kafka", kafkaProduceHandler)
	h.GET("/middleware/kafka", kafkaConsumeHandler)
}

// kafkaProduceHandler 发送一条消息到 Kafka。
//
// 请求体：{"key": "xxx", "value": "xxx"}
// Kafka Writer 未配置时返回 "kafka not configured"。
func kafkaProduceHandler(ctx context.Context, c *app.RequestContext) {
	if kafkaWriter == nil {
		hertzresp.Success(c, map[string]any{
			"status":  "not_configured",
			"message": "kafka not configured",
		})
		return
	}

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		hertzresp.Error(ctx, c, err, "invalid request")
		return
	}

	if req.Key == "" {
		req.Key = fmt.Sprintf("key-%d", time.Now().UnixNano())
	}
	if req.Value == "" {
		req.Value = fmt.Sprintf("hello-kafka-%d", time.Now().UnixNano())
	}

	sendErr := kafkaWriter.SendStr(ctx, req.Key, req.Value)

	result := map[string]any{
		"key":   req.Key,
		"value": req.Value,
	}
	if sendErr != nil {
		result["send_error"] = sendErr.Error()
	} else {
		result["sent"] = true
	}

	hertzresp.Success(c, result)
}

// kafkaConsumeHandler 从 Kafka 消费消息（最多等待 3 秒）。
//
// Kafka Consumer 未配置时返回 "kafka not configured"。
func kafkaConsumeHandler(ctx context.Context, c *app.RequestContext) {
	if kafkaConsumer == nil {
		hertzresp.Success(c, map[string]any{
			"status":  "not_configured",
			"message": "kafka not configured",
		})
		return
	}

	// 设置消费超时（3 秒）。
	consumeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	msg, err := kafkaConsumer.ReadMessage(consumeCtx)

	result := map[string]any{}
	if err != nil {
		if consumeCtx.Err() != nil {
			result["message"] = "no messages within 3s timeout"
		} else {
			result["consume_error"] = err.Error()
		}
	} else {
		result["offset"] = msg.Offset
		result["partition"] = msg.Partition
		result["key"] = string(msg.Key)
		result["value"] = string(msg.Value)
		result["timestamp"] = msg.Time
	}

	hertzresp.Success(c, result)
}
