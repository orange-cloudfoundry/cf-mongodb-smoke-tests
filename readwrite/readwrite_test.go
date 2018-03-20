package readwrite_test

import (
	"time"
	"github.com/satori/go.uuid"
        "gopkg.in/mgo.v2"
        "gopkg.in/mgo.v2/bson"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MongoDB CRUD tests", func() {

	var nodes = len(config.MongoHosts)
	var addrs []string
	for i := 0; i < nodes; i++ {
		addrs = append(addrs, config.MongoHosts[i]+":"+config.MongoPorts[i])
	}
	var connInfo = &mgo.DialInfo{
		Addrs:          addrs,
		Username:       config.MongoRoot,
		Password:       config.MongoRootPassword,
		ReplicaSetName: config.MongoReplicaSetName,
		Timeout:        30 * time.Second,
		FailFast:       true,
	}

	var rootSession *mgo.Session
	var err error
	uid, err := uuid.NewV4()
	var differentiator = uid.String()

	BeforeEach(func() {
		By("Connecting to the instance")
		rootSession, err = mgo.DialWithInfo(connInfo)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		By("Disconnecting from the instance")
		rootSession.LogoutAll()
		rootSession.Close()
	})

	Context("When an admin user is created", func() {

		var databaseName = "TestDatabase-" + differentiator
		var db *mgo.Database

		var admin = mgo.User{
			Username: "TestUsername" + differentiator,
			Password: "TestPassword",
			Roles:    []mgo.Role{mgo.RoleDBAdmin},
		}

		BeforeEach(func() {
			By("Upserting a user Admin")
			db = rootSession.DB(databaseName)
			err := db.UpsertUser(&admin)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			By("Removing the user")
			err := db.RemoveUser(admin.Username)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should log successfully as that user", func() {
			err := db.Login(admin.Username, admin.Password)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("When connected to a database as an admin user", func() {

			var collectionName = "TestCollection"
			var col *mgo.Collection

			type Item struct {
				Id   bson.ObjectId "_id,omitempty"
				Name string        "Name"
			}

			var itemName = "some-item"
			var item = Item{"", itemName}

			BeforeEach(func() {
				err := db.Login(admin.Username, admin.Password)
				Expect(err).NotTo(HaveOccurred())

				By("Inserting data")
				col = db.C(collectionName)
				err = col.Insert(item)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				By("Dropping the collection")
				col.DropCollection()
			})

			It("should find an existing document", func() {
				By("Looking for an existing document")
				items := col.Find(bson.M{"Name": itemName})
				Expect(items.Count()).Should(Equal(1))
			})

			It("should update an existing document", func() {
				By("Updating an existing document")
				newItemName := "Pierre"
				col.Update(bson.M{"Name": itemName}, bson.M{"$set": bson.M{"Name": newItemName}})
				By("Looking for the updating document")
				search := col.Find(bson.M{"Name": newItemName})
				Expect(search.Count()).Should(Equal(1))
			})

			It("should delete an existing document", func() {
				By("Deleting an existing document")
				err := col.Remove(bson.M{"Name": itemName})
				Expect(err).NotTo(HaveOccurred())
				By("Looking for deleted document")
				items := col.Find(bson.M{"Name": itemName})
				Expect(items.Count()).Should(Equal(0))
			})
		})
	})
})
