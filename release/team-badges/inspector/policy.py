"""Data-only card policy. No image decoder, network, or signing authority here."""
import base64
import binascii
import hashlib
import io
import json
import math
import re
import stat
import struct
import unicodedata
import zipfile
import zlib

VERSION = 'card-inspector-1'
POLICY = 'card-data-1'
MIB = 1024 * 1024
LIMITS = {'canonical': 8*MIB, 'poster': 8*MIB, 'preview': 24*MIB, 'report': 8*MIB}
NAME = re.compile(r'(?:design\.json|[a-z0-9][a-z0-9_-]{0,31}\.(?:png|jpg|jpeg|webp|gif))\Z')
HEX = re.compile(r'[a-f0-9]{64}\Z')
COMPOSITION_TEXT_FIELDS = (('name',24),('badge.title',12),('badge.kanji',4),('card.words.tagline',36),('card.words.epithet',12))
REVIEW_IMAGE_BYTES = 768*1024

def require(condition, reason):
    if not condition:
        raise ValueError(reason)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, 'duplicate JSON key')
        result[key] = value
    return result


def loads(raw, limit=256*1024):
    require(len(raw) <= limit, 'JSON budget')
    if isinstance(raw,bytes):raw=raw.decode('utf-8','strict')
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda _: require(False, 'nonfinite JSON'))


def canonical(value):
    def walk(v):
        if isinstance(v, dict):
            require(len(v) <= 64, 'object budget')
            for k, x in v.items():
                require(isinstance(k, str) and k.isascii() and len(k) <= 64, 'invalid key')
                walk(x)
        elif isinstance(v, list):
            require(len(v) <= 128, 'array budget')
            for x in v:
                walk(x)
        elif isinstance(v, str):
            require(len(v) <= 4096 and not any(0xd800 <= ord(c) <= 0xdfff for c in v), 'invalid string')
        elif isinstance(v, bool) or v is None:
            pass
        else:
            require(type(v) is int and abs(v) <= 9007199254740991, 'signed numbers must be safe integers')
    walk(value)
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':'), allow_nan=False).encode('utf-8')


def exact(obj, required, optional=()):
    require(type(obj) is dict and set(required) <= obj.keys() and obj.keys() <= set(required)|set(optional), 'unexpected or missing fields')


def b64(value, length=None):
    require(isinstance(value, str) and len(value) <= 262144, 'base64 field')
    data = base64.b64decode(value, validate=True)
    require(base64.b64encode(data).decode() == value and (length is None or len(data) == length), 'noncanonical base64')
    return data


def manifest(envelope, audience):
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
    exact(envelope, ('payload', 'sig'))
    raw = b64(envelope['payload'])
    p = loads(raw)
    require(raw == canonical(p), 'noncanonical signed JSON')
    exact(p, ('v','cardId','device','originalSha','files','termsVersion','attestationVersion','ownOrLicensed','policyAccepted','consentToReview','nonce','submittedAt','audience','kind'))
    require(type(p['v']) is int and p['v'] == 1 and p['audience'] == audience and p['kind'] in ('design','hero'), 'manifest scope/version')
    device = b64(p['device'], 32)
    require(re.fullmatch('[a-f0-9]{16}', p['cardId']) and p['cardId'] == sha(b'ryoku-card-id-v1\0'+device)[:16], 'device/card mismatch')
    require(HEX.fullmatch(p['originalSha']) is not None, 'original hash')
    for key in ('ownOrLicensed','policyAccepted','consentToReview'):
        require(p[key] is True, 'missing consent')
    for key in ('termsVersion','attestationVersion'):
        require(isinstance(p[key], str) and re.fullmatch('[A-Za-z0-9._-]{1,64}', p[key]), 'policy version')
    require(isinstance(p['nonce'], str) and re.fullmatch('[A-Za-z0-9_-]{22,128}',p['nonce']), 'nonce')
    require(type(p['submittedAt']) is int and p['submittedAt'] > 0, 'submission time')
    require(isinstance(p['files'], list) and 1 <= len(p['files']) <= 24, 'inventory count')
    names = []
    for f in p['files']:
        exact(f, ('name','sha','bytes'))
        require(isinstance(f['name'],str) and NAME.fullmatch(f['name']) and HEX.fullmatch(f['sha']), 'inventory entry')
        require(type(f['bytes']) is int and 0 < f['bytes'] <= 24*MIB, 'inventory size')
        names.append(f['name'])
    require(names == sorted(set(names)), 'inventory order/duplicates')
    Ed25519PublicKey.from_public_bytes(device).verify(b64(envelope['sig'],64), b'ryoku-submit-v1\0'+raw)
    return p, sha(raw)


def zip_extra(raw):
    # Go's current pack writer adds one extended UTC timestamp
    require(not raw or (len(raw)==9 and raw[:5]==b'\x55\x54\x05\x00\x01'), 'ZIP extra field')


def zip_layout(raw, entries, central):
    disk,cdisk,n,total,size,start,comment=struct.unpack('<HHHHIIH',raw[-18:])
    require(disk==cdisk==0 and n==total==len(entries) and comment==0 and start==central and start+size==len(raw)-22,'ZIP central layout')
    position=0
    for f in sorted(entries,key=lambda e:e.header_offset):
        require(f.header_offset==position and raw[position:position+4]==b'PK\x03\x04','ZIP hidden entry/gap')
        require(position+30<=len(raw),'ZIP local header')
        flags,method=struct.unpack('<HH',raw[position+6:position+10])
        namelen,extralen=struct.unpack('<HH',raw[position+26:position+30])
        offset=position+30
        require(raw[offset:offset+namelen]==f.filename.encode('ascii') and flags==f.flag_bits and method==f.compress_type,'ZIP local/central mismatch')
        zip_extra(raw[offset+namelen:offset+namelen+extralen])
        data_start=offset+namelen+extralen
        position=data_start+f.compress_size
        require(position<=central,'ZIP overlapping data')
        if flags&8:
            if raw[position:position+4]==b'PK\x07\x08':position+=4
            require(position+12<=central and struct.unpack('<III',raw[position:position+12])==(f.CRC,f.compress_size,f.file_size),'ZIP descriptor')
            position+=12
        else:
            require(struct.unpack('<III',raw[f.header_offset+14:f.header_offset+26])==(f.CRC,f.compress_size,f.file_size),'ZIP local size/CRC mismatch')
    require(position==central,'ZIP hidden local data')


def archive(raw, inventory=None):
    require(0 < len(raw) <= 8*MIB, 'archive wire budget')
    require(raw[:4] == b'PK\x03\x04' and raw[-22:-18] == b'PK\x05\x06', 'ZIP prefix/comment/trailer')
    with zipfile.ZipFile(io.BytesIO(raw)) as z:
        entries = z.infolist()
        require(1 <= len(entries) <= 24 and not z.comment, 'ZIP count/comment')
        zip_layout(raw,entries,z.start_dir)
        files = {}
        total = 0
        for f in entries:
            require(NAME.fullmatch(f.filename) and f.filename not in files, 'ZIP name/duplicate')
            mode = f.external_attr >> 16
            require(not stat.S_ISLNK(mode) and (stat.S_IFMT(mode) in (0,stat.S_IFREG)), 'ZIP nonregular file')
            require(not f.flag_bits & ~0x808 and f.compress_type in (0,8) and not f.comment, 'ZIP feature')
            zip_extra(f.extra)
            require(0 < f.file_size <= 24*MIB and f.file_size <= max(1,f.compress_size)*50, 'ZIP ratio/entry budget')
            total += f.file_size
            require(total <= 24*MIB, 'ZIP decompressed budget')
            with z.open(f) as stream:
                data = stream.read(f.file_size+1)
            require(len(data) == f.file_size, 'ZIP length')
            files[f.filename] = data
    if inventory is not None:
        require([{'name':n,'sha':sha(d),'bytes':len(d)} for n,d in sorted(files.items())] == inventory, 'manifest inventory mismatch')
    return files


def image_info(raw, name):
    """Bound dimensions/frame work BEFORE native decode. All loops consume input."""
    ext = name.rsplit('.',1)[-1]
    durations = []
    if raw.startswith(b'\x89PNG\r\n\x1a\n'):
        require(ext == 'png' and len(raw) >= 33 and raw[8:16] == b'\x00\x00\x00\x0dIHDR', 'PNG type')
        w,h = struct.unpack('>II',raw[16:24]); pos = 8
        while pos < len(raw):
            require(pos+12 <= len(raw), 'PNG chunk')
            size = int.from_bytes(raw[pos:pos+4],'big'); kind = raw[pos+4:pos+8]
            require(pos+12+size<=len(raw),'PNG bounds')
            require(binascii.crc32(raw[pos+4:pos+8+size])==int.from_bytes(raw[pos+8+size:pos+12+size],'big'),'PNG CRC')
            require(kind not in (b'acTL',b'fcTL',b'fdAT'), 'APNG unsupported')
            pos += 12+size
            require(pos <= len(raw), 'PNG bounds')
            if kind == b'IEND':
                require(size == 0 and pos == len(raw), 'PNG trailer'); break
        require(kind == b'IEND', 'PNG missing end')
        fmt = 'PNG'; durations = [0]
    elif raw[:6] in (b'GIF87a',b'GIF89a'):
        require(ext == 'gif' and len(raw)>=13, 'GIF type')
        w,h = struct.unpack('<HH',raw[6:10]); pos=13
        if raw[10]&128: pos += 3*(2**((raw[10]&7)+1))
        delay = 0
        def blocks(p):
            while True:
                require(p < len(raw), 'GIF truncated block')
                size=raw[p]; p+=1
                if not size: return p
                p+=size; require(p<=len(raw),'GIF block bounds')
        while True:
            require(pos<len(raw),'GIF missing trailer'); tag=raw[pos];pos+=1
            if tag==0x3b:
                require(pos==len(raw),'GIF trailer');break
            if tag==0x21:
                require(pos<len(raw),'GIF extension'); label=raw[pos];pos+=1
                if label==0xf9:
                    require(raw[pos:pos+1]==b'\x04' and pos+6<=len(raw),'GIF control')
                    delay=int.from_bytes(raw[pos+2:pos+4],'little')*10
                pos=blocks(pos)
            elif tag==0x2c:
                require(pos+9<=len(raw),'GIF frame')
                x,y,fw,fh,packed=struct.unpack('<HHHHB',raw[pos:pos+9]);pos+=9
                require(fw and fh and x+fw<=w and y+fh<=h,'GIF frame bounds')
                if packed&128:pos+=3*(2**((packed&7)+1))
                require(pos<len(raw),'GIF code size');pos+=1;pos=blocks(pos)
                durations.append(delay);delay=0
                require(len(durations)<=96,'frame budget')
            else: raise ValueError('GIF block type')
        fmt='GIF'
    elif raw[:4]==b'RIFF' and raw[8:12]==b'WEBP':
        require(ext=='webp' and len(raw)>=20 and int.from_bytes(raw[4:8],'little')+8==len(raw),'WebP type/length')
        pos=12;w=h=0
        while pos<len(raw):
            require(pos+8<=len(raw),'WebP chunk');tag=raw[pos:pos+4];size=int.from_bytes(raw[pos+4:pos+8],'little');p=pos+8;end=p+size
            require(end<=len(raw),'WebP chunk bounds')
            if tag==b'VP8X':
                require(size==10,'WebP VP8X');w=int.from_bytes(raw[p+4:p+7],'little')+1;h=int.from_bytes(raw[p+7:p+10],'little')+1
            elif tag==b'VP8 ' and not w:
                require(size>=10 and raw[p+3:p+6]==b'\x9d\x01\x2a','WebP VP8');w=int.from_bytes(raw[p+6:p+8],'little')&16383;h=int.from_bytes(raw[p+8:p+10],'little')&16383
            elif tag==b'VP8L' and not w:
                require(size>=5 and raw[p]==0x2f,'WebP VP8L');v=int.from_bytes(raw[p+1:p+5],'little');w=(v&16383)+1;h=((v>>14)&16383)+1
            elif tag==b'ANMF':
                require(size>=16 and w and h,'WebP frame');x=2*int.from_bytes(raw[p:p+3],'little');y=2*int.from_bytes(raw[p+3:p+6],'little');fw=1+int.from_bytes(raw[p+6:p+9],'little');fh=1+int.from_bytes(raw[p+9:p+12],'little')
                require(x+fw<=w and y+fh<=h,'WebP frame bounds');durations.append(int.from_bytes(raw[p+12:p+15],'little'));require(len(durations)<=96,'frame budget')
            pos=end+(size&1)
        require(pos==len(raw),'WebP padding');fmt='WEBP'
        if not durations:durations=[0]
    elif raw[:2]==b'\xff\xd8':
        require(ext in ('jpg','jpeg') and raw[-2:]==b'\xff\xd9','JPEG type/trailer')
        pos=2;w=h=0
        while pos<len(raw)-2:
            require(raw[pos]==255,'JPEG marker');pos+=1
            while pos<len(raw) and raw[pos]==255:pos+=1
            require(pos<len(raw),'JPEG marker');tag=raw[pos];pos+=1
            if tag==0xda:break
            if tag in (0xd8,0xd9) or 0xd0<=tag<=0xd7:continue
            require(pos+2<=len(raw),'JPEG segment');size=int.from_bytes(raw[pos:pos+2],'big')
            require(size>=2 and pos+size<=len(raw),'JPEG segment bounds')
            if tag in (0xc0,0xc1,0xc2):
                require(size>=8,'JPEG SOF');h=int.from_bytes(raw[pos+3:pos+5],'big');w=int.from_bytes(raw[pos+5:pos+7],'big')
            pos+=size
        fmt='JPEG';durations=[0]
    else:raise ValueError('unsupported image magic')
    require(1<=w<=2048 and 1<=h<=2048 and 1<=len(durations)<=96,'image/frame budget')
    require(sum(durations)<=20000,'animation duration budget')
    return {'width':w,'height':h,'frames':len(durations),'durations':durations,'format':fmt,'pixels':w*h*len(durations)}


def strict_png(raw, max_width, max_height):
    """Parse stripped RGB/RGBA PNGs without loading a native image decoder.

    Bound inflation to the exact non-interlaced scanline budget, reject ancillary
    chunks, invalid filters, duplicate headers and trailing/compressed payloads.
    """
    require(isinstance(raw,bytes) and 0<len(raw)<=LIMITS['poster'],'PNG byte budget')
    require(raw[:8]==b'\x89PNG\r\n\x1a\n','PNG signature')
    pos=8;header=None;compressed=bytearray();ended=False;chunks=0
    while pos<len(raw):
        require(pos+12<=len(raw),'PNG chunk header')
        size=int.from_bytes(raw[pos:pos+4],'big');tag=raw[pos+4:pos+8];end=pos+12+size
        require(end<=len(raw),'PNG chunk bounds')
        require(binascii.crc32(raw[pos+4:pos+8+size])==int.from_bytes(raw[pos+8+size:end],'big'),'PNG CRC')
        chunks+=1;require(chunks<=4096,'PNG chunk budget')
        data=raw[pos+8:pos+8+size]
        if header is None:
            require(tag==b'IHDR' and size==13,'PNG first header')
            w,h,depth,color,compression,filtering,interlace=struct.unpack('>IIBBBBB',data)
            require(1<=w<=max_width and 1<=h<=max_height,'PNG canvas budget')
            require(depth==8 and color in (2,6) and compression==filtering==interlace==0,'PNG RGB/RGBA format')
            header=(w,h,color)
        elif tag==b'IDAT':
            compressed.extend(data)
        elif tag==b'IEND':
            require(size==0 and compressed and end==len(raw),'PNG end/trailer')
            ended=True
        else:
            raise ValueError('PNG must be stripped IHDR/IDAT/IEND')
        pos=end
    require(ended and header is not None,'PNG missing end')
    w,h,color=header;stride=w*(3 if color==2 else 4)+1;budget=stride*h
    inflater=zlib.decompressobj()
    try:pixels=inflater.decompress(compressed,budget+1)
    except zlib.error:raise ValueError('PNG compressed stream') from None
    require(len(pixels)==budget and inflater.eof and not inflater.unused_data and not inflater.unconsumed_tail,'PNG scanline budget/stream')
    require(all(pixels[offset]<=4 for offset in range(0,budget,stride)),'PNG scanline filter')
    return {'width':w,'height':h,'frames':1,'format':'PNG'}


def composition_text(spec):
    """Extract only typed human text; never filenames, effects or sidecar data."""
    if spec is None:return []
    values=(spec['name'],spec['badge']['title'],spec['badge'].get('kanji',''),spec['card']['words']['tagline'],spec['card']['words']['epithet'])
    result=[]
    for (field,limit),value in zip(COMPOSITION_TEXT_FIELDS,values):
        require(isinstance(value,str) and len(value)<=limit,'composition text budget')
        result.append({'field':field,'value':value})
    return result


def spec_validate(spec, images):
    exact(spec, ('version','name','badge','card')); require(type(spec['version']) is int and spec['version']==1,'design version')
    b=spec['badge'];c=spec['card'];exact(b,('emblem','title','accent'),('kanji',));exact(c,('palette','art','effects','words'),('background','frame','glow'))
    exact(c['palette'],('ink','accent','plate','rim'));exact(c['words'],('tagline','epithet'))
    for text,lo,hi in ((spec['name'],1,24),(b['title'],1,12),(b.get('kanji',''),0,4),(c['words']['tagline'],0,36),(c['words']['epithet'],0,12)):
        require(isinstance(text,str) and lo<=len(text)<=hi and text==text.strip() and text==unicodedata.normalize('NFC',text) and not any(unicodedata.category(ch).startswith('C') or ch in '<>&' for ch in text),'design text')
    def color(value):require(isinstance(value,str) and re.fullmatch('#[0-9a-fA-F]{6}',value),'design color')
    color(b['accent'])
    for value in c['palette'].values():color(value)
    def lum(v):
        rgb=[int(v[i:i+2],16)/255 for i in (1,3,5)];rgb=[x/12.92 if x<=.04045 else ((x+.055)/1.055)**2.4 for x in rgb]
        return sum(x*y for x,y in zip(rgb,(.2126,.7152,.0722)))
    a,d=sorted((lum(c['palette']['ink']),lum(c['palette']['plate'])));require((d+.05)/(a+.05)>=4.5,'design contrast')
    def ref(name,kind):
        require(name in images,'missing image');v=images[name];w=v['width'];h=v['height']
        if kind=='emblem':require(256<=w<=1024 and 256<=h<=1024 and abs(w/h-1)<=.1,'emblem shape')
        if kind=='frame':require(w>=630 and h>=880 and abs(w/h/(630/880)-1)<=.02,'frame shape')
        if kind=='glow':require(abs(w/h/(560/390)-1)<=.02,'glow shape')
    ref(b['emblem'],'emblem')
    for key in ('background','frame','glow'):
        if c.get(key):ref(c[key],key)
    require(isinstance(c['art'],list) and 1<=len(c['art'])<=4,'art count')
    def unit(v):require(type(v) in (int,float) and math.isfinite(v) and 0<=v<=1,'design unit number')
    for art in c['art']:
        exact(art,('image','depth','blend'));ref(art['image'],'art');unit(art['depth']);require(art['blend'] in ('normal','screen','add'),'blend')
    require(isinstance(c['effects'],list) and len(c['effects'])<=6,'effect count');seen=set()
    for e in c['effects']:
        exact(e,('id','strength','speed','color'));require(e['id'] in ('foil','sweep','diamond','hexgrid','circuit','scan','glitch','embers','aurora') and e['id'] not in seen,'effect id');seen.add(e['id']);unit(e['strength']);unit(e['speed']);color(e['color'])
    return spec


def inspect_pack(raw, kind='design', inventory=None):
    files=archive(raw,inventory);infos={n:image_info(d,n) for n,d in files.items() if n!='design.json'}
    require(sum(i['pixels'] for i in infos.values())<=64_000_000,'total decoded pixel budget')
    if kind=='design':
        require('design.json' in files and len(files['design.json'])<=65536,'design JSON missing/budget');spec=spec_validate(loads(files['design.json']),infos)
    else:
        require('design.json' not in files and len(infos)==1,'hero requires exactly one image');spec=None
    return files,infos,spec


def pack(files):
    out=io.BytesIO()
    with zipfile.ZipFile(out,'w',compression=zipfile.ZIP_STORED) as z:
        for name,data in sorted(files.items()):
            info=zipfile.ZipInfo(name,(1980,1,1,0,0,0));info.external_attr=(stat.S_IFREG|0o600)<<16;z.writestr(info,data)
    require(len(out.getvalue())<=LIMITS['canonical'],'canonical archive budget')
    return out.getvalue()


def canonical_pack(raw,kind):
    files,infos,spec=inspect_pack(raw,kind)
    require(pack(files)==raw,'canonical ZIP layout differs')
    for name,data in files.items():
        if name=='design.json':
            require(json.dumps(spec,sort_keys=True,separators=(',',':'),ensure_ascii=False,allow_nan=False).encode()==data,'canonical design JSON differs')
        elif name.endswith('.png'):
            require(data[24:29]==b'\x08\x06\x00\x00\x00','canonical PNG must be 8-bit RGBA')
            position=8
            while position<len(data):
                size=int.from_bytes(data[position:position+4],'big')
                require(data[position+4:position+8] in (b'IHDR',b'IDAT',b'IEND'),'canonical PNG metadata/unknown chunk')
                position+=12+size
        elif name.endswith('.webp'):
            position=12
            while position<len(data):
                tag=data[position:position+4];size=int.from_bytes(data[position+4:position+8],'little');start=position+8;end=start+size
                require(tag in (b'VP8X',b'ANIM',b'ANMF',b'VP8L'),'canonical WebP metadata/unknown chunk')
                if tag==b'VP8X':require(not data[start]&~0x12,'canonical WebP metadata flags')
                if tag==b'ANMF':
                    inner=start+16
                    while inner<end:
                        length=int.from_bytes(data[inner+4:inner+8],'little')
                        require(data[inner:inner+4]==b'VP8L' and inner+8+length<=end,'canonical WebP frame chunk')
                        inner+=8+length+(length&1)
                    require(inner==end,'canonical WebP frame padding')
                position=end+(size&1)
        else:raise ValueError('canonical image extension')
    return files,infos,spec
