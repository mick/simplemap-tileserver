## Tileserver


This tile server reads smtiles files, which are very almost mbtiles (for now, might drift more), but have a single quadkey column for the tiles table rather than z x y in tms format.  Could be converted to support either mbtiles or smtiles.

This uses a VFS for sqlite, so each time that sqlite reads a page it fetches the range of bytes from a GCS object for that page.  This also "overreads" reading in larger changes of multiple pages at a time.  It caches that those extra pages, because they are often fetched in sequence, like when reading a tile or index that spans multiple pages.


Sqlite itself also has a page cache, defaulting to 2Mb.

