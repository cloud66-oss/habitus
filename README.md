
Habitus
=======

![Codeship Status for cloud66/habitus](https://codeship.com/projects/714284d0-e914-0133-1e5d-4eaa3299b296/status?branch=master)]

A Docker Build Flow Tool
------------------------

### <a href="#welcome-to-github-pages" id="welcome-to-github-pages" class="anchor"><span class="octicon octicon-link"></span></a>Welcome to Habitus

Habitus is a standalone Docker build flow tool. It is a command line tool that builds Docker images based on their `Dockerfile` and a `build.yml`.

#### Build files

Habitus uses a yml file as a descriptor for builds. Here is an example:

``` yaml
  
build:
  version: 2016-03-14 # version of the build schema.
  steps:
    builder:
      name: builder
      dockerfile: Dockerfile.builder
      secrets:
        id_rsa:
          type: file
          value: _env(HOME)/.ssh/my_private_key
      artifacts:
        - /go/src/github.com/cloud66/iron-mountain/iron-mountain
        - /go/src/github.com/cloud66/iron-mountain/config.json
        - /go/src/github.com/cloud66/iron-mountain/localhost.crt
        - /go/src/github.com/cloud66/iron-mountain/localhost.key
      cleanup:
        commands:
          - rm -rf /root/.ssh/
    deployment:
      name: ironmountain
      dockerfile: Dockerfile.deployment
      depends_on:
        - builder
    uploader:
      name: uploader
      dockerfile: Dockerfile.uploader
      depends_on:
        - ironmountain
      command: s3cmd --access_key=_env(ACCESS_KEY) --secret_key=_env(SECRET_KEY) put /app/iron-mountain s3://uploads.aws.com  
      
```

Build files can be made up of multiple steps. Each step is independent of the other ones and downstream steps can use upstream ones as source (in `FROM` command).

In the example above, there are three steps: `builder`, `deployment` and `uploader`. All steps work out of the same working directory. `dockerfile` states which Dockerfile is used to build this step.

Here is a list of all step elements:

-   `name:` Name of the generated image
-   `dockerfile:` Dockerfile used to build the step
-   `artifacts:` List of all files to be copied out of the image once it’s built. See below
-   `secrets`: List of all secrets available to the build process. See below
-   `cleanup:` List of all cleanup steps to run on the image once the step is finished. See below
-   `depends_on:`Lists all the steps this step depends on (and should be built prior to this step’s build)
-   `command:`A command that will run in the running container after it’s built

#### Artifacts

Artifacts are files to be copied outside of a build image in a step. This can be used when a step is build of a compiled language like Go or Java where the image requires build dependencies. The next step can then use the build step’s artifacts in a runtime dependency only image.

Each artefact has two parts: source and destination. Source is the path from within the image and destination where the file will be copied to on the “build server”. If destination is missing, the current folder will be used. Full path and file permissions of the source will be preserved during copy. So a file that comes from `/app/build/result/abc` of the image will go to `./app/build/result/abc` of the build server if no destination is set.

Here is an example:

` - /go/src/service/go-service`

or

`- /go/src/service/go-service:/tmp/go-service`

Artifacts are copied from the container and can be used with `ADD` or `COPY` commands in downstream steps. Habitus copies artefact file permissions as well.

Here is an example that uses an artefact generated in step `builder`

      
        FROM ubuntu
        ADD ./iron-mountain /app/iron-mountain
      
#### Secrets
Managing secrets during a Docker build is tricky. You have might need to clone a private git repository as part of your build process and will need to have your private SSH key in the image before that. But that's not a good idea. Here is why:

First of all, you will need to copy your private SSH key to your build context, moving it out of your home directory and leaving it exposed to accidental commits to your Git repo.

Secondly you will end up with the key in the image (unless you use the <code>cleanup</code> step with Habitus, see below). Which will be in the image forever.

Third issue is that you will end up sharing SSH keys even if you decided to take the risk and add SSH keys to the image but squash them later.

Habits can help you in this case by using `secrets`

Habitus has an internal web server. This webserver only serves to requests comming from inside of the building containers. Building containers can ask for "secrets" during build with a simple <code>wget</code> or <code>curl</code>. The secret is delivered to the build process which can be used and removed in the same layer leaving no trace in the image.

Here is an example:

```
# Optional steps to take care of git and Github protocol and server fingerprint issues
RUN git config --global url."git@github.com:".insteadOf "https://github.com/"
RUN ssh-keyscan -H github.com >> ~/.ssh/known_hosts

# using Secrets
ARG host
RUN wget -O ~/.ssh/id_rsa http://$host:8080/v1/secrets/file/id_rsa && chmod 0600 ~/.ssh/id_rsa && ssh -T git@github.com && rm ~/.ssh/id_rsa

```

This is a snippet from a sample Dockerfile that uses the <code>secrets</code> feature of Habitus.

First, we make sure all git requests to Github go through the git protocol rather than HTTP (this is optional)

Second, we are going to add Github's server fingerprint to our <code>known_hosts</code> file to avoid the "yes/no" prompt

Now we can use Habitus and pull our private SSH key as a secret. It will be stored in <code>~/.ssh/id_rsa</code> and the next commands will use it (<code>ssh -T git@github.com</code> is a test that it worked) and the last step (very important) is removing it from the image with no traces and no need to use <code>cleanup</code>.

You notice that here we are using the Build Arguments in the Dockerfile with <code>ARG</code>. This allows us to pass in the Habitus IP address into the container without saving it into the image. You can pass in Build Arguments into Habitus with <code>--build</code> parameter:

<p><kbd>$ habitus --build host=192.168.99.1</kbd></p>

##### Secret configuration

<p><b>IMPORTANT</b>: Secrets are supported in Habitus 0.4 onward and only with <code>build.yml</code> schema versions on or more recent than <code>2016-03-14</code>.</p>

<p>You can define as many secrets for your build. Each secret has 3 attributes:</p>

<ul>
  <li>Name: Name is what the build will refer to this secret as. In this example the name is <code>id_rsa</code></li>
  <li>Type: Currently only <code>file</code> is supported as <code>type</code> which means Habitus only supports secrets stored in a file.</li>
  <li>Value: Value depends on the <code>type</code>. For <code>file</code>, <code>value</code> is the path to the file holding the secret on the build host (ie. your laptop or build server).</li>
</ul>

<pre>
  <code>
secrets:
        id_rsa:
          type: file
          value: _env(HOME)/.ssh/my_private_key
  </code>
</pre>


<h5>Habitus IP address</h5>
<p>Finding the correct IP address to server Habitus API (and secrets) on can be tricky. You don't want to bind it to <code>0.0.0.0</code> since it will make Habitus and your secrets available to your entire local network (and possibly the internet if you're running a tunnel) during the build time.</p>
<p>On a Linux machine where Docker can run natively you can bind Habitus to <code>127.0.0.1</code>. However on a Mac (OSX) Docker runs inside of a VM (VirtualBox in most cases thorugh Boot2Docker). This means you need to find the VM address of your Mac and use that to bind Habitus to. By default, Boot2Docker (and Docker Machine) use <code>192.168.99.1</code> which is what Habitus uses by default.</p>
<p>You can find your VM address by the following command:</p>
<p><kbd>$ ifconfig -a</kbd></p>

<p>What you are looking for is the <code>inet</code> address of a <code>vboxnet</code> like the following:</p>

<pre>
  <code>
    vboxnet3: flags=8943<UP,BROADCAST,RUNNING,PROMISC,SIMPLEX,MULTICAST> mtu 1500
    	ether 0a:00:27:00:00:03
    	inet 192.168.99.1 netmask 0xffffff00 broadcast 192.168.99.255
  </code>
</pre>

<p>On a Mac, if your VM IP address is different from <code>192.168.99.1</code> you can configure Habitus using the <code>--binding</code> parameter:</p>

<p><kbd>$ habitus --binding 10.0.99.1</kbd></p>

<p>You can also change the Habitus API port from the default <code>8080</code> using the <code>--port</code> parameter.</p>


#### Cleanup

Cleanup is a step that runs after the build is finished for a step. At the moment, cleanup is limited to commands:

      
    cleanup:
      commands:
        - apt-get purge -y man  perl-modules vim-common vim-tiny libpython3.4-stdlib:amd64 python3.4-minimal xkb-data libx11-data eject python3 locales golang-go
        - apt-get clean autoclean
        - apt-get autoremove -y
        - rm -rf /var/lib/{apt,dpkg,cache,log}/

This runs the commands in the provided order on the image and then as a last step squashes the image to remove anything that’s been removed. This is particularly useful when it comes to private information like ssh private keys that need to be on the image during the build (to pull git repos for example) but can’t be published as part of the built image.

#### Image sequencing

Habitus allows dovetailing (sequencing) of images from different steps. This means a step can use the image built by the previous step as the source in its Dockerfile `FROM` command. This is done automatically if `FROM` command refers to an image name used by a previous step.

Habitus automatically parses the `FROM` image name and replaces it with the correct name when it is used in multi-tenanted setup. This enables multiple builds of the same build file to run in parallel with different session UIDs (see below).

Please note if you are using step A’s result in step B’s `FROM` statement, you need to make sure A is listed under `depends_on` attribute of B. Otherwise both A and B will be built in parallel.

#### Step dependencies

Steps can depend on each other. This can be specified by the `depends_on` attribute.

Steps can depend on one or more of the other steps. This will determine the build order for steps. Independent steps are built in parallel and according to the build order defined by dependencies.

#### Environment variables

Environment variables can be used in the build file with the `_env(VAR)` format:

      
    artifacts:
          - /go/src/go-service/_env(SERVICE_NAME)
      

This will be replaced before the build file is fed into the build engine. By default Habitus inherits all environment variables of its parent process. This can be overridden by passing environment variables into Habitus explicitly through the env command parameter:

<kbd>$ habitus -env SERVICE\_NAME=abc -env RAILS\_ENV=production</kbd>

In the example above, you can pass in AWS S3 key and secret like this:

<kbd>$ habitus -env ACCESS\_KEY=\(ACCESS_KEY -env SECRET_KEY=\)SECRET\_KEY</kbd>

#### Running commands

Habitus allows you to run an arbitary command inside of a built container. This can be useful in many cases like uploading the build artifacts to webserver, resetting your exception handling service after each build or starting your release process.

`command` attribute is optional. If present, the image is built and a container is started based on it to run the command.

`command` runs after the build, cleanup and copying of the artifacts are done.

An example to upload a build artefact to S3 can be like this

      
        FROM cloud66/uploader
        ADD ./iron-mountain /app/iron-mountain
      

`cloud66/uploader` is a simple Docker image that has [S3CMD] installed on it.

The Dockerfile here is a simple one that starts from `cloud66/uploader` and adds one of the build artifacts to the image so it can be uploaded to S3.

  [S3CMD]: http://s3tools.org/s3cmd
  
  #### Command line parameters

Habitus accepts the following command line parameters:

-   `f`: Path to the build file. If not specified, it will default to `build.yml` in the work directory.
-   `d`: Path to work directory. This is the path where Dockerfiles should exist for each step and the build happens. Defaults to the current directory.
-   `no-cache`: Don’t use docker build caching.
-   `suppress`: Suppress docker build output.
-   `rm`: Remove intermediate built images.
-   `force-rm`: Forcefully remove intermediate images.
-   `uid`: A unique ID used for a build session to allow multi-tenancy of Habitus
-   `level`: Logging level. Acceptable values: `debug`, `info`, `notice`, `warning`, `error` and critical. Defaults to `debug`
-   `host`: Address for Docker daemon to run the build. Defaults to the value of `$DOCKER_HOST`.
-   `certs`: Path of the key and cert files used to connect to the Docker daemon. Defaults to `$DOCKER_CERT_PATH`
-   `env`: Environment variables used in the build process. If not specified Habitus inherits all environment variables of the parent process.
-   `no-cleanup`: Don’t run cleanup commands. This can be used for debugging and removes the need to run as sudo
-   `force-rmi`: Forces removal of unwanted images after the build
-   `noprune-rmi`: Doesn’t prune unwanted images after the build
-   `build`: Dockerfile Build Arguments passed into the build process
-   `binding`: IP address to bind Habitus API to. Habitus API provides services like secrets to the building containers. Default is `192.168.99.1`
-   `port`: Port to server Habitus API on. Default is `8080`

#### Development Environment for Habitus

Habitus requires running in privileged more (sudo) so it can run the squash method (keeping file permissions across images). It also requires the following environment variables: `DOCKER_HOST` and `DOCKER_CERT_PATH`. These are usually available when Docker is running on a machine, but might not be available in sudo mode. To fix this, you can pass them into the app with commandline params:

<kbd>$ sudo habitus –host $DOCKER\_HOST –certs $DOCKER\_CERT\_PATH</kbd>

#### Dependencies

You would also need [gnu tar] to be available on the machine:

##### OSX

##### 

[Instructions for OSX]

#### Multi-tenancy for Habitus

Habitus supports multi-tenancy of builds by using a `uid` parameter.

All builds and images will be tagged with the `uid` for this unless a step name explicitly has a tag. In that case the tag is concatenated with the `-uid`.

  [gnu tar]: https://www.gnu.org/software/tar/
  [Instructions for OSX]: https://github.com/cloud66/habitus/blob/gh-pages/gnu-tar.md
  
#### Building Habitus using Habitus

If you want to contribute to Habitus. You can build Habitus using Habitus, run Habitus in the root directory of this repository. The latest version is generated (after tests) inside the `./artifacts/compiled` directory.

<kbd>$ sudo habitus –host $DOCKER\_HOST –certs $DOCKER\_CERT\_PATH</kbd>

To make sure you a have isolated development environment for contribution. You can use the `docker-compose` for developing, testing and compiling. 

<kbd>$ docker-compose run habitus</kbd>

Building habitus inside a docker container:

<kbd>root@xx:/usr/local/go/src/github.com/cloud66/habitus# go build</kbd>

Running the tests:

<kbd>root@xx:/usr/local/go/src/github.com/cloud66/habitus# go test</kbd>





