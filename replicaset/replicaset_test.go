package replicaset_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/satori/go.uuid"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net"
	"time"
)

var _ = Describe("MongoDB replicaset tests", func() {

	var nodes = len(config.MongoHosts)
	var addrs []string
	for cpt := 0; cpt < nodes; cpt++ {
		addrs = append(addrs, config.MongoHosts[cpt]+":"+config.MongoPorts[cpt])
	}
	var connInfo = &mgo.DialInfo{}
	connInfo.Addrs = addrs
	connInfo.Username = config.MongoRoot
	connInfo.Password = config.MongoRootPassword
	connInfo.ReplicaSetName = config.MongoReplicaSetName
	connInfo.Timeout = 120 * time.Second
	connInfo.FailFast = false

	var primNode *mgo.DialInfo
	var rootCerts = x509.NewCertPool()
	var tlsConfig = &tls.Config{}

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
	var ResultisMas = bson.M{}
	var shutD = bson.M{}

	if (config.MongoRequireSsl == 1) {
		rootCerts = x509.NewCertPool()
		if ca, err := ioutil.ReadFile(config.MongoCACert); err==nil {
			rootCerts.AppendCertsFromPEM(ca)
		}
		tlsConfig.RootCAs = rootCerts

		connInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
			conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
		//	if err != nil {
		//		fmt.Println(err)
		//	}
			return conn, err
		}
	}

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

	if nodes == 1 {
		Context("When deploying 1 instance ", func() {

			BeforeEach(func() {
				err = rootSession.Run(bson.D{{"isMaster", 1}}, &ResultisMas)
				Expect(err).NotTo(HaveOccurred())
			})
			if config.MongoReplicaSetEnable == 1 {
				It("should be verified that it's a replicaset when 'mongodb.replication.enable: true'", func() {
					By("Checking existence of replicaset")
					Expect(ResultisMas["setName"]).To(Equal(config.MongoReplicaSetName))
				})
			} else {
				It("should be verified that it's a standalone when 'mongodb.replication.enable: false'", func() {
					Expect(ResultisMas["ok"]).To(Equal(1.0))
					By("Checking no replicaset")
					Expect(ResultisMas["setName"]).To(BeNil())
				})
			}
		})
	} else {

		Context("When deploying a multi-nodes replicaset", func() {
			if config.MongoReplicaSetEnable != 1 {
				It("should be verified that the parameter 'mongodb.replication.enable' is true", func() {
					Expect(config.MongoReplicaSetEnable).To(Equal(1))
				})
				return
			}

			It("should be able to read inserted data on the secondary nodes", func() {
                                var item_count = 0
				By("Toggling the session to Secondary")
				rootSession.SetMode(mgo.Secondary, true)
				err = rootSession.Run(bson.D{{"isMaster", 1}}, &ResultisMas)
				Expect(err).NotTo(HaveOccurred())
				By("Checking node type")
				if ResultisMas["primary"].(string) == ResultisMas["me"].(string) {
					fmt.Println("This node is not a secondary node")
				}
				Expect(ResultisMas["primary"].(string)).NotTo(Equal(ResultisMas["me"].(string)))
				By("Finding the file on the least lagging secondary node")
				t := time.Now()
				d :=  0 * time.Second
				for d <= 60*time.Second {
                                    item_count,err = col.Find(bson.M{"Name": itemName}).Count()
                                    Expect(err).NotTo(HaveOccurred())
                                    if (item_count != 0 ) { break }
                                    time.Sleep(2 * time.Second)
                                    d = time.Since(t)
                                }
                                Expect(item_count).Should(Equal(1))
			})

			Context("When shutting down the primary in a multi-nodes replicaset", func() {
				var oldPrimary string
				var newPrimary string
				var liveservers []string
				const delay = 2 * time.Second

				BeforeEach(func() {
					By("Identifying the old primary")
					err := rootSession.Run(bson.D{{"isMaster", 1}}, &ResultisMas)
					Expect(err).NotTo(HaveOccurred())
					oldPrimary = ResultisMas["primary"].(string)
					By("Gracefully shutting down the primary")
					primNode = &mgo.DialInfo{}
					primNode.Addrs = []string{oldPrimary}
					primNode.Username = config.MongoRoot
					primNode.Password = config.MongoRootPassword
					primNode.Timeout =  120 * time.Second
					primNode.FailFast = false
					if (config.MongoRequireSsl == 1) {
						primNode.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
							conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
							return conn, err
						}
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
						if err = rootSession.Run(bson.D{{"isMaster", 1}}, &ResultisMas); err == nil {
							Expect(ResultisMas["ok"]).NotTo(Equal(0))
							if ResultisMas["primary"] != nil && ResultisMas["primary"].(string) != oldPrimary {
								newPrimary = ResultisMas["primary"].(string)
								break
							}
						}
						time.Sleep(delay)
						d = time.Since(t)
					}
					Expect(newPrimary).ToNot(Equal("nil"))

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
						time.Sleep(delay)
						d = time.Since(t)

					}
				})

				It("The former primary should have rejoined the cluster", func() {
					By("Checking come back of the old primary node in the cluster")
					Expect(liveservers).To(ContainElement(oldPrimary))
				})

				It("A new primary should have takeover", func() {
					By("Checking Primary node change")
					Expect(newPrimary).ToNot(Equal(oldPrimary))
				})

				It("The former primary node should contain the data", func() { //it's not targeting specifically the former primary but the best suited secondary, which might be sufficient for testing
					By("Opening session on former primary")
					oldprimNode := &mgo.DialInfo{}
					oldprimNode.Addrs = []string{oldPrimary}
					oldprimNode.Username = config.MongoRoot
					oldprimNode.Password = config.MongoRootPassword
					oldprimNode.Timeout =  120 * time.Second
					oldprimNode.FailFast = false
					oldprimNode.Direct = true
					if (config.MongoRequireSsl == 1) {
						oldprimNode.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
							conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
							return conn, err
						}
					}
					oldprimSession, err := mgo.DialWithInfo(oldprimNode)
					Expect(err).NotTo(HaveOccurred())
					oldprimSession.SetMode(mgo.Eventual, true)
					defer func() {
						By("Closing session on former primary")
						oldprimSession.Close()
					}()
					err = oldprimSession.Run(bson.D{{"isMaster", 1}}, &ResultisMas)
					Expect(err).NotTo(HaveOccurred())
					By("Checking the connection on the former primary node")
					if oldPrimary != ResultisMas["me"].(string) {
						fmt.Println("This node is not the former primary node")
					}
					Expect(ResultisMas["me"].(string)).To(Equal(oldPrimary))
					By("Checking the former primary node is up to date")
					olddb := oldprimSession.DB(databaseName)
					oldcol := olddb.C(collectionName)
					olditems := oldcol.Find(bson.M{"Name": itemName})
					Expect(olditems.Count()).Should(Not(Equal(0)))
				})
			})
		})
	}
})
