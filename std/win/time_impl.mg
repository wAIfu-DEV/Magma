mod time_impl_win

use "../errors.mg" err
use "../cast.mg" cast

ext ext_win32_QueryPerformanceCounter        QueryPerformanceCounter(value i64*) i32
ext ext_win32_QueryPerformanceFrequency      QueryPerformanceFrequency(value i64*) i32
ext ext_win32_GetSystemTimePreciseAsFileTime GetSystemTimePreciseAsFileTime(value FileTime*) void
ext ext_win32_Sleep                          Sleep(dwMilliseconds u32) void
ext ext_win32_GetCurrentProcess              GetCurrentProcess() ptr
ext ext_win32_GetProcessTimes                GetProcessTimes(process ptr, creation FileTime*, exit FileTime*, kernel FileTime*, user FileTime*) i32

FileTime(
    lowDateTime u32,
    highDateTime u32,
)

gl_tickFreq u64

fileTimeValue(value FileTime*) u64:
    high u64 = cast.u32to64(value.highDateTime) << 32
    ret high | cast.u32to64(value.lowDateTime)
..

pub processCpuTimeNs() u64:
    creation FileTime
    exit FileTime
    kernel FileTime
    user FileTime
    ok i32 = ext_win32_GetProcessTimes(ext_win32_GetCurrentProcess(), addrof creation, addrof exit, addrof kernel, addrof user)
    if ok == 0:
        ret 0
    ..
    # FILETIME uses 100-nanosecond intervals.
    ret (fileTimeValue(addrof kernel) + fileTimeValue(addrof user)) * 100
..

pub ticks() u64:
    t u64
    ext_win32_QueryPerformanceCounter(addrof t)
    ret t
..

pub tickFrequency() u64:
    if gl_tickFreq != 0:
        ret gl_tickFreq
    ..
    ext_win32_QueryPerformanceFrequency(addrof gl_tickFreq)
    ret gl_tickFreq
..

unixEpochIntervals() u64:
    value := FileTime(lowDateTime=0, highDateTime=0)
    ext_win32_GetSystemTimePreciseAsFileTime(addrof value)

    high u64 = cast.u32to64(value.highDateTime) << 32
    low u64 = cast.u32to64(value.lowDateTime)
    intervals u64 = high | low
    ret intervals - 116444736000000000
..

# Windows system time is measured in 100 ns intervals since 1601-01-01.
pub unixTimestamp() u64:
    ret unixEpochIntervals() / 10000000
..

pub unixTimestampMs() u64:
    ret unixEpochIntervals() / 10000
..

pub unixTimestampUs() u128:
    intervals u128 = cast.u64to128(unixEpochIntervals())
    ret intervals / 10
..

pub unixTimestampNs() u128:
    intervals u128 = cast.u64to128(unixEpochIntervals())
    ret intervals * 100
..

pub sleep(ms u64) void:
    ext_win32_Sleep(cast.u64to32(ms))
..
