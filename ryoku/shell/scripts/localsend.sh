#!/bin/bash
# LocalSend helper for the Ryoku shell file stash: LAN device discovery and send.
# Usage: localsend.sh discover | send <file> <ip> | send-all <dir> <ip>
set -u

# Upload one file to a LocalSend peer; returns 0 on success, no notification of
# its own so callers (single send vs send-all) can report once.
do_send() {
  local FILE="$1" TARGET="$2"
  [ -f "$FILE" ] || return 1
  [ -n "$TARGET" ] || return 1
  local PORT=53317 FILENAME FILESIZE FILETYPE FILE_ID FP_FILE FINGERPRINT BODY RESP SESSION TOKEN
  FILENAME=$(basename "$FILE"); FILESIZE=$(stat -c%s "$FILE"); FILETYPE=$(file -b --mime-type "$FILE"); FILE_ID="qs_$(date +%s%N | md5sum | head -c8)"
  FP_FILE="$HOME/.cache/ryoku_localsend_fp"; [ -f "$FP_FILE" ] || openssl rand -hex 16 > "$FP_FILE"; FINGERPRINT=$(cat "$FP_FILE")
  BODY=$(python3 - "$FILENAME" "$FILESIZE" "$FILETYPE" "$FILE_ID" "$FINGERPRINT" "$PORT" <<'PYEOF'
import json,sys
fn,sz,ft,fid,fp,port=sys.argv[1],int(sys.argv[2]),sys.argv[3],sys.argv[4],sys.argv[5],int(sys.argv[6])
print(json.dumps({"info":{"alias":"Ryoku Stash","version":"2.1","deviceModel":None,"deviceType":"headless","fingerprint":fp,"port":port,"protocol":"https","download":False},"files":{fid:{"id":fid,"fileName":fn,"size":sz,"fileType":ft,"sha256":None,"preview":None,"metadata":None}}}))
PYEOF
)
  RESP=$(curl -sk --max-time 30 -X POST "https://$TARGET:$PORT/api/localsend/v2/prepare-upload" -H "Content-Type: application/json" -d "$BODY")
  SESSION=$(python3 -c "import sys,json; print(json.loads(sys.argv[1]).get('sessionId',''))" "$RESP" 2>/dev/null)
  TOKEN=$(python3 -c "import sys,json; d=json.loads(sys.argv[1]); print(d.get('files',{}).get('$FILE_ID',''))" "$RESP" 2>/dev/null)
  [ -n "$SESSION" ] && [ -n "$TOKEN" ] || return 1
  curl -sk --max-time 120 -X POST "https://$TARGET:$PORT/api/localsend/v2/upload?sessionId=$SESSION&fileId=$FILE_ID&token=$TOKEN" -H "Content-Type: $FILETYPE" --data-binary @"$FILE" >/dev/null
}
cmd="${1:-}"
case "$cmd" in
discover)
  python3 - <<'PYEOF'
import socket, json, time, struct, re, subprocess, asyncio, ssl
MCAST='224.0.0.167'; PORT=53317
out=subprocess.check_output(["ip","-4","addr"],text=True)
local_ips=set(re.findall(r'inet (\d+\.\d+\.\d+\.\d+)',out))
route_out=subprocess.check_output(["ip","route","get",MCAST],text=True)
src_m=re.search(r'src (\d+\.\d+\.\d+\.\d+)',route_out)
lan_ip=src_m.group(1) if src_m else ''
seen=set()
def emit(ip,alias):
    if ip not in seen and ip not in local_ips:
        seen.add(ip); print(f"{alias}\t{ip}",flush=True)
rx=socket.socket(socket.AF_INET,socket.SOCK_DGRAM,socket.IPPROTO_UDP)
rx.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1)
rx.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEPORT,1)
rx.bind(('',PORT))
mreq=(struct.pack('4s4s',socket.inet_aton(MCAST),socket.inet_aton(lan_ip)) if lan_ip else struct.pack('4sL',socket.inet_aton(MCAST),socket.INADDR_ANY))
rx.setsockopt(socket.IPPROTO_IP,socket.IP_ADD_MEMBERSHIP,mreq); rx.settimeout(0.1)
tx=socket.socket(socket.AF_INET,socket.SOCK_DGRAM,socket.IPPROTO_UDP)
tx.setsockopt(socket.IPPROTO_IP,socket.IP_MULTICAST_TTL,4)
if lan_ip: tx.setsockopt(socket.IPPROTO_IP,socket.IP_MULTICAST_IF,socket.inet_aton(lan_ip))
announce=json.dumps({"alias":"Ryoku Stash","version":"2.1","deviceModel":None,"deviceType":"headless","fingerprint":"ryoku_stash_discover","port":PORT,"protocol":"https","download":False,"announce":True}).encode()
tx.sendto(announce,(MCAST,PORT)); deadline=time.time()+2.0; sent_second=False
while time.time()<deadline:
    if not sent_second and time.time()>deadline-1.0:
        tx.sendto(announce,(MCAST,PORT)); sent_second=True
    try:
        data,(ip,_)=rx.recvfrom(65536)
        if ip in local_ips: continue
        try: info=json.loads(data.decode())
        except Exception: continue
        if info.get('fingerprint')=='ryoku_stash_discover': continue
        emit(ip,info.get('alias','Unknown'))
    except socket.timeout: pass
if not lan_ip:
    import sys; sys.exit(0)
prefix='.'.join(lan_ip.split('.')[:3])
ctx=ssl.create_default_context(); ctx.check_hostname=False; ctx.verify_mode=ssl.CERT_NONE
async def probe(ip):
    if ip in local_ips or ip in seen: return
    try:
        r,w=await asyncio.wait_for(asyncio.open_connection(ip,PORT,ssl=ctx),timeout=0.6)
        w.write(f"GET /api/localsend/v2/info HTTP/1.0\r\nHost: {ip}\r\nConnection: close\r\n\r\n".encode())
        await w.drain()
        data=await asyncio.wait_for(r.read(4096),timeout=0.6); w.close()
        body=data.split(b'\r\n\r\n',1)
        if len(body)<2: return
        info=json.loads(body[1].decode()); emit(ip,info.get('alias','Unknown'))
    except Exception: pass
async def scan():
    await asyncio.gather(*[probe(f"{prefix}.{i}") for i in range(1,255)])
asyncio.run(scan())
PYEOF
  ;;
send)
  FILE="$2"; TARGET="$3"
  [ -f "$FILE" ] || { notify-send "LocalSend" "File not found" -i dialog-error; exit 1; }
  [ -n "$TARGET" ] || { notify-send "LocalSend" "No target device" -i dialog-error; exit 1; }
  if do_send "$FILE" "$TARGET"; then notify-send "LocalSend" "Sent: $(basename "$FILE")" -i emblem-ok-symbolic; else notify-send "LocalSend" "Upload failed" -i dialog-error; exit 1; fi
  ;;
send-all)
  DIR="$2"; TARGET="$3"
  [ -d "$DIR" ] || { notify-send "LocalSend" "Stash folder missing" -i dialog-error; exit 1; }
  [ -n "$TARGET" ] || { notify-send "LocalSend" "No target device" -i dialog-error; exit 1; }
  count=0; ok=0
  shopt -s nullglob
  for f in "$DIR"/*; do
    [ -f "$f" ] || continue
    count=$((count+1))
    do_send "$f" "$TARGET" && ok=$((ok+1))
  done
  [ "$count" -gt 0 ] || { notify-send "LocalSend" "Stash is empty" -i dialog-error; exit 1; }
  notify-send "LocalSend" "Sent $ok of $count files" -i emblem-ok-symbolic
  ;;
receive)
  # Run a LocalSend v2 receiver: announce over multicast so other LocalSend apps
  # list us, then hold each incoming transfer at prepare-upload until the shell
  # sends back a verdict on stdin (ACCEPT<tab>sid / DECLINE<tab>sid; auto-declines
  # after RYOKU_LS_ACCEPT_TIMEOUT seconds, default 60). Accepted files save into
  # the stash. Status lines the shell parses (READY/OFFER/SAVED/DECLINED/ERROR)
  # stream out tab-separated and flushed.
  ALIAS="${2:-Ryoku Stash}"
  STASH="${STASH_DIR:-${XDG_DOWNLOAD_DIR:-$HOME/Downloads}/Stash}"
  mkdir -p "$STASH"
  CERT="$HOME/.cache/ryoku_localsend_cert.pem"; KEY="$HOME/.cache/ryoku_localsend_key.pem"
  if [ ! -s "$CERT" ] || [ ! -s "$KEY" ]; then
    openssl req -x509 -newkey rsa:2048 -keyout "$KEY" -out "$CERT" -days 3650 -nodes -subj "/CN=Ryoku Stash" >/dev/null 2>&1 \
      || { echo -e "ERROR\tcould not create certificate"; exit 1; }
  fi
  FP_FILE="$HOME/.cache/ryoku_localsend_fp"; [ -f "$FP_FILE" ] || openssl rand -hex 16 > "$FP_FILE"
  # exec so the receiver IS this process: when the shell stops it (SIGTERM), the
  # signal reaches python directly and the port is released. Without exec, python
  # is a bash child that gets orphaned on stop and keeps holding port 53317, so
  # the next receive fails with "port busy".
  # python3 reads its program off fd 3 (the heredoc), leaving stdin free as the
  # shell's verdict channel for the accept/decline handshake.
  exec env ALIAS="$ALIAS" STASH="$STASH" CERT="$CERT" KEY="$KEY" FINGERPRINT="$(cat "$FP_FILE")" python3 /dev/fd/3 3<<'PYEOF'
import os, ssl, json, socket, struct, threading, time, signal, http.server
from urllib.parse import urlparse, parse_qs
ALIAS=os.environ['ALIAS']; STASH=os.environ['STASH']; FP=os.environ['FINGERPRINT']
CERT=os.environ['CERT']; KEY=os.environ['KEY']; PORT=53317; MCAST='224.0.0.167'
INFO={"alias":ALIAS,"version":"2.1","deviceModel":"Ryoku","deviceType":"headless","fingerprint":FP,"download":False}
sessions={}
pending={}   # sid -> threading.Event, set when the shell's verdict arrives
verdict={}   # sid -> True (accept) / False (decline)
def out(tag,val): print(tag+"\t"+val, flush=True)
def stdin_verdicts():
    # The shell answers each OFFER on our stdin: "ACCEPT<tab>sid" / "DECLINE<tab>sid".
    # readline (not "for line in stdin") so each verdict lands immediately instead
    # of sitting in Python's iterator read-ahead buffer.
    import sys
    for line in iter(sys.stdin.readline, ""):
        parts=line.rstrip("\n").split("\t")
        if len(parts)>=2 and parts[0] in ("ACCEPT","DECLINE"):
            sid=parts[1]; verdict[sid]=(parts[0]=="ACCEPT")
            ev=pending.get(sid)
            if ev: ev.set()
def uniq(name):
    base=os.path.basename(name) or "file"; path=os.path.join(STASH, base)
    if not os.path.exists(path): return path
    stem,ext=os.path.splitext(base); i=1
    while True:
        cand=os.path.join(STASH, "%s (%d)%s"%(stem,i,ext))
        if not os.path.exists(cand): return cand
        i+=1
class H(http.server.BaseHTTPRequestHandler):
    protocol_version='HTTP/1.1'
    def log_message(self,*a): pass
    def _send(self, code, body=b'', ctype='application/json'):
        self.send_response(code); self.send_header('Content-Type',ctype)
        self.send_header('Content-Length',str(len(body))); self.end_headers()
        if body: self.wfile.write(body)
    def _read(self):
        n=int(self.headers.get('Content-Length',0) or 0)
        return self.rfile.read(n) if n>0 else b''
    def do_GET(self):
        if urlparse(self.path).path.endswith('/api/localsend/v2/info'):
            self._send(200, json.dumps(INFO).encode())
        else: self._send(404)
    def do_POST(self):
        p=urlparse(self.path).path
        if p.endswith('/register'):
            self._read(); self._send(200, json.dumps(INFO).encode())
        elif p.endswith('/prepare-upload'):
            try: req=json.loads(self._read().decode() or '{}')
            except Exception: req={}
            files=req.get('files',{}) or {}
            if not files: self._send(204); return
            sid=os.urandom(8).hex()
            alias=((req.get('info') or {}).get('alias') or 'A device')
            alias=alias.replace("\t"," ").replace("\n"," ")[:64]
            # Hold the offer: announce it and block until the shell's verdict lands
            # on stdin. No token is issued unless accepted, and /upload rejects any
            # fileId without a token, so declined bytes never cross the wire.
            ev=threading.Event(); pending[sid]=ev; verdict[sid]=False
            out("OFFER", "%s\t%d\t%s" % (alias, len(files), sid))
            accepted = ev.wait(float(os.environ.get("RYOKU_LS_ACCEPT_TIMEOUT","60"))) and verdict.get(sid, False)
            pending.pop(sid, None); verdict.pop(sid, None)
            if not accepted:
                out("DECLINED", sid); self._send(403); return
            tok={}; reg={}
            for fid,meta in files.items():
                t=os.urandom(8).hex(); name=(meta or {}).get('fileName', fid)
                tok[fid]=t; reg[fid]={'token':t,'fileName':name}
            sessions[sid]=reg
            self._send(200, json.dumps({"sessionId":sid,"files":tok}).encode())
        elif p.endswith('/upload'):
            q=parse_qs(urlparse(self.path).query)
            reg=sessions.get(q.get('sessionId',[''])[0],{}).get(q.get('fileId',[''])[0])
            if not reg or reg['token']!=q.get('token',[''])[0]:
                self._read(); self._send(403); return
            dest=uniq(reg['fileName']); n=int(self.headers.get('Content-Length',0) or 0); got=0
            with open(dest,'wb') as f:
                while got<n:
                    chunk=self.rfile.read(min(65536, n-got))
                    if not chunk: break
                    f.write(chunk); got+=len(chunk)
            out("SAVED", os.path.basename(dest)); self._send(200)
        elif p.endswith('/cancel'):
            self._read(); self._send(200)
        else:
            self._read(); self._send(404)
def announce():
    import re, subprocess, urllib.request
    try:
        # Multicast must leave on the real LAN interface (exactly what discover
        # does) or no other device ever sees us. Find the source IP the kernel
        # would use to reach the group.
        lan_ip = ''
        try:
            r = subprocess.check_output(["ip", "route", "get", MCAST], text=True)
            m = re.search(r'src (\d+\.\d+\.\d+\.\d+)', r); lan_ip = m.group(1) if m else ''
        except Exception: pass
        rx = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
        rx.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        try: rx.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEPORT, 1)
        except OSError: pass
        rx.bind(('', PORT))
        mreq = (struct.pack('4s4s', socket.inet_aton(MCAST), socket.inet_aton(lan_ip)) if lan_ip
                else struct.pack('4sL', socket.inet_aton(MCAST), socket.INADDR_ANY))
        rx.setsockopt(socket.IPPROTO_IP, socket.IP_ADD_MEMBERSHIP, mreq)
        rx.settimeout(2.5)
        tx = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
        tx.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_TTL, 4)
        if lan_ip: tx.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_IF, socket.inet_aton(lan_ip))
        ann = lambda a: json.dumps({**INFO, "port": PORT, "protocol": "https", "announce": a}).encode()
        sslctx = ssl.create_default_context(); sslctx.check_hostname = False; sslctx.verify_mode = ssl.CERT_NONE
        seen = set()
        def register(ip):
            # Tell the peer about us over HTTP too; this path does not depend on our
            # multicast reaching them. Once per peer is enough, the periodic
            # announce covers anyone who joins or restarts later.
            if ip in seen: return
            seen.add(ip)
            try:
                req = urllib.request.Request(
                    "https://%s:%d/api/localsend/v2/register" % (ip, PORT),
                    data=json.dumps({**INFO, "port": PORT, "protocol": "https"}).encode(),
                    headers={"Content-Type": "application/json"}, method="POST")
                urllib.request.urlopen(req, timeout=2, context=sslctx).read()
            except Exception: pass
        tx.sendto(ann(True), (MCAST, PORT)); last = time.time()
        while True:
            try:
                data, (sip, _) = rx.recvfrom(65536)
                try: info = json.loads(data.decode())
                except Exception: info = None
                if info and info.get('fingerprint') != FP and info.get('announce'):
                    tx.sendto(ann(False), (MCAST, PORT))
                    threading.Thread(target=register, args=(sip,), daemon=True).start()
            except socket.timeout: pass
            except Exception: pass
            if time.time() - last > 2.5:
                tx.sendto(ann(True), (MCAST, PORT)); last = time.time()
    except Exception: pass
try:
    ctx=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER); ctx.load_cert_chain(CERT, KEY)
    srv=http.server.ThreadingHTTPServer(('0.0.0.0',PORT), H)
    srv.socket=ctx.wrap_socket(srv.socket, server_side=True)
except OSError:
    out("ERROR", "port %d is busy"%PORT)
    try: __import__("subprocess").run(["notify-send","Stash","Receive needs port 53317, but it is already in use (another LocalSend running?).","-i","dialog-error"])
    except Exception: pass
    raise SystemExit(1)
stop=threading.Event()
signal.signal(signal.SIGTERM, lambda *a: stop.set())
signal.signal(signal.SIGINT,  lambda *a: stop.set())
threading.Thread(target=srv.serve_forever, daemon=True).start()
threading.Thread(target=announce, daemon=True).start()
threading.Thread(target=stdin_verdicts, daemon=True).start()
out("READY", ALIAS)
stop.wait(); srv.shutdown()
PYEOF
  ;;
*) echo "usage: localsend.sh discover | send <file> <ip> | send-all <dir> <ip> | receive [alias]" >&2; exit 2 ;;
esac
