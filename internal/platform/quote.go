package platform

import "github.com/biggs-100/biggz-ai/internal/pathquote"

// QuotePath returns a Windows-safe quoted path preserving bytes exactly.
// It delegates to pathquote.Quote so a Windows path renders with single
// backslashes and strings.Contains(message, path) holds on every platform.
// On Windows, quoting uses double quotes with only embedded double quotes
// escaped; on POSIX the same helper is safe for copy-paste invocations.
func QuotePath(path string) string { return pathquote.Quote(path) }
