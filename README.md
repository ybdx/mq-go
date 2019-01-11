# mq-go

* 断线重连， 时间间隔1s
* queue持久化和message持久化，consume手动ack，保证数据不丢失

### 环境变量
使用NODE_ENV 环境变量区分线上（production） or 测试(qa)

### 使用
库中没有配置新申请的队列时，可以使用如下方法
```
conf := config.Config{
    PublishUrl:    "amqp://admin:admin@127.0.0.1:5672/test?heartbeat=15",
    ConsumeUrl:    "amqp://admin:admin@127.0.0.1:5672/test?heartbeat=15",
    Queue:         "ybdx",
    Key:           "rk.admin.qa",
    Exchange:      "ex-admin-direct.qa",
    PrefetchCount: 100,
}
mq := New("queueName", conf)

// 使用publish
msg := "hello boy, this is the first mq publish"
var opt = Option{
    Header: map[string]interface {
        "traceId": "0202fe12112ffaeb"
    }
}
mq.Publish(msg, opt)

// 使用consume

func handleData(delivery amqp.Delivery) {
    body := struct {
    	Body    AdminMsg `json:"body"`
    	Headers Headers  `json:"headers"`
    }{}
    json.Unmarshal(delivery.Body, &body)
    ...
}
stop := make(chan struct{}) // 是否取消消费
mq.Consume(handleData, nil)
```

### 添加新的mq
参考admin.go
