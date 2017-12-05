package readwrite_test

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"//added

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	//. "github.com/onsi/gomega/gbytes"
	//. "github.com/onsi/gomega/gexec"
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

				Context("When creating a user as admin for a database ", func(){						
					
					BeforeEach(func(){
						session, err= mgo.DialWithInfo(dial)
					  	err = db.UpsertUser(&user)
					})

					AfterEach(func(){
						session.LogoutAll()
						db.RemoveUser(user.Username)
						session.Close()
					})
					It("should successfully create a user as admin", func(){
						Expect(err).NotTo(HaveOccurred())
					})

					It("should login successfully as this user", func(){
						err = db.Login(user.Username, user.Password)
						Expect(err).NotTo(HaveOccurred())
					})
				}) 
				Context("When logging to a database as a user", func(){					

					Context("When a document is inserted in the database ", func(){

						BeforeEach(func(){	
						session, err= mgo.DialWithInfo(dial)
					  	err = db.UpsertUser(&user)
						err= col.Insert(Bob)
						db.Login(user.Username, user.Password)
						
					  	err = db.UpsertUser(&user)
					    })

					    AfterEach(func(){
							err= col.Remove(bson.M{"Name":"Bob"})
							err= col.DropCollection()	
							session.LogoutAll()
							db.RemoveUser(user.Username)
							session.Close()					
						})

						It("should insert a new document", func(){
							err= col.Insert(Jean)
							Expect(err).NotTo(HaveOccurred())
							search:= col.Find(bson.M{"Name":"Jean"})
							Expect(search.Count()).To(Equal(1))
						})

						It("should retrieve an existing document", func(){
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
