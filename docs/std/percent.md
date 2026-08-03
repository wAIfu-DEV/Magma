# `std/percent`

Percent encoding with explicit policies.

```magma
component := try percent.encode(a, value, percent.URI_COMPONENT)
segment := try percent.encode(a, value, percent.PATH_SEGMENT)
form := try percent.encode(a, value, percent.FORM)
bytes := try percent.decode(a, component)
```

`URI_COMPONENT` preserves only unreserved characters. `PATH_SEGMENT` also
preserves RFC path-segment punctuation but still escapes `/`. `FORM` encodes
spaces as `+`; use `decodeForm` for the inverse behavior.

Decoding returns bytes. UTF-8 validation is deliberately separate. Invalid or
truncated percent escapes are rejected.
