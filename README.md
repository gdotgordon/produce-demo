# produce-demo [![Build Status](https://travis-ci.org/gdotgordon/produce-demo.svg?branch=master)](https://travis-ci.org/gdotgordon/produce-demo)
REST service for supermarket produce inventory

## Introduction and Overview
The solution presented here implements the requirements of the Produce Demo via a single-service that runs in a Docker container, with an ephmeral port exposed outside the container for accessing the service.

The service implements operations for adding, deleting and listing produce items, as described in the spec.  Adding both mutliple and single produce items are supported through a single endpoint, as the endpoint can unmarshal either an array or single produce item.  The mock database used is a hash map guarded by a symc.RWMutex, which seems like the approriate semantic for a database.

An add request containing multiple items handles each item in its own goroutine for maximal efficiency.  Likewise, all deletes and list requests are launched in a separate goroutine.  Such an architecture might be useful in cases where an actual database may take some time, and the invoking goroutine could do some other work, such as reporting status back while waiting. In our case, the latency is minimal, gated only by the RW Mutex.

## Accessing and running the demo
The project is hosted at the Git repository https://github.com/gdotgordon/produce-demo.  The project uses a Travis-CI pipeline that runs go checks, unit tests, and a functional test.  To minimally run the server, there is a `docker-compose.yml`file which should be used to run the demo.  To start the server, run `docker-compose up` or `docker-compose up -d` to run in detached mode.  In the latter case, you can use a tool like *Kitematic* or use `docker logs` with the contianer name as such:
```
$ docker  container ls
CONTAINER ID        IMAGE                         COMMAND                  CREATED             STATUS              PORTS                     NAMES
50a6fd13375e        gagordon12/produce-demo:1.0   "./produce-demo /bin…"   2 minutes ago       Up 2 minutes        0.0.0.0:32872->8080/tcp   produce-demo_produce-demo_1
$ docker logs produce-demo_produce-demo_1
{"level":"info","ts":1554931141.186856,"caller":"produce-demo/main.go:89","msg":"Listening for connections","port":8080}
$ 
```

More on container names later.  But notice that running the above command (or better, `docker ps`) shows you the ephemeral porrt number outside the container, in this case, port 32872.

To stop the demo, simply type `docker-compose down` or `docker-compose down --rm all` (to remove the image).

Note `docker-compose` will download the image for you from Docker hub, but for reference, the version of the image to use is **gagordon12/produce-demo:1.0**.
