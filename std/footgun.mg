mod footgun

pub drop[T](x $T) void:
    tmp T[1]
    tmp[0] = x
..
