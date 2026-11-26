import base64
import binascii
import copy
import io
import json
from pathlib import Path
import unittest
import struct
import tempfile
import threading
import time
from unittest import mock
import zlib

from fixtures import TEXT_SENTINEL, fixtures, signed_submission
from policy import LIMITS, POLICY, VERSION, archive, canonical, canonical_pack, composition_text, image_info, inspect_pack, loads, manifest, pack, sha, strict_png
from supervisor import Supervisor, framed, check_composition, check_decode, check_render


class ArchivePolicyTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):cls.inputs=fixtures()

    def test_valid_real_design_schema(self):
        files,infos,spec=inspect_pack(self.inputs['valid-design'])
        self.assertEqual(spec['name'],'Inspector proof')
        self.assertEqual(sum(info['frames'] for info in infos.values()),4)
        self.assertEqual(infos['art.gif']['durations'],[70,130,210])

    def test_every_hostile_pack_rejected_before_native_decode(self):
        for name,raw in self.inputs.items():
            if name.startswith('hostile-'):
                with self.subTest(name=name),self.assertRaises((ValueError,KeyError,TypeError)):
                    inspect_pack(raw)

    def test_manifest_binds_every_original_byte(self):
        raw=self.inputs['valid-design'];envelope=signed_submission(raw,'design')
        p,digest=manifest(envelope,'https://team.ryoku.dev')
        self.assertEqual(p['originalSha'],sha(raw))
        self.assertEqual(digest,sha(base64.b64decode(envelope['payload'])))
        inspect_pack(raw,inventory=p['files'])
        inventory=copy.deepcopy(p['files']);inventory[0]['sha']='0'*64
        with self.assertRaises(ValueError):archive(raw,inventory)
        with self.assertRaises(ValueError):manifest(envelope,'https://other.invalid')

    def test_noncanonical_and_duplicate_json_denied(self):
        with self.assertRaises(ValueError):loads(b'{"a":1,"a":2}')
        for value in (float('nan'),1.5,9007199254740992,'\ud800'):
            with self.subTest(value=repr(value)),self.assertRaises((ValueError,UnicodeError)):
                canonical({'a':value})

    def test_shared_exact_protocol_vector(self):
        v=loads((Path(__file__).parent/'protocol-vectors.json').read_bytes())
        p,digest=manifest({'payload':v['payload'],'sig':v['sig']},'https://team.ryoku.dev')
        self.assertEqual(canonical(p),v['canonicalUtf8'].encode())
        self.assertEqual(digest,v['payloadSha'])
        from cryptography.exceptions import InvalidSignature
        envelope={'payload':v['payload'],'sig':base64.b64encode(bytes(64)).decode()}
        with self.assertRaises(InvalidSignature):manifest(envelope,'https://team.ryoku.dev')
        changed=json.loads(v['canonicalUtf8']);changed['consentToReview']=False
        envelope['payload']=base64.b64encode(canonical(changed)).decode()
        with self.assertRaises(ValueError):manifest(envelope,'https://team.ryoku.dev')


class SyntheticCanonicalTests(unittest.TestCase):
    def test_disposal_alpha_timing_and_complete_review(self):
        from worker import canonicalize
        raw=fixtures()['valid-design']
        output=canonicalize(raw,'design')
        report,infos=check_decode(raw,output,'design')
        self.assertTrue(report['review']['complete'])
        self.assertEqual(report['review']['totalFrames'],4)
        self.assertEqual(len(report['review']['images']),4)
        art=next(f for f in report['assets'] if f['source']=='art.gif')
        self.assertEqual(art['durations'],[70,130,210])
        self.assertEqual(art['loop'],3)
        self.assertEqual(infos[art['canonical']]['durations'],[70,130,210])

    def test_partial_review_never_claims_completeness(self):
        from worker import canonicalize
        raw=fixtures()['valid-hero-partial-review']
        report,_=check_decode(raw,canonicalize(raw,'hero'),'hero')
        self.assertFalse(report['review']['complete'])
        self.assertEqual(report['review']['totalFrames'],9)
        self.assertEqual(len(report['review']['images']),8)

    def test_png_metadata_is_not_carried_forward(self):
        from PIL import Image,PngImagePlugin
        from worker import canonicalize
        from policy import canonical_pack,pack
        image=Image.new('RGBA',(8,8),(30,80,120,90))
        metadata=PngImagePlugin.PngInfo();metadata.add_text('payload','untrusted-metadata-must-not-survive')
        raw=io.BytesIO();image.save(raw,format='PNG',pnginfo=metadata)
        result=canonicalize(pack({'hero.png':raw.getvalue()}),'hero')
        files,_,_=canonical_pack(result['canonical'],'hero')
        self.assertNotIn(b'untrusted-metadata-must-not-survive',next(iter(files.values())))


class OutputBoundaryTests(unittest.TestCase):
    def stream(self,objects):
        out=io.BytesIO();out.write(b'RYOKU-OBJECTS-1\n')
        for name,data in objects:
            out.write(json.dumps({'name':name,'bytes':len(data),'sha':sha(data)}).encode()+b'\n'+data+b'\n')
        out.write(b'END\n');return out.getvalue()

    def test_output_inventory_and_integrity(self):
        valid=self.stream([('canonical',b'zip'),('report',b'{}')])
        self.assertEqual(framed(valid,('canonical','report'))['canonical'],b'zip')
        for raw in (valid+b'junk',valid.replace(b'zip',b'bad'),self.stream([('../escape',b'x')]),self.stream([('canonical',b'x'),('canonical',b'x')])):
            with self.assertRaises(ValueError):framed(raw,('canonical','report'))

    def test_output_size_checked_before_reading(self):
        raw=b'RYOKU-OBJECTS-1\n'+json.dumps({'name':'report','bytes':LIMITS['report']+1,'sha':'0'*64}).encode()+b'\n'
        with self.assertRaises(ValueError):framed(raw,('report',))

    def test_missing_review_and_false_clean_denied(self):
        raw=fixtures()['valid-design']
        with self.assertRaises(ValueError):check_decode(raw,{'canonical':raw,'report':b'{"moderation":"clear"}'},'design')


class CompositionBoundaryTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        from PIL import Image,ImageDraw
        from worker import canonicalize,composition_evidence
        cls.original=fixtures()['valid-design-text-sentinel']
        cls.decoded=canonicalize(cls.original,'design')
        _,cls.infos,cls.spec=canonical_pack(cls.decoded['canonical'],'design')
        # Synthetic full-card pixels test evidence plumbing only. Actual Qt/art
        # fidelity is separately exercised by runtime_proof.py in real containers.
        poster=Image.new('RGBA',(1260,1760),(20,30,40,255))
        ImageDraw.Draw(poster).rectangle((0,0,629,879),fill=(240,120,30,255))
        png=io.BytesIO();poster.save(png,format='PNG');poster.close();cls.poster=png.getvalue()
        gif=io.BytesIO();Image.new('RGB',(630,880),(20,30,40)).save(gif,format='GIF',duration=3000)
        cls.preview=gif.getvalue()
        cls.composition=composition_evidence(cls.decoded['canonical'],cls.poster,cls.preview,'design')
        cls.render_report={'v':1,'pipelineVersion':VERSION,'policyVersion':POLICY,'rendererVersion':'test-only','rendererManifestSha':'1'*64,'canonicalSha':sha(cls.decoded['canonical']),'coverage':[{'name':name,'frames':info['frames'],'qtDecodedFrames':info['frames']} for name,info in sorted(cls.infos.items())],'previewFrames':36,'previewMilliseconds':3000,'previewWidth':630,'previewColors':256,'posterWidth':1260,'posterHeight':1760,'warnings':[],'moderation':'unreviewed','classificationPerformed':False,'composition':cls.composition}

    def rendered(self,report=None):
        return {'poster':self.poster,'preview':self.preview,'report':json.dumps(self.render_report if report is None else report,ensure_ascii=False,separators=(',',':')).encode()}

    def test_benign_art_keeps_every_exact_canonical_word(self):
        from PIL import Image
        expected=[{'field':'name','value':'Canonical text sentinel'},{'field':'badge.title','value':'TEXT PROOF'},{'field':'badge.kanji','value':'文'},{'field':'card.words.tagline','value':TEXT_SENTINEL},{'field':'card.words.epithet','value':'KEEP EXACT'}]
        self.assertEqual(self.composition['text'],expected)
        report=check_render(self.rendered(),self.decoded['canonical'],self.infos,'1'*64,'design')
        self.assertEqual(report['composition']['text'],expected)
        self.assertEqual(self.composition['posterSha'],sha(self.poster))
        with Image.open(io.BytesIO(base64.b64decode(self.composition['image']['base64']))) as thumb:
            self.assertEqual(thumb.getpixel((20,20)),(240,120,30,255))
            self.assertEqual(thumb.getpixel((250,350)),(20,30,40,255))

    def test_missing_optional_kanji_and_hero_have_explicit_text(self):
        from worker import canonicalize,composition_evidence
        spec=copy.deepcopy(self.spec);del spec['badge']['kanji']
        self.assertEqual(composition_text(spec)[2],{'field':'badge.kanji','value':''})
        hero=canonicalize(fixtures()['valid-hero-gif'],'hero')['canonical']
        evidence=composition_evidence(hero,self.poster,self.preview,'hero')
        self.assertEqual(evidence['text'],[])
        check_composition(evidence,hero,self.poster,self.preview,'hero')
        evidence['text']=[{'field':'name','value':'forged'}]
        with self.assertRaises(ValueError):check_composition(evidence,hero,self.poster,self.preview,'hero')

    def test_invalid_composition_png_is_rejected_before_signing(self):
        png=StrictPNGTests()
        report=copy.deepcopy(self.render_report)
        invalid=png.png(width=275,height=384,pixels=b'\0'*(384*(275*4+1)),interlace=1)
        report['composition']['image']={'mime':'image/png','sha256':sha(invalid),'base64':base64.b64encode(invalid).decode()}
        self.assert_rejected_before_signing(self.rendered(report))

    def assert_rejected_before_signing(self,rendered):
        worker=Supervisor.__new__(Supervisor)
        worker.config={'audience':'https://team.ryoku.dev','rendererManifestSha':'1'*64}
        worker.stop=threading.Event();worker.key=mock.Mock();worker.gate=mock.Mock()
        worker.gate.request.return_value=self.original
        jobid='00000000-0000-4000-8000-000000000001'
        job={'jobId':jobid,'lease':'synthetic-test-lease-000000001','inputPath':'/internal/inspection/'+jobid+'/input','manifest':signed_submission(self.original,'design'),'expiresAt':int(time.time())+600}
        with tempfile.TemporaryDirectory() as directory:
            worker.state=Path(directory)
            with mock.patch('supervisor.run_container',side_effect=[self.decoded,rendered]),self.assertRaises((ValueError,KeyError,TypeError)):
                worker.process(job)
        worker.key.sign.assert_not_called()
        self.assertFalse(any(call.args[0]=='PUT' for call in worker.gate.request.call_args_list))
        self.assertTrue(all(not call.args[0].endswith('/result') for call in worker.gate.post.call_args_list))

    def test_omitted_substituted_or_unbound_evidence_never_signed(self):
        variants=[]
        missing=copy.deepcopy(self.render_report);del missing['composition'];variants.append(missing)
        for key in ('canonicalSha','posterSha','previewSha'):
            report=copy.deepcopy(self.render_report);report['composition'][key]='0'*64;variants.append(report)
        for index in range(5):
            report=copy.deepcopy(self.render_report);del report['composition']['text'][index];variants.append(report)
            report=copy.deepcopy(self.render_report);report['composition']['text'][index]['value']='SUBSTITUTED';variants.append(report)
        report=copy.deepcopy(self.render_report);report['composition']['text'].append({'field':'metadata','value':'not trusted'});variants.append(report)
        report=copy.deepcopy(self.render_report);report['composition']['image']['sha256']='0'*64;variants.append(report)
        for index,report in enumerate(variants):
            with self.subTest(variant=index):self.assert_rejected_before_signing(self.rendered(report))
        for name in ('poster','preview'):
            rendered=self.rendered();rendered[name]+=b'not bound to report'
            with self.subTest(object=name):self.assert_rejected_before_signing(rendered)

    def test_decoder_cannot_replace_the_canonical_words(self):
        decoded=copy.deepcopy(self.decoded)
        files,_,spec=canonical_pack(decoded['canonical'],'design')
        spec['card']['words']['tagline']='SUBSTITUTED'
        files['design.json']=json.dumps(spec,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()
        decoded['canonical']=pack(files)
        report=loads(decoded['report'],LIMITS['report']);report['canonicalSha']=sha(decoded['canonical'])
        decoded['report']=json.dumps(report).encode()
        with self.assertRaisesRegex(ValueError,'canonical text changed'):check_decode(self.original,decoded,'design')


class StrictPNGTests(unittest.TestCase):
    def chunk(self,tag,data):
        return struct.pack('>I',len(data))+tag+data+struct.pack('>I',binascii.crc32(tag+data))

    def png(self,depth=8,color=6,compression=0,filtering=0,interlace=0,pixels=b'\0\x11\x22\x33\xff',extra=b'',compressed=None,width=1,height=1):
        header=struct.pack('>IIBBBBB',width,height,depth,color,compression,filtering,interlace)
        return b'\x89PNG\r\n\x1a\n'+self.chunk(b'IHDR',header)+extra+self.chunk(b'IDAT',zlib.compress(pixels) if compressed is None else compressed)+self.chunk(b'IEND',b'')

    def test_stripped_rgb_and_rgba_are_valid(self):
        for color,pixels in ((2,b'\0\x11\x22\x33'),(6,b'\0\x11\x22\x33\xff')):
            self.assertEqual(strict_png(self.png(color=color,pixels=pixels),384,384)['width'],1)
        stream=zlib.compress(b'\0\x11\x22\x33\xff')
        valid=self.png(compressed=stream[:3])
        # Consecutive IDAT chunks are legal, the full stream is checked together.
        valid=valid[:-12]+self.chunk(b'IDAT',stream[3:])+self.chunk(b'IEND',b'')
        strict_png(valid,384,384)

    def test_invalid_headers_crc_streams_chunks_and_budgets_are_rejected(self):
        valid=self.png();badcrc=bytearray(valid);badcrc[29]^=1
        header=struct.pack('>IIBBBBB',1,1,8,6,0,0,0)
        variants=[bytes(badcrc),valid+b'trailer',valid[:-12],self.png(depth=16),self.png(color=0),self.png(color=3),self.png(compression=1),self.png(filtering=1),self.png(interlace=1),self.png(width=385),self.png(height=385),self.png(extra=self.chunk(b'IHDR',header)),self.png(extra=self.chunk(b'tEXt',b'payload')),self.png(extra=self.chunk(b'acTL',b'\0'*8)),self.png(pixels=b'\x05\x11\x22\x33\xff'),self.png(pixels=b'\0\x11'),self.png(pixels=b'\0'*1024*1024),self.png(compressed=b'not zlib'),self.png(compressed=zlib.compress(b'\0\x11\x22\x33\xff')+b'payload'),self.png(compressed=zlib.compress(b'\0\x11\x22\x33\xff')+zlib.compress(b'x'))]
        for index,raw in enumerate(variants):
            with self.subTest(variant=index),self.assertRaises(ValueError):strict_png(raw,384,384)



if __name__=='__main__':unittest.main()
