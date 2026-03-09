package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril client...")
	const rabbitConnString = "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(rabbitConnString)
	if err != nil {
		log.Fatalf("could not connect to RabbitMQ: %v", err)
	}
	defer conn.Close()
	fmt.Println("Peril game client connected to RabbitMQ!")

	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("error in creating channel: %v", err)
	}
	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("could not get username: %v", err)
	}

	gs := gamelogic.NewGameState(username)

	pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+gs.GetUsername(),
		routing.PauseKey,
		pubsub.Transient,
		handlerPause(gs),
	)

	pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"*"+gs.GetUsername(),
		routing.ArmyMovesPrefix+".*",
		pubsub.Transient,
		handlerMove(gs, channel),
	)

	pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		routing.WarRecognitionsPrefix+".*",
		pubsub.Durable,
		handlerWar(gs, channel),
	)

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		switch words[0] {
		case "move":
			armyMove, err := gs.CommandMove(words)
			if err != nil {
				fmt.Println(err)
				continue
			}

			if err := pubsub.PublishJSON(
				channel,
				routing.ExchangePerilTopic,
				routing.ArmyMovesPrefix+"."+armyMove.Player.Username,
				armyMove,
			); err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Println("Move published.")
		case "spawn":
			err = gs.CommandSpawn(words)
			if err != nil {
				fmt.Println(err)
				continue
			}
		case "status":
			gs.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			if len(words) != 2 {
				fmt.Println("number input required")
			}
			secondInput := words[1]
			num, err := strconv.Atoi(secondInput)
			if err != nil {
				fmt.Println(err)
			}
			for num > 0 {
				maliciousLog := gamelogic.GetMaliciousLog()
				if err := pubsub.PublishJSON(channel, routing.ExchangePerilTopic, routing.GameLogSlug+"."+gs.GetUsername(), maliciousLog); err != nil {
					fmt.Println(err)
				}
				num--
			}
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Println("unknown command")
		}
	}

	// wait for ctrl+c
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	fmt.Println("RabbitMQ connection closed.")
}
