package csi_gcs_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCsiGcs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CsiGcs Suite")
}
