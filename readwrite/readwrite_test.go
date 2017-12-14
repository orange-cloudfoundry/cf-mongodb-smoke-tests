package readwrite_test

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var randUser, randPwd, randDb, randCol  = "Username","Password", "Database", "Collection"
var user = mgo.User{
		Username: randUser,
		Password: randPwd,
		Roles: []mgo.Role{ mgo.RoleDBAdmin },
}
type Person struct {
		Id    bson.ObjectId   "_id,omitempty"
		Name  string          "Name"
}
var Bob = Person{"","Bob"}
var Jean = Person{"","Jean"}
var dial = &mgo.DialInfo{
			Addrs: []string{config.MongoHost + ":" + config.MongoPort,},
			ReplicaSetName: config.MongoReplicaSetName,
			Username: config.MongoRoot,
			Password: config.MongoRootPassword,
						}
var session, err= mgo.DialWithInfo(dial)
var db = session.DB(randDb)
var col = db.C(randCol)

var _ = Describe("access to data base", func(){

	Context("When an using a Mongodb intance", func(){

		Context("When creating a session", func(){

			It("should be able to create a session as root", func(){
				_, err := mgo.DialWithInfo(dial)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("When creating a session as root user", func(){
			BeforeEach(func(){
				session, err= mgo.DialWithInfo(dial)
				err = db.UpsertUser(&user)
			})
			AfterEach(func(){
				session.LogoutAll()
				db.RemoveUser(user.Username)
				session.Close()
			})
			It("should successfully create an admin user for a given database", func(){
				Expect(err).NotTo(HaveOccurred())
			})
			Context("When logging to a database as an admin user", func(){
				BeforeEach(func(){
					session, err= mgo.DialWithInfo(dial)
					err = db.UpsertUser(&user)
				})
				AfterEach(func(){
					session.LogoutAll()
					db.RemoveUser(user.Username)
					session.Close()
				})
				It("should login successfully as this user", func(){
					err = db.Login(user.Username, user.Password)
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
		Context("When logging to a database as an admin user", func(){

			Context("When a document is inserted in the database ", func(){

				BeforeEach(func(){
					session, err= mgo.DialWithInfo(dial)
					db.UpsertUser(&user)
					err= col.Insert(Bob)
					db.Login(user.Username, user.Password)
				})

				AfterEach(func(){
					col.Remove(bson.M{"Name":"Bob"})
					col.DropCollection()
					session.LogoutAll()
					db.RemoveUser(user.Username)
					session.Close()
				})

				It("should insert a document and retrieve it", func(){
					search:= col.Find(bson.M{"Name":"Bob"})
					Expect(search.Count()).To(Equal(1))
				})

				It("should be able to update an existing document", func(){
					col.Update(bson.M{"Name": "Bob"}, bson.M{"$set": bson.M{"Name": "Pierre"}})
					search:= col.Find(bson.M{"Name":"Pierre"})
					Expect(search.Count()).To(Equal(1))
				})

				It("should be able to delete an existing document", func(){
					err = col.Remove(bson.M{"Name":"Bob"})
					Expect(err).NotTo(HaveOccurred())
					search:= col.Find(bson.M{"Name":"Bob"})
					Expect(search.Count()).To(Equal(0))
				})
			})

		})
	})
})
