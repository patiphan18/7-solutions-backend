package config

import "testing"

func TestValidateRejectsBadSettings(t *testing.T) {
	valid := Config{
		HTTPAddr:      ":8080",
		MongoURI:      "mongodb://localhost:27017",
		MongoDatabase: "users",
		JWTSecret:     "01234567890123456789012345678901",
	}

	for _, setting := range []string{"address", "port", "grpc-address", "grpc-port", "uri", "database", "placeholder-secret"} {
		t.Run(setting, func(t *testing.T) {
			config := valid

			switch setting {
			case "placeholder-secret":
				config.JWTSecret = "your-random-secret-at-least-32-bytes-long"
			case "address":
				config.HTTPAddr = "8080"
			case "port":
				config.HTTPAddr = ":99999"
			case "grpc-address":
				config.GRPCAddr = "9090"
			case "grpc-port":
				config.GRPCAddr = ":99999"
			case "uri":
				config.MongoURI = "https://localhost"
			case "database":
				config.MongoDatabase = "bad/name"
			}

			if err := config.Validate(); err == nil {
				t.Fatal("invalid setting accepted")
			}
		})
	}
}
