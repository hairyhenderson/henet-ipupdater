![Built for ARM][arm-arch-image]

# hairyhenderson/henet-ipupdater

A simple script to periodically update the correct IPv4 client IP for IPv6 tunnels from https://tunnelbroker.he.net.

It runs in a Docker container, and exits after a given delay. Docker's `--restart=always` can be used to ensure this runs in a continuous loop. 

## Usage

```console
$ docker run -d -e DELAY=720 -e USERNAME=foo -e APIKEY=bar --restart=always hairyhenderson/henet-ipupdater
```

## License

[The MIT License](http://opensource.org/licenses/MIT)

Copyright (c) 2016 Dave Henderson

[arm-arch-image]: https://img.shields.io/badge/built%20for-ARM-blue.svg?style=flat
