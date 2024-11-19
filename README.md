# Integra

## Building
With Go installed, you should be able to run:
```
make build
```
This will build the Integra executable and put it at `./local/integra`. You can
copy it into your PATH or run it from there. 

## Adding Services

Integra supports [OpenAPI Descriptions](https://learn.openapis.org/specification/) or [Google Discovery Documents](https://developers.google.com/discovery/v1/reference/apis). If the API and provider are the same, for example `digitalocean`, you can make a directory under `services`. If a provider has multiple APIs, make a directory for the provider, like `google`, and a subdirectory for the API, `calendar`, which would make a service named `google-caledar`. In either case, the service directory needs a `meta.yaml` file and a directory for specific versions of the API description. This directory is named by the major version number of the API, so an API with version `1.0` would be `1`. Integra expects either an `openapi.yaml` file or a `googleapi.json` file in this directory.

This would end up looking something like this:
```
services
├── digitalocean
│   ├── 2
│   │   └── openapi.yaml
│   └── meta.yaml
├── google
│   └── calendar
│       ├── 3
│       │   └── googleapi.json
│       └── meta.yaml
└── etc...
```
The minimal content of `meta.yaml` is a `latest` key with the major version directory
name as a string value. This file will be used to add extra metadata to services. Here
is the current data of `meta.yaml`:

| Key      | Type    | Description |
| -------- | ------- | ------- |
| latest  | string    | Required. The latest/default version directory to use. |
| dataScope | string | Either "mixed" or "account". "account" means all resources are user-data. Default: "mixed" |
| accountData | list of regexp strings | Matched paths are marked as "account" data scope (user-data) |

Once all this is set up, the service should be available to `integra describe`. Here is what you can 
run to make sure everything looks right:

#### Check service info
```
integra describe --info <service>
```
Ideally all fields are non-empty.

#### Check service resources
```
integra describe --resources <service>
```
For OpenAPI, Integra tries to infer resources from paths. This is where most massaging happens.
Before the listing output, it should log any issues preventing it from inferring resources
from paths. I dump these into a `NOTES` file for the service where we can work out
the best way to improve our inference system. Even if we end up using service specific
techniques, we're trying to make sure any API added will have reasonable resources by default.

Outside of logged issues, the resources need to be inspected to make sure they look right.
Resource names are singular, but some words need to be marked as invariant or as an acronym.
Acronyms should be uppercased and not singularized. Any other weird looking resource names
should have a GitHub issue filed.

#### Check each resource
```
integra describe <service>.<resource>
```
This may output the same errors as before, but any unique errors should be reported. Otherwise
it should should non-empty fields under info, and a list of operations.

#### Check each operation
```
integra describe <service>.<resource>.<operation>
```
Same as before, you may see familiar errors, and new errors should be reported. Info
fields should look correct. After the Info section the sections will be different
depending on the operation, but all involve summarizing the top-level schema of the
data used in the operation. First, there may be a "parameters" section for query
and URL parameters. Then, there may be an "input" section showing the top-level
schema for data sent as request body. Lastly, there should either be an "output"
section or a "response" then "output" section. Output focuses on the principal data
type expected to be returned. If response is shown, it is the schema for the envelope
of the output. For example, pagination data in list operations.

Descriptions in the schemas aren't required, but type information is. If the type
is blank for a field, double check the service document has this information, or if
Integra is not parsing properly. For now, only the latter should be an issue.