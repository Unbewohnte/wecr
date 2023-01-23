# Wecr - simple web crawler 

## Overview

Just a simple HTML web spider with no dependencies. It is possible to search for pages with a text on them or for the text itself, extract images, video, audio and save pages that satisfy the criteria along the way. 

## Configuration

The flow of work fully depends on the configuration file. By default `conf.json` is used as a configuration file, but the name can be changed via `-conf` flag. The default configuration is embedded in the program so on the first launch or by simply deleting the file, a new `conf.json` will be created in the same directory as the executable itself unless the `-wDir` (working directory) flag is set to some other value. To see al available flags run `wecr -h`.

The configuration is split into different branches like `requests` (how requests are made, ie: request timeout, wait time, user agent), `logging` (use logs, output to a file), `save` (output file|directory, save pages or not) or `search` (use regexp, query string) each of which contain tweakable parameters. There are global ones as well such as `workers` (working threads that make requests in parallel) and `depth` (literally, how deep the recursive search should go). The names are simple and self-explanatory so no attribute-by-attribute explanation needed for most of them.

The parsing starts from `initial_pages` and goes deeper while ignoring the pages on domains that are in `blacklisted_domains` or are NOT in `allowed_domains`. If all initial pages are happen to be on blacklisted domains or are not in the allowed list - the program will get stuck. It is important to note that `*_domains` should be specified with an existing scheme (ie: https://en.wikipedia.org). Subdomains and ports **matter**: `https://unbewohnte.su:3000/` and `https://unbewohnte.su/` are **different**.  

### Search query

There are some special `query` values:

- `email` - tells wecr to scrape email addresses and output to `output_file`
- `images` - find all images on pages and output to the corresponding directory in `output_dir` (**IMPORTANT**: set `content_fetch_timeout_ms` to `0` so the images (and other content below) load fully)
- `videos` - find and fetch files that look like videos
- `audio` - find and fetch files that look like audio
- `everything` - find and fetch images, audio and video

When `is_regexp` is enabled, the `query` is treated as a regexp string and pages will be scanned for matches that satisfy it.

### Output

By default, if the query is not something of special values all the matches and other data will be outputted to `output.json` file as separate continuous JSON objects, but if `save_pages` is set to `true` and|or `query` is set to `images`, `videos`, `audio`, etc. - the additional contents will be put in the corresponding directories inside `output_dir`, which is neatly created by the executable's side.

The output almost certainly contains some duplicates and is not easy to work with programmatically, so you can use `-extractData` with the output JSON file argument (like `output.json`, which is the default output file name) to extract the actual data, filter out the duplicates and put each entry on its new line in a new text file. 

## Build

If you're on *nix - it's as easy as `make`.

Otherwise - `go build` in the `src` directory to build `wecr`. 

## Examples

See [page on my website](https://unbewohnte.su/wecr) for some basic examples.

## License
AGPLv3