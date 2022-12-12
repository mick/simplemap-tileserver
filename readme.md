## Tileserver


This tile server reads mbtiles files, that are on a remote storage like Google Cloud Storage or AWS S3.

To do this it uses a VFS for sqlite, so each time that sqlite reads a page it fetches the range of bytes from a GCS/S3 object for that page.  This also "overreads". It reads in larger ranges of multiple pages in a single http request. This proves useful in cases where we'd end up reading multiple pages in sequence like when a tile data is larger than the page size.

Adding to this Sqlite itself also has a page cache, defaulting to 2Mb. But this is only a cache of pages that sqlite perviously read (not the extra that was overread).


The expection is that you provide a bucket and optionally a key prefix, and this tilesever will serve tiles from all mbtiles found on that path.

