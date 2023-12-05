package itest

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestAll(t *testing.T) {
	suite.Run(t, &BasicTestSuite{
		Suite: Suite{namespace: "ambassador-basic"},
	})
	suite.Run(t, &BasicTestSuite{
		Suite:      Suite{namespace: "ambassador-default"},
		namespaces: []string{"default"},
	})
	suite.Run(t, &AESTestSuite{
		Suite: Suite{namespace: "ambassador-aes"},
	})
	suite.Run(t, &CloudTokenTestSuite{
		Suite: Suite{namespace: "ambassador-token"},
	})
}
