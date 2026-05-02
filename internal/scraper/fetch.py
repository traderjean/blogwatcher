"""Fetch a URL via Scrapling's Fetcher (TLS fingerprint impersonation).

Used as a fallback when plain net/http is blocked (e.g. Cloudflare TLS gate).
Returns raw response bytes — suitable for feeding XML/RSS through a parser.

Usage: python3 fetch.py <url> [timeout_seconds]
stdout: raw response body bytes
stderr: error messages
"""

import logging
import sys

try:
    from scrapling import Fetcher
except ImportError:
    print("Scrapling not installed. Run: pip3 install 'scrapling[all]'", file=sys.stderr)
    sys.exit(1)

logging.getLogger("scrapling").setLevel(logging.ERROR)


def main():
    if len(sys.argv) < 2:
        print("Usage: python3 fetch.py <url> [timeout_seconds]", file=sys.stderr)
        sys.exit(1)

    url = sys.argv[1]
    timeout = int(sys.argv[2]) if len(sys.argv) > 2 else 30

    try:
        page = Fetcher.get(url, timeout=timeout)
    except Exception as e:
        print(str(e), file=sys.stderr)
        sys.exit(1)

    if page.status < 200 or page.status >= 300:
        print(f"status {page.status}", file=sys.stderr)
        sys.exit(1)

    sys.stdout.buffer.write(page.body)


if __name__ == "__main__":
    main()
