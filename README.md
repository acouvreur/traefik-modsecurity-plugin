# Traefik Modsecurity Plugin

![Demo](./img/owasp-modsec.png)

Traefik plugin to proxy requests to [owasp/modsecurity-crs](https://hub.docker.com/r/owasp/modsecurity-crs):apache

![Github Actions](https://img.shields.io/github/workflow/status/acouvreur/traefik-modsecurity-plugin/Build?style=flat-square)
![Go Report](https://goreportcard.com/badge/github.com/acouvreur/traefik-modsecurity-plugin?style=flat-square)
![Go Version](https://img.shields.io/github/go-mod/go-version/acouvreur/traefik-modsecurity-plugin?style=flat-square)
![Latest Release](https://img.shields.io/github/release/acouvreur/traefik-modsecurity-plugin/all.svg?style=flat-square)


- [Traefik Modsecurity Plugin](#traefik-modsecurity-plugin)
  - [Demo](#demo)
  - [Usage (docker-compose.yml)](#usage-docker-composeyml)
  - [How it works](#how-it-works)
  - [Local development (docker-compose.local.yml)](#local-development-docker-composelocalyml)

## Demo

Demo with WAF intercepting relative access in query param.

![Demo](./img/waf.gif)

## Usage (docker-compose.yml)

See [docker-compose.yml](docker-compose.yml)

1. docker-compose up
2. Go to http://localhost:8000/website, the request is received without warnings
3. Go to http://localhost:8000/website?test=../etc, the request is intercepted and returned with 403 Forbidden by owasp/modsecurity

## How it works

This is a very simple plugin that proxies the query to the owasp/modsecurity apache container.

The plugin checks that the response from the waf container hasn't an http code > 400 before forwarding the request to the real service.

If it is > 400, then the error page is returned instead.

The *dummy* service is created so the waf container forward the request to a service and respond with 200 OK all the time.


## Local development (docker-compose.local.yml)

See [docker-compose.local.yml](docker-compose.local.yml)

`docker-compose -f docker-compose.local.yml up` to load the local plugin