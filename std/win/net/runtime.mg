mod win_net_runtime
# Shared, process-wide Winsock initialization.

link "ws2_32"

use "std:errors" errors
use "std:cast" cast
use "std:slices" slices

ext ext_WSAStartup WSAStartup(version u16, data ptr) i32

llvm "@magma.winsock.state = internal global i8 0, align 1\n"

loadState() u8:
    unsafe:
        llvm "  %value = load atomic i8, ptr @magma.winsock.state acquire, align 1\n"
        llvm "  ret i8 %value\n"
    ..
..

claimInitialization() bool:
    unsafe:
        llvm "  %result = cmpxchg ptr @magma.winsock.state, i8 0, i8 1 acq_rel acquire\n"
        llvm "  %claimed = extractvalue { i8, i1 } %result, 1\n"
        llvm "  ret i1 %claimed\n"
    ..
..

publishState(value u8) void:
    unsafe:
        llvm "  store atomic i8 %value, ptr @magma.winsock.state release, align 1\n"
        llvm "  ret void\n"
    ..
..

pub ensure() !void:
    state := loadState()
    if state == 2:
        ret
    ..
    if state == 3: throw errors.failure("Winsock initialization previously failed") ..
    if claimInitialization():
        # WSADATA is smaller than this storage on every supported Windows ABI.
        data := array u8[512]
        code i32 = ext_WSAStartup(0x0202, slices.toPtr(data))
        if code != 0:
            publishState(3)
            throw errors.native(cast.u64to32(cast.itou(cast.i32to64(code))), "WSAStartup failed")
        ..
        publishState(2)
        ret
    ..
    state = loadState()
    loop state == 1:
        state = loadState()
    ..
    if state == 2:
        ret
    ..
    throw errors.failure("Winsock initialization previously failed")
..
