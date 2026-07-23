mod footgun
# Explicit escape hatches for intentionally discarding owned values.
# @warning These operations bypass normal ownership cleanup responsibilities.

# Explicitly discards an owned value without running a destructor.
# @complexity O(1)
# @param x owned value to forget
# @warning This can leak resources and should only bridge APIs that manage them elsewhere.
# @example
#   footgun.drop[Resource](resource)
pub drop[T](x $T) void:
    tmp := array T[1]
    tmp[0] = x
..
