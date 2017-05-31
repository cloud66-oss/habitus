package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloud66/habitus/build"
	"github.com/cloud66/habitus/configuration"
	"github.com/cloud66/habitus/api"
	"github.com/op/go-logging"

	"github.com/bugsnag/bugsnag-go"
)

var prettyFormat = logging.MustStringFormatter(
	"%{color}▶ %{message} %{color:reset}",
)
var plainFormat = logging.MustStringFormatter(
	"[%{level}] - %{message}",
)

var (
	flagLevel       string
	flagShowHelp    bool
	flagShowVersion bool
	flagPrettyLog   bool
	VERSION         string = "dev"
	BUILD_DATE      string = ""
)

func init() {
	bugsnag.Configure(bugsnag.Configuration{
		APIKey:     "ba9d7ae6b333e27971d86e5bf7abe996",
		AppVersion: VERSION,
	})
}

const DEFAULT_DOCKER_HOST = "unix:///var/run/docker.sock"

func main() {
	args := os.Args[1:]
	defer bugsnag.AutoNotify()

	var log = logging.MustGetLogger("habitus")
	logging.SetFormatter(plainFormat)

	config := configuration.CreateConfig()
	flag.StringVar(&config.Buildfile, "f", "build.yml", "Build file path. Defaults to build.yml in the workdir")
	flag.StringVar(&config.Workdir, "d", "", "Work directory for this build. Defaults to the current directory")
	flag.BoolVar(&config.NoCache, "no-cache", false, "Don't use cache in build")
	flag.BoolVar(&config.SuppressOutput, "suppress", false, "Suppress build output")
	flag.BoolVar(&config.RmTmpContainers, "rm", true, "Remove intermediate containers")
	flag.BoolVar(&config.ForceRmTmpContainer, "force-rm", false, "Force remove intermediate containers")
	flag.StringVar(&config.UniqueID, "uid", "", "Unique ID for the build. Used only for multi-tenanted build environments")
	flag.StringVar(&flagLevel, "level", "debug", "Log level: debug, info, notice, warning, error and critical")
	flag.BoolVar(&flagPrettyLog, "pretty", true, "Display logs with color and formatting")
	
	dockerhost, ok := os.LookupEnv("DOCKER_HOST")
	if !ok {
		dockerhost = DEFAULT_DOCKER_HOST
	} 
	
	flag.StringVar(&config.DockerHost, "host", dockerhost, "Docker host link. Uses DOCKER_HOST if missing.")
	flag.StringVar(&config.DockerCert, "certs", os.Getenv("DOCKER_CERT_PATH"), "Docker cert folder. Uses DOCKER_CERT_PATH if missing")
	flag.Var(&config.EnvVars, "env", "Environment variables to be used in the build.yml. Uses parent process environment variables if empty")
	flag.Var(&config.BuildArgs, "build", "Build arguments to be used during each Dockerfile build step.")
	flag.BoolVar(&config.KeepSteps, "keep-all", false, "Overrides the keep flag for all steps. Used for debugging")
	flag.BoolVar(&config.KeepArtifacts, "keep-artifacts", false, "Keep the temporary artifacts created on the host during build. Used for debugging")
	flag.BoolVar(&config.UseTLS, "use-tls", false, "Uses TLS connection with Docker daemon")
	flag.BoolVar(&config.NoSquash, "no-cleanup", false, "Skip cleanup commands for this run. Used for debugging")
	flag.BoolVar(&config.FroceRmImages, "force-rmi", false, "Force remove of unwanted images")
	flag.BoolVar(&config.NoPruneRmImages, "noprune-rmi", false, "No pruning of unwanted images")
	flag.BoolVar(&flagShowHelp, "help", false, "Display the help")
	flag.BoolVar(&flagShowVersion, "version", false, "Display version information")
	flag.IntVar(&config.ApiPort, "port", 8080, "Port to server the API")
	flag.StringVar(&config.ApiBinding, "binding", "192.168.99.1", "Network address to bind the API to. (see documentation for more info)")
	flag.BoolVar(&config.SecretService, "secrets", false, "Turn Secrets Service on or off")
	flag.BoolVar(&config.AllowAfterBuildCommands, "after-build-commands", false, "Allow to run arbitrary commands on the host after build")
	flag.StringVar(&config.SecretProviders, "sec-providers", "file,env", "All available secret providers. Comma separated")
	flag.StringVar(&config.DockerMemory, "docker-memory", "", "Memory limits to apply to Docker build operations. More: https://docs.docker.com/engine/reference/commandline/build")
	flag.StringVar(&config.DockerCPUSetCPUs, "docker-cpuset-cpus", "", "CPU binding limits to apply to Docker build operations. More: https://docs.docker.com/engine/reference/commandline/build")
	flag.IntVar(&config.DockerCPUShares, "docker-cpu-shares", 1024, "CPU share weighting to apply to Docker build operations. More: https://docs.docker.com/engine/reference/commandline/build")

	config.Logger = *log
	flag.Parse()

	if flagPrettyLog {
		logging.SetFormatter(prettyFormat)
	}

	if flagShowHelp || (len(args) > 0 && args[0] == "help") {
		fmt.Println("Habitus - (c) 2016 Cloud 66 Inc.")
		flag.PrintDefaults()
		return
	}

	if flagShowVersion || (len(args) > 0 && args[0] == "version") {
		if BUILD_DATE == "" {
			fmt.Printf("Habitus - v%s (c) 2016 Cloud 66 Inc.\n", VERSION)
		} else {
			fmt.Printf("Habitus - v%s (%s) (c) 2016 Cloud 66 Inc.\n", VERSION, BUILD_DATE)
		}
		return
	}

	level, err := logging.LogLevel(flagLevel)
	if err != nil {
		fmt.Println("Invalid log level value. Falling back to debug")
		level = logging.DEBUG
	}
	logging.SetLevel(level, "habitus")

	if config.Workdir == "" {
		if curr, err := os.Getwd(); err != nil {
			log.Fatal("Failed to get the current directory")
			os.Exit(1)
		} else {
			config.Workdir = curr
		}
	}

	if config.Buildfile == "build.yml" {
		config.Buildfile = filepath.Join(config.Workdir, "build.yml")
	}

	c, err := build.LoadBuildFromFile(&config)
	if err != nil {
		log.Fatalf("Failed: %s", err.Error())
	}

	if c.IsPrivileged && os.Getenv("SUDO_USER") == "" {
		log.Fatal("Some of the build steps require admin privileges (sudo). Please run with sudo\nYou might want to use --certs=$DOCKER_CERT_PATH --host=$DOCKER_HOST params to make sure all environment variables are available to the process")
		os.Exit(1)
	}

	b := build.NewBuilder(c, &config)

	if config.SecretService {
		// start the API
		secret_service := &api.Server{Builder: b}
		err = secret_service.StartServer(VERSION)
		if err != nil {
			log.Fatalf("Cannot start API server due to %s", err.Error())
			os.Exit(2)
		}
	}

	err = b.StartBuild()
	if err != nil {
		log.Errorf("Error during build %s", err.Error())
	}
}
