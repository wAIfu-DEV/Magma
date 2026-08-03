mod dialog
# Native user-facing file and folder selection dialogs.
#
# The Windows implementation is a Magma port derived from Native File Dialog
# by Michael Labbe. See std/licenses/NATIVE_FILE_DIALOG.txt.

use "std:allocator" allocator
use "std:slices" slices

@platform("windows")
use "std:win/dialog_impl" impl

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/dialog_impl" impl

# A named group of comma-separated file extensions, without leading dots.
# For example Filter(name="Images", extensions="png,jpg,jpeg").
pub Filter(
    name str
    extensions str
)

# Optional dialog configuration. All strings and filters are borrowed for the
# duration of the call. parent is a native window handle or none.
pub Options(
    filters Filter[]
    defaultPath str
    defaultName str
    title str
    parent ptr
)

# Returns default options with no filters, paths, title, or parent window.
pub defaultOptions() Options:
    ret Options(
        filters=slices.fromPtr(none, 0),
        defaultPath="",
        defaultName="",
        title="",
        parent=none,
    )
..

# Shows the native file-open dialog.
pub openFile(a allocator.Allocator, configuration Options) !$str:
    ret try impl.openFile(a, slices.toPtr(configuration.filters), configuration.filters.count(), configuration.defaultPath, configuration.title, configuration.parent)
..

# Shows the native save-location dialog. This chooses a destination path; it
# does not itself download or write a file.
pub saveFile(a allocator.Allocator, configuration Options) !$str:
    ret try impl.saveFile(a, slices.toPtr(configuration.filters), configuration.filters.count(), configuration.defaultPath, configuration.defaultName, configuration.title, configuration.parent)
..

# Shows the native folder-selection dialog.
pub openDir(a allocator.Allocator, configuration Options) !$str:
    ret try impl.openDir(a, configuration.defaultPath, configuration.title, configuration.parent)
..
