package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	const url = "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ: %v", err)
	} else {
		fmt.Println("Connection to RabbitMQ successful")
	}
	defer conn.Close()

	newChan, _ := conn.Channel()

	// declare queue game_log* and bind it to peril_topic exchange
	_, _, err = pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		"game_log.*",
		pubsub.Durable,
	)

	// print command guidance
	gamelogic.PrintServerHelp()

	for {
		cmd := gamelogic.GetInput()
		if len(cmd) == 0 {
			continue
		}

		if cmd[0] == "pause" {
			fmt.Println("Sending a pause message...")
			err = pubsub.PublishJSON(
				newChan,
				routing.ExchangePerilDirect,
				routing.PauseKey,
				routing.PlayingState{IsPaused: true},
			)
			if err != nil {
				log.Fatalf("Failed to publish json file: %v", err)
			}

		} else if cmd[0] == "resume" {
			fmt.Println("Sending a resume message...")
			err = pubsub.PublishJSON(
				newChan,
				routing.ExchangePerilDirect,
				routing.PauseKey,
				routing.PlayingState{IsPaused: false},
			)
			if err != nil {
				log.Fatalf("Failed to publish json file: %v", err)
			}

		} else if cmd[0] == "quit" {
			fmt.Println("Exiting the game...")
			break

		} else if cmd[0] == "help" {
			gamelogic.PrintServerHelp()

		} else {
			log.Fatal("The command is wrong.")

		}
	}
}
