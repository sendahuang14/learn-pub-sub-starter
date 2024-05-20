package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	return func(ps routing.PlayingState) {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
	}
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) {
	return func(move gamelogic.ArmyMove) {
		defer fmt.Print("> ")
		gs.HandleMove(move)
	}
}

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

	ch, _, err := pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilDirect,
		fmt.Sprintf("%s.%s", routing.PauseKey, userName),
		routing.PauseKey,
		pubsub.Transient,
	)
	if err != nil {
		log.Fatalf("Failed to declare and bind a queue: %v", err)
	}

	gs := gamelogic.NewGameState(userName)

	// subscribe to pause.* queue
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		fmt.Sprintf("pause.%s", userName),
		routing.PauseKey,
		pubsub.Transient,
		handlerPause(gs),
	)
	if err != nil {
		fmt.Printf(
			"Failed to subscribe to pause.* queue: %v",
			err,
		)
	}

	// subscribe to army_moves.* queue
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, userName),
		fmt.Sprintf("%s.*", routing.ArmyMovesPrefix),
		pubsub.Transient,
		handlerMove(gs),
	)
	if err != nil {
		fmt.Printf(
			"Failed to subscribe to army_moves.* queue: %v",
			err,
		)
	}

	// REPL
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
			move, err := gs.CommandMove(cmd)
			if err != nil {
				log.Fatalf("Failed to move unit: %v", err)
			}

			err = pubsub.PublishJSON(
				ch,
				routing.ExchangePerilTopic,
				fmt.Sprintf("%s.*", routing.ArmyMovesPrefix),
				move,
			)
			if err != nil {
				log.Fatalf("Failed to publish move message: %v", err)
			}

			fmt.Println("The move is published successfully")

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
}
