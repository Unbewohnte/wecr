# Wecr - simple web crawler 

## Overview

Just a simple HTML web spider with minimal dependencies. It is possible to search for pages with a text on them or for the text itself, extract images and save pages that satisfy the criteria along the way. 

## Configuration

The flow of work fully depends on the configuration file. By default `conf.json` is used as a configuration file, but the name can be changed via `-conf` flag. The default configuration is embedded in the program so on the first launch or by simply deleting the file, a new `conf.json` will be created in the same directory as the executable itself unless the `wDir` (working directory) flag is set to some other value.

The configuration is split into different branches like `requests` (how requests are made, ie: request timeout, wait time, user agent), `logging` (use logs, output to a file), `save` (output file|directory, save pages or not) or `search` (use regexp, query string) each of which contain tweakable parameters. There are global ones as well such as `workers` (working threads that make requests in parallel) and `depth` (literally, how deep the recursive search should go). The names are simple and self-explanatory so no attribute-by-attribute explanation needed for most of them.

The parsing starts from `initial_pages` and goes deeper while ignoring the pages on domains that are in `blacklisted_domains`. If all initial pages are happen to be blacklisted - the program will end.

### Search query

if `is_regexp` is `false`, then `query` is the text to be searched for, but there are some special values:

- `links` - tells `webscrape` to search for all links there are on the page
- `images` - find all image links and output to the `output_dir`

When `is_regexp` is enabled, the `query` is treated as a regexp string and pages will be scanned for matches that satisfy it.

### Output

By default, if the query is not `images` all the matches and other data will be outputted to `output.json` file as separate continuous JSON objects, but if `save_pages` is set to `true` and|or `query` is set to `images` - the additional contents will be put in the `output_dir` directory neatly created by the executable's side.

## TODO

- **PARSE HTML WITH REGEXP (_EVIL LAUGH_)**

## License
AGPLv3