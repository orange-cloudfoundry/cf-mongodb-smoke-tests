package replicaset_test

import (
	"io"
        "time"
        "gopkg.in/mgo.v2"
        "gopkg.in/mgo.v2/bson"
	"github.com/satori/go.uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		FailFast:       true,
	}
	var primNode *mgo.DialInfo
	var rootSession, primSession *mgo.Session
	var err error
	uid, err := uuid.NewV4()
	var differentiator = uid.String()
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

		By("Connecting to the cluster")
		rootSession, err = mgo.DialWithInfo(connInfo)
		Expect(err).NotTo(HaveOccurred())
		db = rootSession.DB(databaseName)

		By("Writing data on the primary node")
		col = db.C(collectionName)
		err = col.Insert(item)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {

		By("Dropping collection, and closing the root Session")
		col.DropCollection()
		Expect(err).NotTo(HaveOccurred())
		rootSession.Close()
	})

        if (nodes == 1) {
		Context("When deploying 1 instance ", func() {

			BeforeEach(func() {
				err = rootSession.Run(bson.D{{"isMaster", 1}}, &isMas)
				Expect(err).NotTo(HaveOccurred())
			})
		
			if (config.MongoReplicaSetEnable == 1) {
				It("should be verified that it's a replicaset when 'mongodb.replication.enable: true'", func() {
					By("Checking the status of the node")
					Expect(isMas["setName"]).To(Equal(config.MongoReplicaSetName))
				})
			} else {
				It("should be verified that it's a standalone when 'mongodb.replication.enable: false'", func() {
					Expect(isMas["ok"]).To(Equal(0))
				})
			}
		})
	} else {

		Context("When deploying a multi-nodes replicaset", func() {
			if (config.MongoReplicaSetEnable != 1) {
				It("should be verified that the parameter 'mongodb.replication.enable' is true", func() {
                                        Expect(config.MongoReplicaSetEnable).To(Equal(1))
                                })
			        return
			}

			It("should be able to read inserted data on the secondary nodes", func() {

				By("Toggling the session to eventual")
				rootSession.SetMode(mgo.Eventual, true)

				By("Finding the file on the least lagging secondary node")
				items := col.Find(bson.M{"Name": itemName})
				Expect(items.Count()).Should(Equal(1))
			})

			Context("When shutting down the primary in a multi-nodes replicaset", func() {
				var oldPrimary string
				var newPrimary string
				var liveservers []string

				BeforeEach(func() {
					By("Identifying the old primary")
					err := rootSession.Run(bson.D{{"isMaster", 1}}, &isMas)
					Expect(err).NotTo(HaveOccurred())
					var oldPrim = isMas["primary"]
					oldPrimary = oldPrim.(string)
					By("Gracefully shutting down the primary")
					primNode = &mgo.DialInfo{
						Addrs:    []string{oldPrimary},
						Username: config.MongoRoot,
						Password: config.MongoRootPassword,
						Timeout:  10 * time.Second,
						FailFast: false,
					}
					primSession, err = mgo.DialWithInfo(primNode)
					err = primSession.DB("admin").Run(bson.D{{"shutdown", 1}}, &shutD)
					Expect(err).To(Or(Equal(io.EOF), HaveOccurred()))

					By("Reconnecting to the cluster")
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
					Expect(newPrimary).ToNot(Equal(nil))

					By("Putting back the cluster to strong mode")
					rootSession.SetMode(mgo.Strong, true)

					By("Return of all the nodes in the cluster")
					t = time.Now()
					d = 0 * time.Second
					for d <= 180*time.Second {
						liveservers = rootSession.LiveServers()
						if len(liveservers) == nodes {
							break
						}
						d = time.Since(t)

					}
				})

				It("The former primary should have rejoined the cluster", func() {
					By("Checking come back of the old primary node in the cluster")
					Expect(liveservers).To(ContainElement(oldPrimary))
				})

				It("A new primary should have takeover", func() {
					By("Checking Primary node change")
					Expect(newPrimary).ToNot(And(Equal(oldPrimary), Equal("nil")))
				})

				It("The former primary node should contain the data", func() { //it's not targeting specifically the former primary but the best suited secondary, which might be sufficient for testing
					By("Checking the former primary node is up to date")
					rootSession.SetMode(mgo.SecondaryPreferred, true)
					items := col.Find(bson.M{"Name": itemName})
					Expect(items.Count()).Should(Not(Equal(0)))
					rootSession.SetMode(mgo.Strong, true)
				})
			})
		})
	}
})
