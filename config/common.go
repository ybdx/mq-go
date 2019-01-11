package config

type Config struct {
	PublishUrl    string
	ConsumeUrl    string
	Queue         string
	Key           string
	Exchange      string
	PrefetchCount int // 流量控制，表示一个消费者最多可以有prefetchCount个未ack的消息
}

type C struct {
	Mq map[string]Config
}
