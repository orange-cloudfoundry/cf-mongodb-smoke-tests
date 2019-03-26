package shard_mongo

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
	MongoRequireSsl       int      `json:"mongo_require_ssl"`
	MongoCACert           string   `json:"mongo_cert"`
        MongosHosts           []string `json:"mongo_mongos_hosts"`
        MongosPorts           []string `json:"mongo_mongos_ports"`
        MongoCfgHosts         []string `json:"mongo_cfgsrv_hosts"`
        MongoCfgPorts         []string `json:"mongo_cfgsrv_ports"`
        MongoCfgReplicaSetName string  `json:"mongo_cfgsrv_replica_set_name"`
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

func TestShard(t *testing.T) {

	RegisterFailHandler(Fail)

	RunSpecs(t, "MongoDB Sharding Acceptance Tests")
}
