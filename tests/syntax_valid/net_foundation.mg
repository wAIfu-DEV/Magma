mod main

use "std:net/address" as address
use "std:net/socket" as socket
use "std:net/tcp" as tcp
use "std:net/udp" as udp
use "std:net/dns" as dns
use "std:heap" as heap
use "std:net/poll" as poll
use "std:net/event_loop" as event_loop
use "std:net/listener" as listener

main() void:
    endpoint := address.loopbackIpv4(8080)
    endpoint.equal(endpoint)
    address.parseIpv4("127.0.0.1")
    options := dns.defaultOptions()
..
