# `std/base64`

Canonical padded Base64 using the standard or URL-safe alphabet.

```magma
text := try base64.encode(a, bytes)
decoded := try base64.decode(a, text)
urlText := try base64.encodeUrl(a, bytes)
```

Buffer APIs are `encodeTo`, `decodeTo`, `encodeUrlTo`, and `decodeUrlTo`.
Decoding is strict: input length must be a multiple of four, padding must be in
the final quartet, and unused trailing bits must be zero. Whitespace and
unpadded input are not silently accepted.

`encodedSize(inputSize)` returns the required output size for either alphabet.
`decodedSize(text)` and `decodedUrlSize(text)` validate their respective
alphabets and return the decoded byte count. `decodeUrl(a, text)` is the
allocated URL-safe counterpart to `decodeUrlTo`.
