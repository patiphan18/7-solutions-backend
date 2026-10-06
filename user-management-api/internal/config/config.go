package config

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr      string
	GRPCAddr      string
	MongoURI      string
	MongoDatabase string
	JWTSecret     string
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func LoadConfig() (Config, error) {
	config := Config{
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		GRPCAddr:      env("GRPC_ADDR", ":9090"),
		MongoURI:      env("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase: env("MONGO_DATABASE", "user_management"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}

	return config, nil
}

func (c Config) Validate() error {
	if len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET must contain at least 32 bytes")
	}

	if strings.HasPrefix(c.JWTSecret, "replace-with-") || strings.HasPrefix(c.JWTSecret, "your-random-secret") {
		return errors.New("JWT_SECRET must be replaced with a generated secret")
	}

	if c.GRPCAddr != "" {
		_, port, err := net.SplitHostPort(c.GRPCAddr)
		if err != nil {
			return errors.New("GRPC_ADDR must be host:port or :port")
		}

		number, err := strconv.Atoi(port)
		if err != nil || number < 0 || number > 65535 {
			return errors.New("GRPC_ADDR port must be between 0 and 65535")
		}
	}

	_, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return errors.New("HTTP_ADDR must be host:port or :port")
	}

	number, err := strconv.Atoi(port)
	if err != nil || number < 0 || number > 65535 {
		return errors.New("HTTP_ADDR port must be between 0 and 65535")
	}

	if !strings.HasPrefix(c.MongoURI, "mongodb://") && !strings.HasPrefix(c.MongoURI, "mongodb+srv://") {
		return errors.New("MONGO_URI must use mongodb:// or mongodb+srv://")
	}

	if c.MongoDatabase == "" || strings.ContainsAny(c.MongoDatabase, "/\\.\"$\x00 ") {
		return errors.New("invalid MONGO_DATABASE")
	}

	return nil
}
