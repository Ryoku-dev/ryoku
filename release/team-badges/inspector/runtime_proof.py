#!/usr/bin/env python3
"""Parent-run proof: real decoder + real Qt containers, public synthetic keys only.

The fixture transport below does not prove HTTP authentication; GateIntake's
integration tests cover that boundary separately. It cannot run in the service.
"""
import argparse
import base64
import json
import os
from pathlib import Path
import tempfile
import threading
import time

from fixtures import TEXT_SENTINEL, capacity_fixtures, fixtures, signed_submission
from policy import LIMITS, canonical, canonical_pack, composition_text, loads, require, sha
from supervisor import ContainerFailure, Supervisor, check_composition, run_container


class FixtureTransport:
    def __init__(self,raw,out,key,kind):self.raw=raw;self.out=out;self.key=key;self.kind=kind;self.objects={};self.result=None;self.failures=[]

    def request(self,method,path,body=None,lease=None,limit=256*1024,mime=None):
        if method=='GET' and path.endswith('/input'):
            require(lease=='runtime-proof-only-lease-000000000001','proof lease binding');return self.raw
        require(method=='PUT' and '/objects/' in path and lease=='runtime-proof-only-lease-000000000001','proof transport boundary')
        name=path.rsplit('/',1)[-1];require(name in LIMITS and len(body)<=LIMITS[name],'proof object boundary')
        self.objects[name]=body;return b'{}'

    def post(self,path,obj):
        if path.endswith('/heartbeat'):return {'expiresAt':int(time.time())+600}
        if path.endswith('/failure'):self.failures.append(obj['reason']);return {}
        require(path.endswith('/result') and obj['lease']=='runtime-proof-only-lease-000000000001','proof result boundary')
        certificate=obj['certificate'];raw=base64.b64decode(certificate['payload']);self.key.verify(base64.b64decode(certificate['sig']),b'ryoku-inspect-v1\0'+raw)
        p=loads(raw);require(raw==canonical(p) and p['moderation']=='unreviewed','proof canonical certificate')
        require('lease' not in p and p['leaseSha']==sha(obj['lease'].encode()),'proof secret capability not persisted')
        require(set(self.objects)==set(LIMITS),'proof missing object')
        for item in p['files']:
            require(item['bytes']==len(self.objects[item['name']]) and item['sha']==sha(self.objects[item['name']]),'proof certified object hashes')
        report=loads(self.objects['report'],LIMITS['report'])
        composition=check_composition(report['composition'],self.objects['canonical'],self.objects['poster'],self.objects['preview'],self.kind)
        require(composition['canonicalSha']==p['canonicalSha'] and composition['posterSha']==p['posterSha'] and composition['previewSha']==p['previewSha'] and sha(self.objects['report'])==p['reportSha'],'proof composition certificate binding')
        (self.out/'composition.png').write_bytes(base64.b64decode(composition['image']['base64']))
        (self.out/'composition-text.json').write_text(json.dumps(composition['text'],ensure_ascii=False,indent=2)+'\n')
        for name,data in self.objects.items():(self.out/name).write_bytes(data)
        (self.out/'certificate.json').write_text(json.dumps(certificate,sort_keys=True)+'\n');self.result=p
        return {'state':'needs_review'}


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--image',required=True);parser.add_argument('--renderer-manifest-sha',required=True);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--include-hostile',action='store_true');parser.add_argument('--capacity',action='store_true',help='Run maximum-frame/near-pixel-budget and maximum-canvas fixtures instead of the small functional set');args=parser.parse_args()
    require(os.geteuid()==0,'proof requires fixed root-only containment launcher')
    os.umask(0o077);args.out.mkdir(parents=True,exist_ok=True,mode=0o700)
    state=Path('/var/lib/ryoku-inspector/incoming');require(state.is_dir(),'bootstrap incoming directory missing')
    config={'launcher':'/usr/local/sbin/ryoku-inspector-run','image':args.image,'rendererManifestSha':args.renderer_manifest_sha,'audience':'https://team.ryoku.dev','keyId':'synthetic-runtime-proof'}
    inputs=capacity_fixtures() if args.capacity else fixtures();evidence=[]
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
    for name in inputs if args.capacity else ('valid-design','valid-design-text-sentinel','valid-hero-gif','valid-hero-webp','valid-hero-partial-review'):
        kind='hero' if 'hero' in name else 'design';raw=inputs[name]
        out=args.out/name;out.mkdir(mode=0o700)
        worker=Supervisor.__new__(Supervisor);worker.config=config;worker.state=state;worker.stop=threading.Event();worker.key=Ed25519PrivateKey.from_private_bytes(bytes(reversed(range(32))))
        transport=FixtureTransport(raw,out,worker.key.public_key(),kind);worker.gate=transport
        job={'jobId':'00000000-0000-4000-8000-000000000001','lease':'runtime-proof-only-lease-000000000001','inputPath':'/internal/inspection/00000000-0000-4000-8000-000000000001/input','manifest':signed_submission(raw,kind),'expiresAt':int(time.time())+600}
        try:worker.process(job)
        except ContainerFailure as error:
            (out/'worker-stderr.txt').write_bytes(error.stderr)
            raise
        require(transport.result is not None and not transport.failures,'proof certificate missing')
        report=loads(transport.objects['report'],LIMITS['report'])
        complete=report['review']['totalFrames']<=8
        require(report['review']['complete'] is complete and len(report['review']['images'])==min(8,report['totalFrames']) and report['totalFrames']==report['review']['totalFrames'],'proof review coverage evidence')
        _,_,spec=canonical_pack(transport.objects['canonical'],kind)
        require(report['composition']['text']==composition_text(spec),'proof complete canonical text')
        if name=='valid-design-text-sentinel':
            require({'field':'card.words.tagline','value':TEXT_SENTINEL} in report['composition']['text'],'proof sentinel canonical words absent')
        if kind=='hero':require(report['composition']['text']==[],'proof hero has no custom design text')
        certificate=loads((out/'certificate.json').read_bytes())
        evidence.append({'fixture':name,'actualContainerStages':['decode','render'],'moderation':transport.result['moderation'],'canonicalSha':transport.result['canonicalSha'],'reportSha':transport.result['reportSha'],'certificateSha':sha(base64.b64decode(certificate['payload'])),'reviewFrames':report['review']['totalFrames'],'reviewComplete':complete,'compositionSha':report['composition']['image']['sha256'],'compositionCoverage':report['composition']['coverage'],'compositionText':report['composition']['text']})
    if args.include_hostile:
        for name,raw in (fixtures() if args.capacity else inputs).items():
            if not name.startswith('hostile-'):continue
            with tempfile.TemporaryDirectory(prefix='inspection-',dir=state) as directory:
                path=Path(directory)/'hostile.zip';path.write_bytes(raw)
                try:run_container(config,path,'decode','design',threading.Event())
                except ValueError:
                    evidence.append({'fixture':name,'actualContainerStages':['decode'],'rejected':True});continue
                raise ValueError('hostile container input unexpectedly passed: '+name)
    (args.out/'runtime-proof.json').write_text(json.dumps({'v':1,'transport':'fixture-only-not-HTTP-proof','image':args.image,'rendererManifestSha':args.renderer_manifest_sha,'evidence':evidence},indent=2)+'\n')
    print(str(args.out/'runtime-proof.json'))


if __name__=='__main__':main()
