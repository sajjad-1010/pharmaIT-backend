package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	AppName string
	AppEnv  string

	HTTPAddr string

	WorkerConcurrency int

	CORS CORSConfig

	JWT JWTConfig

	Postgres     PostgresConfig
	Redis        RedisConfig
	Notification NotificationConfig

	Payment PaymentConfig

	OutboxChannel string
}

type JWTConfig struct {
	Issuer           string
	AccessSecret     string
	RefreshSecret    string
	AccessTTLMinutes int
	RefreshTTLHours  int
}

type CORSConfig struct {
	AllowedOrigins string
}

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	Timezone string
}

func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		c.Host,
		c.Port,
		c.User,
		c.Password,
		c.DBName,
		c.SSLMode,
		c.Timezone,
	)
}

func (c PostgresConfig) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.DBName,
		c.SSLMode,
	)
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type PaymentConfig struct {
	AccessGrantHours int
	AccessFee        string
	Currency         string
	WebhookSecret    string
}

type NotificationConfig struct {
	PushProvider       string
	FCMCredentialsFile string
	FCMCredentialsJSON string
	FCMDryRun          bool
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("..")
	v.AddConfigPath("../..")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	_ = v.ReadInConfig()

	setDefaults(v)

	cfg := Config{
		AppName: v.GetString("APP_NAME"),
		AppEnv:  v.GetString("APP_ENV"),

		HTTPAddr: v.GetString("HTTP_ADDR"),

		WorkerConcurrency: v.GetInt("WORKER_CONCURRENCY"),
		CORS: CORSConfig{
			AllowedOrigins: strings.TrimSpace(v.GetString("CORS_ALLOWED_ORIGINS")),
		},

		JWT: JWTConfig{
			Issuer:           v.GetString("JWT_ISSUER"),
			AccessSecret:     v.GetString("JWT_ACCESS_SECRET"),
			RefreshSecret:    v.GetString("JWT_REFRESH_SECRET"),
			AccessTTLMinutes: v.GetInt("JWT_ACCESS_TTL_MINUTES"),
			RefreshTTLHours:  v.GetInt("JWT_REFRESH_TTL_HOURS"),
		},
		Postgres: PostgresConfig{
			Host:     v.GetString("POSTGRES_HOST"),
			Port:     v.GetInt("POSTGRES_PORT"),
			User:     v.GetString("POSTGRES_USER"),
			Password: v.GetString("POSTGRES_PASSWORD"),
			DBName:   v.GetString("POSTGRES_DB"),
			SSLMode:  v.GetString("POSTGRES_SSLMODE"),
			Timezone: v.GetString("POSTGRES_TIMEZONE"),
		},
		Redis: RedisConfig{
			Addr:     v.GetString("REDIS_ADDR"),
			Password: v.GetString("REDIS_PASSWORD"),
			DB:       v.GetInt("REDIS_DB"),
		},
		Notification: NotificationConfig{
			PushProvider:       strings.TrimSpace(v.GetString("NOTIFICATION_PUSH_PROVIDER")),
			FCMCredentialsFile: strings.TrimSpace(v.GetString("FCM_CREDENTIALS_FILE")),
			FCMCredentialsJSON: strings.TrimSpace(v.GetString("FCM_CREDENTIALS_JSON")),
			FCMDryRun:          v.GetBool("FCM_DRY_RUN"),
		},
		Payment: PaymentConfig{
			AccessGrantHours: v.GetInt("PAYMENT_ACCESS_GRANT_HOURS"),
			AccessFee:        v.GetString("PAYMENT_ACCESS_FEE"),
			Currency:         v.GetString("PAYMENT_CURRENCY"),
			WebhookSecret:    v.GetString("PAYMENT_WEBHOOK_SECRET"),
		},

		OutboxChannel: v.GetString("OUTBOX_CHANNEL"),
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_NAME", "pharmalink")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("HTTP_ADDR", ":8080")
	v.SetDefault("WORKER_CONCURRENCY", 10)
	v.SetDefault("CORS_ALLOWED_ORIGINS", "*")

	v.SetDefault("JWT_ISSUER", "pharmalink")
	v.SetDefault("JWT_ACCESS_TTL_MINUTES", 30)
	v.SetDefault("JWT_REFRESH_TTL_HOURS", 720)

	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", 5432)
	v.SetDefault("POSTGRES_USER", "pharmalink")
	v.SetDefault("POSTGRES_PASSWORD", "pharmalink")
	v.SetDefault("POSTGRES_DB", "pharmalink")
	v.SetDefault("POSTGRES_SSLMODE", "disable")
	v.SetDefault("POSTGRES_TIMEZONE", "UTC")

	v.SetDefault("REDIS_ADDR", "localhost:6379")
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("NOTIFICATION_PUSH_PROVIDER", "noop")
	v.SetDefault("FCM_DRY_RUN", false)

	v.SetDefault("PAYMENT_ACCESS_GRANT_HOURS", 24)
	v.SetDefault("PAYMENT_ACCESS_FEE", "3")
	v.SetDefault("PAYMENT_CURRENCY", "TJS")

	v.SetDefault("OUTBOX_CHANNEL", "outbox_new")
}

func validate(cfg Config) error {
	switch {
	case cfg.JWT.AccessSecret == "":
		return fmt.Errorf("JWT_ACCESS_SECRET is required")
	case cfg.JWT.RefreshSecret == "":
		return fmt.Errorf("JWT_REFRESH_SECRET is required")
	case cfg.Payment.WebhookSecret == "":
		return fmt.Errorf("PAYMENT_WEBHOOK_SECRET is required")
	}

	if cfg.JWT.AccessTTLMinutes <= 0 {
		return fmt.Errorf("JWT_ACCESS_TTL_MINUTES must be > 0")
	}

	if cfg.JWT.RefreshTTLHours <= 0 {
		return fmt.Errorf("JWT_REFRESH_TTL_HOURS must be > 0")
	}

	if cfg.Payment.AccessGrantHours <= 0 {
		return fmt.Errorf("PAYMENT_ACCESS_GRANT_HOURS must be > 0")
	}

	if cfg.CORS.AllowedOrigins == "" {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS is required")
	}

	return nil
}

func (c JWTConfig) AccessTTL() time.Duration {
	return time.Duration(c.AccessTTLMinutes) * time.Minute
}

func (c JWTConfig) RefreshTTL() time.Duration {
	return time.Duration(c.RefreshTTLHours) * time.Hour
}
