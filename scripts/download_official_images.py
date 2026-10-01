#!/usr/bin/env python3
"""Save official public faculty portraits locally for the Next.js image component."""
import argparse
import json
import time
from pathlib import Path
from fetch_official_index import ROOT, BASE, fetch, resolve_ip

EXPORT = ROOT / 'data/imports/official-faculty-profiles.json'
IMAGES = ROOT / 'public/images/official-faculty'


def extension(data):
    if data.startswith(b'\xff\xd8\xff'):
        return 'jpg'
    if data.startswith(b'\x89PNG\r\n\x1a\n'):
        return 'png'
    if data.startswith((b'GIF87a', b'GIF89a')):
        return 'gif'
    if data.startswith(b'RIFF') and data[8:12] == b'WEBP':
        return 'webp'
    raise ValueError('Image response has an unsupported format')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--limit', type=int, default=0)
    args = parser.parse_args()
    export = json.loads(EXPORT.read_text(encoding='utf-8'))
    records = export['records'][:args.limit] if args.limit else export['records']
    IMAGES.mkdir(parents=True, exist_ok=True)
    ip = resolve_ip()
    failures = []
    for number, record in enumerate(records, start=1):
        source_id = record['sourceId']
        local = next((p for p in IMAGES.glob(f'{source_id}.*') if p.is_file()), None)
        if not local:
            try:
                url = record['imageSource']
                if not url.startswith(BASE):
                    raise ValueError('Portrait URL is outside the official directory')
                content = fetch(url, ip)
                if len(content) > 8_000_000:
                    raise ValueError('Portrait exceeds 8 MB')
                local = IMAGES / f'{source_id}.{extension(content)}'
                local.write_bytes(content)
            except Exception as exc:
                failures.append({'sourceId': source_id, 'error': str(exc)})
        if local:
            record['profile']['image'] = f'/images/official-faculty/{local.name}'
        if number % 20 == 0 or number == len(records):
            EXPORT.write_text(json.dumps(export, ensure_ascii=False, indent=2), encoding='utf-8')
            print(f'image {number}/{len(records)}: saved {number - len(failures)}, failed {len(failures)}', flush=True)
        time.sleep(0.2)
    if failures:
        report = ROOT / 'data/imports/official-image-errors.json'
        report.write_text(json.dumps(failures, ensure_ascii=False, indent=2), encoding='utf-8')
        print(f'{len(failures)} portraits could not be downloaded; profiles will use placeholders. See {report}', flush=True)
    print(f'Updated {EXPORT}', flush=True)


if __name__ == '__main__':
    main()
