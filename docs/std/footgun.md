# `std/footgun`

`drop[T](x $T) void` consumes an owned value without running its destructor. It
exists for rare cases where ownership moved to an API the analyzer cannot model.
This can leak resources; prefer the value's destructor unless another component
has definitely assumed cleanup responsibility.
