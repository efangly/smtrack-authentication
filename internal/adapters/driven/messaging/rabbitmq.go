package messaging

import (
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/tng-coop/auth-service/config"
	"github.com/tng-coop/auth-service/internal/core/ports"
	"github.com/tng-coop/auth-service/pkg/logger"
)

type rabbitmqAdapter struct {
	conn          *amqp.Connection
	deviceChannel *amqp.Channel
	legacyChannel *amqp.Channel
}

func NewRabbitMQAdapter(cfg *config.Config) ports.MessagingPort {
	if cfg.RabbitMQURL == "" {
		logger.Info("RABBITMQ not configured, skipping")
		return &rabbitmqAdapter{}
	}

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		logger.Warn("RabbitMQ connection error", "error", err)
		return &rabbitmqAdapter{}
	}

	deviceCh, _ := conn.Channel()
	legacyCh, _ := conn.Channel()
	authCh, _ := conn.Channel()

	if deviceCh != nil {
		deviceCh.QueueDeclare("log_device_queue", true, false, false, false, nil)
	}
	if legacyCh != nil {
		legacyCh.QueueDeclare("templog_queue", true, false, false, false, nil)
	}
	if authCh != nil {
		authCh.QueueDeclare("auth_queue", true, false, false, false, nil)
	}

	logger.Info("RabbitMQ connected successfully")

	adapter := &rabbitmqAdapter{
		conn:          conn,
		deviceChannel: deviceCh,
		legacyChannel: legacyCh,
	}

	return adapter
}

func (r *rabbitmqAdapter) SendToDevice(queue string, payload any) error {
	if r.deviceChannel == nil {
		return fmt.Errorf("device channel not available")
	}
	return r.publish(r.deviceChannel, "log_device_queue", queue, payload)
}

func (r *rabbitmqAdapter) SendToLegacy(queue string, payload any) error {
	if r.legacyChannel == nil {
		return fmt.Errorf("legacy channel not available")
	}
	return r.publish(r.legacyChannel, "templog_queue", queue, payload)
}

func (r *rabbitmqAdapter) publish(ch *amqp.Channel, queueName string, pattern string, payload any) error {
	body, err := json.Marshal(map[string]any{
		"pattern": pattern,
		"data":    payload,
	})
	if err != nil {
		return err
	}

	return ch.Publish("", queueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
	})
}
