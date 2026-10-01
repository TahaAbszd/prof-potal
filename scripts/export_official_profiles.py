#!/usr/bin/env python3
"""Fetch and map public university faculty pages to the portal's Professor schema."""
import argparse
import json
import re
import time
from pathlib import Path
from urllib.parse import urljoin
from bs4 import BeautifulSoup, Tag
from fetch_official_index import ROOT, BASE, fetch, resolve_ip

INDEX = ROOT / 'data/imports/official-faculty-index.json'
OUTPUT = ROOT / 'data/imports/official-faculty-profiles.json'
EMAIL = re.compile(r'[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}')


def clean(value):
    return ' '.join(str(value or '').replace('\u200c', ' ').split())


def fields_of(box):
    fields = {}
    for heading in box.select('h4'):
        links = heading.find_all('a')
        for index, link in enumerate(links[:-1]):
            label = clean(link.get_text(' ', strip=True)).rstrip(':：')
            if clean(link.get_text(' ', strip=True)).endswith((':', '：')):
                value = clean(links[index + 1].get_text(' ', strip=True))
                if value:
                    fields[label] = value
    return fields


def value(fields, *names):
    for name in names:
        if fields.get(name):
            return fields[name]
    for key, item in fields.items():
        if any(name in key for name in names):
            return item
    return ''


def add_unique(items, item):
    if item and item not in items:
        items.append(item)


def research_title(fields):
    title = value(fields, 'عنوان مقاله', 'عنوان کتاب', 'عنوان اختراع', 'عنوان طرح', 'عنوان همایش', 'عنوان سمینار', 'نام همایش', 'نام سمینار', 'عنوان')
    if not title:
        return ''
    publication = value(fields, 'عنوان نشریه چاپ کننده', 'نام مجله', 'نام کنفرانس', 'نام همایش')
    year = value(fields, 'سال', 'تاریخ چاپ', 'تاریخ ثبت')
    details = [part for part in (publication, year) if part and part not in title]
    return title + (' — ' + '، '.join(details) if details else '')


def parse_profile(html, card):
    soup = BeautifulSoup(html, 'html.parser')
    wrapper = soup.select_one('.showformmanperson')
    if not wrapper:
        raise ValueError('Faculty profile container not found')
    heading = wrapper.select_one('.showformmanpersontit h3')
    heading_text = clean(heading.get_text(' ', strip=True)) if heading else ''
    faculty, department = '', ''
    if heading_text.startswith('دانشکده '):
        heading_text = heading_text.removeprefix('دانشکده ').strip()
    if ' گروه ' in heading_text:
        faculty, department = heading_text.split(' گروه ', 1)
    else:
        faculty = heading_text
    faculty = clean(faculty) or 'نامشخص'
    department = clean(department)

    identity = wrapper.select_one('.formmanperson')
    detail = {}
    if identity:
        for row in identity.select('.formmanpersondetail'):
            text = clean(row.get_text(' ', strip=True))
            if ':' in text:
                label, item = text.split(':', 1)
                detail[clean(label)] = clean(item)
    name_tag = identity.select_one('h5') if identity else None
    name = clean(name_tag.get_text(' ', strip=True)) if name_tag else clean(card['name'])
    rank_raw = detail.get('مرتبه') or clean(card.get('rank'))
    rank = next((r for r in ('استاد تمام', 'دانشیار', 'استادیار', 'مربی', 'استاد') if rank_raw.startswith(r)), rank_raw)
    if rank == 'استاد':
        rank = 'استاد تمام'
    if rank in ('_', '-', ''):
        rank = 'نامشخص'
    emails = EMAIL.findall(detail.get('ایمیل', ''))
    email = next((address for address in emails if address.lower().endswith('@hormozgan.ac.ir')), emails[0] if emails else '')
    source_id = card['sourceId']
    profile = {
        'slug': f'official-{source_id}', 'name': name, 'image': '', 'rank': rank,
        'faculty': faculty, 'department': department,
        'field': detail.get('رشته') or clean(card.get('field')),
        'office': detail.get('آدرس', ''), 'email': email,
        'education': [], 'teaching': [],
        'research': {'journals': [], 'conferences': [], 'books': [], 'projects': [], 'patents': []},
        'theses': [], 'interests': [], 'downloads': [],
    }
    teaching = {}
    section = ''
    for node in wrapper.children:
        if not isinstance(node, Tag):
            continue
        if node.name == 'h6':
            section = clean(node.get_text(' ', strip=True)).rstrip(' »')
            continue
        if node.name != 'div' or 'showresultf' not in node.get('class', []):
            continue
        fields = fields_of(node)
        if section == 'اطلاعات تحصیلی':
            degree = value(fields, 'مقطع تحصیلی')
            university = value(fields, 'نام دانشگاه')
            if degree or university:
                profile['education'].append({
                    'degree': degree, 'university': university,
                    'date': value(fields, 'تاریخ پایان تحصیل'),
                    'thesis': value(fields, 'پایان نامه (زمینه تحقیقاتی)'),
                })
        elif 'اختراع' in section:
            add_unique(profile['research']['patents'], research_title(fields))
        elif 'کتاب' in section:
            add_unique(profile['research']['books'], research_title(fields))
        elif 'مقالات' in section or 'انتشارات' in section:
            article_type = clean(value(fields, 'نوع مقاله')).lower()
            category = 'conferences' if any(word in article_type for word in ('همایش', 'کنفرانس', 'سمینار', 'conference', 'congress', 'symposium')) else 'journals'
            add_unique(profile['research'][category], research_title(fields))
        elif 'سمینار' in section or 'همایش' in section or 'کنفرانس' in section:
            add_unique(profile['research']['conferences'], research_title(fields))
        elif 'طرح' in section and 'پژوهش' in section:
            add_unique(profile['research']['projects'], research_title(fields))
        elif 'پایان نامه' in section:
            title = value(fields, 'عنوان پایان نامه', 'عنوان')
            if title:
                profile['theses'].append({
                    'title': title, 'role': value(fields, 'نقش', 'سمت', 'نوع همکاری'),
                    'degree': value(fields, 'مقطع'), 'year': value(fields, 'سال دفاع', 'سال'),
                })
        elif section == 'تدریس':
            title = value(fields, 'عنوان تدریس')
            if title:
                degree = value(fields, 'مقطع') or 'نامشخص'
                teaching.setdefault(degree, [])
                add_unique(teaching[degree], title)
        elif 'علاقه' in section:
            for heading in node.select('h4'):
                add_unique(profile['interests'], clean(heading.get_text(' ', strip=True)))
        elif section == 'دانلودها':
            for link in node.select('a[href]'):
                url = urljoin(BASE, link.get('href', ''))
                title = clean(link.get_text(' ', strip=True))
                if title and url.startswith(BASE) and ('/UploadedFiles/' in url or '/uploadedfiles/' in url.lower()):
                    entry = {'title': title, 'url': url}
                    if entry not in profile['downloads']:
                        profile['downloads'].append(entry)
    profile['teaching'] = [{'degree': degree, 'subjects': subjects} for degree, subjects in teaching.items()]
    image = identity.select_one('img[src]') if identity else None
    image_source = image.get('src', '') if image else card.get('imageSource', '')
    return {'sourceId': source_id, 'sourceUrl': BASE + f'resualtfni.aspx?m={source_id}',
            'imageSource': urljoin(BASE, image_source) if image_source else '', 'profile': profile}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--limit', type=int, default=0, help='Fetch only the first N profiles for parser inspection')
    parser.add_argument('--fresh', action='store_true', help='Discard a previous partial export')
    args = parser.parse_args()
    index = json.loads(INDEX.read_text(encoding='utf-8'))
    cards = index['cards'][:args.limit] if args.limit else index['cards']
    existing = {} if args.fresh or not OUTPUT.exists() else {r['sourceId']: r for r in json.loads(OUTPUT.read_text(encoding='utf-8'))['records']}
    ip = resolve_ip()
    errors = []
    for number, card in enumerate(cards, start=1):
        source_id = card['sourceId']
        if source_id in existing:
            continue
        try:
            html = fetch(BASE + f'resualtfni.aspx?m={source_id}', ip)
            existing[source_id] = parse_profile(html, card)
            local_image = next((p for p in (ROOT / 'public/images/official-faculty').glob(f'{source_id}.*') if p.is_file()), None)
            if local_image:
                existing[source_id]['profile']['image'] = f'/images/official-faculty/{local_image.name}'
        except Exception as exc:
            errors.append({'sourceId': source_id, 'error': str(exc)})
        if number % 10 == 0 or number == len(cards):
            ordered = [existing[c['sourceId']] for c in index['cards'] if c['sourceId'] in existing]
            OUTPUT.write_text(json.dumps({'source': BASE, 'indexCount': index['count'], 'parsedCount': len(ordered), 'records': ordered}, ensure_ascii=False, indent=2), encoding='utf-8')
            print(f'profile {number}/{len(cards)}: parsed {len(ordered)}, errors {len(errors)}', flush=True)
        time.sleep(0.25)
    if errors:
        error_path = ROOT / 'data/imports/official-faculty-errors.json'
        error_path.write_text(json.dumps(errors, ensure_ascii=False, indent=2), encoding='utf-8')
        raise RuntimeError(f'{len(errors)} profiles failed; see {error_path}')
    print(f'Exported {len(existing)} profiles to {OUTPUT}', flush=True)


if __name__ == '__main__':
    main()
