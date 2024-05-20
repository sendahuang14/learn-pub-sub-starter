package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueType int

const (
	Durable QueueType = iota
	Transient
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
		nil,
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
	handler func(T),
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
			err = json.Unmarshal(d.Body, g) // json to struct g
			if err != nil {
				break
			}
			handler(g)
			err = d.Ack(false)
			if err != nil {
				break
			}
		}
	}()

	return err
}
