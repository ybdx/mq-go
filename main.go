package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"mq-go/config"
	"os"
	"sync"
	"time"
)

var conf config.C
type Env int

const (
	QA Env = iota
	Production
)

const (
	publishSessionType = iota
	consumeSessionType
)

const mineJson = "application/json"
const headerId = "header_id"

type Queue struct {
	name string
	conf config.Config
	sync.RWMutex
	publishOnce sync.Once
	publishCh   *amqp.Channel
	publishConn *amqp.Connection
	consumeOnce sync.Once
	consumeCh   *amqp.Channel
	consumeConn *amqp.Connection
}

type body struct {
	Body   interface{}
	Header map[string]interface{}
}

type Headers struct {
	HeaderID string `json:"header_id"`
}

type Option struct {
	Header map[string]interface{}
}

func (q *Queue) Publish(msg interface{}, opt ...Option) error {
	if len(opt) > 1 {
		return errors.New("option length must less than 1")
	}

	header := make(map[string]interface{})
	if len(opt) == 1 {
		for k, v := range opt[0].Header {
			header[k] = v
		}
	}

	if _, ok := header[headerId]; !ok {
		v, _ := uuid.NewV4()
		header[headerId] = v.String()
	}

	q.publishOnce.Do(func() {
		_ = q.newSession(publishSessionType)
	})

	q.Lock()
	ch := q.publishCh
	q.Unlock()

	if ch == nil {
		err := fmt.Sprintf("[publish] queue %s channel is not init maybe network error", q.name)
		logrus.Errorln(err)
		return errors.New(err)
	}

	body := body{
		Body:   msg,
		Header: header,
	}
	bodyByte, err := json.Marshal(body)
	if err != nil {
		logrus.Errorf("publish %+v to channel %s failed , error is %+v\n", body, q.name, err)
		return err
	}
	publishing := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  mineJson,
		Body:         bodyByte,
	}
	return ch.Publish(q.conf.Exchange, q.conf.Key, false, false, publishing)
}

func (q *Queue) Consume(f func(delivery amqp.Delivery), stop chan struct{}) {
	q.consumeOnce.Do(func() {
		_ = q.newSession(consumeSessionType)
	})

	for {
		q.Lock()
		ch := q.consumeCh
		q.Unlock()
		if ch == nil {
			err := fmt.Sprintf("[consume] queue %s channel is not init maybe network error", q.conf.Queue)
			logrus.Errorln(err)
			time.Sleep(time.Second)
			continue
		}

		data, err := ch.Consume(q.conf.Queue, "", false, false, false, false, nil)
		if err != nil {
			logrus.Errorf("[consume] queue %s consume failed. %s", q.conf.Queue, err)
			time.Sleep(time.Second)
			continue
		}

	inner:
		for {
			select {
			case d, ok := <-data:
				if !ok {
					logrus.Errorln("[consume] delivery channel closed")
					break inner
				}
				// handle delivery
				f(d)
			case <-stop:
				logrus.Infof("stop consume %s", q.conf.Queue)
				return
			}
		}
		time.Sleep(time.Second)
	}
}

func (q *Queue) newSession(typ int) error {
	var (
		ch      *amqp.Channel
		conn    *amqp.Connection
		oldCh   *amqp.Channel
		oldConn *amqp.Connection
	)
	closeCh := false
	closeConn := false
	failed := true
	var sessionName = "publish"
	var url = q.conf.PublishUrl
	if typ == consumeSessionType {
		sessionName = "consume"
		url = q.conf.ConsumeUrl
	}

	q.RLock()
	if typ == publishSessionType {
		oldCh = q.publishCh
		oldConn = q.publishConn
	} else {
		oldCh = q.consumeCh
		oldConn = q.consumeConn
	}
	q.RUnlock()

	defer func() {
		if conn != nil && closeConn {
			_ = conn.Close()
		}
		if ch != nil && closeCh {
			_ = ch.Close()
		}

		if oldCh != nil || oldConn != nil {
			go func() {
				time.Sleep(10 * time.Second)
				if oldCh != nil {
					_ = oldCh.Close()
				}
				if oldConn != nil {
					_ = oldConn.Close()
				}
			}()
		}

		// reconnect if failed
		if failed {
			time.Sleep(time.Second)
			logrus.Errorf("[%s] new channel failed, retry\n", sessionName)
			_ = q.newSession(typ)
		}
	}()

	conn, err := amqp.Dial(url)
	if err != nil {
		logrus.Errorf("[%s] connect to queue %s url %s failed\n", sessionName, q.name, url)
		closeCh = true
		return err
	}

	ch, err = conn.Channel()
	if err != nil {
		logrus.Errorf("[%s] open queue %s url %s failed %+v\n", sessionName, q.name, url, err)
		closeConn = true
		return err
	}

	err = ch.ExchangeDeclare(q.name, amqp.ExchangeDirect, true, false, false, false, nil)
	if err != nil {
		logrus.Errorf("[%s] exchange declare queue %s exchange %s failed %+v\n", sessionName, q.name, q.conf.Exchange, err)
		return err
	}

	if typ == publishSessionType {
		if err := ch.Confirm(false); err != nil {
			closeCh, closeConn = true, true
			logrus.Errorf("[%s] change channel queue %s confirm mode failed %+v\n", sessionName, q.name, err)
			return err
		}
	} else { // the consume need to bind queue
		// set the queue durable
		// the message need to manual ack
		if _, err := ch.QueueDeclare(q.name, true, false, false, false, nil); err != nil {
			closeCh, closeConn = true, true
			logrus.Errorf("[%s] queue declare %s failed %+v\n", sessionName, q.conf.Queue, err)
			return err
		}
		if err := ch.QueueBind(q.name, q.conf.Key, q.conf.Exchange, false, nil); err != nil {
			closeCh, closeConn = true, true
			logrus.Errorf("[%s] bind queue %s key %s exchange %s failed %+v\n", sessionName, q.conf.Queue, q.conf.Key, q.conf.Exchange, err)
			return err
		}
		// set qos, prefetch is the max value that consume can no ack
		if err := ch.Qos(q.conf.PrefetchCount, 0, false); err != nil {
			closeCh, closeConn = true, true
			logrus.Errorf("[%s] set %s channel qos failed %s\n", sessionName, q.name, err)
			return err
		}
	}

	closeCh, closeConn, failed = false, false, false

	notifyChan := make(chan *amqp.Error)
	ch.NotifyClose(notifyChan)
	// listening channel close
	go func() {
		err := <-notifyChan
		logrus.Errorf("[%s] channel %s is closed, err %+v\n", sessionName, q.name, err)
		_ = q.newSession(typ)
	}()

	logrus.Infof("[%s] create channel %s success\n", sessionName, q.name)
	q.Lock()
	if typ == publishSessionType {
		q.publishConn = conn
		q.publishCh = ch
	} else {
		q.consumeConn = conn
		q.consumeCh = ch
	}
	q.Unlock()

	return nil
}


func getConfig(env Env) config.C {
	switch env {
	case QA:
		return config.QA
	case Production:
		return config.Production
	default:
		panic(fmt.Sprintf("unsupported enviroment %d", env))
	}
}

func init() {
	envStr := os.Getenv("NODE_ENV")
	var env Env
	switch envStr {
	case "qa":
		env = QA
	case "production":
		env = Production
	default:
		env = QA
	}
	conf = getConfig(env)
}

func initQueue(name string) *Queue {
	c, ok := conf.Mq[name]
	if !ok {
		logrus.Panicf("queue %s not exist \n", name)
	}
	return New(name, c)
}

// New is for user to configure themselves config which not in qa.go or production.go
func New(name string, c config.Config) *Queue {
	return &Queue{
		name: name,
		conf: c,
	}
}