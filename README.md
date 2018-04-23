# Habitus
Habitus adds workflows to Docker build. This means you can create a chain of builds to generate your final Docker image based on a workflow. This is particularly useful if your code is in compiled languages like Java or Go or if you need to use secrets like SSH keys during the build.

- Website: http://www.habitus.io/
- [Download Habitus](https://github.com/cloud66/habitus/releases?utm_source=Githubdownload&utm_medium=GHDpage&utm_campaign=habitus)
- Slack Channel: ![Join Our Slack Community](https://communityinviter.com/apps/cloud66ers/cloud-66-community).
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

Just run the install script on macOS or Linux!

`curl -sSL https://raw.githubusercontent.com/cloud66/habitus/master/habitus_install.sh | bash`

Or [download](https://github.com/cloud66/habitus/releases?utm_source=Githubdownload&utm_medium=GHDpage&utm_campaign=habitus) Habitus straight from this repo. Habitus can run on Linux, Windows and macOS. Copy the Habitus application into `/usr/local/bin/habitus` and check if it has the executable flags, if not run `chmod a+x /usr/local/bin/habitus`

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

After the build, you can use Habitus to build itself using the following command in the running container:

    # ./habitus

You can run tests by typing 


    # docker-compose run habitus /bin/bash
    # go test    

And you are ready to start your contribution to Habitus. 

### CHANGELOG

Check the changelog [here](https://github.com/cloud66/habitus/blob/master/CHANGELOG.md)
