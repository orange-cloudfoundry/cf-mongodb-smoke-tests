package readwrite_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testConfig struct {
	TimeoutScale                  float64 `json:"timeout_scale"`
	Host                          string `json:"host"`
	Port                          string `json:"port"`
}

func loadConfig(path string) (cfg testConfig) {
	configFile, err := os.Open(path)
	if err != nil {
		fatal(err)
	}

	decoder := json.NewDecoder(configFile)
	if err = decoder.Decode(&cfg); err != nil {
		fatal(err)
	}

	return
}

var (
	config = loadConfig(os.Getenv("CONFIG_PATH"))
)

func fatal(err error) {
	fmt.Printf("ERROR: %s\n", err.Error())
	os.Exit(1)
}

func TestReadwrite(t *testing.T) {

	RegisterFailHandler(Fail)

	RunSpecs(t, "MongoDB Acceptance Tests")
}
