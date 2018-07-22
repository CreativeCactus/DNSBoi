# DNS Boi

DNSBoi is a tool written in Golang for managing your CoreDNS configuration.

CoreDNS supports a hotreloading config file using the `auto` plugin. Those just moving into microservices might like some kind of push pattern for adding DNS records without the overhead of something like EtcD (which you should probably use if you want to scale up).

DNSBoi doesn't lock you into any patterns, and he is intended for production use (though you'd best give the code a once over and ensure that you have your [logging & alerts set up with something like grafana](https://github.com/CreativeCactus/logstack)).

## How he works

DNSBoi will hang out with your CoreDNS instance(s) and listen for HTTP requests. When he gets one, he will add it to his list of friends and call them back periodically (on /health) to make sure they are still down to hang out. He also let's CoreDNS know (via `auto`) who is cool and removes those services who became uncool at a configurable interval.

## Status

DNSBoi (like most things you want to use on GitHub) is under heavy development, but the roadmap isn't very long at all. Some more work with CoreDNS zone files is needed, but after some tests and examples, there isn't much left to do.

## Dev

Example app could use some work, as he is not super stable and sometimes misinterprets the situation. He should wait a while after declaring success in reaching DNSBoi/register before trying to get at CoreDNS.

DNSBoi needs to write his zonefiles better.

Docker workflow could be improved, maybe a `Makefile`? BTW you want `docker-compose up --force-recreate --build` for hacking.

Add option to set host IP for services talking to DNSBoi, as they might be reachable via other IPs than the one they use to call him up.