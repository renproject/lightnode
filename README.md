# `🚀 lightnode`

<<<<<<< Updated upstream
![](https://github.com/renproject/lightnode/workflows/go/badge.svg)
=======
![](https://github.com/renproject//workflows/go/badge.svg)
>>>>>>> Stashed changes
[![Coverage Status](https://coveralls.io/repos/github/renproject/lightnode/badge.svg?branch=master)](https://coveralls.io/github/renproject/lightnode?branch=master)

A node used for querying Darknodes using JSON-RPC 2.0 interfaces. Featuring query caching (for performance) as well as retrying for failed requests (for reliability).

Built with ❤ by Ren.

# Building a Docker image

```sh
docker build . --build-arg GITHUB_TOKEN={your github token} --tag lightnode
```

# Running a Docker image

Ensure you have a working local `.env` file for neccesary env vars set, then

```sh
docker run --env-file=.env --env DATABASE_URL=/lightnode/cache.sql --network host -v `pwd`/cache.sql:/lightnode/cache.sql lightnode
```
