Run this example using Habitus: `habitus -f examples/buildarguments/build.yml -d examples/buildarguments --build BUILD_ARGUMENT=awesome --env NAME=wow`

Or with the alternative build.yaml contains the build argument: `habitus -f examples/buildarguments/build-contained.yml -d examples/buildarguments --env NAME=wow`

It will build the steps and create a dynamic step name called *step_wow* and set the environment variable in the Dockerfile called *AWESOME_ENVIRONMENT* to the *BUILD_ARGUMENT* which is awesome ;-)
