package config

import "testing"

func TestLoadConfig(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("empty secret accepted")
	}

	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("MONGO_URI", "")
	t.Setenv("MONGO_DATABASE", "")

	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if config.HTTPAddr != ":8080" || config.MongoURI != "mongodb://localhost:27017" || config.MongoDatabase != "user_management" {
		t.Fatalf("unexpected defaults: %+v", config)
	}

	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("MONGO_URI", "mongodb://database:27017")
	t.Setenv("MONGO_DATABASE", "custom")

	config, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if config.HTTPAddr != ":9090" || config.MongoURI != "mongodb://database:27017" || config.MongoDatabase != "custom" {
		t.Fatalf("overrides ignored: %+v", config)
	}
}
