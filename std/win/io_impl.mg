mod io_impl_win

use "../writer.mg" writer
use "../reader.mg" reader
use "file_impl.mg" file_impl

# TODO: move writer init out of file_impl
# currently limited by bug, see TODO.md

pub stdout() writer.Writer:
   ret file_impl.stdout()
..

pub stderr() writer.Writer:
   ret file_impl.stderr()
..

pub stdin() reader.Reader:
   ret file_impl.stdin()
..
