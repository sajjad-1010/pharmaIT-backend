package notifications

import (
	"context"
	"fmt"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"pharmalink/server/internal/config"

	"github.com/rs/zerolog"
	"google.golang.org/api/option"
)

type FCMPushProvider struct {
	client *messaging.Client
	dryRun bool
	log    zerolog.Logger
}

func NewFCMPushProvider(cfg config.NotificationConfig, log zerolog.Logger) (*FCMPushProvider, error) {
	if strings.TrimSpace(cfg.FCMCredentialsFile) == "" && strings.TrimSpace(cfg.FCMCredentialsJSON) == "" {
		return nil, fmt.Errorf("FCM credentials are required when NOTIFICATION_PUSH_PROVIDER=fcm")
	}

	opts := make([]option.ClientOption, 0, 1)
	switch {
	case strings.TrimSpace(cfg.FCMCredentialsJSON) != "":
		opts = append(opts, option.WithCredentialsJSON([]byte(cfg.FCMCredentialsJSON)))
	case strings.TrimSpace(cfg.FCMCredentialsFile) != "":
		opts = append(opts, option.WithCredentialsFile(cfg.FCMCredentialsFile))
	}

	app, err := firebase.NewApp(context.Background(), nil, opts...)
	if err != nil {
		return nil, fmt.Errorf("create firebase app: %w", err)
	}

	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("create messaging client: %w", err)
	}

	return &FCMPushProvider{
		client: client,
		dryRun: cfg.FCMDryRun,
		log:    log,
	}, nil
}

func (p *FCMPushProvider) Send(ctx context.Context, msg PushMessage) error {
	if p.client == nil {
		return ErrPushNotConfigured
	}

	payloadJSON := string(msg.Notification.PayloadJSON)
	data := map[string]string{
		"notification_id": msg.Notification.ID.String(),
		"user_id":         msg.Notification.UserID.String(),
		"kind":            string(msg.Notification.Kind),
		"title":           msg.Notification.Title,
		"body":            msg.Notification.Body,
		"payload_json":    payloadJSON,
	}

	message := &messaging.Message{
		Token: msg.Device.Token,
		Data:  data,
		Notification: &messaging.Notification{
			Title: msg.Notification.Title,
			Body:  msg.Notification.Body,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		Webpush: &messaging.WebpushConfig{
			Headers: map[string]string{
				"Urgency": "high",
			},
			Notification: &messaging.WebpushNotification{
				Title: msg.Notification.Title,
				Body:  msg.Notification.Body,
			},
			FCMOptions: &messaging.WebpushFCMOptions{
				Link: "/",
			},
		},
	}

	var err error
	if p.dryRun {
		_, err = p.client.SendDryRun(ctx, message)
	} else {
		_, err = p.client.Send(ctx, message)
	}
	if err == nil {
		return nil
	}

	if isInvalidFCMTokenError(err) {
		return fmt.Errorf("%w: %v", ErrPushTokenInvalid, err)
	}
	return err
}

func isInvalidFCMTokenError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "registration-token-not-registered") ||
		strings.Contains(msg, "requested entity was not found") ||
		strings.Contains(msg, "invalid registration token") ||
		strings.Contains(msg, "registration token is not a valid fcm registration token") ||
		strings.Contains(msg, "unregistered")
}
