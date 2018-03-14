package readwrite_test

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
	"testing"
)

type testConfig struct {
	TimeoutScale          float64  `json:"timeout_scale"`
	MongoHosts            []string `json:"mongo_hosts"`
	MongoPorts            []string `json:"mongo_ports"`
	MongoRoot             string   `json:"mongo_root_username"`
	MongoRootPassword     string   `json:"mongo_root_password"`
	MongoReplicaSetName   string   `json:"mongo_replica_set_name"`
	MongoReplicaSetEnable int      `json:"mongo_replica_set_enable"`
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
