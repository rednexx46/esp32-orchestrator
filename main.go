// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type SensorData struct {
	DeviceID  string    `json:"device_id" bson:"device_id"`
	Payload   string    `json:"payload" bson:"payload"`
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
}

var mongoClient *mongo.Client
var dataCollection *mongo.Collection

func connectMongo() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoUser := os.Getenv("MONGO_USER")
	mongoPass := os.Getenv("MONGO_PASS")
	mongoHost := os.Getenv("MONGO_HOST")
	mongoPort := os.Getenv("MONGO_PORT")
	mongoDB := os.Getenv("MONGO_DATABASE")
	mongoCol := os.Getenv("MONGO_COLLECTION")

	uri := fmt.Sprintf("mongodb://%s:%s@%s:%s", mongoUser, mongoPass, mongoHost, mongoPort)
	clientOpts := options.Client().ApplyURI(uri).SetWriteConcern(writeconcern.New(writeconcern.WMajority()))

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		log.Fatalf("[MongoDB] Connection error: %v", err)
	}
	mongoClient = client
	db := mongoClient.Database(mongoDB)
	dataCollection = db.Collection(mongoCol)
	fmt.Printf("[MongoDB] Connected to %s.%s\n", mongoDB, mongoCol)
}

func storeToMongo(data SensorData) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := dataCollection.InsertOne(ctx, data)
	if err != nil {
		log.Printf("[MongoDB] Insert failed: %v", err)
		return
	}
	fmt.Println("[MongoDB] Data stored.")
}

func messageHandler(client mqtt.Client, msg mqtt.Message) {
	topicParts := strings.Split(msg.Topic(), "/")
	deviceID := topicParts[len(topicParts)-1]

	data := SensorData{
		DeviceID:  deviceID,
		Payload:   string(msg.Payload()),
		Timestamp: time.Now(),
	}
	fmt.Printf("[MQTT] Received from %s: %s\n", deviceID, data.Payload)
	storeToMongo(data)
}

func main() {
	connectMongo()

	mqttBroker := os.Getenv("MQTT_BROKER")
	mqttPort := os.Getenv("MQTT_PORT")
	if mqttPort == "" {
		mqttPort = "1883"
	}

	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%s", mqttBroker, mqttPort)).
		SetClientID("mqtt-orchestrator").
		SetCleanSession(true)

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Println("[MQTT] Connected to broker.")
		if token := c.Subscribe("mesh/data/#", 0, messageHandler); token.Wait() && token.Error() != nil {
			log.Fatalf("[MQTT] Subscribe error: %v", token.Error())
		}
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("[MQTT] Connection failed: %v", token.Error())
	}

	select {} // keep running
}
