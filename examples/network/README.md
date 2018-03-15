Run this example using network

```console
docker run --name mynginx -d --network mynetwork nginx:latest

host=$(docker inspect mynginx | jq -r '.[0].NetworkSettings.Networks.mynetwork.IPAddress')

habitus --build host=$host --build port=80 --network mynetwork
```
