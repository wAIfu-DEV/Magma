# `std/time`

## Example

```magma
start := time.ticks()
# work
elapsedMs := time.elapsedMs(start)
fiveMsInTicks := time.msToTicks(5)
```

Monotonic timing, Unix wall-clock timestamps, and tick conversions. Tick frequency is platform-defined and handled internally.

## Clocks

- `pub ticks() u64` returns a monotonic tick counter suitable for elapsed-time measurement.
- `pub runtime() f64` converts the current monotonic counter to seconds. Its
  origin is platform-defined; use differences, not the absolute value.
- `pub processCpuTimeNs() u64` returns CPU time consumed by the process in
  nanoseconds, excluding blocked or sleeping time.
- `pub unixTimestamp() u64`, `unixTimestampMs() u64`, `unixTimestampUs() u128`, and `unixTimestampNs() u128` return wall-clock time since the Unix epoch at the named resolution.

## Conversion

- `pub ticksToSecFloat(ticks u64) f64` converts ticks to fractional seconds.
- `pub ticksToSec(ticks u64) u64`, `ticksToMs`, `ticksToUs`, and `ticksToNs` convert ticks to integer units.
- `pub secToTicks(sec u64) u64`, `msToTicks`, `usToTicks`, and `nsToTicks` convert integer units to ticks.
- `pub secFloatToTicks(sec f64) u64` converts fractional seconds to ticks.

## Elapsed time

- `pub elapsedTicks(startTicks u64) u64` subtracts a previous monotonic reading from the current one.
- `pub elapsedMs(startTicks u64) u64`, `elapsedUs`, `elapsedSec`, and `elapsedSecFloat` return elapsed time in the named unit.

Integer conversions truncate fractional units and may overflow for sufficiently large inputs.

`pub sleep(milliSecs u64) void` blocks the current thread for approximately the
requested number of milliseconds. Windows passes it to `Sleep`; Unix converts
milliseconds to microseconds for `usleep`. Very large Unix values can overflow
the backend's 32-bit microsecond argument.
