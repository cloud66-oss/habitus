package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cloud66/cxbuild/build"
	"github.com/cloud66/cxbuild/configuration"
	"github.com/op/go-logging"
)

var format = logging.MustStringFormatter(
	"%{color}▶ %{message} %{color:reset}",
)

func main() {
	args := os.Args[1:]

	var log = logging.MustGetLogger("cxbuilder")
	logging.SetFormatter(format)

	config := configuration.CreateConfig()
	flag.StringVar(&config.Buildfile, "f", "./build.yml", "Build file path")
	flag.BoolVar(&config.NoCache, "no-cache", false, "Use cache in build")
	flag.BoolVar(&config.SuppressOutput, "suppress", false, "Suppress build output")
	flag.BoolVar(&config.ForceRmTmpContainer, "force-rm", false, "Force remove intermediate containers")
	flag.StringVar(&config.StartStep, "s", "", "Starting step for the build")
	flag.StringVar(&config.UniqueID, "uid", "", "Unique ID for the build. Used only for multi-tenanted build environments")

	config.Logger = *log

	flag.Parse()

	if len(args) > 0 && args[0] == "help" {
		fmt.Println("cxbuild - (c) 2015 Cloud 66 Inc.")
		flag.PrintDefaults()
		return
	}

	c, err := build.LoadBuildFromFile(&config)
	if err != nil {
		log.Fatalf("Failed: %s\n", err.Error())
	}

	b := build.NewBuilder(c, &config)
	err = b.StartBuild(config.StartStep)
	if err != nil {
		log.Error("Error during build %s", err.Error())
	}
}
