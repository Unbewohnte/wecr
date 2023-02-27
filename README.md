# Wecr - versatile WEb CRawler 

## Overview

A simple HTML web spider with no dependencies. It is possible to search for pages with a text on them or for the text itself, extract images, video, audio and save pages that satisfy the criteria along the way. 

## Configuration Overview

The flow of work fully depends on the configuration file. By default `conf.json` is used as a configuration file, but the name can be changed via `-conf` flag. The default configuration is embedded in the program so on the first launch or by simply deleting the file, a new `conf.json` will be created in the working directory unless the `-wdir` (working directory) flag is set to some other value, in which case it has a bigger importance. To see all available flags run `wecr -h`.

The configuration is split into different branches like `requests` (how requests are made, ie: request timeout, wait time, user agent), `logging` (use logs, output to a file), `save` (output file|directory, save pages or not) or `search` (use regexp, query string) each of which contain tweakable parameters. There are global ones as well such as `workers` (working threads that make requests in parallel) and `depth` (literally, how deep the recursive search should go). The names are simple and self-explanatory so no attribute-by-attribute explanation needed for most of them.

The parsing starts from `initial_pages` and goes deeper while ignoring the pages on domains that are in `blacklisted_domains` or are NOT in `allowed_domains`. If all initial pages are happen to be on blacklisted domains or are not in the allowed list - the program will get stuck. It is important to note that `*_domains` should be specified with an existing scheme (ie: https://en.wikipedia.org). Subdomains and ports **matter**: `https://unbewohnte.su:3000/` and `https://unbewohnte.su/` are **different**.

Previous versions stored the entire visit queue in memory, resulting in gigabytes of memory usage but as of `v0.2.4` it is possible to offload the queue to the persistent storage via `in_memory_visit_queue` option (`false` by default).

You can change search `query` at **runtime** via web dashboard if `launch_dashboard` is set to `true`

### Search query

There are some special `query` values to control the flow of work:

- `email` - tells wecr to scrape email addresses and output to `output_file`
- `images` - find all images on pages and output to the corresponding directory in `output_dir` (**IMPORTANT**: set `content_fetch_timeout_ms` to `0` so the images (and other content below) load fully)
- `videos` - find and fetch files that look like videos
- `audio` - find and fetch files that look like audio
- `documents` - find and fetch files that look like a document
- `everything` - find and fetch images, audio, video, documents and email addresses
- `archive` - no text to be searched, save every visited page

When `is_regexp` is enabled, the `query` is treated as a regexp string (in Go "flavor") and pages will be scanned for matches that satisfy it.

### Data Output

If the query is not something of special value, all text matches will be outputted to `found_text.json` file as separate continuous JSON objects in `output_dir`; if `save_pages` is set to `true` and|or `query` is set to `images`, `videos`, `audio`, etc. - the additional contents will be also put in the corresponding directories inside `output_dir`, which is neatly created in the working directory or, if `-wdir` flag is set - there. If `output_dir` is happened to be empty - contents will be outputted directly to the working directory.

The output almost certainly contains some duplicates and is not easy to work with programmatically, so you can use `-extractData` with the output JSON file argument (like `found_text.json`, which is the default output file name for simple text searches) to extract the actual data, filter out the duplicates and put each entry on its new line in a new text file. 

## Build

If you're on *nix - it's as easy as `make`.

Otherwise - `go build` in the `src` directory to build `wecr`. No dependencies.

## Examples

See [a page on my website](https://unbewohnte.su/wecr) for some basic examples.

Dump of a basic configuration:

```json
{
	"search": {
		"is_regexp": true,
		"query": "(sequence to search)|(other sequence)"
	},
	"requests": {
		"request_wait_timeout_ms": 2500,
		"request_pause_ms": 100,
		"content_fetch_timeout_ms": 0,
		"user_agent": ""
	},
	"depth": 90,
	"workers": 30,
	"initial_pages": [
		"https://en.wikipedia.org/wiki/Main_Page"
	],
	"allowed_domains": [
		"https://en.wikipedia.org/"
	],
	"blacklisted_domains": [
		""
	],
	"in_memory_visit_queue": false,
	"web_dashboard": {
		"launch_dashboard": true,
		"port": 13370
	},
	"save": {
		"output_dir": "scraped",
		"save_pages": false
	},
	"logging": {
		"output_logs": true,
		"logs_file": "logs.log"
	}
}
```

## License
wecr is distributed under AGPLv3 license