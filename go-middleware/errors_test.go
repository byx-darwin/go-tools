package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCodes_Ranges(t *testing.T) {
	// Redis: 20000-20099
	assert.GreaterOrEqual(t, ErrCodeRedisConnect, 20001)
	assert.LessOrEqual(t, ErrCodeRedisConnect, 20099)
	assert.GreaterOrEqual(t, ErrCodeRedisPing, 20001)
	assert.LessOrEqual(t, ErrCodeRedisPing, 20099)

	// Kafka: 20100-20199
	assert.GreaterOrEqual(t, ErrCodeKafkaConnect, 20100)
	assert.LessOrEqual(t, ErrCodeKafkaConnect, 20199)

	// DB: 20200-20299
	assert.GreaterOrEqual(t, ErrCodeDBConnect, 20200)
	assert.LessOrEqual(t, ErrCodeDBConnect, 20299)

	// ES: 20300-20399
	assert.GreaterOrEqual(t, ErrCodeESConnect, 20300)
	assert.LessOrEqual(t, ErrCodeESConnect, 20399)

	// ClickHouse: 20400-20499
	assert.GreaterOrEqual(t, ErrCodeCHConnect, 20400)
	assert.LessOrEqual(t, ErrCodeCHConnect, 20499)

	// TLS: 20500-20599
	assert.GreaterOrEqual(t, ErrCodeTLSConnect, 20500)
	assert.LessOrEqual(t, ErrCodeTLSConnect, 20599)

	// Observability: 20600-20699
	assert.GreaterOrEqual(t, ErrCodeObsInit, 20600)
	assert.LessOrEqual(t, ErrCodeObsInit, 20699)
}

func TestErrorCodes_Uniqueness(t *testing.T) {
	seen := make(map[int]string)
	codes := []struct {
		code    int
		name    string
	}{
		{ErrCodeRedisConnect, "ErrCodeRedisConnect"},
		{ErrCodeRedisPing, "ErrCodeRedisPing"},
		{ErrCodeRedisOp, "ErrCodeRedisOp"},
		{ErrCodeRedisPipeline, "ErrCodeRedisPipeline"},
		{ErrCodeRedisSentinel, "ErrCodeRedisSentinel"},
		{ErrCodeKafkaConnect, "ErrCodeKafkaConnect"},
		{ErrCodeKafkaSend, "ErrCodeKafkaSend"},
		{ErrCodeKafkaConsume, "ErrCodeKafkaConsume"},
		{ErrCodeKafkaCommit, "ErrCodeKafkaCommit"},
		{ErrCodeKafkaRebalance, "ErrCodeKafkaRebalance"},
		{ErrCodeDBConnect, "ErrCodeDBConnect"},
		{ErrCodeDBQuery, "ErrCodeDBQuery"},
		{ErrCodeDBExec, "ErrCodeDBExec"},
		{ErrCodeDBMigrate, "ErrCodeDBMigrate"},
		{ErrCodeESConnect, "ErrCodeESConnect"},
		{ErrCodeESQuery, "ErrCodeESQuery"},
		{ErrCodeCHConnect, "ErrCodeCHConnect"},
		{ErrCodeCHQuery, "ErrCodeCHQuery"},
		{ErrCodeTLSConnect, "ErrCodeTLSConnect"},
		{ErrCodeTLSSend, "ErrCodeTLSSend"},
		{ErrCodeObsInit, "ErrCodeObsInit"},
		{ErrCodeObsExport, "ErrCodeObsExport"},
	}

	for _, c := range codes {
		existing, exists := seen[c.code]
		if exists {
			t.Errorf("duplicate error code %d: %s and %s", c.code, existing, c.name)
		}
		seen[c.code] = c.name
	}
}

func TestErrorCode_RangeBoundaries(t *testing.T) {
	// All codes should be in 20000-20699 range
	allCodes := []int{
		ErrCodeRedisConnect, ErrCodeRedisPing, ErrCodeRedisOp,
		ErrCodeRedisPipeline, ErrCodeRedisSentinel,
		ErrCodeKafkaConnect, ErrCodeKafkaSend, ErrCodeKafkaConsume,
		ErrCodeKafkaCommit, ErrCodeKafkaRebalance,
		ErrCodeDBConnect, ErrCodeDBQuery, ErrCodeDBExec, ErrCodeDBMigrate,
		ErrCodeESConnect, ErrCodeESQuery,
		ErrCodeCHConnect, ErrCodeCHQuery,
		ErrCodeTLSConnect, ErrCodeTLSSend,
		ErrCodeObsInit, ErrCodeObsExport,
	}

	for _, code := range allCodes {
		assert.GreaterOrEqual(t, code, 20000, "code %d should be >= 20000", code)
		assert.LessOrEqual(t, code, 20699, "code %d should be <= 20699", code)
	}
}
