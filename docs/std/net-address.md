# `std/net/address`

Allocation-free Internet address and endpoint values.

- Families: `FAMILY_UNSPECIFIED`, `FAMILY_IPV4`, and `FAMILY_IPV6`.
- `IpAddress(family, word0, word1, word2, word3)` stores words in network byte
  order; IPv4 uses only `word0`.
- `Endpoint(address, port)` pairs an address with a host-order port.
- `unspecified()`, `ipv4(a,b,c,d)`, and `ipv6(w0,w1,w2,w3)` construct values.
- `anyIpv4(port)`, `loopbackIpv4(port)`, `anyIpv6(port)`, and
  `loopbackIpv6(port)` construct common endpoints.
- `parseIpv4`, `parseIpv6`, and `parse` validate textual addresses without
  allocating. IPv6 parsing supports one `::` compression run, but not an
  embedded dotted-decimal IPv4 tail or zone identifier.
- `IpAddress.isIpv4()`, `isIpv6()`, `equal(other)`, and
  `Endpoint.equal(other)` inspect and compare values.

There is currently no address-to-text formatter.
