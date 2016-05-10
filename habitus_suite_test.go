package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os/exec"
	"testing"
	"os"
)

var binPath string = "./habitus"

func TestHabitus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Habitus Suite")
}

var _ = BeforeSuite(func() {
	err := exec.Command("go", "build").Run()
	Expect(err).NotTo(HaveOccurred())

	_, err = os.Stat(binPath)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	err := exec.Command("rm", binPath).Run()
	Expect(err).NotTo(HaveOccurred())
})