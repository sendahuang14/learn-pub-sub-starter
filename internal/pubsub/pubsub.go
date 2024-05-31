package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueType int

const (
	Durable QueueType = iota
	Transient
)

type Acktype int

const (
	Ack Acktype = iota
	NackRequeue
	NackDiscard
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	valByte, _ := json.Marshal(val) // struct to json
	return ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{ContentType: "application/json", Body: valByte},
	)
}

// Declare queue and bind it to the exchange
func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType QueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	newChan, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	queue, err := newChan.QueueDeclare(
		queueName,
		simpleQueueType == Durable,
		simpleQueueType == Transient,
		simpleQueueType == Transient,
		false,
		amqp.Table{"x-dead-letter-exchange": "peril_dlx"},
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	err = newChan.QueueBind(
		queueName,
		key,
		exchange,
		false,
		nil,
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	return newChan, queue, err
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType QueueType,
	handler func(T) Acktype,
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
			var g T
			err = json.Unmarshal(d.Body, &g) // json to struct g
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
