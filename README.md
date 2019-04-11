# produce-demo [![Build Status](https://travis-ci.org/gdotgordon/produce-demo.svg?branch=master)](https://travis-ci.org/gdotgordon/produce-demo)
REST service for supermarket produce inventory

## Introduction and Overview
The solution presented here implements the requirements of the Produce Demo via a single-service that runs in a Docker container, with an ephmeral port exposed outside the container for accessing the service.

The service implements operations for adding, deleting and listing produce items, as described in the spec.  Adding both mutliple and single produce items are supported through a single endpoint, as the endpoint can unmarshal either an array or single produce item.  The mock database used is a hash map guarded by a sync.RWMutex, which seems like the approriate semantic for a database.

An add request containing multiple items handles each item in its own goroutine for maximal efficiency.  Likewise, all deletes and list requests are launched in a separate goroutine.  Such an architecture might be useful in cases where an actual database may take some time, and the invoking goroutine could do some other work, such as reporting status back while waiting. In our case, the latency is minimal, gated only by the RW Mutex.

## Accessing and running the demo
The project is hosted at the Git repository https://github.com/gdotgordon/produce-demo.  The project uses a Travis-CI pipeline that runs go checks, unit tests, and a functional test.  To minimally run the server, there is a `docker-compose.yml`file which should be used to run the demo.  To start the server, run `docker-compose up` or `docker-compose up -d` to run in detached mode.  In the latter case, you can use a tool like *Kitematic* or use `docker logs` with the contianer name as such:
```
$ docker  container ls
CONTAINER ID        IMAGE                         COMMAND                  CREATED             STATUS              PORTS                     NAMES
50a6fd13375e        gagordon12/produce-demo:1.0   "./produce-demo /bin…"   2 minutes ago       Up 2 minutes        0.0.0.0:32872->8080/tcp   produce-demo
$ docker logs produce-demo
{"level":"info","ts":1554931141.186856,"caller":"produce-demo/main.go:89","msg":"Listening for connections","port":8080}
$ 
```

Notice that running the above command (or better, `docker ps`) shows you the ephemeral porrt number outside the container, in this case, port 32872.  The integration test finds this port number, and also depends on the name of the image to be *produce-demo*, which is assured by running it through docker-compose.

To stop the demo, simply type `docker-compose down` or `docker-compose down --rm all` (to remove the image).

Note `docker-compose` will pull the image for you from Docker hub, but for reference, the version of the image to use is **gagordon12/produce-demo:1.0**.

## Key Items and Aritfacts and How To Run the Produce Service
The main item of interest is the `types.Produce` item and it's corresponding JSON, which we'll look at later.
```
type Produce struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	UnitPrice USD    `json:"unit_price"`
}
```

This item obviously represents the produce items worked with by the system.  The items must follow the syntactical rules as laid out in the assignment.

- The code is four sets of alphanumeric [A-Za-z0-9] of length 4, separated by hyphens
- The name may be any alphahnumeric (including unicode), but the leading character may not be a space
- The USD items represents dollars, and may or may not have a dollar sign, and up to two decimal places.

That said, the items are converted (if necessary) to "canonical form"and stored in the database as follows:
- The code has all alphanumerics converted to upper case
- The name has leading word characters in upper case, all other lower, so for example `"grEen pePper"` is stored as `"Green Pepper"
- The currency is as described above.  Pretty much any value is acceptable, even tenths only, as in "$3.4".  Note the currency is marshaled and unmarshaled with a custom JSON marshaler and unmarshaler, so all validation is complete by the time the currency is successfully marshalled.

The JSON for such an item would look as follows:
```
{
  "code": "YRT6-72AS-K736-L4AR",
  "name": "Green Pepper",
  "unit_price": "$0.79"
}
 ```
## The API
Three operations are supported, Add (one or more) produce, delete a produce item, or list all produce

# Add
endpoint: POST to /v1/produce
payload: JSON for a single Produce item as shown above, or an array of Produce items
HTTP return codes:
- 201 (Created) for a successful update for single item or array
- 400 (Bad Request) if the request is non-conformant to the JSON unmarshal or contains invalid field values
- 409 (Conflict) if the item already exists in the database
- 500 (Internal Server Error) typically won't happen unless there is a sysrtem failure
