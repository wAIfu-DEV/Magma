mod duplex
# Type-erased bidirectional byte streams supporting reads and writes.

use "std:writer" wr
use "std:reader" rd

# Callback table used by a Duplex adapter.
# Callbacks receive impl and must follow Reader and Writer count contracts.
pub Vtable(
    fn_write (ptr, str) !u64,
    fn_read (ptr, u8[], u64) !u64,
)

# Non-owning bidirectional stream adapter over caller-provided state.
pub Duplex(
    impl ptr
    vtable Vtable*
)

# Creates a duplex adapter over caller-owned state and a persistent callback table.
# @complexity O(1)
# @param impl opaque state passed to read and write callbacks
# @param vtable callback table
# @ownership impl and vtable must outlive the Duplex and derived adapters.
# @example
#   stream := duplex.new(state, addrof callbacks)
pub new(impl ptr, vtable Vtable*) Duplex:
    ret Duplex(impl=impl, vtable=vtable)
..

# Returns a non-owning Writer view of the duplex stream.
# @complexity O(1)
# @ownership The Duplex state must outlive the returned Writer.
# @example
#   output := stream.writer()
Duplex.writer() wr.Writer:
    ret wr.new(this.impl, this.vtable.fn_write)
..

# Returns a non-owning Reader view of the duplex stream.
# @complexity O(1)
# @ownership The Duplex state must outlive the returned Reader.
# @example
#   input := stream.reader()
Duplex.reader() rd.Reader:
    ret rd.new(this.impl, this.vtable.fn_read)
..
