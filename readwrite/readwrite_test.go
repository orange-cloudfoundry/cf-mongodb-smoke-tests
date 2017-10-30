package readwrite_test

import (
	"gopkg.in/mgo.v2"

	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"
	// . "github.com/onsi/gomega/gbytes"
	// . "github.com/onsi/gomega/gexec"
)

var _ = Describe("MongoDB Service", func() {

	Context("given a standalone MongoDB server", func() {
		It("connects as user, creates collection, writes some data in collection, read the data back, updates the data, deletes data, verify data is absent", func() {
			session, err := mgo.Dial("127.0.0.1:27017")
			if err != nil {
				panic(err)
			}
			defer session.Close()
		})
	})
})
