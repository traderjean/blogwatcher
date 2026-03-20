"""Scrape blog articles using Scrapling with automatic anti-bot escalation.

Usage: python3 scrape.py <url> <selector> [timeout_seconds]
stdout: JSON array of {title, url, published_date}
stderr: error messages
"""

import json
import logging
import sys
from urllib.parse import urljoin

try:
    from scrapling import Fetcher, StealthyFetcher
except ImportError:
    print("Scrapling not installed. Run: pip3 install 'scrapling[all]'", file=sys.stderr)
    sys.exit(1)

logging.getLogger("scrapling").setLevel(logging.ERROR)


def scrape(url, selector, timeout):
    # Try fast static fetcher first (class methods, not instances)
    page = Fetcher.get(url, timeout=timeout)
    results = extract(page, url, selector)

    # Escalate to stealth if blocked or empty
    if page.status == 403 or not results:
        page = StealthyFetcher.fetch(url, headless=True, timeout=timeout)
        results = extract(page, url, selector)

    return results


def extract(page, base_url, selector):
    seen = set()
    articles = []

    for el in page.css(selector):
        # Find anchor: use element itself or first child <a>
        if el.tag == "a":
            link = el
        else:
            anchors = el.css("a")
            if not anchors:
                continue
            link = anchors[0]

        href = link.attrib.get("href", "").strip()
        if not href:
            continue

        resolved = urljoin(base_url, href)
        if resolved in seen:
            continue
        seen.add(resolved)

        # Extract title: link text > title attr > parent text
        title = (link.text or "").strip()
        if not title:
            title = (link.attrib.get("title") or "").strip()
        if not title:
            title = (el.text or "").strip()
        if not title:
            continue

        articles.append({"title": title, "url": resolved, "published_date": None})

    return articles


def main():
    if len(sys.argv) < 3:
        print("Usage: python3 scrape.py <url> <selector> [timeout_seconds]", file=sys.stderr)
        sys.exit(1)

    url = sys.argv[1]
    selector = sys.argv[2]
    timeout = int(sys.argv[3]) if len(sys.argv) > 3 else 30

    try:
        articles = scrape(url, selector, timeout)
        json.dump(articles, sys.stdout)
    except Exception as e:
        print(f"scrape failed: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
