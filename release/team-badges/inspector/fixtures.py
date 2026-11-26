#!/usr/bin/env python3
"""Disposable signed inputs for private inspector proofs, never owner data."""
import argparse
import base64
import io
import json
from pathlib import Path
import stat
import struct
import zipfile

from policy import archive, canonical, pack, sha
TEXT_SENTINEL = 'UNREVIEWED TEXT MUST BE PRESENT'


def fixture_files():
    from PIL import Image, ImageDraw
    emblem=Image.new('RGBA',(256,256),(0,0,0,0))
    ImageDraw.Draw(emblem).polygon([(128,12),(244,128),(128,244),(12,128)],fill=(250,120,30,255))
    png=io.BytesIO();emblem.save(png,format='PNG')
    frames=[]
    for index in range(3):
        frame=Image.new('RGBA',(128,128),(0,0,0,0))
        ImageDraw.Draw(frame).rectangle((8+index*24,20,40+index*24,100),fill=((index+1)*70,160,220,255));frames.append(frame)
    gif=io.BytesIO();frames[0].save(gif,format='GIF',save_all=True,append_images=frames[1:],duration=[70,130,210],loop=2,disposal=[2,3,2],transparency=0,optimize=False)
    webp=io.BytesIO();frames[0].save(webp,format='WEBP',save_all=True,append_images=frames[1:],duration=[70,130,210],loop=2,lossless=True,exact=True)
    spec={'version':1,'name':'Inspector proof','badge':{'emblem':'emblem.png','title':'PROOF','kanji':'力','accent':'#f0781e'},'card':{'palette':{'ink':'#f0e8dc','accent':'#f0781e','plate':'#15100e','rim':'#f0781e'},'art':[{'image':'art.gif','depth':0.3,'blend':'normal'}],'effects':[{'id':'sweep','strength':0.3,'speed':0.5,'color':'#f0781e'}],'words':{'tagline':'Private runtime proof','epithet':'Test'}}}
    return {'design.json':json.dumps(spec,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode(),'emblem.png':png.getvalue(),'art.gif':gif.getvalue()},webp.getvalue()


def loose_zip(entries,compression=zipfile.ZIP_STORED):
    out=io.BytesIO()
    with zipfile.ZipFile(out,'w',compression=compression) as z:
        for name,data in entries:z.writestr(name,data)
    return out.getvalue()


def fixtures():
    files,webp=fixture_files();valid=pack(files)
    textfiles=dict(files);textspec=json.loads(files['design.json'])
    textspec['name']='Canonical text sentinel';textspec['badge']['title']='TEXT PROOF';textspec['badge']['kanji']='文'
    textspec['card']['words']={'tagline':TEXT_SENTINEL,'epithet':'KEEP EXACT'}
    textfiles['design.json']=json.dumps(textspec,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()
    qml=dict(files);qml['evil.qml']=b'import QtQuick\nItem {}'
    duplicate=dict(files);duplicate['design.json']=b'{"version":1,"version":1}'
    mismatch=dict(files);mismatch['emblem.png']=files['art.gif']
    bomb=dict(files);bomb['hidden.png']=b'\x89PNG\r\n\x1a\n'+struct.pack('>I',13)+b'IHDR'+struct.pack('>II',999999,999999)+b'\x08\x06\0\0\0'+b'\0'*4+struct.pack('>I',0)+b'IEND'+b'\0'*4
    symlink=zipfile.ZipInfo('link.png');symlink.create_system=3;symlink.external_attr=(stat.S_IFLNK|0o777)<<16
    from PIL import Image
    frames=[Image.new('RGBA',(2,2),(i,255-i,128,255)) for i in range(97)]
    animated=io.BytesIO();frames[0].save(animated,format='GIF',save_all=True,append_images=frames[1:],duration=20,optimize=False)
    framebomb=dict(files);framebomb['art.gif']=animated.getvalue()
    partial=io.BytesIO();frames[0].save(partial,format='WEBP',save_all=True,append_images=frames[1:9],duration=100,loop=0,lossless=True,exact=True)
    return {'valid-design':valid,'valid-design-text-sentinel':pack(textfiles),'valid-hero-gif':pack({'hero.gif':files['art.gif']}),'valid-hero-webp':pack({'hero.webp':webp}),'valid-hero-partial-review':pack({'hero.webp':partial.getvalue()}),'hostile-traversal':loose_zip([('../evil.png',files['emblem.png'])]),'hostile-qml':loose_zip(qml.items()),'hostile-symlink':loose_zip([(symlink,b'/etc/passwd')]),'hostile-duplicate-name':loose_zip([('emblem.png',files['emblem.png']),('emblem.png',files['emblem.png'])]),'hostile-duplicate-json':pack(duplicate),'hostile-format-confusion':pack(mismatch),'hostile-unused-canvas-bomb':pack(bomb),'hostile-frame-bomb':pack(framebomb),'hostile-compression-bomb':loose_zip([('art.png',b'0'*(1024*1024))],zipfile.ZIP_DEFLATED),'hostile-trailer':valid+b'payload'}


def capacity_fixtures():
    from PIL import Image
    inputs={}
    for name,size,count in (('valid-hero-capacity-96',816,96),('valid-hero-capacity-canvas',2048,2)):
        frames=(Image.new('RGB',(size,size),((i*2+20)%256,(i*3+35)%256,(i*5+70)%256)) for i in range(count))
        first=next(frames);encoded=io.BytesIO()
        first.save(encoded,format='GIF',save_all=True,append_images=frames,duration=200,loop=0,disposal=2,optimize=False)
        first.close()
        inputs[name]=pack({'hero.gif':encoded.getvalue()})
    return inputs



def signed_submission(raw,kind,audience='https://team.ryoku.dev'):
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
    from cryptography.hazmat.primitives.serialization import Encoding,PublicFormat
    # This seed is public fixture material, not a production identity
    key=Ed25519PrivateKey.from_private_bytes(bytes(range(32)))
    device=key.public_key().public_bytes(Encoding.Raw,PublicFormat.Raw)
    files=archive(raw)
    p={'v':1,'cardId':sha(b'ryoku-card-id-v1\0'+device)[:16],'device':base64.b64encode(device).decode(),'originalSha':sha(raw),'files':[{'name':name,'sha':sha(data),'bytes':len(data)} for name,data in sorted(files.items())],'termsVersion':'cards-2026-11','attestationVersion':'cards-2026-11','ownOrLicensed':True,'policyAccepted':True,'consentToReview':True,'nonce':'fixture-only-not-production-00000001','submittedAt':1795132800,'audience':audience,'kind':kind}
    encoded=canonical(p)
    return {'payload':base64.b64encode(encoded).decode(),'sig':base64.b64encode(key.sign(b'ryoku-submit-v1\0'+encoded)).decode()}


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();args.out.mkdir(parents=True,exist_ok=True,mode=0o700)
    for name,data in fixtures().items():
        (args.out/(name+'.zip')).write_bytes(data)
        if name.startswith('valid-'):
            (args.out/(name+'.manifest.json')).write_text(json.dumps(signed_submission(data,'hero' if 'hero' in name else 'design'),sort_keys=True)+'\n')


if __name__=='__main__':main()
