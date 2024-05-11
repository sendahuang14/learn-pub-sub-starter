package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril client...")

	const url = "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ: %v", err)
	} else {
		fmt.Println("Connection to RabbitMQ successful")
	}
	defer conn.Close()

	userName, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("Failed to get client's name: %v", err)
	}

	amqpChan, queue, err := pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilDirect,
		fmt.Sprintf("%s.%s", routing.PauseKey, userName),
		routing.PauseKey,
		pubsub.Transient,
	)
	if err != nil {
		log.Fatalf("Failed to declare and bind a queue: %v", err)
	}
	fmt.Println(amqpChan.IsClosed(), queue)

	gs := gamelogic.NewGameState(userName)

	for {
		cmd := gamelogic.GetInput()
		if len(cmd) == 0 {
			continue
		}

		if cmd[0] == "spawn" {
			err = gs.CommandSpawn(cmd)
			if err != nil {
				log.Fatalf("Failed to spawn a unit: %v", err)
			}

		} else if cmd[0] == "move" {
			_, err := gs.CommandMove(cmd)
			if err != nil {
				log.Fatalf("Failed to move unit: %v", err)
			}

		} else if cmd[0] == "status" {
			gs.CommandStatus()

		} else if cmd[0] == "help" {
			gamelogic.PrintClientHelp()

		} else if cmd[0] == "spam" {
			fmt.Println("Spamming not allowed yet!")

		} else if cmd[0] == "quit" {
			gamelogic.PrintQuit()
			break

		} else {
			log.Fatal("The command is wrong.")

		}
	}

	// wait for ctrl+c
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
}
