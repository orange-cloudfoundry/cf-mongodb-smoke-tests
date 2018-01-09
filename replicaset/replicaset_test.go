package replicaset_test

import (
	//"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/satori/go.uuid"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	//"reflect"
	//"flag"
	"io"
	"os"
	"os/exec"
	//"syscall"
	"time"
)

var _ = Describe("MongoDB CRUD tests", func() {

	var nodes = len(config.MongoHosts)
	var addrs []string
	for cpt := 0; cpt < nodes; cpt++ {
		//addrs = append(addrs, config.MongoHosts[cpt]+":"+config.MongoPort[cpt]) //for test (also put []string in suite test)
		addrs = append(addrs, config.MongoHosts[cpt]+":"+config.MongoPort[0]) //for errand
	}
	fmt.Println(addrs) //for testing
	var connInfo = &mgo.DialInfo{
		Addrs:          addrs,
		Username:       config.MongoRoot,
		Password:       config.MongoRootPassword,
		ReplicaSetName: config.MongoReplicaSetName,
		Timeout:        10 * time.Second,
		FailFast:       false,
	}
	//var restartNode *mgo.DialInfo
	var rootSession, monotonicSession, nodeSession *mgo.Session
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
			Expect(rsConf["setName"]).To(Equal("rs0"))
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
			rootSession, err = mgo.DialWithInfo(connInfo)
			monotonicSession = rootSession.Clone()
		})

		AfterEach(func() {
			//	monotonicSession.Close()
		})

		It("should be able to read existing data on the secondary nodes in slaveok mode", func() {

			By("skipping the non three nodes cases")
			if (nodes != 3) || (config.MongoReplicaSetEnable != 1) {
				Skip("There is not 3 node or mongodb.replication.enable is not 'false'")
			}
			By("toggling the session to slaveok")
			monotonicSession.SetMode(mgo.Eventual, true)
			db := monotonicSession.DB("TestDatabase-" + differentiator)
			col := db.C("TestCollection")

			By("finding the file on the least lagging secondary node")
			items := col.Find(bson.M{"Name": itemName})
			Expect(items.Count()).To(Equal(1))
		})

		Context("when gracefully shutting down the primary node and restarting it", func() {

			var oldPrimary string

			BeforeEach(func() {

				By("identifying the master")
				err = monotonicSession.Run(bson.D{{"isMaster", 1}}, &rsConf)
				Expect(err).NotTo(HaveOccurred())
				if err != nil { //debugging
					fmt.Println("monotonicSessionerr") //debugging
				} //debugging
				var oldPrim = rsConf["primary"]
				oldPrimary = oldPrim.(string)
				fmt.Println("oldPrimary") //for debugging
				fmt.Println(oldPrimary)   //for debugging

				By("gracefully shutdown the primary")
				res := bson.M{}
				err := monotonicSession.DB("admin").Run(bson.D{{"shutdown", 1}}, &res)
				Expect(err).To(Equal(io.EOF))

				By("reconnecting the former primary's instance") //just debugging
				time.Sleep(2 * 10e9)
				restartNode := &mgo.DialInfo{
					Addrs:          []string{oldPrimary},
					Username:       config.MongoRoot,
					Password:       config.MongoRootPassword,
					ReplicaSetName: config.MongoReplicaSetName,
					Timeout:        10 * time.Second,
					FailFast:       false,
				}
				nodeSession, err = mgo.DialWithInfo(restartNode)

				if err != nil { //for testing
					fmt.Fprintln(os.Stderr, "Error creating StdoutPipe for Cmd", err)
					os.Exit(1)
				}

				By("checking the new replication info")
				err = nodeSession.Run(bson.D{{"isMaster", 1}}, &rsConf)
				//Expect(err).NotTo(HaveOccurred())
				Expect(rsConf["ok"]).To(Equal(0))

				if err != nil { //for testing
					fmt.Println("failrestartnodedddddddddddddd") //for testing
				} //for testing */
			})

			It("A new primary should have takeover", func() {

				if (nodes != 3) || (config.MongoReplicaSetEnable != 1) {
					Skip("There is not 3 node or mongodb.replication.enable is not 'false'")
				}
				var newPrim = rsConf["primary"]
				var newPrimary = newPrim.(string)
				fmt.Println("newPrimaryyyyyyyyyyyyyy:" + newPrimary) //debug
				Expect(newPrimary).NotTo(Equal(""))                  //DEBUGGING
				if newPrimary == "" {                                //for testing
					fmt.Println("pas de passage de temoin") //for testing
				} //for testing
				Expect(newPrimary).NotTo(Equal(oldPrimary))
			})

			It("The former primary node should have rejoined the cluster as secondary", func() { //err
				if (nodes != 3) || (config.MongoReplicaSetEnable != 1) {
					Skip("There is not 3 node or mongodb.replication.enable is not 'false'")
				}
				fmt.Println("prim or not")
				fmt.Println(rsConf["primary"])
				Expect(rsConf["primary"]).To(BeTrue())
				fmt.Println("prim or not")
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
			})
		})
	})
})
