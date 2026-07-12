mod time

use "cast.mg" cast

@platform("windows")
use "win/time_impl.mg" impl_time

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/time_impl.mg" impl_time

pub ticks() u64:
    ret impl_time.ticks()
..

pub ticksToSecFloat(ticks u64) f64:
    freq u64 = impl_time.tickFrequency()
    ret cast.utof(ticks) / cast.utof(freq)
..

# Returns the runtime in seconds as a float.
pub runtime() f64:
    t u64 = ticks()
    ret ticksToSecFloat(t)
..

# Returns the number of seconds elapsed since the Unix epoch.
pub unixTimestamp() u64:
    ret impl_time.unixTimestamp()
..

# Returns the number of milliseconds elapsed since the Unix epoch.
pub unixTimestampMs() u64:
    ret impl_time.unixTimestampMs()
..

# Returns the number of microseconds elapsed since the Unix epoch.
pub unixTimestampUs() u128:
    ret impl_time.unixTimestampUs()
..

# Returns the number of nanoseconds elapsed since the Unix epoch.
pub unixTimestampNs() u128:
    ret impl_time.unixTimestampNs()
..

pub ticksToSec(ticks u64) u64:
    freq u64 = impl_time.tickFrequency()
    ret ticks / freq
..

pub ticksToMs(ticks u64) u64:
    freq u64 = impl_time.tickFrequency()
    ret (ticks * 1000) / freq
..

pub ticksToUs(ticks u64) u64:
    freq u64 = impl_time.tickFrequency()
    ret (ticks * 1000000) / freq
..

pub ticksToNs(ticks u64) u64:
    freq u64 = impl_time.tickFrequency()
    ret (ticks * 1000000000) / freq
..

pub secToTicks(sec u64) u64:
    freq u64 = impl_time.tickFrequency()
    ret sec * freq
..

pub msToTicks(ms u64) u64:
    freq u64 = impl_time.tickFrequency()
    ret (ms * freq) / 1000
..

pub usToTicks(us u64) u64:
    freq u64 = impl_time.tickFrequency()
    ret (us * freq) / 1000000
..

pub nsToTicks(ns u64) u64:
    freq u64 = impl_time.tickFrequency()
    ret (ns * freq) / 1000000000
..

pub secFloatToTicks(sec f64) u64:
    freq u64 = impl_time.tickFrequency()
    ret cast.ftou(sec * cast.utof(freq))
..

pub elapsedTicks(startTicks u64) u64:
    t u64 = ticks()
    ret t - startTicks
..

pub elapsedMs(startTicks u64) u64:
    e u64 = elapsedTicks(startTicks)
    ret ticksToMs(e)
..

pub elapsedUs(startTicks u64) u64:
    e u64 = elapsedTicks(startTicks)
    ret ticksToUs(e)
..

pub elapsedSec(startTicks u64) u64:
    e u64 = elapsedTicks(startTicks)
    ret ticksToSec(e)
..

pub elapsedSecFloat(startTicks u64) f64:
    e u64 = elapsedTicks(startTicks)
    ret ticksToSecFloat(e)
..
