package pubsub

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Durable = iota
	Transient
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	valByte, _ := json.Marshal(val)
	return ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{ContentType: "application/json", Body: valByte},
	)
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int, // an enum to represent "durable" or "transient"
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
