package main

import (
	"fmt"

	"github.com/cloud66/cxbuild/build"
	"github.com/cloud66/cxbuild/configuration"
)

func main() {
	c, err := configuration.LoadBuildFromFile("./build.yml")
	if err != nil {
		fmt.Printf("Failed: %s\n", err.Error())
	}

	b := build.NewBuilder()
	b.BuildStep(&c.Steps[0])
}
