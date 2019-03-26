package shard_mongo

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

var _ = Describe("MongoDB Sharding Tests", func() {
  var mongosaddrs, shardaddrs, cfgaddrs []string
  for cpt := 0; cpt < len(config.MongosHosts); cpt++ {
      mongosaddrs = append(mongosaddrs, config.MongosHosts[cpt]+":"+config.MongosPorts[cpt])
  }
  var connMongos = &mgo.DialInfo{}
      connMongos.Addrs = mongosaddrs
      connMongos.Username = config.MongoRoot
      connMongos.Password = config.MongoRootPassword
      connMongos.ReplicaSetName = ""
      connMongos.Timeout = 0 * time.Second
      connMongos.FailFast = true
  
  for cpt := 0; cpt < len(config.MongoHosts); cpt++ {
      shardaddrs = append(shardaddrs, config.MongoHosts[cpt]+":"+config.MongoPorts[cpt])
  }  
  var connShard = &mgo.DialInfo{}
      connShard.Addrs = shardaddrs
      connShard.Username = config.MongoRoot
      connShard.Password = config.MongoRootPassword
      connShard.ReplicaSetName = config.MongoReplicaSetName
      connShard.Timeout = 0 * time.Second
      connShard.FailFast = true
  
  for cpt := 0; cpt < len(config.MongoCfgHosts); cpt++ {
      cfgaddrs = append(cfgaddrs, config.MongoCfgHosts[cpt]+":"+config.MongoCfgPorts[cpt])
  }
  var connCfg = &mgo.DialInfo{}
      connCfg.Addrs = cfgaddrs
      connCfg.Username = config.MongoRoot
      connCfg.Password = config.MongoRootPassword
      connCfg.ReplicaSetName = config.MongoCfgReplicaSetName
      connCfg.Timeout = 0 * time.Second
      connCfg.FailFast = true 
 
  var rootCerts = x509.NewCertPool()
  var tlsConfig = &tls.Config{}
  var rootSession *mgo.Session
    
  var err error
      
  var databaseName string
  var db *mgo.Database
  var shardCollectionName = "TestShardCollection"
  type Item struct {
              Id   bson.ObjectId "_id,omitempty"
              Name string        "Name"
       }
  var admin = &mgo.User{}
  var Result = bson.M{}
    
        
  BeforeEach(func() {
      uid, err := uuid.NewV4()
      var differentiator = uid.String() 
      if (config.MongoRequireSsl == 1) {
         rootCerts = x509.NewCertPool()
         if ca, err := ioutil.ReadFile(config.MongoCACert); err==nil {
            rootCerts.AppendCertsFromPEM(ca)
         }
         tlsConfig.RootCAs = rootCerts

         connMongos.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
            conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
            return conn, err
         }
         connShard.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
            conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
            return conn, err
         }
         connCfg.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
            conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
            return conn, err
         }

         Expect(err).NotTo(HaveOccurred())
      }
      By("Connecting to the cluster")
      rootSession, err = mgo.DialWithInfo(connMongos)
      Expect(err).NotTo(HaveOccurred())
      
      By("Creating new database")
      databaseName = "ShardTestDatabase-" + differentiator
      db = rootSession.DB(databaseName)
      By("Enable Sharding for new database")
      err = rootSession.DB("admin").Run(bson.D{{"enablesharding", databaseName }}, &Result)
      Expect(err).NotTo(HaveOccurred())
      By("Shard a Collection")
      err = rootSession.DB("admin").Run(bson.D{{"shardCollection", fmt.Sprintf("%s.%s",databaseName,shardCollectionName) }, {"key", bson.M{"_id":"hashed" }}}, &Result)
      Expect(err).NotTo(HaveOccurred())
      By("Upserting a user Admin")
      admin.Username = "TestUsername" + differentiator
      admin.Password = "TestPassword"
      admin.Roles = []mgo.Role{mgo.Role(mgo.RoleDBAdmin)}
      err = db.UpsertUser(admin)
      Expect(err).NotTo(HaveOccurred())
        
  })
  AfterEach(func() {
      By("Removing the user")
      err = db.RemoveUser(admin.Username)
      Expect(err).NotTo(HaveOccurred())
      By("Drop the database")
      err = db.DropDatabase()
      Expect(err).NotTo(HaveOccurred())
      By("Disconnecting from the cluster")  
      rootSession.Close()
  })
  
	               
     Describe("MongoDB Crud Sharding Tests", func() {
    	var itemName = "some-item"
        var id = bson.NewObjectId()
    	BeforeEach(func() {
    	    By("Loging as user Admin")
	    err = db.Login(admin.Username, admin.Password)
            Expect(err).NotTo(HaveOccurred())
            By("Insert into Shard Collection")
            err = db.C(shardCollectionName).Insert(Item{id ,itemName})
            Expect(err).NotTo(HaveOccurred())
        })

        AfterEach(func() {
            By("Dropping the collection")
            err = db.C(shardCollectionName).DropCollection()
            Expect(err).NotTo(HaveOccurred())
        })

        It("should log successfully as that user", func() {
            err = db.Login(admin.Username, admin.Password)
            Expect(err).NotTo(HaveOccurred())
        }) 
        
              	
        It("should find an existing document", func() {
            By("Looking for an existing document")
            items := db.C(shardCollectionName).Find(bson.M{"Name": itemName})
            Expect(items.Count()).To(Equal(1))
        })
        
        It("should update an existing document", func() {
            By("Updating an existing document")
            newItemName := "New-Item"
            err = db.C(shardCollectionName).Update(bson.M{"_id": id}, bson.M{"$set": bson.M{"Name": newItemName}})
            Expect(err).NotTo(HaveOccurred()) 
            By("Looking for the updating document")
            items := db.C(shardCollectionName).Find(bson.M{"Name": newItemName})
            Expect(items.Count()).To(Equal(1))
        })

        It("should delete an existing document", func() {
            By("Deleting an existing document")
            err = db.C(shardCollectionName).Remove(bson.M{"_id": id})
            Expect(err).NotTo(HaveOccurred())
            By("Looking for deleted document")
            items := db.C(shardCollectionName).Find(bson.M{"Name": itemName})
            Expect(items.Count()).To(Equal(0))
        })
     })
 
     Describe("MongoDB Sharding Tests", func() {
    	var itemName = "some-item-shard"
    	var item_count = 0
        var shutD = bson.M{}
    	var ResultisMas = bson.M{}
    	BeforeEach(func() {
    	    By("Loging as user Admin")
	    err = db.Login(admin.Username, admin.Password)
            Expect(err).NotTo(HaveOccurred())
            By("Insert into Shard Collection")
            for i:=0;i<200;i++ {
               err :=  db.C(shardCollectionName).Insert(Item{bson.NewObjectId(),fmt.Sprintf("%s%d",itemName,i)})
               Expect(err).NotTo(HaveOccurred())
            } 
            By("Count the number of Shard collection items") 
            item_count,err = db.C(shardCollectionName).Find(nil).Count()
            Expect(err).NotTo(HaveOccurred())
        })
        AfterEach(func() {
       	    By("Dropping the collection")
            err = db.C(shardCollectionName).DropCollection()
       	    Expect(err).NotTo(HaveOccurred())
        })
                    	
        Context("Losing of the shard primary node", func() { 
    	    var shardSession *mgo.Session
            BeforeEach(func() {
                By("connect to the Shard primary node")
	  	shardSession, err = mgo.DialWithInfo(connShard)
	  	Expect(err).NotTo(HaveOccurred())
                By("Verify the node is primary")
                err = shardSession.Run(bson.D{{"isMaster", 1}}, &ResultisMas)
	        Expect(err).NotTo(HaveOccurred())
                By("Stop to the Shard primary node")
                err = shardSession.DB("admin").Run(bson.D{{"shutdown", 1}}, &shutD)
                Expect(err).To(Or(Equal(io.EOF), HaveOccurred()))      
            })
                    	
            It("Verify the number of Shard collection items", func() {
                By("Count the number of Shard collection items")
                new_count,err := db.C(shardCollectionName).Find(nil).Count()
                Expect(err).NotTo(HaveOccurred())
                Expect(new_count).Should(Equal(item_count)) 
            })
        })
        Context("Losing of the Config server primary node", func() {
            var cfgSession *mgo.Session
            BeforeEach(func() {
                By("connect to the Config Server primary node")
                cfgSession, err = mgo.DialWithInfo(connCfg)
                Expect(err).NotTo(HaveOccurred())
                By("Verify the node is primary")
                err = cfgSession.Run(bson.D{{"isMaster", 1}}, &ResultisMas)
                Expect(err).NotTo(HaveOccurred())
                By("Stop to the Config Server primary node")
                err = cfgSession.DB("admin").Run(bson.D{{"shutdown", 1}}, &shutD)
                Expect(err).To(Or(Equal(io.EOF), HaveOccurred()))
            })

            It("Verify the number of Shard collection items", func() {
                By("Count the number of Shard collection items")
                new_count,err := db.C(shardCollectionName).Find(nil).Count()
                Expect(err).NotTo(HaveOccurred())
                Expect(new_count).Should(Equal(item_count))
            })
        })

        Context("All node startup", func(){ 
            var shardSession *mgo.Session
            var dbs *mgo.Database
            BeforeEach(func() {
                By("Connect to the Shard 0 cluster") 
                shardSession, err = mgo.DialWithInfo(connShard)
                Expect(err).NotTo(HaveOccurred())
                By("Use the database")
                dbs = shardSession.DB(databaseName)
            })
            AfterEach(func() {
                By("Disconnecting from the Shard 0 cluster")
                shardSession.Close() 
            })

            It("Verify that the Shard Zero Cluster does not contain all items", func() {
                By("Count the number of Shard Zero collection items")
                new_count,err := dbs.C(shardCollectionName).Find(nil).Count()
                Expect(err).NotTo(HaveOccurred())
                fmt.Printf("Items Number of Shard Zero Cluster %d \n",new_count)
                fmt.Printf("Items Number of Shard Collection %d \n",item_count)
                Expect(new_count).ShouldNot(Equal(item_count))
            })
        })

     })  
})

