package rabbitmq

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	PublishEvent(eventType string, payload any) error
	Close() error
}

type publisher struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	exchangeName string
	mu           sync.Mutex
}

// NewPublisher creates a RabbitMQ publisher and declares the app.events exchange.
func NewPublisher(amqpURL string) (Publisher, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	if err := ch.ExchangeDeclare(
		"app.events",
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &publisher{conn: conn, channel: ch, exchangeName: "app.events"}, nil
}

// NewNoopPublisher returns a publisher that drops events but logs that RabbitMQ is unavailable.
type noopPublisher struct{}

func NewNoopPublisher() Publisher { return &noopPublisher{} }

func (n *noopPublisher) PublishEvent(eventType string, payload any) error {
	log.Printf("warning: RabbitMQ not configured; skipping publish for event %s", eventType)
	return nil
}

func (n *noopPublisher) Close() error { return nil }

func (p *publisher) PublishEvent(eventType string, payload any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.channel == nil {
		return amqp.ErrClosed
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return p.channel.PublishWithContext(ctx,
		p.exchangeName,
		eventType,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
			DeliveryMode: amqp.Persistent,
		},
	)
}

func (p *publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.channel != nil {
		_ = p.channel.Close()
		p.channel = nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
	return nil
}
