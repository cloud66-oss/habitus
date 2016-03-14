package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
	"os"
)

var binPath string = "./habitus"

func TestHabitus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Habitus Suite")
}

var _ = BeforeSuite(func() {
	_, err := os.Stat(binPath)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
})