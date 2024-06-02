package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.Acktype {
	return func(ps routing.PlayingState) pubsub.Acktype {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.ArmyMove) pubsub.Acktype {
	return func(move gamelogic.ArmyMove) pubsub.Acktype {
		defer fmt.Print("> ")

		mvo := gs.HandleMove(move)

		switch mvo {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack

		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(
				ch,
				routing.ExchangePerilTopic,
				fmt.Sprintf(
					"%s.%s",
					routing.WarRecognitionsPrefix,
					gs.Player.Username, // username of the user who just received moveoutcome
				),
				gamelogic.RecognitionOfWar{
					Attacker: move.Player,
					Defender: gs.Player,
				},
			)
			if err != nil {
				return pubsub.NackRequeue
			}

			return pubsub.Ack

		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard

		default:
			fmt.Errorf("HandlerMove returned unexpected value")
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.Acktype {
	return func(rw gamelogic.RecognitionOfWar) pubsub.Acktype {
		defer fmt.Print(">")

		outcome, winner, loser := gs.HandleWar(rw)

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue

		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard

		case gamelogic.WarOutcomeOpponentWon, gamelogic.WarOutcomeYouWon, gamelogic.WarOutcomeDraw:
			var msg string
			switch outcome {
			case gamelogic.WarOutcomeOpponentWon, gamelogic.WarOutcomeYouWon:
				msg = fmt.Sprintf("%s won a war against %s", winner, loser)
			case gamelogic.WarOutcomeDraw:
				msg = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			}

			err := pubsub.PublishGob(
				ch,
				routing.ExchangePerilTopic,
				fmt.Sprintf("%s.%s", routing.GameLogSlug, gs.Player.Username),
				routing.GameLog{
					CurrentTime: time.Now(),
					Message:     msg,
					Username:    gs.Player.Username,
				},
			)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack

		default:
			fmt.Errorf("HandleWar returned unexpected value")
			return pubsub.NackDiscard
		}
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
		handlerMove(gs, ch),
	)
	if err != nil {
		fmt.Printf(
			"Failed to subscribe to army_moves.* queue: %v",
			err,
		)
	}

	// subscribe to war queue
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		fmt.Sprintf("%s.*", routing.WarRecognitionsPrefix),
		pubsub.Durable,
		handlerWar(gs, ch),
	)
	if err != nil {
		fmt.Printf("Failed to subscribe to war queue")
	}

ClientREPL:
	for {
		cmd := gamelogic.GetInput()
		if len(cmd) == 0 {
			continue
		}

		switch cmd[0] {
		case "spawn":
			err = gs.CommandSpawn(cmd)
			if err != nil {
				log.Fatalf("Failed to spawn a unit: %v", err)
			}

		case "move":
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

		case "status":
			gs.CommandStatus()

		case "help":
			gamelogic.PrintClientHelp()

		case "spam":
			if len(cmd) < 2 {
				log.Fatal("The command is wrong.")
			}
			n, err := strconv.Atoi(cmd[1])
			if err != nil {
				log.Fatal("The command is wrong.")
			}

			for i := 0; i < n; i++ {
				msg := gamelogic.GetMaliciousLog()
				err = pubsub.PublishGob(
					ch,
					routing.ExchangePerilTopic,
					fmt.Sprintf("%s.%s", routing.GameLogSlug, userName),
					routing.GameLog{
						CurrentTime: time.Now(),
						Message:     msg,
						Username:    userName,
					},
				)
				if err != nil {
					log.Fatalf("Failed to publish spam message: %v", err)
				}
			}

		case "quit":
			gamelogic.PrintQuit()
			break ClientREPL

		default:
			log.Fatal("The command is wrong.")
		}
	}
}
