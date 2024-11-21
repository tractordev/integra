# Integra

## Building

With Go installed, you should be able to run:

```
make build
```

This will build the Integra executable and put it at `./local/integra`. You can
put it into your PATH or run it from there. To match examples below, you should
move it to a PATH directory like `/usr/local/bin`.

## Working Services

This project is still experimental, piloting with these services:

- github
- digitalocean
- spotify
- google-calendar

There is also `google-keep`, but it can only be described/inspected. Using this API is
only available to Google enterprise users so it is not yet supported.

## Authentication

The `integra call` and `integra fetch` commands will need authentication. This is
still a mostly manual process for each service that involves getting an access token
and setting it as an environment variable before using these commands.

#### github

[Create a personal access token](https://github.com/settings/tokens) with all scopes
and set it in your environment as `GITHUB_TOKEN`.

### digitalocean

[Create a personal access token](https://cloud.digitalocean.com/account/api/tokens) with all scopes
and set it in your environment as `DIGITALOCEAN_TOKEN`.

#### spotify

<details>
<summary>If you don't have OAuth client credentials...</summary>

you need to [make an app](https://developer.spotify.com/dashboard)
in the Spotify Developer Dashboard. It should be set up for "Web API" and "Web Playback API". It should
also have a Redirect URI of `http://localhost:4532/auth/callback`. You want the Client ID and Client Secret
from the app settings once created.

</details>

With Spotify OAuth client credentials, set them in your environment as `SPOTIFY_CLIENT_ID`
and `SPOTIFY_CLIENT_SECRET`. Now run:

```
integra auth spotify
```

It should open your browser to login and authorize, then redirect to a page you can close.
The output of the command should contain an access token valid for 1 hour that you can
set in your environment as `SPOTIFY_TOKEN`.

#### google-calendar

<details>
<summary>If you don't have a Google client credentials JSON file...</summary>

you need to [create a project](https://console.cloud.google.com/projectcreate) on the Google API Console. Enable the "Google Calendar API" for the project by
searching for it in the [API Library](https://console.cloud.google.com/apis/library)
making sure the new project selected in the top bar. Click the result and then the "Enable" button. Now
[create an OAuth client ID](https://console.cloud.google.com/apis/credentials/oauthclient)
with Application Type of "Web application" and an Authorized redirect URI of
`http://localhost:4532/auth/callback`. Download the `client_secret.json` file from the API Console.

There are expanded instructions [here](https://developers.google.com/identity/protocols/oauth2/web-server#enable-apis).

</details>

With a credentials JSON file, open it with a text editor and copy the single line contents into the clipboard. Then
set it in your environment as `GOOGLE_CLIENT_JSON` using single quotes, like this:

```
export GOOGLE_CLIENT_JSON='<json data here>'
```

Now you can run:

```
integra auth google-calendar
```

It should open your browser to login and authorize, then redirect to a page you can close.
The output of the command should contain an access token valid for 1 hour that you can
set in your environment as `GOOGLE_CALENDAR_TOKEN`.

## Using Integra Commands

Integra commands often take a selector in this format: `<service>.<resource>.<operation>`.
The resource and operation parts are both optional, so a selector could just be a
service name. To see available services run `integra describe` without a selector.

### Describe

The `integra describe <selector>` subcommand can take a selector and outputs information about the
selected service, resource, or operation. When describing a service, this includes
the available resource names (often grouped into categories). When describing a
resource, this includes the available operation names.

### Call

The `integra call <selector> [data...]` subcommand will perform an operation by selector. After the selector
you can optionally provide parameters and input data using [CLON syntax](https://github.com/progrium/clon-spec).
In the simple case this is just `key=value` arguments.

This command requires access tokens to be present in the environment for the
selected service.

### Fetch

The `integra fetch <service> <directory>` subcommand will attempt a one-way sync of data from the
service API to the specified directory. This command is a work in progress, so it
likely won't sync everything, and the organizational structure may change. However,
it's important to know it will only fetch data for _subjective_ endpoints,
that is, endpoints that return data specific to the authenticated user. Some APIs like
`digitalocean`, this is every endpoint.

This command requires access tokens to be present in the environment for the
selected service.

## Development

### Adding Services

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

| Key         | Type                   | Description                                                                                |
| ----------- | ---------------------- | ------------------------------------------------------------------------------------------ |
| latest      | string                 | Required. The latest/default version directory to use.                                     |
| dataScope   | string                 | Either "mixed" or "account". "account" means all resources are user-data. Default: "mixed" |
| accountData | list of regexp strings | Matched paths are marked as "account" data scope (user-data)                               |

Once all this is set up, the service should be available to `integra describe` after rebuilding. Here is what you can
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
