# `std/net/dns`

Thread-safe DNS resolver with bounded positive and negative caching.

`defaultOptions()` selects 256 cache entries, a 60-second positive TTL, a
5-second negative TTL, and at most 16 results per lookup. `new(a, options)`
requires nonzero `capacity` and `maxResults`; the resolver owns cache storage
allocated with `a`.

- `resolveTo(host, service, family, output)` writes into caller storage and
  returns the count. It fails if the complete result does not fit.
- `resolve(host, service, family)` returns an owned endpoint slice allocated
  with the resolver's allocator. Free it with `slices.free` and that allocator.
- `clear()` evicts all positive and negative entries.
- `close()` clears the cache, frees storage, and closes its mutex.

`family` is one of the address-family constants. `host` and `service` are
borrowed during a call and copied when cached. Concurrent cache misses may
perform duplicate native lookups; insertion remains synchronized.
