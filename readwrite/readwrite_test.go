package readwrite_test

import (
	"fmt"
	"time"

	"github.com/pborman/uuid"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("MongoDB Service", func() {

	Context("given a standalone MongoDB server", func() {
		It("connects as user, creates collection, writes some data in collection, read the data back, updates the data, deletes data, verify data is absent", func() {
			// ...
		})
	})
})
