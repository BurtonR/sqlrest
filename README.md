# sqlrest
 sqlrest is an API proxy for MS-SQL databases for easier database access in serverless functions 

# About
The idea came from the ~need~ want to turn full size APIs into a group of serverless functions. It was quickly discovered that the connections to a database take around 5-7 seconds. No good for a serverless function!

The thought is to create an ultra-minimalist API that acts as a proxy in front of a database in order to maintain the connection, as well as handle the pooling while the functions can focus on being functional.

The function would build the required SQL statment to be executed, and send it to SqlRest for execution on the remote DB. SqlRest does nothing more than passing the query (or procedure name) to the connected database server for execution. The response will depend on the route/command. Select/Query would return a json object with an array of column names and an array of arrays of row results

Example Results:
```
{
  "Columns": [
        "Column1",
        "Column2"
  ],
  "Data": [
      [
        "Result1_1",
        "Result1_2"
      ],
      [
        "Result2_1",
        "Result2_2"
      ]
  ]
}
```
Errors will be returned with an appropriate HTTP response code and a "message" property containing the reason of the error

Example Error Result:
```
{
  "message": "Query must contain at least 1 'SELECT' statement for 'Query' operation"
}
```
Also, errors encountered in the database will include an "error" property with the error being passed directly from the database
Example DB Error Response:
```
{
  "error": {
    "Number": 102,
    "State": 1,
    "Class": 15,
    "Message": "Incorrect syntax near 'Blah:'.",
    "ServerName": "efe87d4ca854",
    "ProcName": "",
    "LineNo": 1
  },
  "message": "Error returned from database"
}
```

## Examples

There are some examples of how to use SqlRest with various languages in the `examples` directory as well as a sample database

# Run with Docker
Pull the image from [Docker Hub](https://hub.docker.com):

    docker pull burtonr/sqlrest:0.2

## Required Environment variables
|Name | Value |
|-----|-------|
|DATABASE_USERNAME  | The username to log in to the SQL Server with |
|DATABASE_PASSWORD  | The password associated with the user |
|DATABASE_SERVER | The IP address, or hostname, of the SQL server. Do not include the instance, or port number, we got that covered for you (assuming 1433 (default)) :) |
|SQLREST_ALLOWED_REALMS | A comma separated list of strings to designate what service is permitted to access this instance of sqlrest |
|SQLREST_API_KEY | The shared secret key used to hash the request

Run the image with the following command (replacing the environment variables with your own)

    docker run -d -p 5050:5050 -e DATABASE_USERNAME=sa -e DATABASE_PASSWORD=secretSauce2! -e DATABASE_SERVER=172.17.0.2 -e SQLREST_ALLOWED_REALMS=testing-func,qa-func -e SQLREST_API_KEY=sqlrestTestKey --name sqlrest burtonr/sqlrest:0.3


## Optional Environment variables
|Name | Value | Default |
|-----|-------|---------|
|DATABASE_NAME  | The name of the database to connect to | _blank_ (i.e. `master`) |


# Usage
The API exposes the following endpoints:
* `/ping`
* `/connect`
* `v1/procedure`
* `v1/query`
* `v1/insert`
* `v1/delete`
* `v1/update`

_(the `v1` is an example of the API version that will update as breaking changes happen)_

## Headers
Each request must include an `Authorization` header

The value of this header includes 4 parts separated by a colon ( `:` )

* Realm
  * This is the origination of the request. The accepted values are read from the `SQLREST_ALLOWED_REALMS` environment variable
* Signature
  * This is the hmac hash of the message + api key used to validate that the request was not altered e.g. _Hash(apikey + Hash(body + apikey))_
  * References here: [Wikipedia](https://en.wikipedia.org/wiki/HMAC), [RFC2104](https://tools.ietf.org/html/rfc2104), and [Acquia](https://github.com/acquia/http-hmac-spec)
* Nonce
  * Unique value per request to prevent replay attacks (may be removed to rely on timestamp only)
* Timestamp
  * This is the epoch time in milliseconds (time since Jan 1, 1970)

Example:
```
Authorization: testing-func:fb1ded9f15a6d8134b3db1640c21cff2b0b22860a1720c54e7fd4938ba46b7f2:bbd37ca7-f270-45ee-9e5c-fa4a5de59a30:1520527620822
```

### Connect
Sending a `GET` request to this endpoint forces the API to attempt to reconnect to the database using the environment variables provided.

There is a process that runs every 2 minutes to ping the database that will reconnect if it fails. Use this endpoint if you don't want to wait for that process to run

If it is already connected, it will return with a success, otherwise it will attempt the connection and return either a `200` or `500`

> Note: For this `GET` request, the `signature` section of the `Authorization` header is not evaluated and may be left blank

### Procedure
Send a `POST` request to this endpoint to execute a SQL stored procedure and optionally get the results back

* The request body **must** contain a `name` property.
  * You must include the full name of the procedure unless you've set the `DATABASE_NAME` env var. By default, this will run against the `master` database
* Optionally, also include a `parameters` object that includes the parameter name and value
* Additionally, you may also specify a flag `executeOnly` that, when false, returns the result set from the stored procedure

A full request (to execute with parameters and return the results) would look like this:

    {
      "name": "sales.dbo.sp_get_customers",
      "parameters": {
        "title": "scuba",
        "firstName": "Steve",
      },
      "executeOnly": false
    }
The `executeOnly` property defaults to **true** so that procedures will not return values unless explicitly requested

This endpoint builds the SQL command to be executed as a string. 

The above example will generate the following string to be sent to the SQL server:

    EXEC sales.dbo.sp_get_customers @title = "scuba", @firstName = "Steve"

### Query
Send a `POST` request to this endpoint to execute a SQL query and get the results back

There are some basic syntax checks. 
* There must be a field `query` in the request body
* There must be at least 1 `SELECT` command 
* _told you it was basic..._

The query passed in is sent to the database directly with no modifications. 

**Note:** SQL connects to the `master` database by default, so be sure to include a `USE` statement
> `USE Database_Name; SELECT 1 FROM Table_Name` 

_or_ use the full object name in the table definition

>`SELECT 1 FROM [Database_Name].[dbo].[Table_Name]`

You could also set the environment variable `DATABASE_NAME` to set a default database name. Note, that if the default schema is not `dbo`, you will need to include that in the query as well even with the database name being set.

See above for examples of error and success responses

### Insert
Send a `PUT` request to this endpoint to execure a SQL insert

There are some basic syntax checks.
* There must be a field `insert` in the request body
* There must be at least 1 `INSERT INTO` command

The function that handles executing inserts will first create a transaction, then execute the command. If there is an error, or something goes wrong (`panic`), the transaction will roll back.

The request and command passed in follow the same rules as the `Query` endpoint. Be sure to include the database name

No results are returned with this command. To get the inserted values, you will need to `Query` for them.

### Delete
_Not yet implemented_

### Update
Send a `POST` request to this endpoint to execute a SQL update

There are some basic syntax checks.
* There must be a field `update` in the request body
* There must be at least 1 `UPDATE` command
* There must be at least 1 `WHERE` clause
  * This is for your own protection!

The function that handles executing updates will first create a transaction, then execute the command. If there is an error, or something goes wrong (`panic`), the transaction will roll back. 

The request and command passed in follow the same rules as the `Query` endpoint. Be sure to include the database name

No results are returned with this command. To get the updated values, you will need to `Query`.


# Security
sqlrest uses HMAC authorization to validate the requests being sent. The `Authorization` header is used to send the validation criteria to sqlrest (see [Headers](#Headers))

The validation uses the following environment variables:
* `SQLREST_ALLOWED_REALMS`
  * This is a comma separated list of strings to designate what service is permitted to call sqlrest
* `SQLREST_API_KEY`
  * This is the shared secret key that is used to hash the request

___

## _Developer Notes_ 
## API
This (mostly) follows the familiar REST practices as well as special handling of requests based on the route.

* Versioning
    * The API routes will be versioned. `localhost:8080/v1/query`
    * The handler files will include a version if they are of a previous version
        * When a new version is developed, the previous file and the exported `func` will have the version appended to it
        * `queryHandler.go` -> `queryHandler.v1.go`
        * `ExecuteQuery()` -> `ExecuteQueryV1()`

## Validation
* There should be some way to know the requested SQL statement is valid SQL syntax before sending it to the database. This will help avoid unnecessary connections and return a useful error.
* Possible future implementation could look for potential SQL injection attacks (maybe)

## _Future_
## Security
* Look in to implementing HMAC security or possibly API keys
  * Something that's easily added to a serverless function, Docker container, or other API, but still secure
* Security may be required to limit access to certain functions (no deletes), or certain databases, possibly even certain tables

## Considerations
* Should the `Update` and `Insert` calls return the modified/created entry?
  * I like that from a uasability perspective so you don't need to make 2 calls
  * On the otherhand, what about performance? If I (caller) don't need the result, why wait for it?
