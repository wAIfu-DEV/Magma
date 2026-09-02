# `std/abort`

Cooperative abort requests shared by synchronous and asynchronous work.

```magma
control := try abort.new()
defer control.close()
signal := control.signal()
try control.abortAfter(250) # relative milliseconds
try work(signal)
```

`Controller.abort()` publishes a manual request. `abortAfter(relativeMs)`
publishes a timeout request after the relative delay. `Signal.check()` throws
`cancelled` for manual requests and `timedOut` for elapsed deadlines. Signals
are borrowed; the controller must remain alive until all signal users finish.

Aborting is advisory. Work must check its signal or use an operation with a
native cancellation implementation.
