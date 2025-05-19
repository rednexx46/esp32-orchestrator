# Orchestrator (MQTT to MongoDB)

This service listens to MQTT messages forwarded by the ESP32 gateway, which collects sensor data from nearby nodes over ESP-NOW and publishes it via MQTT. It stores the received sensor data in MongoDB. The orchestrator is written in Go and designed to run as a lightweight Docker container on a Raspberry Pi or similar edge gateway.

---

## 📦 Features

* Subscribes to `mesh/data/#` MQTT topics
* Extracts device ID and payload
* Saves data to MongoDB with timestamp
* Fully configurable via environment variables
* Lightweight and production-ready

---

## 📐 Architecture

```text
[ESP32 Nodes] --> [ESP32 Gateway] --> [MQTT Broker] --> [Orchestrator] --> [MongoDB]
```

---

## 🧪 Prerequisites

* Running MQTT broker (e.g. Mosquitto)
* MongoDB instance (can be local or in Docker)

---

## ⚙️ Environment Variables

| Variable           | Description               | Example           |
| ------------------ | ------------------------- | ----------------- |
| `MONGO_USER`       | MongoDB username          | `iotuser`         |
| `MONGO_PASS`       | MongoDB password          | `iotpass`         |
| `MONGO_HOST`       | MongoDB host name or IP   | `mongodb`         |
| `MONGO_PORT`       | MongoDB port              | `27017`           |
| `MONGO_DATABASE`   | Target MongoDB database   | `iot_mesh`        |
| `MONGO_COLLECTION` | Target MongoDB collection | `sensor_data`     |
| `MQTT_BROKER`      | MQTT broker host          | `mosquitto`       |
| `MQTT_PORT`        | MQTT broker port          | `1883`            |
| `MQTT_TOPIC`       | MQTT topic to subscribe   | `mesh/data/`      |
| `MQTT_USERNAME`    | MQTT username (optional)  | `orchestrator`    |
| `MQTT_PASSWORD`    | MQTT password (optional)  | `mqtt_pass` |

---

## 🚀 Running with Docker Compose

```bash
docker compose up --build -d
```

Make sure your `docker-compose.yml` has all required environment variables and that your MQTT and MongoDB services are reachable.

---

## 📂 Folder Structure

```
.
├── main.go             # Main orchestrator logic
├── Dockerfile          # Docker build for Go binary
├── docker-compose.yml  # Docker runtime configuration
└── README.md           # Project documentation
```

---

## 🧱 MongoDB Schema

Each document inserted has the following structure:

```json
{
  "device_id": "24a160e5a1fc",
  "payload": "T=24.5C H=45% P=1013hPa",
  "timestamp": "2024-05-16T16:35:00Z"
}
```

---

## 🔒 Security Notes

* Be sure to protect MongoDB with authentication.
* Use Docker secrets or .env for managing sensitive values.
* If using MQTT auth, match credentials with your broker config.
