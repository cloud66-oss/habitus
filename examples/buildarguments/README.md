Run this example using Habitus: `habitus -f examples/buildarguments/build.yml -d examples/buildarguments --build BUILD_ARGUMENT=awesome --env NAME=wow`

It will build the steps and create a dynamic step name called *step_wow* and set the environment variable in the Dockerfile called *AWESOME_ENVIRONMENT* to the *BUILD_ARGUMENT* which is awesome ;-)
