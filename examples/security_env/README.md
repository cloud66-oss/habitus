Run this example using secrets

`habitus -f examples/security_env/build.yml -d examples/security_env --secrets=true --authentication-secret-server=true --binding=[your ip]  --build habitus_host=[your ip]  --build habitus_port=8080 --build habitus_password=admin  --build habitus_user=habitus`

Make sure you set the EnvVar

`export HABITUS_HOME=my_secret`