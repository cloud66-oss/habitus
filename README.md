# Habitus
Habitus adds workflows to Docker build. This means you can create a chain of builds to generate your final Docker image based on a workflow. This is particularly useful if your code is in compiled languages like Java or Go or if you need to use secrets like SSH keys during the build.

![Codeship Status for cloud66/habitus](https://codeship.com/projects/714284d0-e914-0133-1e5d-4eaa3299b296/status)

- Website: http://www.habitus.io/
- [Download Habitus](https://github.com/cloud66/habitus/releases?utm_source=Githubdownload&utm_medium=GHDpage&utm_campaign=habitus)
- Slack Channel: https://cloud66ers.slack.com/messages/habitus/
- Articles: http://blog.cloud66.com/tag/habitus/

![Logo habitus](https://lh5.googleusercontent.com/_PbaTkJhpA9zVRW_pj3Mt1ntpAZ6IUjTn0yDkVCsUZnJnE3jAxr5ciGF5SqdtR45--EHlIdYyr3dj7DcjRVfLBSS6BQPaGrwzzvMqqEcDJc47sDY4d2s9QQlJi3ZXUYPkODWOF2a)
A build flow tool for Docker 

Habitus is a standalone build flow tool for Docker. It’s a command line tool that builds Docker images based on their Dockerfile and a build.yml. 



### Key features:
__________________________________________________________________
- Use **build.yml** to build the image
- Supports **multi-tenancy** of builds by using `uid` parameters
- Allows to run **arbitrary commands** inside of the build container
- Allows **dovetailing (sequencing)** of the images from different steps
- After build, Habitus will run **Cleanup command**. This will result in 'squashing' the image, therefore removing any traces of unwanted layers
- Allows you to define and manage **secrets configuration** for your build
- Allows you specify any **Artifacts** - they'll be copied from the built image onto the work directory, so they'll be available for next steps.
- Support for non TLS connections

### Why Habitus? (Problem → solution)
______________________________________________________________________


- If you need to pull code from a private git repository:                           
  Your private SSH key will be needed in the image during the build process. By using Habitus, the web server only exposes your secrets to the internal Docker network of your machine, and only for the duration of the build. No traces of your secrets are left behind in the image.


- If you want to add compile-time libraries to your image, but don’t want them in the run-time:                      
   Habitus will solve it by using different build steps in building your artifact and only place that artifact in smallest possible image.


- If you want to combine multiple Dockerfiles into complex build and  deployments workflows:                             
  Take the example of an app written in Go: it lives in a container and serves content to visitors based on the latest trending hashtags on Twitter. To get this app into a container, you need to build it with Go compile time libraries. This makes the image large, increasing the attack surface of your service. Habitus solves this issue by compiling your Go app in one container with all compile-time dependencies, and then moves the compiled build artifacts to another smaller image with only the minimum packages required to run it.


- If you need to run images in production:                                
  Habitus helps you build a small, secure, performant, stable and immutable image in production. This will allow you to run non-leaky containers and healthy applications in production.

### Documentations:
_________________________________________________________________________________________________________

Comprehensive documentation is available on the Habitus website:

http://www.habitus.io/



### Quick Start: 
________________________________________________________________________________________________________

First step is to download Habitus.
Build files can be made up of multiple steps. Each step is independent of the other ones and downstream steps can use upstream ones as source (in `FROM` command). When habitus is installed, create a simple **build.yml** with just one build step and run Habitus. 

    build:
      version: 2016-03-14 # version of the build schema.
      steps:
        builder:
          name: builder
          dockerfile: Dockerfile

Run habitus

    # habitus

And you’re ready to start using Habitus.  Comprehensive documentation about build.yml is available on the Habitus website: http://www.habitus.io/

### Developing Habitus:
________________________________________________________________________________________________________

If you wish to work on the Habitus project itself. We provided a docker-compose.yml to spin up the development environment and link the source into the running container.


    # docker-compose run habitus /bin/bash
    # go build

You can run tests by typing 


    # docker-compose run habitus /bin/bash
    # go test

And you are ready to start your contribution to Habitus. 

#### Difference with Windows or OS or Linux
______________________________________________________________________________________________________
- On a Linux machine where Docker can run natively you can bind Habitus to 127.0.0.1.
- On a Mac (OSX) Docker runs inside of a VM (VirtualBox in most cases through Boot2Docker). This means you need to find the VM address of your Mac and use that to bind Habitus to. By default, Boot2Docker (and Docker Machine) use 192.168.99.1 which is what Habitus uses by default.
