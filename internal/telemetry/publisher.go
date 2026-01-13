package telemetry

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	Publish(ctx context.Context, routingKey string, event Envelope) error
	Close() error
}

type RabbitPublisher struct {
	amqpURL  string
	exchange string
	conn     *amqp.Connection
	channel  *amqp.Channel
	mu       sync.Mutex
}

func NewRabbitPublisher(amqpURL, exchange string) *RabbitPublisher {
	return &RabbitPublisher{amqpURL: amqpURL, exchange: exchange}
}

func (p *RabbitPublisher) Connect() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	conn, err := amqp.Dial(p.amqpURL)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}

	if err := ch.ExchangeDeclare(
		p.exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return err
	}

	p.conn = conn
	p.channel = ch
	return nil
}

func (p *RabbitPublisher) Publish(ctx context.Context, routingKey string, event Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.channel == nil {
		return amqp.ErrClosed
	}

	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return p.channel.PublishWithContext(
		ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			Timestamp:    time.Now(),
			DeliveryMode: amqp.Persistent,
		},
	)
}

func (p *RabbitPublisher) Close() error {
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

type NoopPublisher struct{}

func NewNoopPublisher() Publisher {
	return &NoopPublisher{}
}

func (n *NoopPublisher) Publish(ctx context.Context, routingKey string, event Envelope) error {
	log.Printf("warning: RabbitMQ not configured; skipping publish for %s", routingKey)
	return nil
}

func (n *NoopPublisher) Close() error { return nil }
