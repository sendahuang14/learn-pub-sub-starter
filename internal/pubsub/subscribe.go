package pubsub

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType QueueType,
	handler func(T) Acktype,
) error {
	jsonUnmarshaller := func(b []byte) (T, error) {
		var g T
		err := json.Unmarshal(b, &g)
		return g, err
	}

	return subscribe(
		conn,
		exchange,
		queueName,
		key,
		simpleQueueType,
		handler,
		jsonUnmarshaller,
	)
}

func SubscribeGob[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType QueueType,
	handler func(T) Acktype,
) error {
	gobDecoder := func(b []byte) (T, error) {
		buf := bytes.NewBuffer(b)
		dec := gob.NewDecoder(buf)
		var g T
		err := dec.Decode(&g)
		return g, err
	}

	return subscribe(
		conn,
		exchange,
		queueName,
		key,
		simpleQueueType,
		handler,
		gobDecoder,
	)
}

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType QueueType,
	handler func(T) Acktype,
	unmarshaller func([]byte) (T, error),
) error {
	// declare a queue and bind it to an exchange
	newChan, _, err := DeclareAndBind(
		conn,
		exchange,
		queueName,
		key,
		simpleQueueType,
	)
	if err != nil {
		return fmt.Errorf("Failed to declare and bind a queue: %v", err)
	}

	// deliver queued messages
	deliveryChan, err := newChan.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)

	// Ack all the delivered messages
	go func() {
		for d := range deliveryChan {
			g, err := unmarshaller(d.Body)
			if err != nil {
				break
			}

			acktype := handler(g)

			switch acktype {
			case NackRequeue:
				err = d.Nack(false, true)
				gamelogic.WriteLog(routing.GameLog{
					CurrentTime: time.Now(),
					Message:     "Nack and requeued",
					Username:    queueName,
				})

			case NackDiscard:
				err = d.Nack(false, false)
				gamelogic.WriteLog(routing.GameLog{
					CurrentTime: time.Now(),
					Message:     "Nack and discarded",
					Username:    queueName,
				})

			case Ack:
				err = d.Ack(false)
				gamelogic.WriteLog(routing.GameLog{
					CurrentTime: time.Now(),
					Message:     "Ack",
					Username:    queueName,
				})
			}

			if err != nil {
				break
			}
		}
	}()

	return err
}
