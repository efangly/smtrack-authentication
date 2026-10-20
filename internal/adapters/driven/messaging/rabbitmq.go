package messaging

import (
	"encoding/json"
	"fmt"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	amqp "github.com/rabbitmq/amqp091-go"
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

	if deviceCh != nil {
		deviceCh.QueueDeclare("log_device_queue", true, false, false, false, nil)
		// auth_queue is declared here (rather than on its own channel) so the
		// channel is not left open and unreferenced.
		deviceCh.QueueDeclare("auth_queue", true, false, false, false, nil)
	}
	if legacyCh != nil {
		legacyCh.QueueDeclare("templog_queue", true, false, false, false, nil)
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

// Close shuts the channels and the connection down.
func (r *rabbitmqAdapter) Close() error {
	if r.deviceChannel != nil {
		r.deviceChannel.Close()
	}
	if r.legacyChannel != nil {
		r.legacyChannel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
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
