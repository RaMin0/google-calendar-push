# Google Calendar Push

> Removes reminders from Google calendar placeholder events

## Storyline

**TL;DR:** To make my life easier, I wanted to be able to block slots on my work calendar that correspond to events on my personal calendar. Regardless of this causing duplicate events to show up on my combined calendar app, which I can tolerate, what drove me crazy was having duplicate reminders.

I wanted to have silent placeholder events. Whenever I created an event on my work calendar with the title "Busy", any default reminders should be stripped off it.

## Getting Started

### Prerequisites

* Go 1.18, but can theoretically work with older versions
* PostgreSQL 13.6, but can theoretically work with older versions

#### Google OAuth Client

1. Go to https://console.cloud.google.com/apis/credentials
1. Create a new project, if you don't have one already
1. Create a new "OAuth Client ID" credential
1. Note down the client ID and secret

### Dependencies

```sh
$ go mod download
```

### Usage

* Make sure you make your own copy of `.env`. Use [`.env-example`](.env-example) for reference

  | Environment Variable | Default        | Description |
  |----------------------|----------------|-------------|
  | `PORT`               | `3000`         | Port for the server to listen on
  | `LOG_ENVIRONMENT`    | `"production"` | Any of `"development"` or `"production"`
  | `DATABASE_URL`       | N/A            | _Required._ A PostgreSQL DSN<br />_Example:_ `postgres://user:password@localhost:5432/db?sslmode=disable`
  | `DATABASE_LOG_LEVEL` | `"none"`       | Any of `"none"`, `"error"`, `"warn"`, `"info"`, `"debug"`, or `"trace"`
  | `AUTH_CLIENT_ID`     | N/A            | _Required._ Check [Google OAuth Client](#google-oauth-client)
  | `AUTH_CLIENT_SECRET` | N/A            | _Required._ Check [Google OAuth Client](#google-oauth-client)

* Run the server
  ```sh
  $ go run ./cmd/server
  ```
  or simply
  ```sh
  $ make
  ```

## Development

> Requires [`gin`](https://github.com/codegangsta/gin) for hot-reloading

```sh
$ make dev
```

## Roadmap

- [ ] Add tests

## Contributing

1. Fork the project
1. Create your feature branch
    ```sh
    $ git checkout -b feature/add-magic
    ```
1. Commit your changes
    ```sh
    $ git commit -m 'Add some magic'
    ```
1. Push to the branch
    ```sh
    $ git push origin feature/add-magic
    ```
1. Open a pull request

## License

Distributed under the [MIT](https://choosealicense.com/licenses/mit) license. See [LICENSE.txt](LICENSE.txt) for more information.
