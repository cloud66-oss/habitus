# CHANGELOG

## V1.0.0-pre.1

- containers containing artefacts don't require `stat` anymore (fix #8)
- you can specify for each step if you want to use cache `no_cache: true` (feature #9)
- after a build step, you can run an arbitrary command on the host  `after_build_command: <command>` (feature #19)

example `no_cache` feature: https://github.com/cloud66/habitus/tree/master/examples/no_cache
example `after_build_command` feature: https://github.com/cloud66/habitus/tree/master/examples/after_build_command

NOTE: If you want to use the `no_cache` feature you must enable this for security reasons on the command line:

`habitus --after-build-command=true ...`\



 




