mod main

use "std:net/address" address
use "std:net/socket" socket
use "std:net/tcp" tcp
use "std:net/udp" udp
use "std:net/dns" dns
use "std:heap" heap
use "std:net/poll" poll
use "std:net/event_loop" event_loop
use "std:net/listener" listener

main() void:
    endpoint := address.loopbackIpv4(8080)
    endpoint.equal(endpoint)
    address.parseIpv4("127.0.0.1")
    options := dns.defaultOptions()
..
