#!/usr/bin/env python3
"""One hostile job, inside the fixed no-network container. Never holds authority."""
import argparse
import base64
import io
import json
import os
from pathlib import Path
import resource
import sys
import time
import warnings

from policy import LIMITS, MIB, REVIEW_IMAGE_BYTES, VERSION, POLICY, canonical_pack, composition_text, inspect_pack, image_info, loads, pack, require, sha, strict_png


def emit(objects):
    out = sys.stdout.buffer
    out.write(b'RYOKU-OBJECTS-1\n')
    for name,data in sorted(objects.items()):
        require(name in LIMITS and 0 < len(data) <= LIMITS[name], 'output budget')
        header = {'name':name,'bytes':len(data),'sha':sha(data)}
        out.write(json.dumps(header,sort_keys=True,separators=(',',':')).encode()+b'\n')
        out.write(data);out.write(b'\n')
    out.write(b'END\n');out.flush()


def decode_all(data, info):
    from PIL import Image
    Image.MAX_IMAGE_PIXELS = 2048*2048
    warnings.simplefilter('error', Image.DecompressionBombWarning)
    with Image.open(io.BytesIO(data)) as image:
        require(image.format==info['format'] and image.size==(info['width'],info['height']), 'decoder/header disagreement')
        require(image.n_frames==info['frames'] if hasattr(image,'n_frames') else info['frames']==1, 'decoder frame disagreement')
        frames=[];durations=[]
        loop=image.info.get('loop',1)
        require(type(loop) is int and 0<=loop<=65535,'animation loop')
        if image.format=='GIF':
            # GIF counts repeats after the first play, WebP counts total plays
            loop=1 if 'loop' not in image.info else (0 if loop==0 else loop+1)
        require(loop<=65535,'finite GIF loop cannot be represented losslessly')
        for index in range(info['frames']):
            image.seek(index)
            image.load()
            require(image.size==(info['width'],info['height']), 'frame changed canvas')
            # Pillow composites GIF/WebP rectangles using original disposal/blending before
            # we store independent lossless RGBA frames. No uploaded metadata survives.
            frame=image.convert('RGBA').copy()
            frame.info.clear()
            frames.append(frame)
            durations.append(int(image.info.get('duration',0)))
        require(durations==info['durations'], 'decoder timing disagreement')
    return frames,durations,loop


def lossless_animation(frames,durations,loop):
    # Pillow's animated encoder ignores exact=True, so encode exact still frames
    def chunk(tag,data):
        return tag+len(data).to_bytes(4,'little')+data+(b'\0' if len(data)&1 else b'')
    def u24(value):return value.to_bytes(3,'little')
    width,height=frames[0].size
    alpha=any(frame.getchannel('A').getextrema()[0]<255 for frame in frames)
    body=io.BytesIO()
    body.write(chunk(b'VP8X',bytes([0x12 if alpha else 0x02])+b'\0'*3+u24(width-1)+u24(height-1)))
    body.write(chunk(b'ANIM',b'\0'*4+loop.to_bytes(2,'little')))
    for frame,duration in zip(frames,durations):
        encoded=io.BytesIO()
        frame.save(encoded,format='WEBP',lossless=True,exact=True,quality=100,method=4)
        data=encoded.getvalue();position=12;lossless=None
        while position<len(data):
            size=int.from_bytes(data[position+4:position+8],'little')
            end=position+8+size+(size&1);tag=data[position:position+4]
            require(tag in (b'VP8X',b'VP8L') and end<=len(data),'unexpected still WebP chunk')
            if tag==b'VP8L':
                require(lossless is None,'duplicate lossless frame')
                lossless=data[position:end]
            position=end
        require(position==len(data) and lossless is not None,'missing lossless frame')
        header=b'\0'*6+u24(width-1)+u24(height-1)+u24(duration)+b'\x02'
        body.write(chunk(b'ANMF',header+lossless))
        require(body.tell()+12<=8*MIB,'canonical animation budget')
    data=body.getvalue()
    return b'RIFF'+(len(data)+4).to_bytes(4,'little')+b'WEBP'+data


def canonicalize(raw,kind):
    files,infos,spec=inspect_pack(raw,kind)
    result={};mapping={};facts=[]
    total_frames=sum(info['frames'] for info in infos.values())
    selected=set(range(total_frames)) if total_frames<=8 else {round(i*(total_frames-1)/7) for i in range(8)}
    review={'complete':total_frames<=8,'totalFrames':total_frames,'images':[]}
    frame_offset=0
    for index,(name,info) in enumerate(sorted(infos.items())):
        frames,durations,loop=decode_all(files[name],info)
        animated=len(frames)>1 or (info['format'] in ('GIF','WEBP') and sum(durations)>0)
        target=f'a{index:02d}.'+('webp' if animated else 'png')
        output=io.BytesIO()
        if animated:
            output.write(lossless_animation(frames,durations,loop))
        else:
            frames[0].save(output,format='PNG',compress_level=6)
        data=output.getvalue();require(len(data)<=8*MIB,'canonical image budget')
        new=image_info(data,target)
        decoded,timing,newloop=decode_all(data,new)
        # Compare the displayed timeline as well as the independent header facts
        def timeline(fs,ds):
            spans=[]
            for frame,duration in zip(fs,ds):
                pixels=sha(frame.tobytes())
                if spans and spans[-1][0]==pixels:
                    spans[-1][1]+=duration
                else:spans.append([pixels,duration])
            return spans
        require(timeline(frames,durations)==timeline(decoded,timing), 'canonical pixels/timing changed')
        require(not animated or newloop==loop,'canonical loop changed')
        mapping[name]=target;result[target]=data
        facts.append({'source':name,'canonical':target,'width':info['width'],'height':info['height'],'sourceFrames':info['frames'],'canonicalFrames':new['frames'],'durations':durations,'loop':loop,'decodedPixels':info['pixels'],'allFramesDecoded':True})
        from PIL import Image
        for frame_index,frame in enumerate(frames):
            if frame_offset+frame_index not in selected:continue
            thumbnail=frame.copy();thumbnail.thumbnail((384,384),Image.Resampling.LANCZOS)
            thumb=io.BytesIO();thumbnail.save(thumb,format='PNG',compress_level=6);thumbnail.close()
            data=thumb.getvalue()
            review['images'].append({'mime':'image/png','sha256':sha(data),'base64':base64.b64encode(data).decode(),'sourceAsset':name,'frameIndex':frame_index})
        frame_offset+=len(frames)
        for frame in frames+decoded:frame.close()
    if spec is not None:
        b=spec['badge'];c=spec['card'];b['emblem']=mapping[b['emblem']]
        for key in ('background','frame','glow'):
            if c.get(key):c[key]=mapping[c[key]]
        for layer in c['art']:layer['image']=mapping[layer['image']]
        result['design.json']=json.dumps(spec,sort_keys=True,separators=(',',':'),ensure_ascii=False,allow_nan=False).encode()
    canonical=pack(result);inspect_pack(canonical,kind)
    report={'v':1,'pipelineVersion':VERSION,'policyVersion':POLICY,'assets':facts,'moderation':'unreviewed','classificationPerformed':False,'canonicalSha':sha(canonical),'review':review}
    encoded_report=json.dumps(report,sort_keys=True,separators=(',',':')).encode()
    require(len(encoded_report)<=LIMITS['report'],'review report budget')
    return {'canonical':canonical,'report':encoded_report}


def renderer_manifest(root):
    manifest=loads((root/'manifest.json').read_bytes())
    require(manifest.get('protocol')==1 and isinstance(manifest.get('files'),dict),'renderer manifest')
    require(len(manifest['files'])<=1024,'renderer file count')
    for name,digest in manifest['files'].items():
        path=root/name
        require(not path.is_symlink() and path.is_file() and root in path.resolve().parents,'renderer file path')
        require(sha(path.read_bytes())==digest,'renderer file hash')
    return manifest


def composition_evidence(raw,poster,preview,kind):
    """Run only in the render sandbox, using the real full-card poster bytes."""
    from PIL import Image
    _,_,spec=canonical_pack(raw,kind)
    strict_png(poster,1260,1760)
    with Image.open(io.BytesIO(poster)) as source:
        thumbnail=source.convert('RGBA')
    thumbnail.thumbnail((384,384),Image.Resampling.LANCZOS)
    thumbnail.info.clear()
    encoded=io.BytesIO();thumbnail.save(encoded,format='PNG');thumbnail.close()
    data=encoded.getvalue()
    require(len(data)<=REVIEW_IMAGE_BYTES,'composition thumbnail budget')
    strict_png(data,384,384)
    return {'v':1,'canonicalSha':sha(raw),'posterSha':sha(poster),'previewSha':sha(preview),'text':composition_text(spec),'image':{'mime':'image/png','sha256':sha(data),'base64':base64.b64encode(data).decode()},'coverage':'poster-and-canonical-text'}


def render(raw,kind,root):
    """Reuse Mika's REAL TradingCard stage and capture driver, never uploaded QML."""
    from PySide6.QtGui import QImageReader
    bundle=renderer_manifest(root)
    files,infos,spec=canonical_pack(raw,kind)
    work=Path('/work');packdir=work/'pack';packdir.mkdir();scratch=work/'frames';scratch.mkdir()
    for name,data in files.items():
        (packdir/name).write_bytes(data)
    # Only references already validated against the flat inventory become local paths.
    if spec:
        b=spec['badge'];c=spec['card'];b['emblem']=str(packdir/b['emblem'])
        for key in ('background','frame','glow'):
            if c.get(key):c[key]=str(packdir/c[key])
        for layer in c['art']:layer['image']=str(packdir/layer['image'])
    sys.path.insert(0,str(root))
    import render_card as trusted
    xvfb,display=trusted._start_xvfb()
    os.environ.update({'DISPLAY':display,'QT_QPA_PLATFORM':'xcb','QT_SCALE_FACTOR':'2','QT_QUICK_BACKEND':'rhi','QSG_RHI_BACKEND':'opengl','QSG_RENDER_LOOP':'basic','QML_IMPORT_PATH':str(root/'qml'),'QML_DISABLE_DISK_CACHE':'1','LIBGL_ALWAYS_SOFTWARE':'1','GALLIUM_DRIVER':'llvmpipe'})
    data={'cardId':'0000000000000000','name':'Inspection preview','role':'contributor','design':spec,'hero':{},'machine':{'cpu':'Review sample CPU','threads':16,'ram':'32 GB','refresh':165,'resolution':'2560x1440','gpu':'Integrated review GPU','distro':'Ryoku','wm':'niri','uptime':'Preview','packages':'Preview','kernel':'Preview'}}
    data['levels']=[[(index+band)%4 for index in range(138)] for band in range(3)]
    data['machine']['gpuClass']=0
    animationdir=packdir
    if kind=='hero':
        hero=next(iter(infos))
        filename='hero.'+hero.rsplit('.',1)[-1]
        animationdir=Path('/tmp/config/ryoku/profile');animationdir.mkdir(parents=True)
        (animationdir/filename).write_bytes(files[hero])
        data['hero']={'kind':'custom','source':filename,'style':'photo','focalX':.5,'focalY':.5}
    stage=trusted.CardStage(data)
    stage.settle()
    # Exercise every canonical asset and every frame in Qt as well as Pillow, including
    # unused assets. This is separate from the bounded 12fps whole-card review sample.
    coverage=[]
    for name,info in sorted(infos.items()):
        frames,_,_=decode_all(files[name],info)
        for frame in frames:frame.close()
        reader=QImageReader(str(packdir/name))
        require(reader.size().width()==info['width'] and reader.size().height()==info['height'],'Qt canvas mismatch')
        count=0
        while True:
            image=reader.read();require(not image.isNull(),'Qt image decode failure')
            count+=1;require(count<=96,'Qt frame budget')
            if count>=info['frames']:break
        require(count==info['frames'],'Qt missing frames')
        coverage.append({'name':name,'qtDecodedFrames':count,'frames':info['frames']})
    poster=trusted._pil(stage.grab()).convert('RGBA');poster.info.clear()
    encoded=io.BytesIO();poster.save(encoded,format='PNG');poster.close()
    if kind=='hero' and next(iter(infos.values()))['frames']>1:
        # the real hero creates CardMotion only once the card is live
        stage.root.setProperty('live',True)
        deadline=time.monotonic()+5
        while True:
            stage.pump(20)
            moving=stage.pack_animations(animationdir)
            if moving and all(item.property('frameCount')==len(durations) for item,durations in moving):break
            require(time.monotonic()<deadline,'real hero animation did not load')
            time.sleep(.005)
    else:
        moving=stage.pack_animations(animationdir)
    # Canonical animations have <=20 seconds; preview is explicitly an 8-second
    # sample, not a claim that human moderation inspected all frames.
    seconds=trusted._loop(moving)
    framepaths=stage.capture(moving,seconds,12,scratch)
    bounds=[round(i*100/12)*10 for i in range(len(framepaths))]+[round(seconds*100)*10]
    durations=[max(20,b-a) for a,b in zip(bounds,bounds[1:])]
    preview,width,colors=trusted._fit_gif(framepaths,durations,trusted.GIF_LADDER,LIMITS['preview'])
    stage.close();xvfb.terminate()
    composition=composition_evidence(raw,encoded.getvalue(),preview,kind)
    report={'v':1,'pipelineVersion':VERSION,'policyVersion':POLICY,'rendererVersion':bundle['version'],'rendererManifestSha':sha((root/'manifest.json').read_bytes()),'canonicalSha':sha(raw),'coverage':coverage,'previewFrames':len(framepaths),'previewMilliseconds':round(seconds*1000),'previewWidth':width,'previewColors':colors,'posterWidth':1260,'posterHeight':1760,'warnings':stage.warnings[-20:],'moderation':'unreviewed','classificationPerformed':False,'composition':composition}
    report_bytes=json.dumps(report,sort_keys=True,separators=(',',':'),ensure_ascii=False,allow_nan=False).encode()
    require(len(report_bytes)<=LIMITS['report'],'render report budget')
    return {'poster':encoded.getvalue(),'preview':preview,'report':report_bytes}


def main():
    parser=argparse.ArgumentParser();parser.add_argument('stage',choices=('decode','render'));parser.add_argument('--kind',choices=('design','hero'),required=True);args=parser.parse_args()
    # Kernel-enforced outer cgroup/PID/wall/output limits are mandatory. These
    # additional limits do not replace the containment launcher.
    resource.setrlimit(resource.RLIMIT_CORE,(0,0));resource.setrlimit(resource.RLIMIT_FSIZE,(64*MIB,64*MIB));resource.setrlimit(resource.RLIMIT_NOFILE,(1024,1024))
    os.umask(0o077);os.environ['XDG_CONFIG_HOME']='/tmp/config'
    require(not any(k in os.environ for k in ('INSPECTOR_WORKER_TOKEN','INSPECTOR_SIGNING_KEY','CARD_GATE_TOKEN')), 'authority leaked into worker')
    raw=Path('/input/submission').read_bytes();require(len(raw)<=8*MIB,'input budget')
    objects=canonicalize(raw,args.kind) if args.stage=='decode' else render(raw,args.kind,Path('/opt/ryoku-inspector/renderer'))
    emit(objects)


if __name__=='__main__':
    try:
        main()
    except Exception as error:
        print(f'inspection failed: {type(error).__name__}: {str(error)[:1000]}',file=sys.stderr,flush=True)
        os._exit(1)
    # Avoid Qt's teardown after its private X server has been closed.
    sys.stdout.flush();sys.stderr.flush();os._exit(0)
