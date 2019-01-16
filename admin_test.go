package mq

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"mq-go/config"
	"testing"
	"time"
)

func TestAdmin(t *testing.T) {
	adminMq := Admin()
	msg := AdminMsg{
		Name:  "ybdx",
		Value: "hello",
	}
	adminMq.Publish(msg)
}

func TestQueue_Consume(t *testing.T) {
	adminMq := Admin()
	adminMq.Consume(ack, nil)
}

func nack(delivery amqp.Delivery) {
	data := &body{}
	json.Unmarshal(delivery.Body, data)
	fmt.Printf("%s\n", data.Body)
	delivery.Nack(false, true)
	time.Sleep(time.Second * 5)
}

func ack(delivery amqp.Delivery) {
	//data := &body{}
	body := struct {
		Body    AdminMsg `json:"body"`
		Headers Headers  `json:"header"`
	}{}
	json.Unmarshal(delivery.Body, &body)
	fmt.Printf("%+v\n", body)
	delivery.Ack(false)
}

func TestNew(t *testing.T) {
	ybdxConfig := config.Config{
		PublishUrl:    "amqp://admin:admin@127.0.0.1:5672/test?heartbeat=15",
		ConsumeUrl:    "amqp://admin:admin@127.0.0.1:5672/test?heartbeat=15",
		Queue:         "ybdx",
		Key:           "rk.admin.qa",
		Exchange:      "ex-admin-direct.qa",
		PrefetchCount: 100,
	}
	MqInstance := New("ybdx", ybdxConfig)
	//MqInstance.Publish("this is the first")
	//MqInstance.Publish("this is the Second")
	time.Sleep(1 * time.Second)
	MqInstance.Consume(ack, nil)
}
