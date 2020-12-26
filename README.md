[![Go Reference](https://pkg.go.dev/badge/github.com/fishy/url2epub.svg)](https://pkg.go.dev/github.com/fishy/url2epub)
[![Go Report Card](https://goreportcard.com/badge/github.com/fishy/url2epub)](https://goreportcard.com/report/github.com/fishy/url2epub)
[![Gitter](https://badges.gitter.im/url2epub/community.svg)](https://gitter.im/url2epub/community)

# url2epub
Create ePub files from URLs

## Overview

The [root][root] directory provides a Go library that creates ePub files out of
URLs, with limitations (currently only support articles with an AMP version).

[`rmapi/`][rmapi] directory provides a Go library that implements
[reMarkable API][remarkable],
so that the ePub files generated can be sent to reMarkable paper tablet
directly.

[`tgbot/`][tgbot] directory provides a Go library that implements partial
[Telegram bot API][telegram], so all this can be done in a Telegram message.

[`appengine/`](appengine/) directory provides the AppEngine implementation of
the [Telegram Bot][bot] that does all this.

## License

[BSD 3-Clause](LICENSE).

[root]: https://pkg.go.dev/github.com/fishy/url2epub
[rmapi]: https://pkg.go.dev/github.com/fishy/url2epub/rmapi
[tgbot]: https://pkg.go.dev/github.com/fishy/url2epub/tgbot
[remarkable]: https://github.com/splitbrain/ReMarkableAPI/wiki
[telegram]: https://core.telegram.org/bots/api
[bot]: https://t.me/url2rM_bot?start=1
