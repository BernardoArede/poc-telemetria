package alticeopentelemetry

import (
	"context"
	"net/http"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func PublishWithTrace(ctx context.Context, nc *nats.Conn, subject string, data []byte) error {
	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  make(nats.Header),
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(http.Header(msg.Header)))

	return nc.PublishMsg(msg)
}

func SubscribeWithTrace(nc *nats.Conn, subject string, queueGroup string, handler func(ctx context.Context, msg *nats.Msg)) (*nats.Subscription, error) {
	
	return nc.QueueSubscribe(subject, queueGroup, func(msg *nats.Msg) {
		ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(http.Header(msg.Header)))
		handler(ctx, msg)
	})
}