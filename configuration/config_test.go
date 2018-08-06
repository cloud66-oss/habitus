package configuration_test

import (
  . "github.com/onsi/ginkgo"
  . "github.com/onsi/gomega"
  "testing"
  "github.com/cloud66/habitus/configuration"
)

var _ = Describe("Config", func() {
  var c configuration.Config

  BeforeEach(func() {
    c = configuration.CreateConfig()
  })

  Describe("--os allowed values", func() {
    It("should accept debian", func() {
      c.OsType = "debian"
      Expect(c.ValidateOsType()).To(Equal(true))
    })

    It("should accept redhat", func() {
      c.OsType = "redhat"
      Expect(c.ValidateOsType()).To(Equal(true))
    })

    It("should accept alpine", func() {
      c.OsType = "alpine"
      Expect(c.ValidateOsType()).To(Equal(true))
    })

    It("should accept busybox", func() {
      c.OsType = "busybox"
      Expect(c.ValidateOsType()).To(Equal(true))
    })

    It("should not accept msdos", func() {
      c.OsType = "msdos"
      Expect(c.ValidateOsType()).To(Equal(false))
    })
  })

})


func ConfigTest(t *testing.T) {
  RegisterFailHandler(Fail)
  RunSpecs(t, "Habitus Config Suite")
}
