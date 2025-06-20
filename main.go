// main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.mongodb.org/mongo-driver/bson"
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
var kpiCollection *mongo.Collection
var statusCollection *mongo.Collection

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
	kpiCollection = db.Collection("kpis")
	statusCollection = db.Collection("status")
	fmt.Printf("[MongoDB] Connected to %s at %s:%s\n", mongoDB, mongoHost, mongoPort)
}

func storeToMongo(data SensorData) {
	encryption := os.Getenv("ENCRYPTION")
	if strings.ToLower(encryption) == "true" {
		cipherAPI := os.Getenv("ENCRYPT_API_URL")
		if cipherAPI == "" {
			log.Println("[CipherAPI] Encryption enabled but API URL not set.")
			return
		}

		payload := fmt.Sprintf(`{"text": "%s"}`, data.Payload)
		req, err := http.NewRequest("POST", cipherAPI+"encrypt", strings.NewReader(payload))
		if err != nil {
			log.Printf("[CipherAPI] Request creation failed: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[CipherAPI] Request failed: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("[CipherAPI] Non-200 response: %d", resp.StatusCode)
			return
		}

		var result struct {
			Result string `json:"result"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Printf("[CipherAPI] Decode failed: %v", err)
			return
		}

		data.Payload = result.Result
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := dataCollection.InsertOne(ctx, data)
	if err != nil {
		log.Printf("[MongoDB] Insert failed: %v", err)
		return
	}
	fmt.Println("[MongoDB] Data stored.")
}

func storeKPIToMongo(data SensorData) {
	fields := strings.Split(data.Payload, ";")
	kpiDoc := bson.M{
		"device_id": data.DeviceID,
		"timestamp": data.Timestamp,
	}

	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := parts[1]

		// Convert numeric fields to int
		if num, err := strconv.Atoi(value); err == nil {
			kpiDoc[key] = num
		} else {
			kpiDoc[key] = value
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := kpiCollection.InsertOne(ctx, kpiDoc)
	if err != nil {
		log.Printf("[MongoDB] KPI insert failed: %v", err)
		return
	}
	fmt.Println("[MongoDB] KPI data stored.")
}

func storeStatusToMongo(payload string, timestamp time.Time) {
	var statusData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &statusData); err != nil {
		log.Printf("[MongoDB] Failed to parse status JSON: %v", err)
		return
	}

	statusData["timestamp"] = timestamp

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := statusCollection.InsertOne(ctx, statusData)
	if err != nil {
		log.Printf("[MongoDB] Status insert failed: %v", err)
		return
	}
	fmt.Println("[MongoDB] Status data stored.")
}

func messageHandler(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	topicParts := strings.Split(topic, "/")
	deviceID := topicParts[len(topicParts)-1]

	payloadStr := string(msg.Payload())
	timestamp := time.Now()

	fmt.Printf("[MQTT] Received on %s: %s\n", topic, payloadStr)

	switch {
	case strings.HasPrefix(topic, os.Getenv("MQTT_KPI_TOPIC")):
		data := SensorData{DeviceID: deviceID, Payload: payloadStr, Timestamp: timestamp}
		storeKPIToMongo(data)

	case strings.HasPrefix(topic, os.Getenv("MQTT_MESH_STATUS_TOPIC")):
		storeStatusToMongo(payloadStr, timestamp)

	default:
		data := SensorData{DeviceID: deviceID, Payload: payloadStr, Timestamp: timestamp}
		storeToMongo(data)
	}
}

func main() {
	connectMongo()

	mqttBroker := os.Getenv("MQTT_BROKER")
	mqttPort := os.Getenv("MQTT_PORT")
	mqttTopic := os.Getenv("MQTT_TOPIC")
	mqttKpiTopic := os.Getenv("MQTT_KPI_TOPIC")
	mqttStatusTopic := os.Getenv("MQTT_MESH_STATUS_TOPIC")
	mqttUser := os.Getenv("MQTT_USERNAME")
	mqttPass := os.Getenv("MQTT_PASSWORD")

	if mqttPort == "" {
		mqttPort = "1883"
	}
	if mqttTopic == "" {
		mqttTopic = "mesh/data/"
	}
	if mqttKpiTopic == "" {
		mqttKpiTopic = "mesh/kpi/"
	}
	if mqttStatusTopic == "" {
		mqttStatusTopic = "mesh/status/"
	}

	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%s", mqttBroker, mqttPort)).
		SetClientID("mqtt-orchestrator").
		SetCleanSession(true)

	if mqttUser != "" {
		opts.SetUsername(mqttUser)
	}
	if mqttPass != "" {
		opts.SetPassword(mqttPass)
	}

	opts.OnConnect = func(c mqtt.Client) {
		fmt.Println("[MQTT] Connected to broker.")

		topics := map[string]byte{
			mqttTopic + "#":       0,
			mqttKpiTopic + "#":    0,
			mqttStatusTopic + "#": 0,
		}

		if token := c.SubscribeMultiple(topics, messageHandler); token.Wait() && token.Error() != nil {
			log.Fatalf("[MQTT] SubscribeMultiple error: %v", token.Error())
		}
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("[MQTT] Connection failed: %v", token.Error())
	}

	select {} // keep running
}
