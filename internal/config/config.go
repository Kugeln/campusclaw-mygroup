package config

import (
	"fmt"
	"net"
	"strconv"
)

type Config struct{ DBHost, DBPort, DBName, DBUser, DBPassword, UploadRoot, Listen string }

func Load(get func(string) string) (Config, error) {
	c := Config{DBHost: get("DB_HOST"), DBPort: get("DB_PORT"), DBName: get("DB_NAME"), DBUser: get("DB_USER"), DBPassword: get("DB_PASSWORD"), UploadRoot: get("UPLOAD_ROOT"), Listen: get("LISTEN_ADDR")}
	for _, item := range []struct{ name, value string }{{"DB_HOST", c.DBHost}, {"DB_NAME", c.DBName}, {"DB_USER", c.DBUser}, {"DB_PASSWORD", c.DBPassword}, {"UPLOAD_ROOT", c.UploadRoot}} {
		if item.value == "" {
			return Config{}, fmt.Errorf("missing required configuration: %s", item.name)
		}
	}
	if c.DBPort == "" {
		c.DBPort = "3306"
	}
	port, err := strconv.Atoi(c.DBPort)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("invalid DB_PORT")
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if _, _, err = net.SplitHostPort(c.Listen); err != nil {
		return Config{}, fmt.Errorf("invalid LISTEN_ADDR")
	}
	return c, nil
}
