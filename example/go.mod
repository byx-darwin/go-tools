module github.com/byx-darwin/go-tools/example

go 1.25

require (
	github.com/byx-darwin/go-tools/go-auth v0.0.0
	github.com/byx-darwin/go-tools/go-common v0.0.0
	github.com/byx-darwin/go-tools/go-framework v0.0.0
	github.com/byx-darwin/go-tools/go-middleware v0.0.0
	github.com/cloudwego/hertz v0.9.6
	github.com/cloudwego/kitex v0.13.3
	github.com/redis/go-redis/v9 v9.7.3
	github.com/segmentio/kafka-go v0.4.47
	github.com/go-sql-driver/mysql v1.8.1
	github.com/elastic/go-elasticsearch/v8 v8.17.0
	github.com/ClickHouse/clickhouse-go/v2 v2.30.3
	github.com/polarismesh/polaris-go v1.6.2
	gopkg.in/yaml.v3 v3.0.1
)

replace (
	github.com/byx-darwin/go-tools/go-auth => ../go-auth
	github.com/byx-darwin/go-tools/go-common => ../go-common
	github.com/byx-darwin/go-tools/go-framework => ../go-framework
	github.com/byx-darwin/go-tools/go-middleware => ../go-middleware
)
