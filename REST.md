# url2epub REST APIs

This documentation describes how REST APIs on https://url2epub.fishy.me/
work. For the [Telegram bot][bot], just talk to the bot from Telegram.

## Overview

Unless specified otherwise by the endpoint, all endpoints:

1. Take both `GET` or `POST` requests
   - For `POST` requests, you need to use [form][form] instead of JSON.
1. Upon error, the response will be in plain text.
1. Upon success, the response will be in JSON.

## Endpoints

### `/epub`

Generate an epub file from the given URL.

#### Args

| Arg | Type | Description |
| --- | --- | --- |
| `url` | string | The URL of the article. |
| `gray` | [bool][bool] | Whether to grayscale all images. |
| `passthrough-user-agent` | [bool][bool] | Use the same `User-Agent` from the original request. |

#### Response

The response will be the epub file,
with proper `Content-Disposition`, `Content-Type` headers set.
Note that this is not JSON.

[bot]: https://t.me/url2rM_bot?start=1
[form]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Methods/POST
[bool]: https://pkg.go.dev/strconv#ParseBool
