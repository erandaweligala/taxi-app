// Package kafkax wraps kafka-go with the few helpers this platform needs.
// Kafka decouples the producers (ingest edge, trip API) from the consumers
// (location worker, trip worker, analytics) so each scales independently and
// the firehose never blocks a reader.
package kafkax

import (
	"context"
	"errors"
	"time"

	"github.com/segmentio/kafka-go"
)

// EnsureTopic creates a topic if it does not already exist. Partitions are the
// unit of consumer parallelism; we default generously so multiple workers per
// hot region can share the load.
func EnsureTopic(ctx context.Context, broker, topic string, partitions int) error {
	var conn *kafka.Conn
	var err error
	for i := 0; i < 30; i++ {
		conn, err = kafka.DialContext(ctx, "tcp", broker)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return err
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return err
	}
	cc, err := kafka.DialContext(ctx, "tcp", net(controller.Host, controller.Port))
	if err != nil {
		return err
	}
	defer cc.Close()

	err = cc.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     partitions,
		ReplicationFactor: 1,
	})
	if errors.Is(err, kafka.TopicAlreadyExists) {
		return nil
	}
	return err
}

// NewWriter returns a writer that hashes the message Key to choose a partition.
// We key by region, so all traffic for a region lands on one partition and is
// processed in order.
func NewWriter(brokers []string, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		BatchTimeout: 10 * time.Millisecond, // small batches keep ingest latency low
		Async:        false,
		RequiredAcks: kafka.RequireOne,
	}
}

// NewReader returns a consumer-group reader for a topic.
func NewReader(brokers []string, topic, group string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        group,
		MinBytes:       1,
		MaxBytes:       10 << 20,
		CommitInterval: time.Second,
	})
}

func net(host string, port int) string {
	return host + ":" + itoa(port)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
