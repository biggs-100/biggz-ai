# Host asset notes

The host assets spawn git with `execFileSync("git", ["status"])`. Prose like this,
and any documentation-like file under this prefix, is never classified as a spawn
site: only `.ts` and `.js` calls are scanned here.
