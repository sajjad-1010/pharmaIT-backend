package sse

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type redisPacket struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data"`
}

func StartRedisBridge(ctx context.Context, redisClient *redis.Client, channel string, broker *Broker, log zerolog.Logger) {
	if redisClient == nil || broker == nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		pubsub := redisClient.Subscribe(ctx, channel)
		_, err := pubsub.Receive(ctx)
		if err != nil {
			log.Error().Err(err).Str("channel", channel).Msg("sse redis subscribe failed; retrying")
			time.Sleep(2 * time.Second)
			continue
		}

		ch := pubsub.Channel()
		log.Info().Str("channel", channel).Msg("sse redis bridge started")

	BridgeLoop:
		for {
			select {
			case <-ctx.Done():
				_ = pubsub.Close()
				return
			case msg, ok := <-ch:
				if !ok {
					break BridgeLoop
				}
				var pkt redisPacket
				if err := json.Unmarshal([]byte(msg.Payload), &pkt); err != nil {
					continue
				}
				if pkt.Event == "" {
					continue
				}
				broker.Publish(pkt.Event, pkt.Data)
			}
		}

		_ = pubsub.Close()
		time.Sleep(1 * time.Second)
	}
}

