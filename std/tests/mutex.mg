mod main

use "../mutex.mg" mutex

pub main() !void:
    lock := try mutex.new()
    try lock.lock()
    try lock.unlock()
    try lock.lock()
    try lock.unlock()
    try lock.free()
..
