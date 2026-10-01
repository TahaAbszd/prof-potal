#!/usr/bin/env python3
"""Export the public faculty cards from Hormozgan University's official directory."""
import json
import re
import subprocess
import time
from pathlib import Path
from urllib.parse import quote
from bs4 import BeautifulSoup

ROOT = Path(__file__).resolve().parents[1]
HOST = 'ostad.hormozgan.ac.ir'
BASE = f'https://{HOST}/ostad/'


def resolve_ip():
    result = subprocess.run(['dig', '+short', HOST, 'A'], capture_output=True, text=True, check=True)
    ips = [line for line in result.stdout.splitlines() if re.fullmatch(r'\d+(?:\.\d+){3}', line)]
    if not ips:
        raise RuntimeError(f'No IPv4 address for {HOST}')
    return ips[-1]


def fetch(url, ip, attempts=3):
    if not url.startswith(BASE):
        raise ValueError('Source URL is outside the official directory')
    for attempt in range(attempts):
        response = subprocess.run(['curl', '-fsSL', '--max-time', '20', '--resolve', f'{HOST}:443:{ip}', url], capture_output=True)
        if response.returncode == 0:
            return response.stdout
        if attempt + 1 < attempts:
            time.sleep(1 + attempt)
    raise RuntimeError(f'Could not fetch {url}: {response.stderr.decode(errors="replace")[-250:]}')


def parse_cards(html):
    soup = BeautifulSoup(html, 'html.parser')
    cards = []
    for box in soup.select('.imagec'):
        link = box.select_one('.tf a[href*="resualtfni.aspx?m="]')
        if not link:
            continue
        match = re.search(r'[?&]m=(\d+)', link.get('href', ''))
        if not match:
            continue
        spans = box.select('.te span')
        image = box.select_one('img[src]')
        cards.append({
            'sourceId': match.group(1),
            'name': (link.select_one('span') or link).get('title', '') or link.get_text(' ', strip=True),
            'rank': (spans[0].get('title', '') or spans[0].get_text(' ', strip=True)) if len(spans) > 0 else '',
            'field': (spans[1].get('title', '') or spans[1].get_text(' ', strip=True)) if len(spans) > 1 else '',
            'imageSource': image.get('src', '') if image else '',
        })
    return cards


def main():
    ip = resolve_ip()
    found = {}
    pages = 0
    for offset in range(0, 1200, 12):
        path = '' if offset == 0 else f'default.aspx?nam=Empty&fnam=Empty&dnam=Empty&gnam=Empty&pp={offset}'
        cards = parse_cards(fetch(BASE + path, ip))
        pages += 1
        for card in cards:
            found.setdefault(card['sourceId'], card)
        print(f'page {pages}: {len(cards)} cards, {len(found)} unique', flush=True)
        if not cards or len(cards) < 12:
            break
        time.sleep(0.3)
    else:
        raise RuntimeError('Reached page safety limit; source pagination may have changed')
    if len(found) < 100:
        raise RuntimeError(f'Only {len(found)} cards found; refusing an incomplete export')
    output = ROOT / 'data/imports/official-faculty-index.json'
    output.write_text(json.dumps({'source': BASE, 'pages': pages, 'count': len(found), 'cards': list(found.values())}, ensure_ascii=False, indent=2), encoding='utf-8')
    print(f'Wrote {output} with {len(found)} unique faculty profiles', flush=True)


if __name__ == '__main__':
    main()
