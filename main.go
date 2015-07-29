package main

import (
	"fmt"

	"github.com/cloud66/cxbuild/build"
)

func main() {
	c, err := build.LoadBuildFromFile("./build.yml")
	if err != nil {
		fmt.Printf("Failed: %s\n", err.Error())
	}

	b := build.NewBuilder(c, "cxbuilder") // TODO: This is passed in as a parameter
	err = b.StartBuild()
	if err != nil {
		fmt.Printf("Error during build %s", err.Error())
	}
}
