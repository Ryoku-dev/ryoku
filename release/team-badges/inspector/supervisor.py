#!/usr/bin/env python3
"""Authenticated lease queue supervisor. Only this process may sign certificates.

The hostile decoder and independent real Qt renderer run in separate fixed,
credential-free containers. The host never decodes images or accepts pack code.
"""
import argparse
import base64
import fcntl
import io
import json
import os
from pathlib import Path
import re
import selectors
import signal
import stat
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

from policy import LIMITS, MIB, REVIEW_IMAGE_BYTES, VERSION, POLICY, canonical, canonical_pack, composition_text, exact, image_info, inspect_pack, loads, manifest, require, sha, strict_png

MIME={'canonical':'application/zip','poster':'image/png','preview':'image/gif','report':'application/json'}


class ContainerFailure(ValueError):
    def __init__(self,status,stderr):
        super().__init__('container failed')
        self.status=status
        self.stderr=stderr


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self,*args,**kwargs):
        raise ValueError('gate redirect refused')


def private_file(path,limit):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    with os.fdopen(fd,'rb') as f:
        s=os.fstat(f.fileno())
        require(stat.S_ISREG(s.st_mode) and s.st_uid==os.geteuid() and not s.st_mode&0o077 and s.st_size<=limit,'private config/key mode/owner/size')
        raw=f.read(limit+1);require(len(raw)<=limit,'private file budget');return raw


class Gate:
    def __init__(self,url,token):
        parsed=urllib.parse.urlsplit(url)
        require(parsed.scheme=='https' and parsed.hostname and not parsed.username and not parsed.password and parsed.path in ('','/') and not parsed.query and not parsed.fragment,'HTTPS origin required')
        require(token and len(token)<=4096 and '\n' not in token and '\r' not in token,'worker token')
        self.url=url.rstrip('/');self.token=token
        self.opener=urllib.request.build_opener(NoRedirect())

    def request(self,method,path,body=None,lease=None,limit=256*1024,mime='application/json'):
        require(path.startswith('/internal/inspection/') and '?' not in path and '#' not in path,'internal fixed path required')
        headers={'Authorization':'Bearer '+self.token,'Accept':'application/json'}
        if body is not None:headers['Content-Type']=mime;headers['Content-Length']=str(len(body))
        if lease is not None:headers['X-Ryoku-Lease']=lease
        request=urllib.request.Request(self.url+path,data=body,headers=headers,method=method)
        with self.opener.open(request,timeout=20) as response:
            require(response.status in (200,201,202,204),'gate response')
            length=response.headers.get('Content-Length')
            if length:require(length.isdigit() and int(length)<=limit,'gate length budget')
            raw=response.read(limit+1);require(len(raw)<=limit,'gate response budget')
            if length:require(len(raw)==int(length),'gate truncated body')
            return raw

    def post(self,path,obj):
        raw=self.request('POST',path,canonical(obj))
        return loads(raw) if raw else None


def framed(raw,names):
    require(len(raw)<=sum(LIMITS[n] for n in names)+8192,'worker stream budget')
    f=io.BytesIO(raw);require(f.readline(64)==b'RYOKU-OBJECTS-1\n','worker magic')
    objects={}
    while True:
        header=f.readline(512)
        if header==b'END\n':break
        require(header.endswith(b'\n'),'worker header')
        item=loads(header);exact(item,('name','bytes','sha'));name=item['name']
        require(name in names and name not in objects and type(item['bytes']) is int and 0<item['bytes']<=LIMITS[name],'worker object name/size')
        data=f.read(item['bytes']);require(len(data)==item['bytes'] and sha(data)==item['sha'] and f.read(1)==b'\n','worker object hash/length')
        objects[name]=data
    require(set(objects)==set(names) and f.read()==b'','worker object inventory/trailer')
    return objects


def run_container(config,input_path,stage,kind,cancel):
    args=[config['launcher'],'--renderer',config['image'],str(input_path),'/usr/local/bin/python','/opt/ryoku-inspector/worker.py',stage,'--kind',kind]
    process=subprocess.Popen(args,stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True,env={'PATH':'/usr/sbin:/usr/bin:/sbin:/bin','HOME':'/nonexistent','LANG':'C.UTF-8'})
    selector=selectors.DefaultSelector();selector.register(process.stdout,selectors.EVENT_READ,0);selector.register(process.stderr,selectors.EVENT_READ,1)
    buffers=[bytearray(),bytearray()];caps=[sum(LIMITS.values())+8192,65536];deadline=time.monotonic()+195
    try:
        while selector.get_map():
            require(not cancel.is_set(),'lease lost or shutdown')
            require(time.monotonic()<deadline,'container deadline')
            for key,_ in selector.select(.25):
                chunk=os.read(key.fileobj.fileno(),65536)
                if not chunk:selector.unregister(key.fileobj);continue
                buffers[key.data].extend(chunk);require(len(buffers[key.data])<=caps[key.data],'container output cap')
        status=process.wait(timeout=5)
        if status:raise ContainerFailure(status,bytes(buffers[1]))
        return framed(bytes(buffers[0]),('canonical','report') if stage=='decode' else ('poster','preview','report'))
    finally:
        selector.close()
        if process.poll() is None:
            os.killpg(process.pid,signal.SIGTERM)
            try:process.wait(timeout=8)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid,signal.SIGKILL);process.wait()
        process.stdout.close();process.stderr.close()


def check_decode(original,decoded,kind):
    _,sourceinfos,source_spec=inspect_pack(original,kind)
    _,targetinfos,target_spec=canonical_pack(decoded['canonical'],kind)
    require(composition_text(source_spec)==composition_text(target_spec),'canonical text changed during decode')
    report=loads(decoded['report'],LIMITS['report'])
    exact(report,('v','pipelineVersion','policyVersion','assets','moderation','classificationPerformed','canonicalSha','review'))
    require(report['v']==1 and report['pipelineVersion']==VERSION and report['policyVersion']==POLICY and report['moderation']=='unreviewed' and report['classificationPerformed'] is False and report['canonicalSha']==sha(decoded['canonical']),'decode report provenance')
    require(isinstance(report['assets'],list) and len(report['assets'])==len(sourceinfos)==len(targetinfos),'decode coverage inventory')
    seen=set();targets=set()
    for fact in report['assets']:
        exact(fact,('source','canonical','width','height','sourceFrames','canonicalFrames','durations','loop','decodedPixels','allFramesDecoded'))
        src=fact['source'];dst=fact['canonical'];require(src in sourceinfos and dst in targetinfos and src not in seen and dst not in targets,'decode coverage names');seen.add(src);targets.add(dst)
        a=sourceinfos[src];b=targetinfos[dst]
        require(fact['width']==a['width']==b['width'] and fact['height']==a['height']==b['height'] and fact['sourceFrames']==a['frames'] and fact['canonicalFrames']==b['frames'] and fact['durations']==a['durations'] and sum(a['durations'])==sum(b['durations']) and fact['decodedPixels']==a['pixels'] and fact['allFramesDecoded'] is True,'decode coverage facts')
    review=report['review'];exact(review,('complete','totalFrames','images'))
    total=sum(info['frames'] for info in sourceinfos.values())
    require(review['totalFrames']==total and review['complete'] is (total<=8) and isinstance(review['images'],list) and len(review['images'])==min(total,8),'review completeness')
    selected=set(range(total)) if total<=8 else {round(i*(total-1)/7) for i in range(8)}
    positions={};offset=0
    for name,info in sorted(sourceinfos.items()):
        for index in range(info['frames']):positions[(name,index)]=offset+index
        offset+=info['frames']
    observed=set()
    for item in review['images']:
        exact(item,('mime','sha256','base64','sourceAsset','frameIndex'))
        require(item['mime']=='image/png' and type(item['frameIndex']) is int and (item['sourceAsset'],item['frameIndex']) in positions,'review source binding')
        position=positions[(item['sourceAsset'],item['frameIndex'])]
        require(position in selected and position not in observed,'review frame inventory');observed.add(position)
        require(isinstance(item['base64'],str) and len(item['base64'])<=1024*1024,'review thumbnail wire budget')
        thumbnail=base64.b64decode(item['base64'],validate=True)
        require(base64.b64encode(thumbnail).decode()==item['base64'] and sha(thumbnail)==item['sha256'],'review thumbnail digest')
        strict_png(thumbnail,384,384)
    require(observed==selected,'review missing frame')
    return report,targetinfos


def check_composition(composition,canonical_data,poster,preview,kind):
    exact(composition,('v','canonicalSha','posterSha','previewSha','text','image','coverage'))
    require(type(composition['v']) is int and composition['v']==1 and composition['coverage']=='poster-and-canonical-text','composition version/coverage')
    require(composition['canonicalSha']==sha(canonical_data) and composition['posterSha']==sha(poster) and composition['previewSha']==sha(preview),'composition object binding')
    _,_,spec=canonical_pack(canonical_data,kind)
    require(composition['text']==composition_text(spec),'composition canonical text binding')
    image=composition['image'];exact(image,('mime','sha256','base64'))
    require(image['mime']=='image/png' and isinstance(image['base64'],str) and len(image['base64'])<=REVIEW_IMAGE_BYTES*4//3,'composition image wire budget')
    thumbnail=base64.b64decode(image['base64'],validate=True)
    require(0<len(thumbnail)<=REVIEW_IMAGE_BYTES and base64.b64encode(thumbnail).decode()==image['base64'] and sha(thumbnail)==image['sha256'],'composition image digest')
    info=strict_png(thumbnail,384,384)
    require(info['height']==384 and abs(info['width']-1260*384/1760)<=1,'composition whole-poster geometry')
    return composition


def check_render(rendered,canonical_data,targetinfos,expected_renderer,kind):
    report=loads(rendered['report'],LIMITS['report'])
    exact(report,('v','pipelineVersion','policyVersion','rendererVersion','rendererManifestSha','canonicalSha','coverage','previewFrames','previewMilliseconds','previewWidth','previewColors','posterWidth','posterHeight','warnings','moderation','classificationPerformed','composition'))
    require(report['v']==1 and report['pipelineVersion']==VERSION and report['policyVersion']==POLICY and report['canonicalSha']==sha(canonical_data) and report['rendererManifestSha']==expected_renderer,'renderer provenance')
    require(report['moderation']=='unreviewed' and report['classificationPerformed'] is False,'moderation is not a technical verdict')
    require(isinstance(report['coverage'],list) and len(report['coverage'])==len(targetinfos),'Qt coverage inventory')
    seen=set()
    for fact in report['coverage']:
        exact(fact,('name','qtDecodedFrames','frames'));name=fact['name'];require(name in targetinfos and name not in seen,'Qt coverage name');seen.add(name)
        require(fact['qtDecodedFrames']==fact['frames']==targetinfos[name]['frames'],'Qt full frame coverage')
    poster=strict_png(rendered['poster'],1260,1760);preview=image_info(rendered['preview'],'preview.gif')
    require(poster['width']==report['posterWidth']==1260 and poster['height']==report['posterHeight']==1760 and poster['frames']==1,'real card poster geometry')
    # GIF can coalesce identical whole-card frames; time is invariant, not encoded count.
    require(1<=preview['frames']<=report['previewFrames']<=96 and 3000<=report['previewMilliseconds']<=8000 and sum(preview['durations'])==report['previewMilliseconds'] and preview['width']==report['previewWidth'],'preview timeline/geometry')
    require(isinstance(report['warnings'],list) and len(report['warnings'])<=20,'warning budget')
    check_composition(report['composition'],canonical_data,rendered['poster'],rendered['preview'],kind)
    return report


class Supervisor:
    def __init__(self,config):
        exact(config,('gateOrigin','audience','tokenFile','signingKeyFile','keyId','image','rendererManifestSha','launcher','stateDir'))
        require(re.fullmatch(r'(?:[a-zA-Z0-9][a-zA-Z0-9._:/-]*@)?sha256:[a-f0-9]{64}',config['image']),'immutable image reference required')
        require(re.fullmatch('[a-f0-9]{64}',config['rendererManifestSha']),'pinned renderer manifest required')
        require(re.fullmatch('[A-Za-z0-9._-]{1,64}',config['keyId']),'narrow inspector key id')
        require(config['launcher']=='/usr/local/sbin/ryoku-inspector-run' and config['stateDir']=='/var/lib/ryoku-inspector/incoming','fixed launcher/state boundary')
        require(config['audience'].startswith('https://'),'submission audience')
        self.config=config;self.gate=Gate(config['gateOrigin'],private_file(config['tokenFile'],4096).decode().strip())
        from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
        self.key=Ed25519PrivateKey.from_private_bytes(private_file(config['signingKeyFile'],32))
        self.stop=threading.Event()
        self.state=Path(config['stateDir']);self.state.mkdir(parents=True,exist_ok=True,mode=0o700)
        s=self.state.lstat();require(stat.S_ISDIR(s.st_mode) and s.st_uid==os.geteuid() and not s.st_mode&0o077,'private incoming directory')
        self.lock=open(self.state/'supervisor.lock','a+b');fcntl.flock(self.lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
        # Crash recovery only touches our private, direct-child temporary directories;
        # it never resumes/replays stale capabilities. Gate owns lease expiry/retries.
        import shutil
        for path in self.state.glob('inspection-*'):
            s=path.lstat()
            require(stat.S_ISDIR(s.st_mode) and s.st_uid==os.geteuid() and not s.st_mode&0o077,'unsafe recovery path')
            shutil.rmtree(path)

    def process(self,job):
        exact(job,('jobId','lease','inputPath','manifest','expiresAt'))
        require(re.fullmatch('[a-f0-9-]{36}',job['jobId']) and isinstance(job['lease'],str) and re.fullmatch('[A-Za-z0-9_-]{22,256}',job['lease']),'job/lease identity')
        base='/internal/inspection/'+job['jobId'];lease=job['lease']
        require(job['inputPath']==base+'/input','input path binding')
        require(type(job['expiresAt']) is int and job['expiresAt']>int(time.time()),'expired lease')
        p,manifest_sha=manifest(job['manifest'],self.config['audience'])
        cancel=threading.Event();finished=threading.Event()
        def heartbeat():
            while not finished.wait(10):
                if self.stop.is_set():cancel.set();return
                try:
                    response=self.gate.post(base+'/heartbeat',{'lease':lease})
                    require(isinstance(response,dict) and type(response.get('expiresAt')) is int and response['expiresAt']>int(time.time()),'lease renewal')
                except Exception:cancel.set();return
        thread=threading.Thread(target=heartbeat,daemon=True);thread.start()
        try:
            original=self.gate.request('GET',job['inputPath'],lease=lease,limit=8*MIB)
            require(sha(original)==p['originalSha'],'original hash mismatch')
            inspect_pack(original,p['kind'],p['files'])
            with tempfile.TemporaryDirectory(prefix='inspection-',dir=self.state) as directory:
                directory=Path(directory);input_path=directory/'original.zip';input_path.write_bytes(original);input_path.chmod(0o600)
                decoded=run_container(self.config,input_path,'decode',p['kind'],cancel)
                decode_report,targetinfos=check_decode(original,decoded,p['kind'])
                canonical_path=directory/'canonical.zip';canonical_path.write_bytes(decoded['canonical']);canonical_path.chmod(0o600)
                rendered=run_container(self.config,canonical_path,'render',p['kind'],cancel)
                render_report=check_render(rendered,decoded['canonical'],targetinfos,self.config['rendererManifestSha'],p['kind'])
                require(not cancel.is_set() and not self.stop.is_set(),'lease lost or shutdown')
                # Authority and technical flags are constructed here, never copied from
                # worker JSON. No classifier is configured: human review is mandatory.
                preview_info=image_info(rendered['preview'],'preview.gif')
                coverage={'assets':[{'name':name,'frames':info['frames'],'durations':info['durations'],'durationMs':sum(info['durations'])} for name,info in sorted(targetinfos.items())],'totalFrames':sum(info['frames'] for info in targetinfos.values()),'durationMs':max(sum(info['durations']) for info in targetinfos.values()),'previewFrames':preview_info['frames'],'previewDurationMs':sum(preview_info['durations'])}
                review=decode_report.pop('review')
                composition=render_report.pop('composition')
                report=json.dumps({'v':1,'pipelineVersion':VERSION,'policyVersion':POLICY,'originalSha':p['originalSha'],'canonicalSha':sha(decoded['canonical']),'totalFrames':review['totalFrames'],'durationMs':max(sum(asset['durations']) for asset in decode_report['assets']),'decode':decode_report,'render':render_report,'coverage':coverage,'review':review,'composition':composition,'moderation':'unreviewed','classificationPerformed':False},ensure_ascii=False,sort_keys=True,separators=(',',':'),allow_nan=False).encode()
                require(len(report)<=LIMITS['report'],'trusted report budget')
                objects={'canonical':decoded['canonical'],'poster':rendered['poster'],'preview':rendered['preview'],'report':report}
                inventory=[{'name':n,'sha':sha(data),'bytes':len(data)} for n,data in sorted(objects.items())]
                payload={'v':1,'keyId':self.config['keyId'],'jobId':job['jobId'],'leaseSha':sha(lease.encode()),'manifestSha':manifest_sha,'originalSha':p['originalSha'],'canonicalSha':sha(objects['canonical']),'files':inventory,'posterSha':sha(objects['poster']),'previewSha':sha(objects['preview']),'reportSha':sha(report),'pipelineVersion':VERSION,'policyVersion':POLICY,'inspectedAt':int(time.time()),'technical':{'archive':'pass','decode':'pass','canonical':'pass','render':'pass'},'moderation':'unreviewed'}
                raw=canonical(payload);certificate={'payload':base64.b64encode(raw).decode(),'sig':base64.b64encode(self.key.sign(b'ryoku-inspect-v1\0'+raw)).decode()}
                for name,data in sorted(objects.items()):
                    require(not cancel.is_set(),'lease lost')
                    self.gate.request('PUT',base+'/objects/'+name,data,lease=lease,mime=MIME[name])
                require(not cancel.is_set(),'lease lost')
                self.gate.post(base+'/result',{'lease':lease,'certificate':certificate})
        except Exception:
            # Never persist capabilities, original text, HTTP bodies, QML warnings,
            # keys, or hostile filenames in service logs/failure reasons.
            try:self.gate.post(base+'/failure',{'lease':lease,'reason':'inspection technical failure'})
            except Exception:pass  # Gate lease expiry is authoritative if transport died.
            raise
        finally:
            finished.set();thread.join(timeout=21)

    def once(self):
        job=self.gate.post('/internal/inspection/claim',{})
        if job is None:return False
        self.process(job);return True


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--config',required=True);parser.add_argument('--loop',action='store_true');args=parser.parse_args()
    os.umask(0o077)
    supervisor=Supervisor(loads(private_file(args.config,16384)))
    signal.signal(signal.SIGTERM,lambda *_:supervisor.stop.set());signal.signal(signal.SIGINT,lambda *_:supervisor.stop.set())
    while not supervisor.stop.is_set():
        try:
            worked=supervisor.once()
        except Exception as error:
            print('inspection rejected: '+type(error).__name__,flush=True)
            if not args.loop:return 1
            worked=False
        if not args.loop:return 0
        if not worked:supervisor.stop.wait(5)
    return 0


if __name__=='__main__':
    raise SystemExit(main())
