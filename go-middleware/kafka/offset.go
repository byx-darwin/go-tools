package kafka

import (
	"context"
	"errors"
	"strconv"

	"github.com/segmentio/kafka-go"
)

var errEmptyBrokers = errors.New("kafka: brokers must not be empty")

// PartitionOffsets 通过 kafka.Conn 查询 topic 各分区的 log-end offset
// （分区最新写入位置），不依赖 consumer group 状态，查询完成后立即关闭连接。
// 返回值不包含任何"已消费到"的信息——如需 lag，见 Consumer.Lag。
func PartitionOffsets(ctx context.Context, brokers []string, topic string) (map[int]int64, error) {
	if len(brokers) == 0 {
		return nil, ErrOffsetQuery.Wrap(errEmptyBrokers)
	}

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return nil, ErrOffsetQuery.Wrap(err)
	}
	defer func() { _ = conn.Close() }()

	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return nil, ErrOffsetQuery.Wrap(err)
	}

	offsets := make(map[int]int64, len(partitions))
	for _, p := range partitions {
		pConn, dialErr := kafka.DialLeader(ctx, "tcp", brokers[0], topic, p.ID)
		if dialErr != nil {
			return nil, ErrOffsetQuery.Wrap(dialErr)
		}
		last, offErr := pConn.ReadLastOffset()
		_ = pConn.Close()
		if offErr != nil {
			return nil, ErrOffsetQuery.Wrap(offErr)
		}
		offsets[p.ID] = last
	}
	return offsets, nil
}

// Lag 返回当前 Consumer 自身各分区的消费延迟：PartitionOffsets 得到的
// log-end offset 减去 Reader.Stats() 中已消费的 offset（仅对本 Consumer
// 当前分配到的 partition 生效——kafka-go 的 ReaderStats.Partition 是
// string 类型，按字符串比较匹配；其余分区直接返回 log-end offset 本身，
// 因为本 Consumer 尚未消费其中任何数据）。仅反映本 Consumer 实例已消费的
// 进度，不能查询同一 group 内其他 consumer 的 lag。
func (c *Consumer) Lag(ctx context.Context) (map[int]int64, error) {
	logEnd, err := PartitionOffsets(ctx, c.brokers, c.topic)
	if err != nil {
		return nil, err
	}

	stats := c.r.Stats()
	lag := make(map[int]int64, len(logEnd))
	for partition, end := range logEnd {
		if strconv.Itoa(partition) == stats.Partition {
			lag[partition] = end - stats.Offset
			continue
		}
		lag[partition] = end
	}
	return lag, nil
}

// Seek 定位到指定 offset 重新消费。要求 Consumer 以非 consumer-group 模式
// （ReaderConfig.GroupID 为空）创建，否则返回 ErrSeek（kafka-go 限制：
// consumer-group 模式不支持 Seek）。
func (c *Consumer) Seek(ctx context.Context, offset int64) error {
	if err := c.r.SetOffset(offset); err != nil {
		return ErrSeek.Wrap(err)
	}
	return nil
}
