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

	b := build.NewBuilder(c)
	err = b.StartBuild()
	if err != nil {
		fmt.Printf("Error during build %s", err.Error())
	}
}
