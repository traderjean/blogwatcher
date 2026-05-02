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
    from scrapling import StealthyFetcher
except ImportError:
    print("Scrapling not installed. Run: pip3 install 'scrapling[all]'", file=sys.stderr)
    sys.exit(1)

logging.getLogger("scrapling").setLevel(logging.ERROR)


def scrape(url, selector, timeout):
    # Always use StealthyFetcher — blogs frequently sit behind anti-bot walls,
    # and the per-call cost is acceptable for a periodic scanner.
    # StealthyFetcher.timeout is in milliseconds (Playwright convention).
    # network_idle=True lets SPAs hydrate before we read the DOM (~+1-2s).
    page = StealthyFetcher.fetch(
        url, headless=True, timeout=timeout * 1000, network_idle=True
    )
    return extract(page, url, selector)


def extract(page, base_url, selector):
    # url -> (title, score). Multiple anchors can point at the same article
    # (image, title, "read more" button). Score the candidate title at each
    # encounter so a later occurrence with a better source (e.g. heading)
    # can replace an earlier weak one (e.g. "Read more").
    best = {}
    order = []

    for el in page.css(selector):
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
        title, score = best_title(link, el)

        if resolved not in best:
            best[resolved] = (title, score)
            order.append(resolved)
        elif score > best[resolved][1]:
            best[resolved] = (title, score)

    return [
        {"title": best[url][0], "url": url, "published_date": None}
        for url in order
        if best[url][0]
    ]


def best_title(link, el):
    # Returns (title, score). Higher score = stronger signal.
    headings = link.css("h1, h2, h3, h4, h5, h6")
    if headings:
        t = headings[0].get_all_text().strip()
        if t:
            return (t, 3)
    t = (link.text or "").strip()
    if t:
        return (t, 2)
    t = (link.attrib.get("title") or "").strip()
    if t:
        return (t, 2)
    if hasattr(link, "get_all_text"):
        all_text = link.get_all_text().strip()
        if all_text:
            return (all_text.splitlines()[0].strip(), 1)
    t = (el.text or "").strip()
    if t:
        return (t, 1)
    return ("", 0)


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
