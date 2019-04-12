# produce-demo [![Build Status](https://travis-ci.org/gdotgordon/produce-demo.svg?branch=master)](https://travis-ci.org/gdotgordon/produce-demo)
REST service for supermarket produce inventory

## Introduction and Overview
The solution presented here implements the requirements of the Produce Demo via a single-service that runs in a Docker container, with an ephemeral port exposed outside the container for accessing the service.

The service implements operations for adding, deleting and listing produce items, as described in the spec.  Adding both multiple and single produce items are supported through a single endpoint, as the endpoint can unmarshal either an array or single produce item.  The mock database used is a hash map guarded by a sync.RWMutex, which seems like the appropriate semantic for a database.

An add request containing multiple items handles each item in its own goroutine for maximal efficiency.  Likewise, all deletes and list requests are launched in a separate goroutine.  Such an architecture might be useful in cases where an actual database may take some time, and the invoking goroutine could do some other work, such as reporting status back while waiting. In our case, the latency is minimal, gated only by the RW Mutex.

As required by the spec, the database is initially seeded on startup by reading the four records from the seed.json file in the top-level directory of the repo.

## Accessing and running the demo
The project is hosted at the Git repository https://github.com/gdotgordon/produce-demo.  The project uses a Travis-CI pipeline that runs go checks, unit tests, and a functional test.  To run the server, there is a `docker-compose.yml`file to launch it.

- To start the server, run `docker-compose up` or `docker-compose up -d` to run in detached mode.  In the latter case, you can use a tool like *Kitematic* or use `docker logs` with the container name as such:

```
$ docker  container ls
CONTAINER ID        IMAGE                         COMMAND                  CREATED             STATUS              PORTS                     NAMES
50a6fd13375e        gagordon12/produce-demo:1.0   "./produce-demo /bin…"   2 minutes ago       Up 2 minutes        0.0.0.0:32872->8080/tcp   produce-demo
$ docker logs produce-demo
{"level":"info","ts":1554931141.186856,"caller":"produce-demo/main.go:89","msg":"Listening for connections","port":8080}
$
```

Notice that running the above command (or better, `docker ps`) shows you the ephemeral port number outside the container, in this case, port 32872.  The integration test finds this port number, and also depends on the name of the image to be *produce-demo*, which is assured by running it through docker-compose.

- To stop the server, simply type `docker-compose down` or `docker-compose down --rm all` (to remove the image).

Note `docker-compose` will pull the image for you from Docker hub, but for reference, the version of the image to use is **gagordon12/produce-demo:1.0**.

To summarize, here are the steps:
1. `docker-compose up`
2. `docker ps` to find the ephemeral port to connect to the server, e.g "0.0.0.0:32874" means you can use "localhost:32874"
3. Use a tool like Postman to invoke the endpoints.
4. `docker-compose down`

### Tests
To run the unit tests, you don't need the container running, just run `go test -race ./...` from the top-level directory.

The tests are quite comprehensive, and I used the "table-driven" approach to writing tests where possible.

There is also an integration test under *tests/integration* that focuses heavily on concurrent execution.  You can run that from the root directory by invoking: `go test -tags=integration -v -race -count=1 ./tests/integration`.  Note you should restart the server immediately before running the test, or else you'll get a warning about the database not being in the required state.  This test runs outside the container, and looks for the ephemeral port by searching for the container named "produce-demo".  If you've started the container through `docker-compose`, this should work fine.

## Key Items and Artifacts and How To Run the Produce Service

The main item of interest is the `types.Produce` item and it's corresponding JSON, which we'll look at later.

```
type Produce struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	UnitPrice USD    `json:"unit_price"`
}
```

This item obviously represents the produce items worked with by the system.  The items must follow the syntactical rules as laid out in the assignment.

- The code is four sets of alphanumeric [A-Za-z0-9] of length 4, separated by hyphens.
- The name may be any alphanumeric (including unicode), but the leading character may not be a space.
- The USD items represents dollars, and may or may not have a dollar sign, and up to two decimal places.

That said, the items are converted (if necessary) to "canonical form" and stored in the database as follows:
- The code has all alphanumerics converted to upper case
- The name has leading word characters in upper case, all other lower, so for example `"grEen pePper"` is stored as `"Green Pepper"`
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
Three operations are supported, Add (one or more) produce, delete a produce item, or list all produce.

Note, there are also endpoints to check liveness (/v1/status) and clear the database (v1/reset).

### Add
endpoint: POST to /v1/produce

payload: JSON for a single Produce item as shown above, or an array of Produce items

HTTP return codes:
- 200 (OK) for requests with multiple produce items, but mixed results storing them (detailed discussion below)
- 201 (Created) for a successful update for single item or an array where all updates succeeded
- 400 (Bad Request) if the request is non-conformant to the JSON unmarshal or contains invalid field values
- 409 (Conflict) if the item already exists in the database
- 500 (Internal Server Error) typically won't happen unless there is a system failure

The 200 status above merits further discussion.  A typical REST POST endpoint will create a single resource, but here, we're allowed to create multiple ones.  There are several solutions proposed to this in the literature, none of which is perfect, as this is arguably not a perfect REST use case.

If a request with multiple payloads has all it's payloads uploaded successfully, then a 201 is returned.  But if one or more items fail, we return a 200 with an additional JSON payload sent that shows the individual results for each item, as per the codes listed above.  For example, consider this payload that yields to a response for a partially successful POST:

```
[
    	{
	  "code": "B12T-4GH7-QPL9-3N4M",
    	  "name": "Peas",
    	  "unit_price": "$3.46"
    	},
    	{
	  "code": "dvE56-9UI3-TH15-QR88",
    	  "name": "mince tart",
    	  "unit_price": "$2.9"
    	},
    	{
	  "code": "YRT6-72AS-K736-L4AR",
    	  "name": "-Green pepper",
    	  "unit_price": "$0.79"
    	},
    	{
	  "code": "B12T-4GH7-QPL9-3N4M",
    	  "name": "Peas",
    	  "unit_price": "$3.46"
    	}
]
```

Note that two of the items are invalid (one bad code, one bad name, plus we've sent the same item twice.  The response is:
```
[
    {
        "code": "B12T-4GH7-QPL9-3N4M",
        "status_code": 201
    },
    {
        "code": "dvE56-9UI3-TH15-QR88",
        "status_code": 400,
        "error": "invalid item format: invalid code: 'dvE56-9UI3-TH15-QR88'"
    },
    {
        "code": "YRT6-72AS-K736-L4AR",
        "status_code": 400,
        "error": "invalid item format: invalid name: '-Green pepper'"
    },
    {
        "code": "B12T-4GH7-QPL9-3N4M",
        "status_code": 409,
        "error": "produce code 'B12T-4GH7-QPL9-3N4M' already exists"
    },
]
```

### List Items
endpoint: GET to /v1/produce

payload: none

returns: a JSON array of all the items in the database.  Note it is legitimate to get a response with zero items, and this is not deemed an error.  It *would* be an error if a specific resource were requested, but not found.  This is a matter
of interpretation, I suppose, but I am documenting my take on it and justification.

HTTP return codes:
- 200 (OK) list successfully returned
- 500 (Internal Server Error) typically won't happen unless there is a system failure

Sample response:
```
[
    {
        "code": "TQ4C-VV6T-75ZX-1RMR",
        "name": "Gala Apple",
        "unit_price": "$3.59"
    },
    {
        "code": "A12T-4GH7-QPL9-3N4M",
        "name": "Lettuce",
        "unit_price": "$3.46"
    },
    {
        "code": "E5T6-9UI3-TH15-QR88",
        "name": "Peach",
        "unit_price": "$2.99"
    },
    {
        "code": "YRT6-72AS-K736-L4AR",
        "name": "Green Pepper",
        "unit_price": "$0.79"
    },
    {
        "code": "B12T-4GH7-QPL9-3N4M",
        "name": "Peas",
        "unit_price": "$3.46"
    }
]
```

### Delete Items
endpoint: DELETE to /v1/produce/{produce code} example: /v1/produce/YRT6-72AS-K736-L4AR

payload: none

HTTP return codes:
- 204 No Content if successfully deleted
- 400 Bad Request if request is syntactically invalid
- 404 Not Found if produce code is not in database
- 500 (Internal Server Error) typically won't happen unless there is a system failure

Note: another approach would be to include the Produce code as a query parameter, but in REST, it is common to have the resource itself be part of the actual URL, where the query parameters are more for modifiers.

## Architecture and Code Layout
The code has a main package which starts the HTTP server.  This package creates a signal handler which is tied to a context cancel function.  This allows for clean shutdown.  The main code creates a *service* object, which is a wrapper around the store package, which is the mock database.  This service is then passed to the *api* layer, for use with the mux'ed incoming requests.

Here is a more-specific roadmap of the packages:

### *types* package
Contains the definitions for the Produce item, the USD custom data type and the Request and Response Objects for the various REST invocations.

### *api* package
Contains the HTTP handlers for the various endpoints.  Primary responsibility is to unmarshal incoming requests, convert them to Go objects, and pass them off to the service layer, get the responses back from the service layer, convert any errors (or not) to appropriate HTTP status codes and send them back to the HTTP layer.

### *service* package
Takes the request Go object (if any), does semantic checks for correctness (e.g. valid Produce Code format), and launches goroutines that talk to the storage layer, gets the results back, and passes any errors or return objects back to the api layer for conversion to an HTTP response.  The service implements the *Service* interface, but the ProduceService is returned not masked in an interface, as it is not an object which is meant to be replaced.  The presence of the interface facilitates creating mocks for testing.

### *storage* package
Implements the store using a hash map.  The is no ordering to the objects when retrieved, which is reasonable, as any application could choose to sort the results based on name, code, price, whatever.  Note there is a *ProduceStore* interface, and `New()` returns a concrete implementation, which is hidden from the caller.  This  facilitates swapping in a real database without changing the code.  So the reason for using an interface here is somewhat different than the service package.

## A Note on Contexts
If you look at the API, you'll note that I've pretty much followed the rule of passing the context.Context with the cancel on signal around as the first parameter.  The intent is to not have goroutines lock up and allow for a clean shutdown.  Typically, I like to listen for `ctx.Done()` in a select statement, or depend on a layer I call to handle the cancel appropriately.  In the current program, the goroutines that communicate with the store pass this context to the store on every call.  A real database that is well-written would honor those cancels.  Here, however, the calls to the store only block for however long it takes to get the RW Mutex, which is minimal.  So in summary, the service invokes the store with a blocking call that returns quickly, and given the API is not channel-based, it is not possible to select on both the context and a response form the server - to solve this would require a more sophisticated mechanism that seems beyond the scope of this project.  On the other hand, passing the context off to the store and asking it to not lock up if a context cancel occurs is a reasonable expectation.
