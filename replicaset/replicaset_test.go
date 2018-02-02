package replicaset_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/satori/go.uuid"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"io"
	"time"
)

var _ = Describe("MongoDB replicaset tests", func() {

	var nodes = len(config.MongoHosts)
	var addrs []string
	for cpt := 0; cpt < nodes; cpt++ {
		addrs = append(addrs, config.MongoHosts[cpt]+":"+config.MongoPorts[cpt])
	}
	var connInfo = &mgo.DialInfo{
		Addrs:          addrs,
		Username:       config.MongoRoot,
		Password:       config.MongoRootPassword,
		ReplicaSetName: config.MongoReplicaSetName,
		Timeout:        10 * time.Second,
		FailFast:       false,
	}
	var primNode *mgo.DialInfo
	var rootSession, primSession *mgo.Session
	var err error
	var differentiator = uuid.NewV4().String()
	var databaseName = "TestDatabase-" + differentiator
	var db *mgo.Database
	var collectionName = "TestCollection"
	var col *mgo.Collection
	type Item struct {
		Id   bson.ObjectId "_id,omitempty"
		Name string        "Name"
	}
	var itemName = "some-item"
	var item = Item{"", itemName}
	var isMas = bson.M{}
	var shutD = bson.M{}

	BeforeEach(func() {
		By("connecting to the cluster")
		rootSession, err = mgo.DialWithInfo(connInfo)
		Expect(err).NotTo(HaveOccurred())
		db = rootSession.DB(databaseName)

		By("writing data on the primary node")
		col = db.C(collectionName)
		err = col.Insert(item)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		By("dropping collection, and closing all the sessions")
		col.DropCollection()
		Expect(err).NotTo(HaveOccurred())
		rootSession.Close()
	})

	Context("When deploying 1 instance", func() {

		BeforeEach(func() {
			err = rootSession.Run(bson.D{{"isMaster", 1}}, &isMas)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be verified that it's a replicaset when 'mongodb.replication.enable: true'", func() {

			By("skipping the non 1 node cases")
			if nodes != 1 || config.MongoReplicaSetEnable != 1 {
				Skip("There is not 1 node")
			}
			By("checking the status of the node")
			Expect(err).NotTo(HaveOccurred())
			Expect(isMas["setName"]).To(Equal(config.MongoReplicaSetName))
		})

		It("should be verified that it's a standalone when 'mongodb.replication.enable: false'", func() {

			By("skipping the non 1 node cases")
			if (nodes != 1) || (config.MongoReplicaSetEnable == 1) {
				Skip("There is not 1 node or mongodb.replication.enable is not 'false'")
			}
			Expect(isMas["ok"]).To(Equal(0))
		})
	})

	Context("When deploying a 3-nodes replicaset", func() {

		It("should be able to read inserted data on the secondary nodes", func() {

			By("skipping the non three nodes cases")
			if (nodes != 3) || (config.MongoReplicaSetEnable != 1) {
				Skip("There is not 3 node or mongodb.replication.enable is not 'false'")
			}
			By("toggling the session to eventual")
			rootSession.SetMode(mgo.Eventual, true)

			By("finding the file on the least lagging secondary node")
			items := col.Find(bson.M{"Name": itemName})
			//fmt.Print(items.Count()) //for tests
			Expect(items.Count()).To(Equal(1))
		})

		Context("When shutting down the primary in a 3-nodes replicaset", func() {
			var oldPrimary string
			var newPrimary string
			var liveservers []string

			BeforeEach(func() {

				By("skipping the non three nodes cases")
				if nodes != 3 {
					return
				}

				By("identifying the old primary")
				err := rootSession.Run(bson.D{{"isMaster", 1}}, &isMas)
				Expect(err).NotTo(HaveOccurred())
				var oldPrim = isMas["primary"]
				oldPrimary = oldPrim.(string)
				//fmt.Println("oldprim" + oldPrimary) //for test

				By("gracefully shutting down the primary")
				primNode = &mgo.DialInfo{
					Addrs:    []string{oldPrimary},
					Username: config.MongoRoot,
					Password: config.MongoRootPassword,
					Timeout:  10 * time.Second,
					FailFast: false,
				}
				primSession, err = mgo.DialWithInfo(primNode)
				err = primSession.DB("admin").Run(bson.D{{"shutdown", 1}}, &shutD)
				//fmt.Println("bson.M:", shutD) //for tests
				//fmt.Println("error:", err)    // for tests
				Expect(err).To(Or(Equal(io.EOF), HaveOccurred()))
				//fmt.Println("shutdown ok") //for tests

				By("reconnecting to the cluster")
				t := time.Now()
				d := 0 * time.Second
				newPrimary = "nil"
				rootSession.SetMode(mgo.SecondaryPreferred, true)
				for d <= 60*time.Second {
					if err = rootSession.Run(bson.D{{"isMaster", 1}}, &isMas); err == nil {
						err = rootSession.Run(bson.D{{"isMaster", 1}}, &isMas)
						Expect(isMas["ok"]).NotTo(Equal(0))
						if isMas["primary"] != nil && isMas["primary"] != oldPrimary {
							newPrim := isMas["primary"]
							newPrimary = newPrim.(string)
							break
						}
					}
					d = time.Since(t)
				}
				//fmt.Println("time.Duration=", d)        // for test
				//fmt.Println("newPrimary:" + newPrimary) // for test
				Expect(newPrimary).ToNot(Equal(nil))
				rootSession.SetMode(mgo.Strong, true)
			})

			It("The former primary should have rejoined the cluster", func() {
				t := time.Now()
				d := 0 * time.Second
				for d <= 10*time.Second {
					liveservers = rootSession.LiveServers()
					if len(liveservers) == 3 {
						break
					}
					d = time.Since(t)
				}
				Expect(liveservers).To(ContainElement(oldPrimary))
			})

			It("A new primary should have takeover", func() {
				if (nodes != 3) || (config.MongoReplicaSetEnable != 1) {
					Skip("There is not 3 node or mongodb.replication.enable is not 'false'")
				}
				fmt.Println("last test=" + newPrimary) //for tests
				Expect(newPrimary).ToNot(And(Equal(oldPrimary), Equal("nil")))
			})

			It("The former primary node should contain the data", func() {
				if (nodes != 3) || (config.MongoReplicaSetEnable != 1) {
					Skip("There is not 3 node or mongodb.replication.enable is not 'false'")
				}
				rootSession.SetMode(mgo.SecondaryPreferred, true)
				items := col.Find(bson.M{"Name": itemName})
				fmt.Print(items.Count()) //for tests
				Expect(items.Count()).NotTo(Equal(0))
				rootSession.SetMode(mgo.Strong, true)
			})

			/*It("alternative test", func() {
				err = rootSession.Run(bson.D{{"replSetGetConfig", 1}}, &isMas)
			}) */

		})
	})
})
