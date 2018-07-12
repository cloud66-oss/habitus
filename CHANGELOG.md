# CHANGELOG

## Not Released

- Fixing the test suite by adding the `github.com/sirupsen/logrus`
go dependency in the `Dockerfile.tester` (transitive dependency).

## V1.0.3

- Adding the keyword `context` for a step to allow specifying a custom
context directory. Adapted the `examples/context` example.

## V1.0.0

- containers are coping artifacts with the required `stat` inside a container.  if you want to disable this, use the `--use-stat=false` command parameter.
- when enabling the secret provider `env` with the command parameter `--secrets=true -=sec-providers="env"` all the env you want to inject need HABITUS_ prefix. For example if you expose the secret named HOME in the build.yml if should be called HABITUS_HOME in the host environment.

## V1.0.0-pre.1

- containers containing artifacts don't require `stat` anymore (fix [#8](https://github.com/cloud66-oss/habitus/issues/8))
- you can specify for each step if you want to use cache `no_cache: true` (feature [#9](https://github.com/cloud66-oss/habitus/issues/9))
- after a build step, you can run an arbitrary command on the host  `after_build_command: <command>` (feature [#19](https://github.com/cloud66-oss/habitus/issues/19))

example `no_cache` feature: https://github.com/cloud66-oss/habitus/tree/master/examples/no_cache
example `after_build_command` feature: https://github.com/cloud66-oss/habitus/tree/master/examples/after_build_command

NOTE: If you want to use the `after_build_command` feature you must enable this for security reasons on the command line:

`habitus --after-build-command=true ...`\

Download [V1.0.0-pre.1](https://github.com/cloud66-oss/habitus/releases/tag/1.0.0-pre.1)
