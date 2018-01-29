package replicaset_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/satori/go.uuid"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	//"flag"
	"io"
	//"os"
	//"os/exec"
	//"syscall"
	"time"
)

var _ = Describe("MongoDB CRUD tests", func() {

	var nodes = len(config.MongoHosts)
	var addrs []string
	for cpt := 0; cpt < nodes; cpt++ {
		addrs = append(addrs, config.MongoHosts[cpt]+":"+config.MongoPorts[cpt])
	}
	fmt.Println(addrs) //for testing
	var connInfo = &mgo.DialInfo{
		Addrs:          addrs,
		Username:       config.MongoRoot,
		Password:       config.MongoRootPassword,
		ReplicaSetName: config.MongoReplicaSetName,
		Timeout:        10 * time.Second,
		FailFast:       true,
	}
	var restartNode *mgo.DialInfo
	var rootSession, Session, nodeSession *mgo.Session
	var err error
	var differentiator = uuid.NewV4().String()
	var databaseName = "TestDatabase-" + differentiator
	var db *mgo.Database
	var collectionName = "TestCollection"
	var col *mgo.Collection

	var admin = mgo.User{
		Username: "TestUsername" + differentiator,
		Password: "TestPassword",
		Roles:    []mgo.Role{mgo.RoleDBAdmin},
	}
	type Item struct {
		Id   bson.ObjectId "_id,omitempty"
		Name string        "Name"
	}
	var itemName = "some-item"
	var item = Item{"", itemName}
	var rsConf = bson.M{}

	BeforeEach(func() {

		By("connecting to the cluster as root")
		rootSession, err = mgo.DialWithInfo(connInfo)
		Expect(err).NotTo(HaveOccurred())
		db = rootSession.DB(databaseName)

		By("creating an admin user")
		err = db.UpsertUser(&admin)
		Expect(err).NotTo(HaveOccurred())

		By("Login as an admin user")
		err = db.Login(admin.Username, admin.Password)
		Expect(err).NotTo(HaveOccurred())

		By("writing data on the primary node")
		col = db.C(collectionName)
		err = col.Insert(item)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {

		By("dropping collection, removing user, logging out and closing from the session")
		col.DropCollection()
		err := db.RemoveUser(admin.Username)
		Expect(err).NotTo(HaveOccurred())
		rootSession.LogoutAll()
		rootSession.Close()
	})

	Context("When deploying 1 instance", func() {

		BeforeEach(func() {
			err = rootSession.Run(bson.D{{"isMaster", 1}}, &rsConf)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be verified that it's a replicaset when 'mongodb.replication.enable: true'", func() {

			By("skipping the non 1 node cases")
			if nodes != 1 || config.MongoReplicaSetEnable != 1 {
				Skip("There is not 1 node")
			}

			By("checking the status of the node")
			Expect(err).NotTo(HaveOccurred())
			Expect(rsConf["setName"]).To(Equal(config.MongoReplicaSetName))
		})

		It("should be verified that it's a standalone when 'mongodb.replication.enable: false'", func() {

			By("skipping the non 1 node cases")
			if (nodes != 1) || (config.MongoReplicaSetEnable == 1) {
				Skip("There is not 1 node or mongodb.replication.enable is not 'false'")
			}
			Expect(rsConf["ok"]).To(Equal(0))
		})
	})

	Context("When deploying a 3-nodes replicaset", func() {
		BeforeEach(func() {
			Session = rootSession.Copy()
		})
		AfterEach(func() {
			Session.Close()
		})
		It("should be able to read existing data on the secondary nodes in slaveok mode", func() {

			By("skipping the non three nodes cases")
			if (nodes != 3) || (config.MongoReplicaSetEnable != 1) {
				Skip("There is not 3 node or mongodb.replication.enable is not 'false'")
			}
			By("toggling the session to slaveok")
			monotonicSession.SetMode(mgo.Eventual, true)
			db := Session.DB("TestDatabase-" + differentiator)
			col := db.C("TestCollection")

			By("finding the file on the least lagging secondary node")
			items := col.Find(bson.M{"Name": itemName})
			Expect(items.Count()).To(Equal(1))
		})
		Context("when gracefully shutting down the primary node and restarting it", func() {

			var oldPrimary string
			var newPrimary string
			var rsConfNode = bson.M{}

			BeforeEach(func() {
				By("skipping the non three nodes cases")
				if nodes != 3 {
					return
				}

				By("identifying the primary")
				err := Session.Run(bson.D{{"isMaster", 1}}, &rsConf)
				Expect(err).NotTo(HaveOccurred())
				var oldPrim = rsConf["primary"]
				oldPrimary = oldPrim.(string)

				By("gracefully shutting down the primary")
				res := bson.M{}
				err = monotonicSession.DB("admin").Run(bson.D{{"shutdown", 1}}, &res)
				Expect(err).To(Equal(io.EOF))

				By("reconnecting to the cluster")
				time.Sleep(4 * 10e9)
				restartNode = &mgo.DialInfo{
					Addrs:          []string{oldPrimary},
					Username:       config.MongoRoot,
					Password:       config.MongoRootPassword,
					ReplicaSetName: config.MongoReplicaSetName,
					Timeout:        10 * time.Second,
					FailFast:       false,
				}
				nodeSession, err = mgo.DialWithInfo(restartNode)
				Expect(err).NotTo(HaveOccurred())

				By("checking the new replicaset info")
				err = monotonicSession.Run(bson.D{{"isMaster", 1}}, &rsConf)
				monotonicSession.SetMode(mgo.Eventual, true)
				newPrim := rsConf["primary"]
				newPrimary = newPrim.(string)
			})

			It("A new primary should have takeover", func() {
				if (nodes != 3) || (config.MongoReplicaSetEnable != 1) {
					Skip("There is not 3 node or mongodb.replication.enable is not 'false'")
				}
				Expect(newPrimary).ToNot(Equal(oldPrimary))
				Expect(newPrimary).NotTo(Equal(""))
			})

			It("The former primary node should have rejoined the cluster as secondary", func() {
				if (nodes != 3) || (config.MongoReplicaSetEnable != 1) {
					Skip("There is not 3 node or mongodb.replication.enable is not 'false'")
				}
				err := nodeSession.Run(bson.D{{"isMaster", 1}}, &rsConfNode)
				Expect(err).NotTo(HaveOccurred())
				var isNodeSec = rsConfNode["secondary"]
				var isNodeSecondary = isNodeSec.(bool)
				Expect(isNodeSecondary).To(BeTrue())
				//seems to work when connecting to the former primary specifying the rs in auth params/second solution by connecting to all the clusters
				/*		var hosts = rsConf["hosts"].([]array)
							var i = 0
							_,v := range hosts {
						if v == oldPrimary {
						i++
						}
						Expect(i).NotTo(Equal(0))
				*/
			})

			It("The former primary node should contain the data", func() {
				if (nodes != 3) || (config.MongoReplicaSetEnable != 1) {
					Skip("There is not 3 node or mongodb.replication.enable is not 'false'")
				}
				nodeSession.SetMode(mgo.Eventual, true)
				db := nodeSession.DB(databaseName)
				col := db.C(collectionName)
				items := col.Find(bson.M{"Name": itemName})
				Expect(items.Count()).To(Equal(1))
				nodeSession.Close()
			})
		})
	})
})
