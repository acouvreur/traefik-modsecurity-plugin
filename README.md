# Traefik Modsecurity Plugin

![Demo](./img/owasp-modsec.png)

Traefik plugin to proxy requests to [owasp/modsecurity-crs](https://hub.docker.com/r/owasp/modsecurity-crs):apache

![Github Actions](https://img.shields.io/github/workflow/status/acouvreur/traefik-modsecurity-plugin/Build?style=flat-square)
![Go Report](https://goreportcard.com/badge/github.com/acouvreur/traefik-modsecurity-plugin?style=flat-square)
![Go Version](https://img.shields.io/github/go-mod/go-version/acouvreur/traefik-modsecurity-plugin?style=flat-square)
![Latest Release](https://img.shields.io/github/release/acouvreur/traefik-modsecurity-plugin/all.svg?style=flat-square)


- [Traefik Modsecurity Plugin](#traefik-modsecurity-plugin)
  - [Demo](#demo)
  - [Full Configuration with docker-compose](#full-configuration-with-docker-compose)
  - [How ?](#how-)

## Demo

Demo with WAF intercepting relative access in query param.

![Demo](./img/waf.gif)

## Full Configuration with docker-compose

```yml
version: "3.7"

services:
  traefik:
    image: traefik
    ports:
      - "8000:80"
      - "8080:8080"
    command:
      - --api.dashboard=true
      - --api.insecure=true
      - --pilot.token=$TRAEFIK_PILOT_TOKEN
      - --experimental.localPlugins.traefik-modsecurity-plugin.moduleName=github.com/acouvreur/traefik-modsecurity-plugin
      - --providers.docker=true
      - --entrypoints.http.address=:80
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
      - '.:/plugins-local/src/github.com/acouvreur/traefik-modsecurity-plugin'
    environment:
      - TRAEFIK_PILOT_TOKEN
    labels:
      - traefik.enable=true
      - traefik.http.services.traefik.loadbalancer.server.port=8080
      - traefik.http.middlewares.waf.plugin.traefik-modsecurity-plugin.modSecurityUrl=http://waf:80

  waf:
    image: owasp/modsecurity-crs:apache
    environment:
      - PARANOIA=1
      - ANOMALY_INBOUND=10
      - ANOMALY_OUTBOUND=5
      - BACKEND=http://dummy

  dummy:
    image: containous/whoami

  website:
    image: containous/whoami
    labels:
      - traefik.enable=true
      - traefik.http.routers.website.rule=PathPrefix(`/website`)
      - traefik.http.routers.website.middlewares=waf@docker
```

1. docker-compose up
2. Go to http://localhost:8000/website, the request is received without warnings
3. Go to http://localhost:8000/website?test=../etc, the request is intercepted and returned with 403 Forbidden by owasp/modsecurity

## How ?

This is a very simple plugin that proxies the query to the owasp/modsecurity apache container.

The plugin checks that the response from the waf container hasn't an http code > 400 before forwarding the request to the real service.

If it is > 400, then the error page is returned instead.

The *dummy* service is created so the waf container forward the request to a service and respond with 200 OK all the time.