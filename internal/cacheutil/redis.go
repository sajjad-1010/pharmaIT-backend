package cacheutil

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func InvalidateByPrefix(ctx context.Context, client *redis.Client, prefix string) error {
	if client == nil {
		return nil
	}

	iter := client.Scan(ctx, 0, prefix+"*", 300).Iterator()
	keys := make([]string, 0, 64)
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 300 {
			if err := client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			keys = keys[:0]
		}
	}
	if len(keys) > 0 {
		if err := client.Del(ctx, keys...).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}
