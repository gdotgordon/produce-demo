# produce-demo [![Build Status](https://travis-ci.org/gdotgordon/produce-demo.svg?branch=master)](https://travis-ci.org/gdotgordon/produce-demo)
REST service for supermarket produce inventory

## Introduction and Overview
The solution presented here implements the requirements of the Produce Demo via a single-service that runs in a Docker container, with an ephemeral port exposed outside the container for accessing the service.

The service implements operations for adding, deleting and listing produce items, as described in the spec.  Adding both multiple and single produce items are supported through a single endpoint, as the endpoint can unmarshal either an array or single produce item.  The mock database used is a hash map guarded by a sync.RWMutex, which seems like the appropriate semantic for a database.

An add request containing multiple items handles each item in its own goroutine for maximal efficiency.  Likewise, all deletes and list requests are launched in a separate goroutine.  Such an architecture might be useful in cases where an actual database may take some time, and the invoking goroutine could do some other work, such as reporting status back while waiting. In our case, the latency is minimal, gated only by the RW Mutex.

## Accessing and running the demo
The project is hosted at the Git repository https://github.com/gdotgordon/produce-demo.  The project uses a Travis-CI pipeline that runs go checks, unit tests, and a functional test.  To minimally run the server, there is a `docker-compose.yml`file which should be used to run the demo.

- To start the server, run `docker-compose up` or `docker-compose up -d` to run in detached mode.  In the latter case, you can use a tool like *Kitematic* or use `docker logs` with the container name as such:

```
$ docker  container ls
CONTAINER ID        IMAGE                         COMMAND                  CREATED             STATUS              PORTS                     NAMES
50a6fd13375e        gagordon12/produce-demo:1.0   "./produce-demo /bin…"   2 minutes ago       Up 2 minutes        0.0.0.0:32872->8080/tcp   produce-demo
$ docker logs produce-demo
{"level":"info","ts":1554931141.186856,"caller":"produce-demo/main.go:89","msg":"Listening for connections","port":8080}
$
```

Notice that running the above command (or better, `docker ps`) shows you the ephemeral porrt number outside the container, in this case, port 32872.  The integration test finds this port number, and also depends on the name of the image to be *produce-demo*, which is assured by running it through docker-compose.

- To stop the server, simply type `docker-compose down` or `docker-compose down --rm all` (to remove the image).

Note `docker-compose` will pull the image for you from Docker hub, but for reference, the version of the image to use is **gagordon12/produce-demo:1.0**.

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

- The code is four sets of alphanumeric [A-Za-z0-9] of length 4, separated by hyphens
- The name may be any alphanumeric (including unicode), but the leading character may not be a space
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

### Add
endpoint: POST to /v1/produce
payload: JSON for a single Produce item as shown above, or an array of Produce items
HTTP return codes:
- 200 (OK) for requests with multiple produce items, but mixed results storing them
- 201 (Created) for a successful update for single item or array
- 400 (Bad Request) if the request is non-conformant to the JSON unmarshal or contains invalid field values
- 409 (Conflict) if the item already exists in the database
- 500 (Internal Server Error) typically won't happen unless there is a system failure

The 200 status above merits further discussion.  A typical REST POST end point will create a single resource, but here, we're allowed to create multiple ones.  There are several solutions proposed to this, none of which is perfect, as this is arguably not a perfect REST use case.

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
of interpretation, I suppose, but I am documenting my take on it.
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
