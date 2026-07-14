mod main
use "../unix/time_impl.mg" time_impl
pub main() void:
    time_impl.ticks()
    time_impl.unixTimestamp()
..
