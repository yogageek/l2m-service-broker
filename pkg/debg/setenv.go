package debg

import (
	"os"
)

var envmap = map[string]string{
	"MONGODB_HOST1":    "172.22.23.155",
	"MONGODB_HOST2":    "172.22.23.155",
	"MONGODB_HOST3":    "172.22.23.155",
	"MONGODB_PORT1":    "32237",
	"MONGODB_PORT2":    "30900",
	"MONGODB_PORT3":    "31095",
	"MONGODB_USERNAME": "q5XFqg9DYgQnxUrqhuMthgUF",
	"MONGODB_PASSWORD": "KrafJt2sV2eV3GQSKm6BpXgV",
	"PSEUDO_ID":        "abc-123",
	"SHARED_USE":       "true",
	"DATABASE":         "admin",
	"PG_HOST1":         "172.22.23.155",
	"PG_PORT1":         "31429",
	"PG_USERNAME":      "postgres",
	"PG_PASSWORD":      "cXPG0vZDdZ",
}

func SetEnv() {
	for k, v := range envmap {
		os.Setenv(k, v)
	}
}

func RemovEnv() {
	for k := range envmap {
		os.Unsetenv(k)
	}
}
