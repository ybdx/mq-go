package config

var QA = C{
	Mq: map[string]Config{
		"admin": {
			PublishUrl: "amqp://admin:admin@127.0.0.1:5672/test?heartbeat=15",
			ConsumeUrl: "amqp://admin:admin@127.0.0.1:5672/test?heartbeat=15",
			Queue: "q.admin.qa",
			Key:"rk.admin.qa",
			Exchange:"ex-admin-direct.qa",
			PrefetchCount:100,
		},
	},
}
