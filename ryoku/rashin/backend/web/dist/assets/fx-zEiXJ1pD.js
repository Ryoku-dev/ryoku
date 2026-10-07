import{p as Br,c as Ur,f as Vr,i as Xe,g as Q,a as de,b as Gr,u as ie,d as tr,s as or,h as Zr,t as _e,e as Qr,j as Jr,k as Le,l as Kr,m as et,r as rt,n as tt,o as ar,q as ot}from"./bits-BwIgp5un.js";var at=(r,e)=>`mo-root${r==="always"?" mo-always":""}${e?" mo-expr":""}`;function st(r){let e=Math.round(r.num("motion.blink",3500,6500)),t=Math.round(r.num("motion.saccade",4200,7600)),a=r.num("motion.lookX",1,2.2),s=r.num("motion.lookY",.8,1.7),o=i=>Math.round(i*100)/100;return{phase:Math.round(r.num("motion.phase",0,2800)),bob:Math.round(r.num("motion.bob",0,3400)),blink:e,blinkPhase:Math.round(r.num("motion.blinkPhase",0,e)),saccade:t,saccadePhase:Math.round(r.num("motion.saccadePhase",0,t)),lookX:o(a)*(r.bool("motion.lookXFlip")?-1:1),lookY:o(s)*(r.bool("motion.lookYFlip")?-1:1),lookMX:o(a),lookMY:o(s)}}function it(r){let e=a=>`${-a}ms`,t=st(r);return{"--mo-phase":e(t.phase),"--mo-bob-phase":e(t.bob),"--mo-blink":`${t.blink}ms`,"--mo-blink-phase":e(t.blinkPhase),"--mo-look-x":String(t.lookX),"--mo-look-mx":String(t.lookMX),"--mo-look-y":String(t.lookY),"--mo-look-my":String(t.lookMY),"--mo-saccade":`${t.saccade}ms`,"--mo-saccade-phase":e(t.saccadePhase)}}function Re({l:r,c:e,h:t}){let a=t*Math.PI/180,s=e*Math.cos(a),o=e*Math.sin(a),i=r+.3963377774*s+.2158037573*o,c=r-.1055613458*s-.0638541728*o,p=r-.0894841775*s-1.291485548*o,g=i*i*i,b=c*c*c,n=p*p*p;return[4.0767416621*g-3.3077115913*b+.2309699292*n,-1.2684380046*g+2.6097574011*b-.3413193965*n,-.0041960863*g-.7034186147*b+1.707614701*n]}var sr=r=>r.every(e=>e>=-1e-4&&e<=1.0001);function kr(r){let e=Re(r);if(!sr(e)){let t=0,a=r.c;for(let s=0;s<12;s++){let o=(t+a)/2;sr(Re({...r,c:o}))?t=o:a=o}e=Re({...r,c:t})}return e.map(t=>Math.min(1,Math.max(0,t)))}function ir(r){let[e,t,a]=kr(r);return .2126*e+.7152*t+.0722*a}function ze(r,e){let t=ir(r),a=ir(e);return(Math.max(t,a)+.05)/(Math.min(t,a)+.05)}function wr(r,e,t){if(ze(r,e)>=t)return r;let a=r.l>=e.l?1:-1;for(let i of[a,-a]){let c={...r};for(let p=0;p<60;p++){if(c.l=Math.min(1,Math.max(0,c.l+i*.02)),ze(c,e)>=t)return c;if(c.l===0||c.l===1)break}}let s={...r,l:0,c:0},o={...r,l:1,c:0};return ze(s,e)>=ze(o,e)?s:o}function nt(r){return"#"+kr(r).map(e=>{let t=e<=.0031308?12.92*e:1.055*Math.pow(e,.4166666666666667)-.055;return Math.round(t*255).toString(16).padStart(2,"0")}).join("")}var nr=[[.2,{l:.86,c:.085}],[.36,{l:.9,c:.028}],[.62,{l:.73,c:.135}],[.8,{l:.62,c:.165}],[.93,{l:.87,c:.16}],[1,{l:.34,c:.035}]],ct=r=>nr.find(([e])=>r<e)?.[1]??nr[0][1],lt={l:.145,c:0,h:0},pt=1.5,ft=(r,e)=>{let t=ct(e),a=wr({l:t.l,c:t.c,h:r},lt,pt);return{bg:{l:.965,c:.01,h:r},head:a,eye:a.l>=.5?{l:.17,c:.02,h:r}:{l:.97,c:.012,h:r}}},bt=[["head","bg",1.25],["eye","head",4.5]];function ut(r,e=!0,t=0){let a=ft(r,t);if(e)for(let[s,o,i]of bt)a[s]=wr(a[s],a[o],i);return a}function gt(r,e=!0,t=0){let a=ut(r,e,t),s={};for(let o in a)s[o]=nt(a[o]);return s}var A=r=>{let e=Math.round(r*100)/100;return Object.is(e,-0)?"0":String(e)};function Ie({cx:r,cy:e,rx:t,ry:a,n:s=4,rot:o=0}){let i=Math.min(1,(8*Math.pow(2,-1/s)-4)/3),c=t,p=a,g=c*i,b=p*i,n=[[c,0],[c,b],[g,p],[0,p],[-g,p],[-c,b],[-c,0],[-c,-b],[-g,-p],[0,-p],[g,-p],[c,-b],[c,0]],f=o*Math.PI/180,u=Math.cos(f),h=Math.sin(f),x=m=>{let[z,M]=n[m];return`${A(r+z*u-M*h)} ${A(e+z*h+M*u)}`},d=`M${x(0)}`;for(let m=1;m<13;m+=3)d+=`C${x(m)} ${x(m+1)} ${x(m+2)}`;return d+"Z"}function dt(r,e,t,a,s,o=0){let i=s.length,c=o*Math.PI/180,p=s.map((n,f)=>{let u=c+2*Math.PI*f/i;return[r+t*n*Math.cos(u),e+a*n*Math.sin(u)]}),g=n=>p[(n%i+i)%i],b=`M${A(g(0)[0])} ${A(g(0)[1])}`;for(let n=0;n<i;n++){let[f,u]=g(n-1),[h,x]=g(n),[d,m]=g(n+1),[z,M]=g(n+2);b+=`C${A(h+(d-f)/6)} ${A(x+(m-u)/6)} ${A(d-(z-h)/6)} ${A(m-(M-x)/6)} ${A(d)} ${A(m)}`}return b+"Z"}function mt({cx:r,cy:e,rx:t,ry:a,sides:s,round:o=.3,rot:i=0}){let c=o>0?o<1?o/2:.5:0,p=i*Math.PI/180-Math.PI/2,g=Array.from({length:s},(u,h)=>{let x=p+2*Math.PI*h/s;return[r+t*Math.cos(x),e+a*Math.sin(x)]}),b=u=>g[(u%s+s)%s],n=(u,h)=>{let[x,d]=b(u),[m,z]=b(h);return`${A(x+(m-x)*c)} ${A(d+(z-d)*c)}`},f=`M${n(0,-1)}`;for(let u=0;u<s;u++){let[h,x]=b(u);f+=`Q${A(h)} ${A(x)} ${n(u,u+1)}`,c<.5&&(f+=`L${n(u+1,u)}`)}return f+"Z"}function xt(r,e,t,a){let s=A(r-t),o=A(r+t);return`M${s} ${A(e-a)}H${o}V${A(e+a)}H${s}Z`}function ht(r,e,t,a,s){let o=Math.max(1.05,s),i=t*Math.sqrt(1-1/(o*o)),c=e-a/o,p=e-o*a,g=i*.14,b=c+.86*(p-c);return`M${A(r-i)} ${A(c)}L${A(r-g)} ${A(b)}Q${A(r)} ${A(p)} ${A(r+g)} ${A(b)}L${A(r+i)} ${A(c)}Z`}function je(r,e){for(let t=0;t<e.length;t++)r=Math.imul(r^e[t],3432918353),r=r<<13|r>>>19;return r}function $t(r){return r=Math.imul(r^r>>>16,2246822507),r=Math.imul(r^r>>>13,3266489909),(r^r>>>16)>>>0}var Hr=new TextEncoder;function yt(r){return r.normalize("NFC").trim().toLowerCase()}function zt(r,e=!0){let t=e?yt(r):r;return je(1779033703^t.length,Hr.encode(t))}function cr(r,e){return $t(je(je(r,Uint8Array.of(255)),Hr.encode(e)))/4294967296}function vt(r,e=!0,t){let a=zt(r,e),s=o=>{let i=t?.[o],c=Array.isArray(i)?i[Math.floor(cr(a,o)*i.length)]:i;return c===void 0?cr(a,o):c>0?c<1?c:.999999:0};return s.num=(o,i,c)=>i+s(o)*(c-i),s.int=(o,i,c)=>i+Math.floor(s(o)*(c-i+1)),s.pick=(o,i)=>i[Math.floor(s(o)*i.length)],s.bool=(o,i=.5)=>s(o)<i,s.jitter=(o,i)=>(s(o)*2-1)*i,s}function Mt(r,e,t){let a=e.expression;return t||!a?{l:r,wrap:""}:a.bake(r,a.p)}var kt=(r,e)=>e?`<g transform="${e}">${r}</g>`:r;function wt(r,e){let t=vt(r,e.normalize??!0,e.traits);return{t,palette:{...gt(e.hue??t.num("hue",0,360),e.contrast??!0,e.tone??t("tone")),...e.palette}}}function Ht(r,e,t){let a=e.background??r.background;if(a!==!1)return{d:a==="square"?"M0 0H100V100H0Z":Ie({cx:50,cy:50,rx:50,ry:50,n:a==="circle"?2:6}),fill:t.bg}}function Yt(r){return(e,t={},a)=>{let{t:s,palette:o}=wt(e,t),i=a?.(s,o),c=Mt(r.layout(s),t,i);return{cls:i?.cls,bg:Ht(r,t,o),inner:kt(r.render(c.l,o,!!i),c.wrap),vars:i?.vars}}}var Wt=(r,e,t)=>{let a=e.rx,s=r.num("eye.rx",.075,.105)*a,o=r.num("eye.ratio",1.9,3.2),i=r.num("eye.scale",.78,1.24),c=r.num("eye.stretch",.85,1.18),p=r.num("eye.gap",.1,.24)*a,g=s*Math.max(1,i),b=s*o*Math.max(1,i*c),n=g+a*.03+p,f=r.jitter("gaze.x",.09)*t.rx,u=r.num("gaze.y",-.2,.08)*t.ry,h=r.jitter("eye.dy",.04)*t.ry,x=Math.hypot(g,b),d=Math.hypot((Math.abs(f)+n+x)/t.rx,(Math.abs(u)+Math.abs(h)+x)/t.ry),m=d>.9?.9/d:1,z=s*m,M=z*o,w=n*m,v=Math.max(0,Math.min(1,p/b)),H=Math.min(12,Math.asin(v)*180/Math.PI),k=r.num("eye.lean",-1,1)*H,y=Math.max(-12,Math.min(12,k+r.jitter("eye.lean2",3.5))),_=t.cx+f*m,X=t.cy+u*m;return[{cx:_-w,cy:X,rx:z,ry:M,n:r.num("eye.n",3.5,6),rot:k},{cx:_+w,cy:X+h*m,rx:z*i,ry:M*i*c,n:r.num("eye.n",3.5,6),rot:y}]};function Xt(r,e){let t=o=>(r.find(([,i])=>o<i)??r[r.length-1])[0];function a(o){let i=t(o("shape")),c=o.num("body.r",31,38)*i.core,p={cx:50+o.jitter("body.x",1.5),cy:50+o.jitter("body.y",1.5),rx:c,ry:c*o.num("body.ratio",.92,1.08),n:o.num("body.n",1.9,2.5),rot:0,radii:Array.from({length:o.int("body.pts",6,8)},(n,f)=>1+o.jitter(`body.r${f}`,.16))};i.body?.(o,p);let g=i.face?.(p)??p,b={petals:[],extra:[]};return i.decorate?.(o,p,b),{shape:i.name,draw:i.path,body:p,face:g,petals:b.petals,extra:b.extra,eyes:e(o,p,g)}}function s(o,i,c){let p=n=>Math.round(n*100)/100,g=(n,f)=>{let u=`<path d="${Ie(n)}"/>`;return c?`<g class="mo-eye" style="--mo-wrap:${f?1:-1};--mo-lean:${p(n.rot)};transform-origin:${p(n.cx)}px ${p(n.cy)}px">${u}</g>`:u},b=`<g fill="${i.head}">`+o.petals.map(n=>`<circle cx="${p(n.cx)}" cy="${p(n.cy)}" r="${p(n.r)}"/>`).join("")+o.extra.map(n=>`<path d="${n}"/>`).join("")+`<path d="${o.draw?o.draw(o.body):Ie(o.body)}"/></g><g fill="${i.eye}"${c?' class="mo-eyes"':""}>`+o.eyes.map(g).join("")+"</g>";return c?`<g class="mo-breathe"><g class="mo-bob">${b}</g></g>`:b}return{layout:a,render:s,background:!1}}var Yr=r=>mt(r),Wr=r=>dt(r.cx,r.cy,r.rx,r.ry,r.radii,r.rot),Be=r=>e=>({cx:e.cx,cy:e.cy,rx:e.rx*r,ry:e.ry*r}),Xr=r=>Be(Math.min(...r.radii)*.95)(r),_t=r=>Be(.84)(r),Rt={name:"round",core:1},Et={name:"organic",core:.98,path:Wr,face:Xr},Ot={name:"boxy",core:.86,body:(r,e)=>{e.n=r.num("body.n",3.4,6),e.rot=r.num("body.rot",-20,20)}},St={name:"capsule",core:1.02,body:(r,e)=>{e.ry*=r.num("capsule.squat",.55,.68)},face:Be(.94),decorate:(r,e,t)=>{for(let a of[-1,1])t.petals.push({cx:e.cx+a*(e.rx-e.ry),cy:e.cy,r:e.ry})},path:r=>xt(r.cx,r.cy,r.rx-r.ry,r.ry)},Pt={name:"nub",core:.88,decorate:(r,e,t)=>{let a=r.int("nub.n",1,2);for(let s=0;s<a;s++){let o=r.num(`nub.a${s}`,0,2*Math.PI);t.petals.push({cx:e.cx+Math.cos(o)*e.rx*.88,cy:e.cy+Math.sin(o)*e.rx*.88,r:e.rx*r.num(`nub.r${s}`,.24,.4)})}}},Tt={name:"cloud",core:.78,face:Xr,path:Wr,decorate:(r,e,t)=>{let a=r.int("cloud.n",4,6);for(let s=0;s<a;s++){let o=Math.PI+Math.PI*(s+.5)/a;t.petals.push({cx:e.cx+Math.cos(o)*e.rx*.8,cy:e.cy+Math.sin(o)*e.rx*.5,r:e.rx*r.num(`cloud.r${s}`,.44,.62)})}}},Ct={name:"droplet",core:.78,body:(r,e)=>{e.cy+=.22*e.ry,e.n=2},face:r=>({cx:r.cx,cy:r.cy+r.ry*.05,rx:r.rx*.88,ry:r.ry*.88}),decorate:(r,e,t)=>{t.extra.push(ht(e.cx,e.cy,e.rx,e.ry,r.num("droplet.tip",1.4,1.65)))}},At={name:"hexagon",core:1.05,path:Yr,face:_t,body:(r,e)=>{e.sides=6,e.rot=r.num("body.rot",-12,12),e.round=r.num("poly.round",.24,.5)}},It={name:"sun",core:.7,decorate:(r,e,t)=>{let a=r.int("sun.n",6,9),s=e.rx*r.num("sun.dist",1,1.08),o=e.rx*r.num("sun.r",.2,.26),i=r.num("sun.rot",0,2*Math.PI);for(let c=0;c<a;c++){let p=i+2*Math.PI*c/a;t.petals.push({cx:e.cx+Math.cos(p)*s,cy:e.cy+Math.sin(p)*s,r:o})}}},jt={name:"triangle",core:1.15,path:Yr,body:(r,e)=>{e.sides=3,e.rot=r.num("body.rot",-5,5),e.round=r.num("poly.round",.24,.5)},face:r=>({cx:r.cx,cy:r.cy+r.ry*.1,rx:r.rx*.54,ry:r.ry*.36})},Ft=[[Rt,.22],[Et,.48],[Ot,.6],[St,.7],[Pt,.79],[Tt,.86],[Ct,.915],[At,.95],[It,.98],[jt,1]],qt=Xt(Ft,Wt),Dt=(r,e)=>(t,a)=>{let s=e?e.vars(e.p):{},o=e?.tint?e.tint(a,e.p):a;return{cls:at(r,!!Object.keys(s).length||!!e?.tint),vars:{...it(t),"--mo-head":o.head,"--mo-eye":o.eye,...s}}};function Nt(r,e={}){return Yt(qt)(r,e,e.animate&&Dt(e.animate,e.expression))}function Ee({l:r,c:e,h:t}){let a=t*Math.PI/180,s=e*Math.cos(a),o=e*Math.sin(a),i=r+.3963377774*s+.2158037573*o,c=r-.1055613458*s-.0638541728*o,p=r-.0894841775*s-1.291485548*o,g=i*i*i,b=c*c*c,n=p*p*p;return[4.0767416621*g-3.3077115913*b+.2309699292*n,-1.2684380046*g+2.6097574011*b-.3413193965*n,-.0041960863*g-.7034186147*b+1.707614701*n]}var lr=r=>r.every(e=>e>=-1e-4&&e<=1.0001);function _r(r){let e=Ee(r);if(!lr(e)){let t=0,a=r.c;for(let s=0;s<12;s++){let o=(t+a)/2;lr(Ee({...r,c:o}))?t=o:a=o}e=Ee({...r,c:t})}return e.map(t=>Math.min(1,Math.max(0,t)))}function pr(r){let[e,t,a]=_r(r);return .2126*e+.7152*t+.0722*a}function ve(r,e){let t=pr(r),a=pr(e);return(Math.max(t,a)+.05)/(Math.min(t,a)+.05)}function Rr(r,e,t){if(ve(r,e)>=t)return r;let a=r.l>=e.l?1:-1;for(let i of[a,-a]){let c={...r};for(let p=0;p<60;p++){if(c.l=Math.min(1,Math.max(0,c.l+i*.02)),ve(c,e)>=t)return c;if(c.l===0||c.l===1)break}}let s={...r,l:0,c:0},o={...r,l:1,c:0};return ve(s,e)>=ve(o,e)?s:o}function Lt(r){return"#"+_r(r).map(e=>{let t=e<=.0031308?12.92*e:1.055*Math.pow(e,.4166666666666667)-.055;return Math.round(t*255).toString(16).padStart(2,"0")}).join("")}var fr=[[.2,{l:.86,c:.085}],[.36,{l:.9,c:.028}],[.62,{l:.73,c:.135}],[.8,{l:.62,c:.165}],[.93,{l:.87,c:.16}],[1,{l:.34,c:.035}]],Bt=r=>fr.find(([e])=>r<e)?.[1]??fr[0][1],Ut={l:.145,c:0,h:0},Vt=1.5,Gt=(r,e)=>{let t=Bt(e),a=Rr({l:t.l,c:t.c,h:r},Ut,Vt);return{bg:{l:.965,c:.01,h:r},head:a,eye:a.l>=.5?{l:.17,c:.02,h:r}:{l:.97,c:.012,h:r}}},Zt=[["head","bg",1.25],["eye","head",4.5]];function Qt(r,e=!0,t=0){let a=Gt(r,t);if(e)for(let[s,o,i]of Zt)a[s]=Rr(a[s],a[o],i);return a}function Jt(r,e=!0,t=0){let a=Qt(r,e,t),s={};for(let o in a)s[o]=Lt(a[o]);return s}var I=r=>{let e=Math.round(r*100)/100;return Object.is(e,-0)?"0":String(e)};function Fe({cx:r,cy:e,rx:t,ry:a,n:s=4,rot:o=0}){let i=Math.min(1,(8*Math.pow(2,-1/s)-4)/3),c=t,p=a,g=c*i,b=p*i,n=[[c,0],[c,b],[g,p],[0,p],[-g,p],[-c,b],[-c,0],[-c,-b],[-g,-p],[0,-p],[g,-p],[c,-b],[c,0]],f=o*Math.PI/180,u=Math.cos(f),h=Math.sin(f),x=m=>{let[z,M]=n[m];return`${I(r+z*u-M*h)} ${I(e+z*h+M*u)}`},d=`M${x(0)}`;for(let m=1;m<13;m+=3)d+=`C${x(m)} ${x(m+1)} ${x(m+2)}`;return d+"Z"}function Kt(r,e,t,a,s,o=0){let i=s.length,c=o*Math.PI/180,p=s.map((n,f)=>{let u=c+2*Math.PI*f/i;return[r+t*n*Math.cos(u),e+a*n*Math.sin(u)]}),g=n=>p[(n%i+i)%i],b=`M${I(g(0)[0])} ${I(g(0)[1])}`;for(let n=0;n<i;n++){let[f,u]=g(n-1),[h,x]=g(n),[d,m]=g(n+1),[z,M]=g(n+2);b+=`C${I(h+(d-f)/6)} ${I(x+(m-u)/6)} ${I(d-(z-h)/6)} ${I(m-(M-x)/6)} ${I(d)} ${I(m)}`}return b+"Z"}function eo({cx:r,cy:e,rx:t,ry:a,sides:s,round:o=.3,rot:i=0}){let c=o>0?o<1?o/2:.5:0,p=i*Math.PI/180-Math.PI/2,g=Array.from({length:s},(u,h)=>{let x=p+2*Math.PI*h/s;return[r+t*Math.cos(x),e+a*Math.sin(x)]}),b=u=>g[(u%s+s)%s],n=(u,h)=>{let[x,d]=b(u),[m,z]=b(h);return`${I(x+(m-x)*c)} ${I(d+(z-d)*c)}`},f=`M${n(0,-1)}`;for(let u=0;u<s;u++){let[h,x]=b(u);f+=`Q${I(h)} ${I(x)} ${n(u,u+1)}`,c<.5&&(f+=`L${n(u+1,u)}`)}return f+"Z"}function ro(r,e,t,a){let s=I(r-t),o=I(r+t);return`M${s} ${I(e-a)}H${o}V${I(e+a)}H${s}Z`}function to(r,e,t,a,s){let o=Math.max(1.05,s),i=t*Math.sqrt(1-1/(o*o)),c=e-a/o,p=e-o*a,g=i*.14,b=c+.86*(p-c);return`M${I(r-i)} ${I(c)}L${I(r-g)} ${I(b)}Q${I(r)} ${I(p)} ${I(r+g)} ${I(b)}L${I(r+i)} ${I(c)}Z`}function qe(r,e){for(let t=0;t<e.length;t++)r=Math.imul(r^e[t],3432918353),r=r<<13|r>>>19;return r}function oo(r){return r=Math.imul(r^r>>>16,2246822507),r=Math.imul(r^r>>>13,3266489909),(r^r>>>16)>>>0}var Er=new TextEncoder;function ao(r){return r.normalize("NFC").trim().toLowerCase()}function so(r,e=!0){let t=e?ao(r):r;return qe(1779033703^t.length,Er.encode(t))}function br(r,e){return oo(qe(qe(r,Uint8Array.of(255)),Er.encode(e)))/4294967296}function io(r,e=!0,t){let a=so(r,e),s=o=>{let i=t?.[o],c=Array.isArray(i)?i[Math.floor(br(a,o)*i.length)]:i;return c===void 0?br(a,o):c>0?c<1?c:.999999:0};return s.num=(o,i,c)=>i+s(o)*(c-i),s.int=(o,i,c)=>i+Math.floor(s(o)*(c-i+1)),s.pick=(o,i)=>i[Math.floor(s(o)*i.length)],s.bool=(o,i=.5)=>s(o)<i,s.jitter=(o,i)=>(s(o)*2-1)*i,s}function no(r,e,t){let a=e.expression;return a?a.bake(r,a.p):{l:r,wrap:""}}var co=(r,e)=>e?.tint?e.tint(r,e.p):r,lo=(r,e)=>e?`<g transform="${e}">${r}</g>`:r,po=r=>r.replace(/[&<>]/g,e=>e==="&"?"&amp;":e==="<"?"&lt;":"&gt;");function fo(r,e){let t=io(r,e.normalize??!0,e.traits);return{t,palette:{...Jt(e.hue??t.num("hue",0,360),e.contrast??!0,e.tone??t("tone")),...e.palette}}}var bo=r=>r.title?`<title>${po(r.title)}</title>`:"";function uo(r,e,t){let a=e.background??r.background;if(a!==!1)return{d:a==="square"?"M0 0H100V100H0Z":Fe({cx:50,cy:50,rx:50,ry:50,n:a==="circle"?2:6}),fill:t.bg}}var go=r=>r?`<path d="${r.d}" fill="${r.fill}"/>`:"";function mo(r){return(e,t={})=>{let{t:a,palette:s}=fo(e,t),o=co(s,t.expression),i=t.size?` width="${t.size}" height="${t.size}"`:"",c=no(r.layout(a),t),p=bo(t)+go(uo(r,t,o))+lo(r.render(c.l,o),c.wrap);return`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"${i}>${p}</svg>`}}var xo=(r,e,t)=>{let a=e.rx,s=r.num("eye.rx",.075,.105)*a,o=r.num("eye.ratio",1.9,3.2),i=r.num("eye.scale",.78,1.24),c=r.num("eye.stretch",.85,1.18),p=r.num("eye.gap",.1,.24)*a,g=s*Math.max(1,i),b=s*o*Math.max(1,i*c),n=g+a*.03+p,f=r.jitter("gaze.x",.09)*t.rx,u=r.num("gaze.y",-.2,.08)*t.ry,h=r.jitter("eye.dy",.04)*t.ry,x=Math.hypot(g,b),d=Math.hypot((Math.abs(f)+n+x)/t.rx,(Math.abs(u)+Math.abs(h)+x)/t.ry),m=d>.9?.9/d:1,z=s*m,M=z*o,w=n*m,v=Math.max(0,Math.min(1,p/b)),H=Math.min(12,Math.asin(v)*180/Math.PI),k=r.num("eye.lean",-1,1)*H,y=Math.max(-12,Math.min(12,k+r.jitter("eye.lean2",3.5))),_=t.cx+f*m,X=t.cy+u*m;return[{cx:_-w,cy:X,rx:z,ry:M,n:r.num("eye.n",3.5,6),rot:k},{cx:_+w,cy:X+h*m,rx:z*i,ry:M*i*c,n:r.num("eye.n",3.5,6),rot:y}]};function ho(r,e){let t=o=>(r.find(([,i])=>o<i)??r[r.length-1])[0];function a(o){let i=t(o("shape")),c=o.num("body.r",31,38)*i.core,p={cx:50+o.jitter("body.x",1.5),cy:50+o.jitter("body.y",1.5),rx:c,ry:c*o.num("body.ratio",.92,1.08),n:o.num("body.n",1.9,2.5),rot:0,radii:Array.from({length:o.int("body.pts",6,8)},(n,f)=>1+o.jitter(`body.r${f}`,.16))};i.body?.(o,p);let g=i.face?.(p)??p,b={petals:[],extra:[]};return i.decorate?.(o,p,b),{shape:i.name,draw:i.path,body:p,face:g,petals:b.petals,extra:b.extra,eyes:e(o,p,g)}}function s(o,i,c){let p=n=>Math.round(n*100)/100,g=(n,f)=>{let u=`<path d="${Fe(n)}"/>`;return c?`<g class="mo-eye" style="--mo-wrap:${f?1:-1};--mo-lean:${p(n.rot)};transform-origin:${p(n.cx)}px ${p(n.cy)}px">${u}</g>`:u},b=`<g fill="${i.head}">`+o.petals.map(n=>`<circle cx="${p(n.cx)}" cy="${p(n.cy)}" r="${p(n.r)}"/>`).join("")+o.extra.map(n=>`<path d="${n}"/>`).join("")+`<path d="${o.draw?o.draw(o.body):Fe(o.body)}"/></g><g fill="${i.eye}"${c?' class="mo-eyes"':""}>`+o.eyes.map(g).join("")+"</g>";return c?`<g class="mo-breathe"><g class="mo-bob">${b}</g></g>`:b}return{layout:a,render:s,background:!1}}var Or=r=>eo(r),Sr=r=>Kt(r.cx,r.cy,r.rx,r.ry,r.radii,r.rot),Ue=r=>e=>({cx:e.cx,cy:e.cy,rx:e.rx*r,ry:e.ry*r}),Pr=r=>Ue(Math.min(...r.radii)*.95)(r),$o=r=>Ue(.84)(r),yo={name:"round",core:1},zo={name:"organic",core:.98,path:Sr,face:Pr},vo={name:"boxy",core:.86,body:(r,e)=>{e.n=r.num("body.n",3.4,6),e.rot=r.num("body.rot",-20,20)}},Mo={name:"capsule",core:1.02,body:(r,e)=>{e.ry*=r.num("capsule.squat",.55,.68)},face:Ue(.94),decorate:(r,e,t)=>{for(let a of[-1,1])t.petals.push({cx:e.cx+a*(e.rx-e.ry),cy:e.cy,r:e.ry})},path:r=>ro(r.cx,r.cy,r.rx-r.ry,r.ry)},ko={name:"nub",core:.88,decorate:(r,e,t)=>{let a=r.int("nub.n",1,2);for(let s=0;s<a;s++){let o=r.num(`nub.a${s}`,0,2*Math.PI);t.petals.push({cx:e.cx+Math.cos(o)*e.rx*.88,cy:e.cy+Math.sin(o)*e.rx*.88,r:e.rx*r.num(`nub.r${s}`,.24,.4)})}}},wo={name:"cloud",core:.78,face:Pr,path:Sr,decorate:(r,e,t)=>{let a=r.int("cloud.n",4,6);for(let s=0;s<a;s++){let o=Math.PI+Math.PI*(s+.5)/a;t.petals.push({cx:e.cx+Math.cos(o)*e.rx*.8,cy:e.cy+Math.sin(o)*e.rx*.5,r:e.rx*r.num(`cloud.r${s}`,.44,.62)})}}},Ho={name:"droplet",core:.78,body:(r,e)=>{e.cy+=.22*e.ry,e.n=2},face:r=>({cx:r.cx,cy:r.cy+r.ry*.05,rx:r.rx*.88,ry:r.ry*.88}),decorate:(r,e,t)=>{t.extra.push(to(e.cx,e.cy,e.rx,e.ry,r.num("droplet.tip",1.4,1.65)))}},Yo={name:"hexagon",core:1.05,path:Or,face:$o,body:(r,e)=>{e.sides=6,e.rot=r.num("body.rot",-12,12),e.round=r.num("poly.round",.24,.5)}},Wo={name:"sun",core:.7,decorate:(r,e,t)=>{let a=r.int("sun.n",6,9),s=e.rx*r.num("sun.dist",1,1.08),o=e.rx*r.num("sun.r",.2,.26),i=r.num("sun.rot",0,2*Math.PI);for(let c=0;c<a;c++){let p=i+2*Math.PI*c/a;t.petals.push({cx:e.cx+Math.cos(p)*s,cy:e.cy+Math.sin(p)*s,r:o})}}},Xo={name:"triangle",core:1.15,path:Or,body:(r,e)=>{e.sides=3,e.rot=r.num("body.rot",-5,5),e.round=r.num("poly.round",.24,.5)},face:r=>({cx:r.cx,cy:r.cy+r.ry*.1,rx:r.rx*.54,ry:r.ry*.36})},_o=[[yo,.22],[zo,.48],[vo,.6],[Mo,.7],[ko,.79],[wo,.86],[Ho,.915],[Yo,.95],[Wo,.98],[Xo,1]],Ro=ho(_o,xo),Eo=mo(Ro);function Oo(r,e){return"data:image/svg+xml,"+Eo(r,e).replace(/"/g,"'").replace(/[%#<>{}|\\^[\]`]/g,t=>"%"+t.charCodeAt(0).toString(16).toUpperCase()).replace(/\s+/g," ")}var So=new Set(["$$slots","$$events","$$legacy","name","size","background","palette","hue","tone","normalize","contrast","title","animate","expression","traits","class","style"]),Po=Le("<title> </title>"),To=Le("<path></path>"),Co=Le("<svg><!><!><g></g></svg>"),Ao=et("<img/>");function Ka(r,e){Br(e,!0);let t=rt(e,So),a=ie(()=>({size:e.size,background:e.background,palette:e.palette,hue:e.hue,tone:e.tone,normalize:e.normalize,contrast:e.contrast,title:e.title,expression:e.expression,traits:e.traits})),s=ie(()=>e.animate?"":Oo(e.name,Q(a))),o=ie(()=>e.animate?Nt(e.name,{...Q(a),animate:e.animate}):null),i=ie(()=>t),c=ie(()=>t),p=ie(()=>{const{alt:h,...x}=Q(c);return x}),g=ie(()=>[...Object.entries(Q(o)?.vars??{}).map(([h,x])=>`${h}:${x}`),...e.style?[e.style]:[]].join(";"));var b=Ur(),n=Vr(b);{var f=h=>{var x=Co();tr(x,()=>({xmlns:"http://www.w3.org/2000/svg",viewBox:"0 0 100 100",width:e.size,height:e.size,role:e.title?"img":void 0,"aria-hidden":e.title?void 0:!0,class:e.class,style:Q(g),...Q(i)}));var d=Kr(x);{var m=v=>{var H=Po(),k=ot(H,!0);_e(()=>tt(k,e.title)),de(v,H)};Xe(d,v=>{e.title&&v(m)})}var z=or(d);{var M=v=>{var H=To();_e(()=>{ar(H,"d",Q(o).bg.d),ar(H,"fill",Q(o).bg.fill)}),de(v,H)};Xe(z,v=>{Q(o).bg&&v(M)})}var w=or(z);Zr(w,()=>Q(o).inner,!0),_e(()=>Qr(w,0,Jr(Q(o).cls))),de(h,x)},u=h=>{var x=Ao();tr(x,()=>({src:Q(s),width:e.size,height:e.size,alt:Q(c).alt??e.title??"",class:e.class,style:e.style,...Q(p)})),de(h,x)};Xe(n,h=>{Q(o)?h(f):h(u,-1)})}de(r,b),Gr()}function Oe(r,e,t){return r+(e-r)*t}function Tr(r){return r-Math.floor(r)}function Se(r,e){const t=Math.floor(r),a=Math.floor(e);let s=r-t,o=e-a;s=s*s*(3-2*s),o=o*o*(3-2*o);const i=re(t,a),c=re(t+1,a),p=re(t,a+1),g=re(t+1,a+1);return i+(c-i)*s+(p-i)*o+(i-c-p+g)*s*o}function re(r,e){const t=Math.sin(r*12.9898+e*78.233)*43758.5453;return t-Math.floor(t)}function Ve(r,e){const t=Math.PI*(3-Math.sqrt(5)),a=1-2*(r+.5)/e,s=Math.sqrt(1-a*a),o=r*t;return[s*Math.cos(o),a,s*Math.sin(o)]}function Io(r,e){return Math.atan2(Math.sin(r-e),Math.cos(r-e))}function ce(r,e,t,a,s){const o=Math.sin(e),i=Math.cos(e),c=Math.sin(r),p=Math.cos(r);return(g,b,n)=>{const f=g*p+n*c,u=-g*c+n*p,h=b*i-u*o,x=b*o+u*i;return[t+f*s,a-h*s,x]}}function Cr(r,e,t,a){{const s=Math.round((t?1-r:r)*255);return`rgba(${s},${s},${s},${e})`}}function jo(r,e,t,a=.3,s){for(const o of e){const i=o.a??1,c=Math.min(1,Math.max(0,o.white));r.fillStyle=Cr(c,i,t),r.beginPath(),r.arc(o.x,o.y,o.r,0,Math.PI*2),r.fill()}}function Fo(r,e,t,a){for(const s of e){const o=s.a??1,i=Math.min(1,Math.max(0,s.white));r.strokeStyle=Cr(i,o,t),r.lineWidth=s.w,r.beginPath(),r.moveTo(s.x1,s.y1),r.lineTo(s.x2,s.y2),r.stroke()}}function se(r,e,t=.3){const a=[];for(const s of r)(s.a??1)<.02||(s.r=Math.max(t,s.r),a.push(s));return a.sort((s,o)=>s.z-o.z),{dots:a,lines:e.filter(s=>(s.a??1)>=.02)}}function qo(r,e,t,a){e.lines.length&&Fo(r,e.lines,t),jo(r,e.dots,t,.3)}function le(r,e){return(r/300)**e}const Do=[["latRings","lonDensity"],["rings","lonDensity"],["lanes","segs"]],No=["orbitN","ghostN","nodeN","strandN","signals"],Lo=["iconD"],Bo=["rBase","rDepth","rActive","rDot","ghostR","partR","partRDepth","nodeR","nodeRDepth"];function Uo(r,e){const t={...r},a=new Set,s=Math.sqrt(e);for(const[o,i]of Do){const c=t[o],p=t[i];c!=null&&p!=null&&!a.has(o)&&!a.has(i)&&(t[o]=Math.max(2,Math.round(c*s)),t[i]=Math.max(2,Math.round(p*s)),a.add(o),a.add(i))}for(const o of No){const i=t[o];i!=null&&i!==0&&!a.has(o)&&(t[o]=Math.max(1,Math.round(i*e)))}for(const o of Lo){const i=t[o];i!=null&&(t[o]=Math.max(.02,i*e))}return t}function Vo(r,e){const t={...r};for(const a of Bo){const s=t[a];s!=null&&(t[a]=s*e)}return t.rSizeMul=(t.rSizeMul??1)*e,t}const Go={globe:{latRings:17,lonDensity:44,rBase:.6,rDepth:1.7,rBoost:1,inkFar:.62,inkSpan:.54,rsPow:.6,rMin:.3},orbits:{orbitN:12,ghostN:40,ghostR:.9,ghostA:.5,particles:3,partR:1.2,partRDepth:1.6,rsPow:.6,rMin:.3},rubik:{latRings:15,lonDensity:40,moveCount:14,rBase:.6,rDepth:1.7,rActive:.3,inkFar:.62,inkSpan:.54,rsPow:.6,rMin:.3},wave:{rings:15,lonDensity:40,rBase:.6,rDepth:1.7,rsPow:.6,rMin:.3},web:{nodeN:30,thr:.72,signals:5,nodeR:1.4,nodeRDepth:1.8,lineW:.8,rsPow:.6,rMin:.3},braid:{strandN:52,turns:3,ghostN:150,rBase:1.2,rDepth:1.8,rsPow:.6,rMin:.3},ribbon:{lanes:5,segs:88,ghostN:150,rBase:1.1,rDepth:1.7,rsPow:.6,rMin:.3},ring:{lanes:5,segs:88,ghostN:0,faceOn:1,rBase:1.1,rDepth:1.7,rsPow:.6,rMin:.3},morph:{rDot:.021,iconD:1,rMin:.25}},Zo=(r,e,t)=>{const a=r/2,s=r/2,o=r/2*.76,i=ce(e*.4,.3,a,s,1),c=le(r,t.rsPow??.6),p=[],g=t.ghostN??150;for(let f=0;f<g;f++){const u=Ve(f,g),[h,x,d]=i(u[0]*o,u[1]*o,u[2]*o),m=(d/o+1)/2;p.push({x:h,y:x,z:d,r:.8*c,white:.78,a:.1+.22*m})}const b=t.strandN??52,n=t.turns??3;for(let f=0;f<3;f++){const u=f/3*2*Math.PI;for(let h=0;h<b;h++){const x=(Tr(h/b+e*.045)*2-1)*.96,d=Math.sqrt(Math.max(0,1-x*x)),m=Math.min(1,(1-Math.abs(x))/.1),z=x*Math.PI*n+u,M=1+.075*Math.sin(x*Math.PI*n*2+u*2+e*.8),w=d*o*M,[v,H,k]=i(Math.cos(z)*w,x*o*M,Math.sin(z)*w),y=(k/o+1)/2;p.push({x:v,y:H,z:k,r:((t.rBase??1.2)+(t.rDepth??1.8)*y)*c,white:.55-.45*y,a:m*(.45+.55*y)})}}return se(p,[],t.rMin)};function Qo(r,e,t,a){const s=2*e*t+a,o=r%s,i=new Array(e).fill(0);let c=-1;if(o<2*e*t){const p=Math.floor(o/t),g=(o-p*t)/t,n=1-(1-Math.min(1,g/.7))**3;if(p<e){for(let f=0;f<p;f++)i[f]=1;i[p]=n,c=p}else{const f=2*e-1-p;for(let u=0;u<f;u++)i[u]=1;i[f]=1-n,c=f}}return{amount:i,active:c}}function Jo(r,e,t){let[a,s,o]=r,i=!1;for(let c=0;c<e.length;c++){if(t.amount[c]<=0)continue;const p=e[c],g=p.axis===0?a:p.axis===1?s:o;if(g<p.lo||g>=p.hi)continue;c===t.active&&(i=!0);const b=p.ang*t.amount[c],n=Math.cos(b),f=Math.sin(b);if(p.axis===0){const u=s*n-o*f;o=s*f+o*n,s=u}else if(p.axis===1){const u=a*n+o*f;o=-a*f+o*n,a=u}else{const u=a*n-s*f;s=a*f+s*n,a=u}}return[a,s,o,i]}function Ko(r){const e=[];for(let t=0;t<r;t++){const a=Math.min(2,Math.floor(re(t,2.3)*3)),s=-1+.5*Math.min(3,Math.floor(re(t,5.9)*4)),o=re(t,7.7)<.5?1:-1;e.push({axis:a,lo:s,hi:s+.5,ang:o*Math.PI/2})}return e}const ea=(r,e,t)=>{const s=r/2,o=r/2,i=r/2*.82,c=.4+.06*Math.sin(e*.35),p=ce(e*.5,c,s,o,i),g=e*(.5+(1.7-.5)*(t.scanMul??1)),b=le(r,t.rsPow??.6),n=t.dimBase??1,f=[],u=t.latRings??17,h=t.lonDensity??44;for(let x=0;x<=u;x++){const d=-Math.PI/2+x/u*Math.PI,m=Math.cos(d),z=Math.sin(d),M=Math.max(1,Math.round(Math.abs(m)*h));for(let w=0;w<M;w++){const v=w/M*2*Math.PI,[H,k,y]=p(m*Math.cos(v),z,m*Math.sin(v)),_=(y+1)/2,X=Io(v+e*.5,g),E=Math.exp(-(X*X)/.18)*Math.max(0,y);f.push({x:H,y:k,z:y,r:((t.rBase??.6)+(t.rDepth??1.7)*_+(t.rBoost??1)*E)*b,white:(t.inkFar??.62)-(t.inkSpan??.54)*_,a:n+(1-n)*Math.min(1,E)})}}return se(f,[],t.rMin)},ra=(r,e,t)=>{const a=r/2,s=r/2,o=r/2*.82,i=ce(e*.55,.35+.1*Math.sin(e*.9),a,s,o),c=le(r,t.rsPow??.6),p=t.moveCount??14,g=Ko(p),b=Qo(e,p,.42,1.2),n=[],f=t.latRings??15,u=t.lonDensity??40;for(let h=0;h<=f;h++){const x=-Math.PI/2+h/f*Math.PI,d=Math.cos(x),m=Math.sin(x),z=Math.max(1,Math.round(Math.abs(d)*u));for(let M=0;M<z;M++){const w=M/z*2*Math.PI,[v,H,k,y]=Jo([d*Math.cos(w),m,d*Math.sin(w)],g,b),[_,X,E]=i(v,H,k),O=(E+1)/2;n.push({x:_,y:X,z:E,r:((t.rBase??.6)+(t.rDepth??1.7)*O+(y?t.rActive??.3:0))*c,white:(t.inkFar??.62)-(t.inkSpan??.54)*O-(y?.14:0)})}}return se(n,[],t.rMin)},ta=(r,e,t)=>{const a=r/2,s=r/2,o=r/2*.874,i=ce(e*.18,.38,a,s,1),c=le(r,t.rsPow??.6),p=[],g=t.rings??15,b=t.lonDensity??40;for(let n=0;n<=g;n++){const f=-Math.PI/2+n/g*Math.PI,u=Math.cos(f),h=Math.sin(f),x=.62*Math.sin(e*2.1-n*.52)+.38*Math.sin(e*1.27+n*.83),d=o*(.88+.105*x),m=Math.max(1,Math.round(Math.abs(u)*b));for(let z=0;z<m;z++){const M=z/m*2*Math.PI,[w,v,H]=i(u*Math.cos(M)*d,h*d,u*Math.sin(M)*d),k=(H/o+1)/2,y=Math.max(0,x);p.push({x:w,y:v,z:H,r:((t.rBase??.6)+(t.rDepth??1.7)*k)*(1+.4*y)*c,white:.66-.56*k-.1*y})}}return se(p,[],t.rMin)};function oa(r){return r*r*(3-2*r)}function Ar(r){const e=r.length,t=[];let a=0;for(let s=0;s<e;s++){const o=r[s],i=r[(s+1)%e],c=Math.hypot(i[0]-o[0],i[1]-o[1]);t.push(c),a+=c}return s=>{let o=s*a,i=0;for(;o>t[i]&&i<e-1;)o-=t[i],i++;const c=r[i],p=r[(i+1)%e],g=t[i]?Math.min(1,o/t[i]):0;return[c[0]+(p[0]-c[0])*g,c[1]+(p[1]-c[1])*g]}}const aa=r=>{const e=-Math.PI/2+r*2*Math.PI;return[Math.cos(e)*.24,Math.sin(e)*.24]},sa=Ar([[0,-.26],[.24,.16],[-.24,.16]]),ia=Ar([[0,-.2],[.2,-.2],[.2,.2],[-.2,.2],[-.2,-.2]]),Pe=[aa,sa,ia];function na(r){return Math.max(6,Math.round(34*r))}const De=1.4,Ir=.9,Me=De+Ir,ca=(r,e,t)=>{const a=Pe.length,s=e%(Me*a),o=t.shape!=null&&t.shape>=0&&t.shape<a?Math.floor(t.shape):-1,i=o>=0?o:Math.floor(s/Me),c=o>=0?e%Me:s-i*Me,p=o>=0?0:c>De?oa((c-De)/Ir):0,g=t.spread??1,b=Pe[i],n=o>=0?b:Pe[(i+1)%a],f=160,u=[];for(let k=0;k<f;k++){const y=k/f,_=b(y),X=n(y);u.push([(_[0]+(X[0]-_[0])*p)*g,(_[1]+(X[1]-_[1])*p)*g])}const h=[];let x=0;for(let k=0;k<f;k++){const y=u[k],_=u[(k+1)%f],X=Math.hypot(_[0]-y[0],_[1]-y[1]);h.push(X),x+=X}const d=na(t.iconD??1),m=(t.rDot??.021)*1.35*g,z=1+.02*Math.sin(c*3.1),M=[],w=r/2;let v=0,H=0;for(let k=0;k<d;k++){const y=k/d*x;for(;H+h[v]<y&&v<f-1;)H+=h[v],v++;const _=u[v],X=u[(v+1)%f],E=h[v]?Math.min(1,(y-H)/h[v]):0,O=(_[0]+(X[0]-_[0])*E)*z,B=(_[1]+(X[1]-_[1])*E)*z;M.push({x:w+O*r,y:w+B*r,z:0,r:Math.max(.35,m*r),white:.1})}return se(M,[],t.rMin)},la=(r,e,t)=>{const a=r/2,s=r/2,o=r/2*.82,i=ce(e*.12,.3,a,s,1),c=le(r,t.rsPow??.6),p=[],g=t.orbitN??12,b=t.ghostN??40,n=t.particles??3;for(let f=0;f<g;f++){const u=re(f,1.7),h=re(f,5.2),x=re(f,8.9),d=o*(.45+.52*u),m=u*2*Math.PI,z=Math.acos(2*h-1),M=Math.sin(z)*Math.cos(m),w=Math.cos(z),v=Math.sin(z)*Math.sin(m);let H=-w,k=M;const y=0,_=Math.max(1e-6,Math.sqrt(H*H+k*k));H/=_,k/=_;const X=w*y-v*k,E=v*H-M*y,O=M*k-w*H,B=(.25+.55*x)*(x>.5?1:-1);for(let N=0;N<b;N++){const T=N/b*2*Math.PI,[j,D,V]=i((H*Math.cos(T)+X*Math.sin(T))*d,(k*Math.cos(T)+E*Math.sin(T))*d,(y*Math.cos(T)+O*Math.sin(T))*d),U=(V/d+1)/2;p.push({x:j,y:D,z:V,r:(t.ghostR??.9)*c,white:.72,a:(t.ghostA??.5)*(.4+.6*U)})}for(let N=0;N<n;N++){const T=e*B+N/n*2*Math.PI+h*6,[j,D,V]=i((H*Math.cos(T)+X*Math.sin(T))*d,(k*Math.cos(T)+E*Math.sin(T))*d,(y*Math.cos(T)+O*Math.sin(T))*d),U=(V/d+1)/2;p.push({x:j,y:D,z:V,r:((t.partR??1.2)+(t.partRDepth??1.6)*U)*c,white:.3-.22*U})}}return se(p,[],t.rMin)},ur=(r,e,t)=>{const a=r/2,s=r/2,o=r/2*.78,i=t.spin??1,c=.3,p=ce(e*.1*i,c,a,s,1),g=le(r,t.rsPow??.6),b=[],n=t.ghostN??150;for(let O=0;O<n;O++){const B=Ve(O,n),[N,T,j]=p(B[0]*o,B[1]*o,B[2]*o),D=(j/o+1)/2;b.push({x:N,y:T,z:j,r:.8*g,white:.78,a:.1+.22*D})}const f=e*.24*i,u=t.faceOn?-c:.55+.3*Math.sin(e*.18)*i,h=Math.cos(f),x=0,d=Math.sin(f),m=-d*Math.sin(u),z=Math.cos(u),M=h*Math.sin(u),w=x*M-d*z,v=d*m-h*M,H=h*z-x*m,k=.23*(t.wobMul??1),y=t.faceOn?o/(1+.85*k):o,_=t.lanes??5,X=t.segs??88,E=Math.max(1,Math.round(_*(t.bandMul??1)));for(let O=0;O<E;O++){const B=(O-(E-1)/2)*.075,N=Math.abs(O-(E-1)/2)/Math.max(1,(E-1)/2);for(let T=0;T<X;T++){const j=T/X*2*Math.PI,D=(.16*Math.sin(j*3-e*1.7+O*.22)+.07*Math.sin(j*5+e*1.1))*(t.wobMul??1),V=t.faceOn?1+D:1,U=t.faceOn?B:B+D,q=h*Math.cos(j)+m*Math.sin(j)+w*U,K=x*Math.cos(j)+z*Math.sin(j)+v*U,te=d*Math.cos(j)+M*Math.sin(j)+H*U,oe=Math.sqrt(q*q+K*K+te*te),l=y*V,[$,Y,W]=p(q/oe*l,K/oe*l,te/oe*l),S=(W/o+1)/2;b.push({x:$,y:Y,z:W,r:((t.rBase??1.1)+(t.rDepth??1.7)*S)*(1-.25*N)*g,white:.52-.44*S+.18*N,a:.4+.6*S})}}return se(b,[],t.rMin)},pa=(r,e,t)=>{const a=r/2,s=r/2,o=r/2*.8*(t.spread??1),i=ce(e*.12,.32,a,s,o),c=le(r,t.rsPow??.6),p=t.nodeN??30,g=t.thr??.72,b=t.nodeR??1.4,n=t.nodeRDepth??1.8,f=[];for(let d=0;d<p;d++){const m=Ve(d,p),z=m[0]+.3*(Se(d*.31+9,e*.24)-.5)*2,M=m[1]+.3*(Se(d*.53+27,e*.21)-.5)*2,w=m[2]+.3*(Se(d*.77+55,e*.27)-.5)*2,v=Math.sqrt(z*z+M*M+w*w);f.push([z/v,M/v,w/v])}const u=[],h=[];for(let d=0;d<p;d++)for(let m=d+1;m<p;m++){const z=f[d][0]-f[m][0],M=f[d][1]-f[m][1],w=f[d][2]-f[m][2],v=Math.sqrt(z*z+M*M+w*w);if(v>=g)continue;const[H,k,y]=i(f[d][0],f[d][1],f[d][2]),[_,X,E]=i(f[m][0],f[m][1],f[m][2]),O=((y+E)/2+1)/2;u.push({x1:H,y1:k,x2:_,y2:X,white:.42,a:(1-v/g)*(.3+.55*O),w:Math.max(.6,(t.lineW??.8)*c)})}for(let d=0;d<p;d++){const[m,z,M]=i(f[d][0],f[d][1],f[d][2]),w=(M+1)/2,v=1+.25*Math.sin(e*1.4+d*2.7);h.push({x:m,y:z,z:M,r:(b+n*w)*v*c,white:.55-.45*w})}const x=t.signals??5;for(let d=0;d<x;d++){const m=Math.floor(e*.55+d*7.31),z=Math.floor(re(m,d*3.1+1.7)*p),M=Math.floor(re(m,d*5.7+4.2)*p);if(z===M)continue;const w=Tr(e*.55+d*7.31),v=Oe(f[z][0],f[M][0],w),H=Oe(f[z][1],f[M][1],w),k=Oe(f[z][2],f[M][2],w),y=Math.max(1e-6,Math.sqrt(v*v+H*H+k*k)),[_,X,E]=i(v/y,H/y,k/y),O=(E+1)/2;h.push({x:_,y:X,z:E,r:(b*1.5+n*O)*c,white:.05,a:.5+.5*O})}return se(h,u,t.rMin)},fa={orbits:la,globe:ea,rubik:ra,wave:ta,web:pa,braid:Zo,ribbon:ur,ring:ur,morph:ca};Object.fromEntries(Object.entries(fa).map(([r,e])=>[r,(t,a,s,o,i)=>qo(t,e(a,s,i),o)]));const ba={working:"orbits",searching:"globe",solving:"rubik",listening:"wave",connecting:"web",weaving:"braid",composing:"ribbon",breathing:"ring",shaping:"morph"},ua={orbits:{64:{speed:1.885,count:1,size:1},32:{speed:2.9072,count:.4251,size:1.6849},20:{speed:3.9,count:.238,size:2.4}},globe:{64:{speed:2.015,count:.42,size:1.15,extra:{scanMul:4.08,dimBase:.45}},32:{speed:2.3803,count:.1839,size:1.4769,extra:{scanMul:4.2301,dimBase:.45}},20:{speed:2.665,count:.105,size:1.75,extra:{scanMul:4.335,dimBase:.45}}},rubik:{64:{speed:1.82,count:.35,size:1.05},32:{speed:1.8964,count:.1537,size:1.4951},20:{speed:1.95,count:.088,size:1.9}},wave:{64:{speed:4.388,count:.341,size:1},32:{speed:4.1512,count:.169,size:1.3232},20:{speed:3.998,count:.105,size:1.6}},web:{64:{speed:3.315,count:1.35,size:.95},32:{speed:5.0104,count:.4942,size:1.2571},20:{speed:6.63,count:.25,size:1.52}},braid:{64:{speed:1.625,count:.5,size:1},32:{speed:2.2234,count:.2056,size:1.2011},20:{speed:2.75,count:.1125,size:1.36}},ribbon:{64:{speed:2.34,count:.25,size:.85,extra:{spin:0,bandMul:3.9,wobMul:1}},32:{speed:2.7776,count:.0969,size:.9766,extra:{spin:0,bandMul:4.49,wobMul:1}},20:{speed:3.12,count:.051,size:1.073,extra:{spin:0,bandMul:4.94,wobMul:1}}},ring:{64:{speed:3.24,count:.25,size:.956,extra:{spin:0,bandMul:3.627,wobMul:.368}},32:{speed:3.5517,count:.0678,size:1.31,extra:{spin:0,bandMul:3.8265,wobMul:.4751}},20:{speed:3.78,count:.028,size:1.622,extra:{spin:0,bandMul:3.968,wobMul:.565}}},morph:{64:{speed:2.405,count:.702,size:.395,extra:{spread:1.45}},32:{speed:2.2057,count:.5937,size:.6916,extra:{spread:1.45}},20:{speed:2.08,count:.53,size:1.011,extra:{spread:1.45}}}},gr=new Map;function e0(r,e){const t=`${r}-${e}`,a=gr.get(t);if(a)return a;const s=ba[r],o=ua[s][e];let i={...Go[s]};o.count!==1&&(i=Uo(i,o.count)),o.size!==1&&(i=Vo(i,o.size)),o.extra&&(i={...i,...o.extra});const c={mode:s,speed:o.speed,opts:i};return gr.set(t,c),c}var Te={exports:{}},me={};var dr;function ga(){if(dr)return me;dr=1;var r=Symbol.for("react.transitional.element"),e=Symbol.for("react.fragment");function t(a,s,o){var i=null;if(o!==void 0&&(i=""+o),s.key!==void 0&&(i=""+s.key),"key"in s){o={};for(var c in s)c!=="key"&&(o[c]=s[c])}else o=s;return s=o.ref,{$$typeof:r,type:a,key:i,ref:s!==void 0?s:null,props:o}}return me.Fragment=e,me.jsx=t,me.jsxs=t,me}var mr;function da(){return mr||(mr=1,Te.exports=ga()),Te.exports}var xe=da(),Ce={exports:{}},R={};var xr;function ma(){if(xr)return R;xr=1;var r=Symbol.for("react.transitional.element"),e=Symbol.for("react.portal"),t=Symbol.for("react.fragment"),a=Symbol.for("react.strict_mode"),s=Symbol.for("react.profiler"),o=Symbol.for("react.consumer"),i=Symbol.for("react.context"),c=Symbol.for("react.forward_ref"),p=Symbol.for("react.suspense"),g=Symbol.for("react.memo"),b=Symbol.for("react.lazy"),n=Symbol.for("react.activity"),f=Symbol.for("react.view_transition"),u=Symbol.iterator;function h(l){return l===null||typeof l!="object"?null:(l=u&&l[u]||l["@@iterator"],typeof l=="function"?l:null)}var x={isMounted:function(){return!1},enqueueForceUpdate:function(){},enqueueReplaceState:function(){},enqueueSetState:function(){}},d=Object.assign,m={};function z(l,$,Y){this.props=l,this.context=$,this.refs=m,this.updater=Y||x}z.prototype.isReactComponent={},z.prototype.setState=function(l,$){if(typeof l!="object"&&typeof l!="function"&&l!=null)throw Error("takes an object of state variables to update or a function which returns an object of state variables.");this.updater.enqueueSetState(this,l,$,"setState")},z.prototype.forceUpdate=function(l){this.updater.enqueueForceUpdate(this,l,"forceUpdate")};function M(){}M.prototype=z.prototype;function w(l,$,Y){this.props=l,this.context=$,this.refs=m,this.updater=Y||x}var v=w.prototype=new M;v.constructor=w,d(v,z.prototype),v.isPureReactComponent=!0;var H=Array.isArray;function k(){}var y={H:null,A:null,T:null,S:null},_=Object.prototype.hasOwnProperty;function X(l,$,Y){var W=Y.ref;return{$$typeof:r,type:l,key:$,ref:W!==void 0?W:null,props:Y}}function E(l,$){return X(l.type,$,l.props)}function O(l){return typeof l=="object"&&l!==null&&l.$$typeof===r}function B(l){var $={"=":"=0",":":"=2"};return"$"+l.replace(/[=:]/g,function(Y){return $[Y]})}var N=/\/+/g;function T(l,$){return typeof l=="object"&&l!==null&&l.key!=null?B(""+l.key):$.toString(36)}function j(l){switch(l.status){case"fulfilled":return l.value;case"rejected":throw l.reason;default:switch(typeof l.status=="string"?l.then(k,k):(l.status="pending",l.then(function($){l.status==="pending"&&(l.status="fulfilled",l.value=$)},function($){l.status==="pending"&&(l.status="rejected",l.reason=$)})),l.status){case"fulfilled":return l.value;case"rejected":throw l.reason}}throw l}function D(l,$,Y,W,S){var P=typeof l;(P==="undefined"||P==="boolean")&&(l=null);var C=!1;if(l===null)C=!0;else switch(P){case"bigint":case"string":case"number":C=!0;break;case"object":switch(l.$$typeof){case r:case e:C=!0;break;case b:return C=l._init,D(C(l._payload),$,Y,W,S)}}if(C)return S=S(l),C=W===""?"."+T(l,0):W,H(S)?(Y="",C!=null&&(Y=C.replace(N,"$&/")+"/"),D(S,$,Y,"",function(F){return F})):S!=null&&(O(S)&&(S=E(S,Y+(S.key==null||l&&l.key===S.key?"":(""+S.key).replace(N,"$&/")+"/")+C)),$.push(S)),1;C=0;var ee=W===""?".":W+":";if(H(l))for(var Z=0;Z<l.length;Z++)W=l[Z],P=ee+T(W,Z),C+=D(W,$,Y,P,S);else if(Z=h(l),typeof Z=="function")for(l=Z.call(l),Z=0;!(W=l.next()).done;)W=W.value,P=ee+T(W,Z++),C+=D(W,$,Y,P,S);else if(P==="object"){if(typeof l.then=="function")return D(j(l),$,Y,W,S);throw $=String(l),Error("Objects are not valid as a React child (found: "+($==="[object Object]"?"object with keys {"+Object.keys(l).join(", ")+"}":$)+"). If you meant to render a collection of children, use an array instead.")}return C}function V(l,$,Y){if(l==null)return l;var W=[],S=0;return D(l,W,"","",function(P){return $.call(Y,P,S++)}),W}function U(l){if(l._status===-1){var $=l._result,Y=$();Y.then(function(W){(l._status===0||l._status===-1)&&(l._status=1,l._result=W,Y.status===void 0&&(Y.status="fulfilled",Y.value=W))},function(W){(l._status===0||l._status===-1)&&(l._status=2,l._result=W,Y.status===void 0&&(Y.status="rejected",Y.reason=W))}),l._status===-1&&(l._status=0,l._result=Y)}if(l._status===1)return l._result.default;throw l._result}var q=typeof reportError=="function"?reportError:function(l){if(typeof window=="object"&&typeof window.ErrorEvent=="function"){var $=new window.ErrorEvent("error",{bubbles:!0,cancelable:!0,message:typeof l=="object"&&l!==null&&typeof l.message=="string"?String(l.message):String(l),error:l});if(!window.dispatchEvent($))return}else if(typeof process=="object"&&typeof process.emit=="function"){process.emit("uncaughtException",l);return}console.error(l)};function K(l){var $=y.T,Y={};Y.types=$!==null?$.types:null,y.T=Y;try{var W=l(),S=y.S;S!==null&&S(Y,W),typeof W=="object"&&W!==null&&typeof W.then=="function"&&W.then(k,q)}catch(P){q(P)}finally{$!==null&&Y.types!==null&&($.types=Y.types),y.T=$}}function te(l){var $=y.T;if($!==null){var Y=$.types;Y===null?$.types=[l]:Y.indexOf(l)===-1&&Y.push(l)}else K(te.bind(null,l))}var oe={map:V,forEach:function(l,$,Y){V(l,function(){$.apply(this,arguments)},Y)},count:function(l){var $=0;return V(l,function(){$++}),$},toArray:function(l){return V(l,function($){return $})||[]},only:function(l){if(!O(l))throw Error("React.Children.only expected to receive a single React element child.");return l}};return R.Activity=n,R.Children=oe,R.Component=z,R.Fragment=t,R.Profiler=s,R.PureComponent=w,R.StrictMode=a,R.Suspense=p,R.ViewTransition=f,R.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE=y,R.__COMPILER_RUNTIME={__proto__:null,c:function(l){return y.H.useMemoCache(l)}},R.addTransitionType=te,R.cache=function(l){return function(){return l.apply(null,arguments)}},R.cacheSignal=function(){return null},R.cloneElement=function(l,$,Y){if(l==null)throw Error("The argument must be a React element, but you passed "+l+".");var W=d({},l.props),S=l.key;if($!=null)for(P in $.key!==void 0&&(S=""+$.key),$)!_.call($,P)||P==="key"||P==="__self"||P==="__source"||P==="ref"&&$.ref===void 0||(W[P]=$[P]);var P=arguments.length-2;if(P===1)W.children=Y;else if(1<P){for(var C=Array(P),ee=0;ee<P;ee++)C[ee]=arguments[ee+2];W.children=C}return X(l.type,S,W)},R.createContext=function(l){return l={$$typeof:i,_currentValue:l,_currentValue2:l,_threadCount:0,Provider:null,Consumer:null},l.Provider=l,l.Consumer={$$typeof:o,_context:l},l},R.createElement=function(l,$,Y){var W,S={},P=null;if($!=null)for(W in $.key!==void 0&&(P=""+$.key),$)_.call($,W)&&W!=="key"&&W!=="__self"&&W!=="__source"&&(S[W]=$[W]);var C=arguments.length-2;if(C===1)S.children=Y;else if(1<C){for(var ee=Array(C),Z=0;Z<C;Z++)ee[Z]=arguments[Z+2];S.children=ee}if(l&&l.defaultProps)for(W in C=l.defaultProps,C)S[W]===void 0&&(S[W]=C[W]);return X(l,P,S)},R.createRef=function(){return{current:null}},R.forwardRef=function(l){return{$$typeof:c,render:l}},R.isValidElement=O,R.lazy=function(l){return{$$typeof:b,_payload:{_status:-1,_result:l},_init:U}},R.memo=function(l,$){return{$$typeof:g,type:l,compare:$===void 0?null:$}},R.startTransition=K,R.unstable_useCacheRefresh=function(){return y.H.useCacheRefresh()},R.use=function(l){return y.H.use(l)},R.useActionState=function(l,$,Y){return y.H.useActionState(l,$,Y)},R.useCallback=function(l,$){return y.H.useCallback(l,$)},R.useContext=function(l){return y.H.useContext(l)},R.useDebugValue=function(){},R.useDeferredValue=function(l,$){return y.H.useDeferredValue(l,$)},R.useEffect=function(l,$){return y.H.useEffect(l,$)},R.useEffectEvent=function(l){return y.H.useEffectEvent(l)},R.useId=function(){return y.H.useId()},R.useImperativeHandle=function(l,$,Y){return y.H.useImperativeHandle(l,$,Y)},R.useInsertionEffect=function(l,$){return y.H.useInsertionEffect(l,$)},R.useLayoutEffect=function(l,$){return y.H.useLayoutEffect(l,$)},R.useMemo=function(l,$){return y.H.useMemo(l,$)},R.useOptimistic=function(l,$){return y.H.useOptimistic(l,$)},R.useReducer=function(l,$,Y){return y.H.useReducer(l,$,Y)},R.useRef=function(l){return y.H.useRef(l)},R.useState=function(l){return y.H.useState(l)},R.useSyncExternalStore=function(l,$,Y){return y.H.useSyncExternalStore(l,$,Y)},R.useTransition=function(){return y.H.useTransition()},R.version="19.3.0",R}var hr;function jr(){return hr||(hr=1,Ce.exports=ma()),Ce.exports}var L=jr();const xa={sm:{borderRadius:32,borderWidth:1,width:70,height:36},md:{borderRadius:16,borderWidth:1},line:{borderRadius:16,borderWidth:1},"pulse-outside":{borderRadius:16,borderWidth:1},"pulse-inner":{borderRadius:16,borderWidth:1}},ha={sm:{dark:{strokeOpacity:.46,innerOpacity:.24,bloomOpacity:.38,innerShadow:"rgba(255, 255, 255, 0.3)",saturation:1.2},light:{strokeOpacity:.12,innerOpacity:.3,bloomOpacity:.16,innerShadow:"rgba(0, 0, 0, 0.14)",saturation:1.8}},md:{dark:{strokeOpacity:.26,innerOpacity:.42,bloomOpacity:.24,innerShadow:"rgba(255, 255, 255, 0.27)",saturation:1.2},light:{strokeOpacity:.12,innerOpacity:.26,bloomOpacity:.34,innerShadow:"rgba(0, 0, 0, 0.14)",saturation:1.5}},line:{dark:{strokeOpacity:1.14,innerOpacity:.7,bloomOpacity:.8,innerShadow:"rgba(255, 255, 255, 0.1)",saturation:1.2},light:{strokeOpacity:.16,innerOpacity:.32,bloomOpacity:.3,innerShadow:"rgba(0, 0, 0, 0.14)",saturation:1.95}},"pulse-outside":{dark:{strokeOpacity:.94,innerOpacity:.34,bloomOpacity:.3,innerShadow:"transparent",saturation:1.2,brightness:1.9,hairlineOpacity:0},light:{strokeOpacity:1.96,innerOpacity:1.04,bloomOpacity:.42,innerShadow:"transparent",saturation:.6,brightness:1.7,hairlineOpacity:0}},"pulse-inner":{dark:{strokeOpacity:1.54,innerOpacity:.44,bloomOpacity:.66,innerShadow:"transparent",saturation:1.2,brightness:.75},light:{strokeOpacity:.32,innerOpacity:.4,bloomOpacity:.8,innerShadow:"transparent",saturation:.75,brightness:1.3}}},pe={colorful:{border:[{color:"rgb(255, 50, 100)",pos:"33% -7.4%",size:"70px 40px"},{color:"rgb(40, 140, 255)",pos:"12% -5%",size:"60px 35px"},{color:"rgb(50, 200, 80)",pos:"2.1% 68.3%",size:"40px 70px"},{color:"rgb(30, 185, 170)",pos:"2.1% 68.3%",size:"20px 35px"},{color:"rgb(100, 70, 255)",pos:"74.4% 100%",size:"180px 32px"},{color:"rgb(40, 140, 255)",pos:"55% 100%",size:"85px 26px"},{color:"rgb(255, 120, 40)",pos:"93.9% 0%",size:"74px 32px"},{color:"rgb(240, 50, 180)",pos:"100% 27.1%",size:"26px 42px"},{color:"rgb(180, 40, 240)",pos:"100% 27.1%",size:"52px 48px"}],spike:{primary:"rgb(255, 60, 80)",secondary:"rgba(40, 190, 180, 0.98)"},spikeLt:{primary:"rgb(200, 30, 60)",secondary:"rgb(20, 150, 140)"}},mono:{border:[{color:"rgb(180, 180, 180)",pos:"33% -7.4%",size:"70px 40px"},{color:"rgb(140, 140, 140)",pos:"12% -5%",size:"60px 35px"},{color:"rgb(160, 160, 160)",pos:"2.1% 68.3%",size:"40px 70px"},{color:"rgb(130, 130, 130)",pos:"2.1% 68.3%",size:"20px 35px"},{color:"rgb(170, 170, 170)",pos:"74.4% 100%",size:"180px 32px"},{color:"rgb(150, 150, 150)",pos:"55% 100%",size:"85px 26px"},{color:"rgb(190, 190, 190)",pos:"93.9% 0%",size:"74px 32px"},{color:"rgb(145, 145, 145)",pos:"100% 27.1%",size:"26px 42px"},{color:"rgb(165, 165, 165)",pos:"100% 27.1%",size:"52px 48px"}],spike:{primary:"rgb(200, 200, 200)",secondary:"rgb(170, 170, 170)"},spikeLt:{primary:"rgb(80, 80, 80)",secondary:"rgb(120, 120, 120)"}},ocean:{border:[{color:"rgb(100, 80, 220)",pos:"33% -7.4%",size:"70px 40px"},{color:"rgb(60, 120, 255)",pos:"12% -5%",size:"60px 35px"},{color:"rgb(80, 100, 200)",pos:"2.1% 68.3%",size:"40px 70px"},{color:"rgb(50, 140, 220)",pos:"2.1% 68.3%",size:"20px 35px"},{color:"rgb(120, 80, 255)",pos:"74.4% 100%",size:"180px 32px"},{color:"rgb(70, 130, 255)",pos:"55% 100%",size:"85px 26px"},{color:"rgb(140, 100, 240)",pos:"93.9% 0%",size:"74px 32px"},{color:"rgb(90, 110, 230)",pos:"100% 27.1%",size:"26px 42px"},{color:"rgb(130, 70, 255)",pos:"100% 27.1%",size:"52px 48px"}],spike:{primary:"rgb(100, 120, 255)",secondary:"rgba(130, 100, 220, 0.98)"},spikeLt:{primary:"rgb(60, 60, 180)",secondary:"rgb(80, 100, 200)"}},sunset:{border:[{color:"rgb(255, 80, 50)",pos:"33% -7.4%",size:"70px 40px"},{color:"rgb(255, 160, 40)",pos:"12% -5%",size:"60px 35px"},{color:"rgb(255, 120, 60)",pos:"2.1% 68.3%",size:"40px 70px"},{color:"rgb(255, 200, 50)",pos:"2.1% 68.3%",size:"20px 35px"},{color:"rgb(255, 100, 80)",pos:"74.4% 100%",size:"180px 32px"},{color:"rgb(255, 180, 60)",pos:"55% 100%",size:"85px 26px"},{color:"rgb(255, 60, 60)",pos:"93.9% 0%",size:"74px 32px"},{color:"rgb(255, 140, 50)",pos:"100% 27.1%",size:"26px 42px"},{color:"rgb(255, 90, 70)",pos:"100% 27.1%",size:"52px 48px"}],spike:{primary:"rgb(255, 140, 80)",secondary:"rgba(255, 100, 60, 0.98)"},spikeLt:{primary:"rgb(200, 80, 40)",secondary:"rgb(220, 120, 30)"}},forest:{border:[{color:"rgb(46, 160, 90)",pos:"33% -7.4%",size:"70px 40px"},{color:"rgb(30, 190, 120)",pos:"12% -5%",size:"60px 35px"},{color:"rgb(70, 180, 70)",pos:"2.1% 68.3%",size:"40px 70px"},{color:"rgb(20, 150, 130)",pos:"2.1% 68.3%",size:"20px 35px"},{color:"rgb(90, 200, 80)",pos:"74.4% 100%",size:"180px 32px"},{color:"rgb(40, 170, 110)",pos:"55% 100%",size:"85px 26px"},{color:"rgb(120, 210, 70)",pos:"93.9% 0%",size:"74px 32px"},{color:"rgb(35, 145, 100)",pos:"100% 27.1%",size:"26px 42px"},{color:"rgb(60, 195, 140)",pos:"100% 27.1%",size:"52px 48px"}],spike:{primary:"rgb(46, 160, 90)",secondary:"rgba(30, 190, 120,, 0.98)"},spikeLt:{primary:"rgb(33, 115, 65)",secondary:"rgb(22, 137, 86)"}},candy:{border:[{color:"rgb(240, 70, 170)",pos:"33% -7.4%",size:"70px 40px"},{color:"rgb(255, 90, 140)",pos:"12% -5%",size:"60px 35px"},{color:"rgb(215, 60, 200)",pos:"2.1% 68.3%",size:"40px 70px"},{color:"rgb(255, 110, 180)",pos:"2.1% 68.3%",size:"20px 35px"},{color:"rgb(200, 80, 240)",pos:"74.4% 100%",size:"180px 32px"},{color:"rgb(250, 60, 150)",pos:"55% 100%",size:"85px 26px"},{color:"rgb(230, 120, 220)",pos:"93.9% 0%",size:"74px 32px"},{color:"rgb(245, 85, 165)",pos:"100% 27.1%",size:"26px 42px"},{color:"rgb(210, 70, 230)",pos:"100% 27.1%",size:"52px 48px"}],spike:{primary:"rgb(240, 70, 170)",secondary:"rgba(255, 90, 140,, 0.98)"},spikeLt:{primary:"rgb(173, 50, 122)",secondary:"rgb(184, 65, 101)"}},ice:{border:[{color:"rgb(90, 200, 240)",pos:"33% -7.4%",size:"70px 40px"},{color:"rgb(60, 175, 230)",pos:"12% -5%",size:"60px 35px"},{color:"rgb(130, 220, 250)",pos:"2.1% 68.3%",size:"40px 70px"},{color:"rgb(70, 190, 215)",pos:"2.1% 68.3%",size:"20px 35px"},{color:"rgb(110, 210, 255)",pos:"74.4% 100%",size:"180px 32px"},{color:"rgb(50, 165, 220)",pos:"55% 100%",size:"85px 26px"},{color:"rgb(150, 230, 250)",pos:"93.9% 0%",size:"74px 32px"},{color:"rgb(85, 195, 235)",pos:"100% 27.1%",size:"26px 42px"},{color:"rgb(65, 180, 245)",pos:"100% 27.1%",size:"52px 48px"}],spike:{primary:"rgb(90, 200, 240)",secondary:"rgba(60, 175, 230,, 0.98)"},spikeLt:{primary:"rgb(65, 144, 173)",secondary:"rgb(43, 126, 166)"}},gold:{border:[{color:"rgb(240, 190, 60)",pos:"33% -7.4%",size:"70px 40px"},{color:"rgb(255, 210, 90)",pos:"12% -5%",size:"60px 35px"},{color:"rgb(225, 165, 40)",pos:"2.1% 68.3%",size:"40px 70px"},{color:"rgb(250, 200, 70)",pos:"2.1% 68.3%",size:"20px 35px"},{color:"rgb(255, 225, 120)",pos:"74.4% 100%",size:"180px 32px"},{color:"rgb(230, 175, 50)",pos:"55% 100%",size:"85px 26px"},{color:"rgb(245, 205, 85)",pos:"93.9% 0%",size:"74px 32px"},{color:"rgb(215, 155, 35)",pos:"100% 27.1%",size:"26px 42px"},{color:"rgb(255, 215, 100)",pos:"100% 27.1%",size:"52px 48px"}],spike:{primary:"rgb(240, 190, 60)",secondary:"rgba(255, 210, 90,, 0.98)"},spikeLt:{primary:"rgb(173, 137, 43)",secondary:"rgb(184, 151, 65)"}}},Fr={colorful:{border:[{color:"rgb(50, 200, 80)",pos:"2% 68%",size:"9px 18px"},{color:"rgb(30, 185, 170)",pos:"2% 68%",size:"4px 8px"},{color:"rgb(255, 120, 40)",pos:"72% -3%",size:"59px 9px"},{color:"rgb(100, 70, 255)",pos:"74% 100%",size:"42px 7px"},{color:"rgb(240, 50, 180)",pos:"100% 27%",size:"10px 17px"},{color:"rgb(180, 40, 240)",pos:"100% 27%",size:"10px 18px"},{color:"rgb(40, 140, 255)",pos:"100% 27%",size:"5px 10px"},{color:"rgb(255, 50, 100)",pos:"100% 27%",size:"11px 12px"}],inner:[{color:"rgba(50, 200, 80, 0.5)",pos:"2% 68%",size:"9px 18px"},{color:"rgba(30, 185, 170, 0.45)",pos:"2% 68%",size:"4px 8px"},{color:"rgba(255, 120, 40, 0.35)",pos:"72% -3%",size:"59px 9px"},{color:"rgba(100, 70, 255, 0.35)",pos:"74% 100%",size:"42px 7px"},{color:"rgba(240, 50, 180, 0.3)",pos:"100% 27%",size:"10px 17px"},{color:"rgba(180, 40, 240, 0.4)",pos:"100% 27%",size:"10px 18px"},{color:"rgba(40, 140, 255, 0.3)",pos:"100% 27%",size:"5px 10px"},{color:"rgba(255, 50, 100, 0.3)",pos:"100% 27%",size:"11px 12px"}]},mono:{border:[{color:"rgb(160, 160, 160)",pos:"2% 68%",size:"9px 18px"},{color:"rgb(140, 140, 140)",pos:"2% 68%",size:"4px 8px"},{color:"rgb(180, 180, 180)",pos:"72% -3%",size:"59px 9px"},{color:"rgb(150, 150, 150)",pos:"74% 100%",size:"42px 7px"},{color:"rgb(170, 170, 170)",pos:"100% 27%",size:"10px 17px"},{color:"rgb(155, 155, 155)",pos:"100% 27%",size:"10px 18px"},{color:"rgb(145, 145, 145)",pos:"100% 27%",size:"5px 10px"},{color:"rgb(165, 165, 165)",pos:"100% 27%",size:"11px 12px"}],inner:[{color:"rgba(160, 160, 160, 0.25)",pos:"2% 68%",size:"9px 18px"},{color:"rgba(140, 140, 140, 0.22)",pos:"2% 68%",size:"4px 8px"},{color:"rgba(180, 180, 180, 0.17)",pos:"72% -3%",size:"59px 9px"},{color:"rgba(150, 150, 150, 0.17)",pos:"74% 100%",size:"42px 7px"},{color:"rgba(170, 170, 170, 0.15)",pos:"100% 27%",size:"10px 17px"},{color:"rgba(155, 155, 155, 0.20)",pos:"100% 27%",size:"10px 18px"},{color:"rgba(145, 145, 145, 0.15)",pos:"100% 27%",size:"5px 10px"},{color:"rgba(165, 165, 165, 0.15)",pos:"100% 27%",size:"11px 12px"}]},ocean:{border:[{color:"rgb(60, 140, 200)",pos:"2% 68%",size:"9px 18px"},{color:"rgb(50, 120, 180)",pos:"2% 68%",size:"4px 8px"},{color:"rgb(100, 80, 220)",pos:"72% -3%",size:"59px 9px"},{color:"rgb(80, 100, 255)",pos:"74% 100%",size:"42px 7px"},{color:"rgb(120, 70, 240)",pos:"100% 27%",size:"10px 17px"},{color:"rgb(90, 80, 220)",pos:"100% 27%",size:"10px 18px"},{color:"rgb(70, 110, 255)",pos:"100% 27%",size:"5px 10px"},{color:"rgb(110, 90, 230)",pos:"100% 27%",size:"11px 12px"}],inner:[{color:"rgba(60, 140, 200, 0.5)",pos:"2% 68%",size:"9px 18px"},{color:"rgba(50, 120, 180, 0.45)",pos:"2% 68%",size:"4px 8px"},{color:"rgba(100, 80, 220, 0.35)",pos:"72% -3%",size:"59px 9px"},{color:"rgba(80, 100, 255, 0.35)",pos:"74% 100%",size:"42px 7px"},{color:"rgba(120, 70, 240, 0.3)",pos:"100% 27%",size:"10px 17px"},{color:"rgba(90, 80, 220, 0.4)",pos:"100% 27%",size:"10px 18px"},{color:"rgba(70, 110, 255, 0.3)",pos:"100% 27%",size:"5px 10px"},{color:"rgba(110, 90, 230, 0.3)",pos:"100% 27%",size:"11px 12px"}]},sunset:{border:[{color:"rgb(255, 180, 50)",pos:"2% 68%",size:"9px 18px"},{color:"rgb(255, 150, 40)",pos:"2% 68%",size:"4px 8px"},{color:"rgb(255, 80, 60)",pos:"72% -3%",size:"59px 9px"},{color:"rgb(255, 100, 80)",pos:"74% 100%",size:"42px 7px"},{color:"rgb(255, 60, 80)",pos:"100% 27%",size:"10px 17px"},{color:"rgb(255, 120, 60)",pos:"100% 27%",size:"10px 18px"},{color:"rgb(255, 200, 50)",pos:"100% 27%",size:"5px 10px"},{color:"rgb(255, 90, 70)",pos:"100% 27%",size:"11px 12px"}],inner:[{color:"rgba(255, 180, 50, 0.5)",pos:"2% 68%",size:"9px 18px"},{color:"rgba(255, 150, 40, 0.45)",pos:"2% 68%",size:"4px 8px"},{color:"rgba(255, 80, 60, 0.35)",pos:"72% -3%",size:"59px 9px"},{color:"rgba(255, 100, 80, 0.35)",pos:"74% 100%",size:"42px 7px"},{color:"rgba(255, 60, 80, 0.3)",pos:"100% 27%",size:"10px 17px"},{color:"rgba(255, 120, 60, 0.4)",pos:"100% 27%",size:"10px 18px"},{color:"rgba(255, 200, 50, 0.3)",pos:"100% 27%",size:"5px 10px"},{color:"rgba(255, 90, 70, 0.3)",pos:"100% 27%",size:"11px 12px"}]},forest:{border:[{color:"rgb(46, 160, 90)",pos:"2% 68%",size:"9px 18px"},{color:"rgb(30, 190, 120)",pos:"2% 68%",size:"4px 8px"},{color:"rgb(70, 180, 70)",pos:"72% -3%",size:"59px 9px"},{color:"rgb(20, 150, 130)",pos:"74% 100%",size:"42px 7px"},{color:"rgb(90, 200, 80)",pos:"100% 27%",size:"10px 17px"},{color:"rgb(40, 170, 110)",pos:"100% 27%",size:"10px 18px"},{color:"rgb(120, 210, 70)",pos:"100% 27%",size:"5px 10px"},{color:"rgb(35, 145, 100)",pos:"100% 27%",size:"11px 12px"}],inner:[{color:"rgba(60, 195, 140,, 0.5)",pos:"2% 68%",size:"9px 18px"},{color:"rgba(46, 160, 90,, 0.45)",pos:"2% 68%",size:"4px 8px"},{color:"rgba(30, 190, 120,, 0.35)",pos:"72% -3%",size:"59px 9px"},{color:"rgba(70, 180, 70,, 0.35)",pos:"74% 100%",size:"42px 7px"},{color:"rgba(20, 150, 130,, 0.3)",pos:"100% 27%",size:"10px 17px"},{color:"rgba(90, 200, 80,, 0.4)",pos:"100% 27%",size:"10px 18px"},{color:"rgba(40, 170, 110,, 0.3)",pos:"100% 27%",size:"5px 10px"},{color:"rgba(120, 210, 70,, 0.3)",pos:"100% 27%",size:"11px 12px"}]},candy:{border:[{color:"rgb(240, 70, 170)",pos:"2% 68%",size:"9px 18px"},{color:"rgb(255, 90, 140)",pos:"2% 68%",size:"4px 8px"},{color:"rgb(215, 60, 200)",pos:"72% -3%",size:"59px 9px"},{color:"rgb(255, 110, 180)",pos:"74% 100%",size:"42px 7px"},{color:"rgb(200, 80, 240)",pos:"100% 27%",size:"10px 17px"},{color:"rgb(250, 60, 150)",pos:"100% 27%",size:"10px 18px"},{color:"rgb(230, 120, 220)",pos:"100% 27%",size:"5px 10px"},{color:"rgb(245, 85, 165)",pos:"100% 27%",size:"11px 12px"}],inner:[{color:"rgba(210, 70, 230,, 0.5)",pos:"2% 68%",size:"9px 18px"},{color:"rgba(240, 70, 170,, 0.45)",pos:"2% 68%",size:"4px 8px"},{color:"rgba(255, 90, 140,, 0.35)",pos:"72% -3%",size:"59px 9px"},{color:"rgba(215, 60, 200,, 0.35)",pos:"74% 100%",size:"42px 7px"},{color:"rgba(255, 110, 180,, 0.3)",pos:"100% 27%",size:"10px 17px"},{color:"rgba(200, 80, 240,, 0.4)",pos:"100% 27%",size:"10px 18px"},{color:"rgba(250, 60, 150,, 0.3)",pos:"100% 27%",size:"5px 10px"},{color:"rgba(230, 120, 220,, 0.3)",pos:"100% 27%",size:"11px 12px"}]},ice:{border:[{color:"rgb(90, 200, 240)",pos:"2% 68%",size:"9px 18px"},{color:"rgb(60, 175, 230)",pos:"2% 68%",size:"4px 8px"},{color:"rgb(130, 220, 250)",pos:"72% -3%",size:"59px 9px"},{color:"rgb(70, 190, 215)",pos:"74% 100%",size:"42px 7px"},{color:"rgb(110, 210, 255)",pos:"100% 27%",size:"10px 17px"},{color:"rgb(50, 165, 220)",pos:"100% 27%",size:"10px 18px"},{color:"rgb(150, 230, 250)",pos:"100% 27%",size:"5px 10px"},{color:"rgb(85, 195, 235)",pos:"100% 27%",size:"11px 12px"}],inner:[{color:"rgba(65, 180, 245,, 0.5)",pos:"2% 68%",size:"9px 18px"},{color:"rgba(90, 200, 240,, 0.45)",pos:"2% 68%",size:"4px 8px"},{color:"rgba(60, 175, 230,, 0.35)",pos:"72% -3%",size:"59px 9px"},{color:"rgba(130, 220, 250,, 0.35)",pos:"74% 100%",size:"42px 7px"},{color:"rgba(70, 190, 215,, 0.3)",pos:"100% 27%",size:"10px 17px"},{color:"rgba(110, 210, 255,, 0.4)",pos:"100% 27%",size:"10px 18px"},{color:"rgba(50, 165, 220,, 0.3)",pos:"100% 27%",size:"5px 10px"},{color:"rgba(150, 230, 250,, 0.3)",pos:"100% 27%",size:"11px 12px"}]},gold:{border:[{color:"rgb(240, 190, 60)",pos:"2% 68%",size:"9px 18px"},{color:"rgb(255, 210, 90)",pos:"2% 68%",size:"4px 8px"},{color:"rgb(225, 165, 40)",pos:"72% -3%",size:"59px 9px"},{color:"rgb(250, 200, 70)",pos:"74% 100%",size:"42px 7px"},{color:"rgb(255, 225, 120)",pos:"100% 27%",size:"10px 17px"},{color:"rgb(230, 175, 50)",pos:"100% 27%",size:"10px 18px"},{color:"rgb(245, 205, 85)",pos:"100% 27%",size:"5px 10px"},{color:"rgb(215, 155, 35)",pos:"100% 27%",size:"11px 12px"}],inner:[{color:"rgba(255, 215, 100,, 0.5)",pos:"2% 68%",size:"9px 18px"},{color:"rgba(240, 190, 60,, 0.45)",pos:"2% 68%",size:"4px 8px"},{color:"rgba(255, 210, 90,, 0.35)",pos:"72% -3%",size:"59px 9px"},{color:"rgba(225, 165, 40,, 0.35)",pos:"74% 100%",size:"42px 7px"},{color:"rgba(250, 200, 70,, 0.3)",pos:"100% 27%",size:"10px 17px"},{color:"rgba(255, 225, 120,, 0.4)",pos:"100% 27%",size:"10px 18px"},{color:"rgba(230, 175, 50,, 0.3)",pos:"100% 27%",size:"5px 10px"},{color:"rgba(245, 205, 85,, 0.3)",pos:"100% 27%",size:"11px 12px"}]}};function $a(r){return Fr[r].border.map(e=>`radial-gradient(ellipse ${e.size} at ${e.pos}, ${e.color}, transparent)`).join(`,
    `)}function ya(r){return Fr[r].inner.map(e=>`radial-gradient(ellipse ${e.size} at ${e.pos}, ${e.color}, transparent)`).join(`,
    `)}function za(r){return pe[r].border.map(e=>`radial-gradient(ellipse ${e.size} at ${e.pos}, ${e.color}, transparent)`).join(`,
    `)}function va(r){const e=pe[r],t=r==="mono"?.225:.45;return e.border.map(a=>{const s=a.color.replace("rgb(","rgba(").replace(")",`, ${t})`);return`radial-gradient(ellipse ${a.size.split(" ").map(o=>{const i=parseInt(o);return`${Math.round(i*.9)}px`}).join(" ")} at ${a.pos}, ${s}, transparent)`}).join(`,
    `)}function Ma(r,e){const t=pe[r];return e?t.spike:t.spikeLt}const ka={colorful:{dark:[{color:"rgb(255, 50, 100)",sizeW:36,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(40, 180, 220)",sizeW:30,sizeH:32,offsetX:39,offsetY:0},{color:"rgb(50, 200, 80)",sizeW:33,sizeH:28,offsetX:-36,offsetY:2},{color:"rgb(180, 40, 240)",sizeW:29,sizeH:34,offsetX:-54,offsetY:0},{color:"rgb(255, 160, 30)",sizeW:27,sizeH:30,offsetX:51,offsetY:-1},{color:"rgb(100, 70, 255)",sizeW:36,sizeH:24,offsetX:21,offsetY:1},{color:"rgb(40, 140, 255)",sizeW:30,sizeH:22,offsetX:-21,offsetY:0},{color:"rgb(240, 50, 180)",sizeW:25,sizeH:28,offsetX:66,offsetY:1},{color:"rgb(30, 185, 170)",sizeW:23,sizeH:30,offsetX:-66,offsetY:-1}],light:[{color:"rgb(255, 50, 100)",sizeW:45,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(40, 140, 255)",sizeW:35,sizeH:32,offsetX:65,offsetY:0},{color:"rgb(50, 200, 80)",sizeW:40,sizeH:28,offsetX:-60,offsetY:2},{color:"rgb(180, 40, 240)",sizeW:35,sizeH:34,offsetX:-90,offsetY:0},{color:"rgb(30, 185, 170)",sizeW:38,sizeH:30,offsetX:85,offsetY:-1},{color:"rgb(100, 70, 255)",sizeW:50,sizeH:24,offsetX:35,offsetY:1},{color:"rgb(40, 140, 255)",sizeW:40,sizeH:22,offsetX:-35,offsetY:0},{color:"rgb(255, 120, 40)",sizeW:35,sizeH:28,offsetX:110,offsetY:1},{color:"rgb(240, 50, 180)",sizeW:30,sizeH:30,offsetX:-110,offsetY:-1}]},mono:{dark:[{color:"rgb(200, 200, 200)",sizeW:36,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(170, 170, 170)",sizeW:30,sizeH:32,offsetX:39,offsetY:0},{color:"rgb(155, 155, 155)",sizeW:33,sizeH:28,offsetX:-36,offsetY:2},{color:"rgb(185, 185, 185)",sizeW:29,sizeH:34,offsetX:-54,offsetY:0},{color:"rgb(165, 165, 165)",sizeW:27,sizeH:30,offsetX:51,offsetY:-1},{color:"rgb(180, 180, 180)",sizeW:36,sizeH:24,offsetX:21,offsetY:1},{color:"rgb(160, 160, 160)",sizeW:30,sizeH:22,offsetX:-21,offsetY:0},{color:"rgb(175, 175, 175)",sizeW:25,sizeH:28,offsetX:66,offsetY:1},{color:"rgb(190, 190, 190)",sizeW:23,sizeH:30,offsetX:-66,offsetY:-1}],light:[{color:"rgb(100, 100, 100)",sizeW:45,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(80, 80, 80)",sizeW:35,sizeH:32,offsetX:65,offsetY:0},{color:"rgb(90, 90, 90)",sizeW:40,sizeH:28,offsetX:-60,offsetY:2},{color:"rgb(70, 70, 70)",sizeW:35,sizeH:34,offsetX:-90,offsetY:0},{color:"rgb(85, 85, 85)",sizeW:38,sizeH:30,offsetX:85,offsetY:-1},{color:"rgb(95, 95, 95)",sizeW:50,sizeH:24,offsetX:35,offsetY:1},{color:"rgb(75, 75, 75)",sizeW:40,sizeH:22,offsetX:-35,offsetY:0},{color:"rgb(105, 105, 105)",sizeW:35,sizeH:28,offsetX:110,offsetY:1},{color:"rgb(65, 65, 65)",sizeW:30,sizeH:30,offsetX:-110,offsetY:-1}]},ocean:{dark:[{color:"rgb(100, 80, 220)",sizeW:36,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(60, 120, 255)",sizeW:30,sizeH:32,offsetX:39,offsetY:0},{color:"rgb(80, 100, 200)",sizeW:33,sizeH:28,offsetX:-36,offsetY:2},{color:"rgb(130, 70, 255)",sizeW:29,sizeH:34,offsetX:-54,offsetY:0},{color:"rgb(70, 130, 255)",sizeW:27,sizeH:30,offsetX:51,offsetY:-1},{color:"rgb(120, 80, 255)",sizeW:36,sizeH:24,offsetX:21,offsetY:1},{color:"rgb(90, 110, 230)",sizeW:30,sizeH:22,offsetX:-21,offsetY:0},{color:"rgb(110, 90, 240)",sizeW:25,sizeH:28,offsetX:66,offsetY:1},{color:"rgb(140, 100, 255)",sizeW:23,sizeH:30,offsetX:-66,offsetY:-1}],light:[{color:"rgb(80, 60, 200)",sizeW:45,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(50, 100, 220)",sizeW:35,sizeH:32,offsetX:65,offsetY:0},{color:"rgb(70, 90, 190)",sizeW:40,sizeH:28,offsetX:-60,offsetY:2},{color:"rgb(110, 60, 220)",sizeW:35,sizeH:34,offsetX:-90,offsetY:0},{color:"rgb(60, 110, 230)",sizeW:38,sizeH:30,offsetX:85,offsetY:-1},{color:"rgb(100, 70, 240)",sizeW:50,sizeH:24,offsetX:35,offsetY:1},{color:"rgb(80, 100, 210)",sizeW:40,sizeH:22,offsetX:-35,offsetY:0},{color:"rgb(90, 80, 225)",sizeW:35,sizeH:28,offsetX:110,offsetY:1},{color:"rgb(120, 90, 245)",sizeW:30,sizeH:30,offsetX:-110,offsetY:-1}]},sunset:{dark:[{color:"rgb(255, 100, 60)",sizeW:36,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(255, 180, 50)",sizeW:30,sizeH:32,offsetX:39,offsetY:0},{color:"rgb(255, 140, 70)",sizeW:33,sizeH:28,offsetX:-36,offsetY:2},{color:"rgb(255, 80, 80)",sizeW:29,sizeH:34,offsetX:-54,offsetY:0},{color:"rgb(255, 200, 60)",sizeW:27,sizeH:30,offsetX:51,offsetY:-1},{color:"rgb(255, 120, 50)",sizeW:36,sizeH:24,offsetX:21,offsetY:1},{color:"rgb(255, 160, 80)",sizeW:30,sizeH:22,offsetX:-21,offsetY:0},{color:"rgb(255, 90, 60)",sizeW:25,sizeH:28,offsetX:66,offsetY:1},{color:"rgb(255, 70, 70)",sizeW:23,sizeH:30,offsetX:-66,offsetY:-1}],light:[{color:"rgb(220, 80, 40)",sizeW:45,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(230, 150, 30)",sizeW:35,sizeH:32,offsetX:65,offsetY:0},{color:"rgb(210, 110, 50)",sizeW:40,sizeH:28,offsetX:-60,offsetY:2},{color:"rgb(200, 60, 60)",sizeW:35,sizeH:34,offsetX:-90,offsetY:0},{color:"rgb(220, 170, 40)",sizeW:38,sizeH:30,offsetX:85,offsetY:-1},{color:"rgb(210, 100, 30)",sizeW:50,sizeH:24,offsetX:35,offsetY:1},{color:"rgb(230, 130, 60)",sizeW:40,sizeH:22,offsetX:-35,offsetY:0},{color:"rgb(190, 70, 50)",sizeW:35,sizeH:28,offsetX:110,offsetY:1},{color:"rgb(180, 50, 50)",sizeW:30,sizeH:30,offsetX:-110,offsetY:-1}]},forest:{dark:[{color:"rgb(46, 160, 90)",sizeW:36,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(30, 190, 120)",sizeW:30,sizeH:32,offsetX:39,offsetY:0},{color:"rgb(70, 180, 70)",sizeW:33,sizeH:28,offsetX:-36,offsetY:2},{color:"rgb(20, 150, 130)",sizeW:29,sizeH:34,offsetX:-54,offsetY:0},{color:"rgb(90, 200, 80)",sizeW:27,sizeH:30,offsetX:51,offsetY:-1},{color:"rgb(40, 170, 110)",sizeW:36,sizeH:24,offsetX:21,offsetY:1},{color:"rgb(120, 210, 70)",sizeW:30,sizeH:22,offsetX:-21,offsetY:0},{color:"rgb(35, 145, 100)",sizeW:25,sizeH:28,offsetX:66,offsetY:1},{color:"rgb(60, 195, 140)",sizeW:23,sizeH:30,offsetX:-66,offsetY:-1}],light:[{color:"rgb(33, 115, 65)",sizeW:45,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(22, 137, 86)",sizeW:35,sizeH:32,offsetX:65,offsetY:0},{color:"rgb(50, 130, 50)",sizeW:40,sizeH:28,offsetX:-60,offsetY:2},{color:"rgb(14, 108, 94)",sizeW:35,sizeH:34,offsetX:-90,offsetY:0},{color:"rgb(65, 144, 58)",sizeW:38,sizeH:30,offsetX:85,offsetY:-1},{color:"rgb(29, 122, 79)",sizeW:50,sizeH:24,offsetX:35,offsetY:1},{color:"rgb(86, 151, 50)",sizeW:40,sizeH:22,offsetX:-35,offsetY:0},{color:"rgb(25, 104, 72)",sizeW:35,sizeH:28,offsetX:110,offsetY:1},{color:"rgb(43, 140, 101)",sizeW:30,sizeH:30,offsetX:-110,offsetY:-1}]},candy:{dark:[{color:"rgb(240, 70, 170)",sizeW:36,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(255, 90, 140)",sizeW:30,sizeH:32,offsetX:39,offsetY:0},{color:"rgb(215, 60, 200)",sizeW:33,sizeH:28,offsetX:-36,offsetY:2},{color:"rgb(255, 110, 180)",sizeW:29,sizeH:34,offsetX:-54,offsetY:0},{color:"rgb(200, 80, 240)",sizeW:27,sizeH:30,offsetX:51,offsetY:-1},{color:"rgb(250, 60, 150)",sizeW:36,sizeH:24,offsetX:21,offsetY:1},{color:"rgb(230, 120, 220)",sizeW:30,sizeH:22,offsetX:-21,offsetY:0},{color:"rgb(245, 85, 165)",sizeW:25,sizeH:28,offsetX:66,offsetY:1},{color:"rgb(210, 70, 230)",sizeW:23,sizeH:30,offsetX:-66,offsetY:-1}],light:[{color:"rgb(173, 50, 122)",sizeW:45,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(184, 65, 101)",sizeW:35,sizeH:32,offsetX:65,offsetY:0},{color:"rgb(155, 43, 144)",sizeW:40,sizeH:28,offsetX:-60,offsetY:2},{color:"rgb(184, 79, 130)",sizeW:35,sizeH:34,offsetX:-90,offsetY:0},{color:"rgb(144, 58, 173)",sizeW:38,sizeH:30,offsetX:85,offsetY:-1},{color:"rgb(180, 43, 108)",sizeW:50,sizeH:24,offsetX:35,offsetY:1},{color:"rgb(166, 86, 158)",sizeW:40,sizeH:22,offsetX:-35,offsetY:0},{color:"rgb(176, 61, 119)",sizeW:35,sizeH:28,offsetX:110,offsetY:1},{color:"rgb(151, 50, 166)",sizeW:30,sizeH:30,offsetX:-110,offsetY:-1}]},ice:{dark:[{color:"rgb(90, 200, 240)",sizeW:36,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(60, 175, 230)",sizeW:30,sizeH:32,offsetX:39,offsetY:0},{color:"rgb(130, 220, 250)",sizeW:33,sizeH:28,offsetX:-36,offsetY:2},{color:"rgb(70, 190, 215)",sizeW:29,sizeH:34,offsetX:-54,offsetY:0},{color:"rgb(110, 210, 255)",sizeW:27,sizeH:30,offsetX:51,offsetY:-1},{color:"rgb(50, 165, 220)",sizeW:36,sizeH:24,offsetX:21,offsetY:1},{color:"rgb(150, 230, 250)",sizeW:30,sizeH:22,offsetX:-21,offsetY:0},{color:"rgb(85, 195, 235)",sizeW:25,sizeH:28,offsetX:66,offsetY:1},{color:"rgb(65, 180, 245)",sizeW:23,sizeH:30,offsetX:-66,offsetY:-1}],light:[{color:"rgb(65, 144, 173)",sizeW:45,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(43, 126, 166)",sizeW:35,sizeH:32,offsetX:65,offsetY:0},{color:"rgb(94, 158, 180)",sizeW:40,sizeH:28,offsetX:-60,offsetY:2},{color:"rgb(50, 137, 155)",sizeW:35,sizeH:34,offsetX:-90,offsetY:0},{color:"rgb(79, 151, 184)",sizeW:38,sizeH:30,offsetX:85,offsetY:-1},{color:"rgb(36, 119, 158)",sizeW:50,sizeH:24,offsetX:35,offsetY:1},{color:"rgb(108, 166, 180)",sizeW:40,sizeH:22,offsetX:-35,offsetY:0},{color:"rgb(61, 140, 169)",sizeW:35,sizeH:28,offsetX:110,offsetY:1},{color:"rgb(47, 130, 176)",sizeW:30,sizeH:30,offsetX:-110,offsetY:-1}]},gold:{dark:[{color:"rgb(240, 190, 60)",sizeW:36,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(255, 210, 90)",sizeW:30,sizeH:32,offsetX:39,offsetY:0},{color:"rgb(225, 165, 40)",sizeW:33,sizeH:28,offsetX:-36,offsetY:2},{color:"rgb(250, 200, 70)",sizeW:29,sizeH:34,offsetX:-54,offsetY:0},{color:"rgb(255, 225, 120)",sizeW:27,sizeH:30,offsetX:51,offsetY:-1},{color:"rgb(230, 175, 50)",sizeW:36,sizeH:24,offsetX:21,offsetY:1},{color:"rgb(245, 205, 85)",sizeW:30,sizeH:22,offsetX:-21,offsetY:0},{color:"rgb(215, 155, 35)",sizeW:25,sizeH:28,offsetX:66,offsetY:1},{color:"rgb(255, 215, 100)",sizeW:23,sizeH:30,offsetX:-66,offsetY:-1}],light:[{color:"rgb(173, 137, 43)",sizeW:45,sizeH:36,offsetX:0,offsetY:2},{color:"rgb(184, 151, 65)",sizeW:35,sizeH:32,offsetX:65,offsetY:0},{color:"rgb(162, 119, 29)",sizeW:40,sizeH:28,offsetX:-60,offsetY:2},{color:"rgb(180, 144, 50)",sizeW:35,sizeH:34,offsetX:-90,offsetY:0},{color:"rgb(184, 162, 86)",sizeW:38,sizeH:30,offsetX:85,offsetY:-1},{color:"rgb(166, 126, 36)",sizeW:50,sizeH:24,offsetX:35,offsetY:1},{color:"rgb(176, 148, 61)",sizeW:40,sizeH:22,offsetX:-35,offsetY:0},{color:"rgb(155, 112, 25)",sizeW:35,sizeH:28,offsetX:110,offsetY:1},{color:"rgb(184, 155, 72)",sizeW:30,sizeH:30,offsetX:-110,offsetY:-1}]}};function wa(r,e,t){return ka[r][e?"dark":"light"].map(a=>{const s=a.offsetX===0?"":a.offsetX>0?` + ${a.offsetX}px`:` - ${Math.abs(a.offsetX)}px`,o=a.offsetY===0?"":a.offsetY>0?` + ${a.offsetY}px`:` - ${Math.abs(a.offsetY)}px`;return`radial-gradient(ellipse calc(${a.sizeW}px * var(--beam-w-${t})) calc(${a.sizeH}px * var(--beam-h-${t})) at calc(var(--beam-x-${t}) * 100%${s}) calc(100%${o}), ${a.color}, transparent)`}).join(`,
       `)}const Ha={colorful:[{color:"rgba(255, 50, 100, 0.48)",sizeW:33,sizeH:30,offsetX:0,offsetY:0},{color:"rgba(40, 180, 220, 0.42)",sizeW:24,sizeH:26,offsetX:39,offsetY:-3},{color:"rgba(50, 200, 80, 0.48)",sizeW:27,sizeH:24,offsetX:-36,offsetY:0},{color:"rgba(180, 40, 240, 0.42)",sizeW:23,sizeH:28,offsetX:-54,offsetY:-2},{color:"rgba(255, 160, 30, 0.50)",sizeW:24,sizeH:24,offsetX:51,offsetY:-1},{color:"rgba(100, 70, 255, 0.45)",sizeW:30,sizeH:20,offsetX:21,offsetY:0},{color:"rgba(40, 140, 255, 0.40)",sizeW:25,sizeH:18,offsetX:-21,offsetY:-2},{color:"rgba(240, 50, 180, 0.45)",sizeW:21,sizeH:24,offsetX:66,offsetY:0},{color:"rgba(30, 185, 170, 0.52)",sizeW:18,sizeH:26,offsetX:-66,offsetY:-1}],mono:[{color:"rgba(200, 200, 200, 0.48)",sizeW:33,sizeH:30,offsetX:0,offsetY:0},{color:"rgba(170, 170, 170, 0.42)",sizeW:24,sizeH:26,offsetX:39,offsetY:-3},{color:"rgba(155, 155, 155, 0.48)",sizeW:27,sizeH:24,offsetX:-36,offsetY:0},{color:"rgba(185, 185, 185, 0.42)",sizeW:23,sizeH:28,offsetX:-54,offsetY:-2},{color:"rgba(165, 165, 165, 0.50)",sizeW:24,sizeH:24,offsetX:51,offsetY:-1},{color:"rgba(180, 180, 180, 0.45)",sizeW:30,sizeH:20,offsetX:21,offsetY:0},{color:"rgba(160, 160, 160, 0.40)",sizeW:25,sizeH:18,offsetX:-21,offsetY:-2},{color:"rgba(175, 175, 175, 0.45)",sizeW:21,sizeH:24,offsetX:66,offsetY:0},{color:"rgba(190, 190, 190, 0.52)",sizeW:18,sizeH:26,offsetX:-66,offsetY:-1}],ocean:[{color:"rgba(100, 80, 220, 0.48)",sizeW:33,sizeH:30,offsetX:0,offsetY:0},{color:"rgba(60, 120, 255, 0.42)",sizeW:24,sizeH:26,offsetX:39,offsetY:-3},{color:"rgba(80, 100, 200, 0.48)",sizeW:27,sizeH:24,offsetX:-36,offsetY:0},{color:"rgba(130, 70, 255, 0.42)",sizeW:23,sizeH:28,offsetX:-54,offsetY:-2},{color:"rgba(70, 130, 255, 0.50)",sizeW:24,sizeH:24,offsetX:51,offsetY:-1},{color:"rgba(120, 80, 255, 0.45)",sizeW:30,sizeH:20,offsetX:21,offsetY:0},{color:"rgba(90, 110, 230, 0.40)",sizeW:25,sizeH:18,offsetX:-21,offsetY:-2},{color:"rgba(110, 90, 240, 0.45)",sizeW:21,sizeH:24,offsetX:66,offsetY:0},{color:"rgba(140, 100, 255, 0.52)",sizeW:18,sizeH:26,offsetX:-66,offsetY:-1}],sunset:[{color:"rgba(255, 100, 60, 0.48)",sizeW:33,sizeH:30,offsetX:0,offsetY:0},{color:"rgba(255, 180, 50, 0.42)",sizeW:24,sizeH:26,offsetX:39,offsetY:-3},{color:"rgba(255, 140, 70, 0.48)",sizeW:27,sizeH:24,offsetX:-36,offsetY:0},{color:"rgba(255, 80, 80, 0.42)",sizeW:23,sizeH:28,offsetX:-54,offsetY:-2},{color:"rgba(255, 200, 60, 0.50)",sizeW:24,sizeH:24,offsetX:51,offsetY:-1},{color:"rgba(255, 120, 50, 0.45)",sizeW:30,sizeH:20,offsetX:21,offsetY:0},{color:"rgba(255, 160, 80, 0.40)",sizeW:25,sizeH:18,offsetX:-21,offsetY:-2},{color:"rgba(255, 90, 60, 0.45)",sizeW:21,sizeH:24,offsetX:66,offsetY:0},{color:"rgba(255, 70, 70, 0.52)",sizeW:18,sizeH:26,offsetX:-66,offsetY:-1}],forest:[{color:"rgba(46, 160, 90,, 0.48)",sizeW:33,sizeH:30,offsetX:0,offsetY:0},{color:"rgba(30, 190, 120,, 0.42)",sizeW:24,sizeH:26,offsetX:39,offsetY:-3},{color:"rgba(70, 180, 70,, 0.48)",sizeW:27,sizeH:24,offsetX:-36,offsetY:0},{color:"rgba(20, 150, 130,, 0.42)",sizeW:23,sizeH:28,offsetX:-54,offsetY:-2},{color:"rgba(90, 200, 80,, 0.50)",sizeW:24,sizeH:24,offsetX:51,offsetY:-1},{color:"rgba(40, 170, 110,, 0.45)",sizeW:30,sizeH:20,offsetX:21,offsetY:0},{color:"rgba(120, 210, 70,, 0.40)",sizeW:25,sizeH:18,offsetX:-21,offsetY:-2},{color:"rgba(35, 145, 100,, 0.45)",sizeW:21,sizeH:24,offsetX:66,offsetY:0},{color:"rgba(60, 195, 140,, 0.52)",sizeW:18,sizeH:26,offsetX:-66,offsetY:-1}],candy:[{color:"rgba(240, 70, 170,, 0.48)",sizeW:33,sizeH:30,offsetX:0,offsetY:0},{color:"rgba(255, 90, 140,, 0.42)",sizeW:24,sizeH:26,offsetX:39,offsetY:-3},{color:"rgba(215, 60, 200,, 0.48)",sizeW:27,sizeH:24,offsetX:-36,offsetY:0},{color:"rgba(255, 110, 180,, 0.42)",sizeW:23,sizeH:28,offsetX:-54,offsetY:-2},{color:"rgba(200, 80, 240,, 0.50)",sizeW:24,sizeH:24,offsetX:51,offsetY:-1},{color:"rgba(250, 60, 150,, 0.45)",sizeW:30,sizeH:20,offsetX:21,offsetY:0},{color:"rgba(230, 120, 220,, 0.40)",sizeW:25,sizeH:18,offsetX:-21,offsetY:-2},{color:"rgba(245, 85, 165,, 0.45)",sizeW:21,sizeH:24,offsetX:66,offsetY:0},{color:"rgba(210, 70, 230,, 0.52)",sizeW:18,sizeH:26,offsetX:-66,offsetY:-1}],ice:[{color:"rgba(90, 200, 240,, 0.48)",sizeW:33,sizeH:30,offsetX:0,offsetY:0},{color:"rgba(60, 175, 230,, 0.42)",sizeW:24,sizeH:26,offsetX:39,offsetY:-3},{color:"rgba(130, 220, 250,, 0.48)",sizeW:27,sizeH:24,offsetX:-36,offsetY:0},{color:"rgba(70, 190, 215,, 0.42)",sizeW:23,sizeH:28,offsetX:-54,offsetY:-2},{color:"rgba(110, 210, 255,, 0.50)",sizeW:24,sizeH:24,offsetX:51,offsetY:-1},{color:"rgba(50, 165, 220,, 0.45)",sizeW:30,sizeH:20,offsetX:21,offsetY:0},{color:"rgba(150, 230, 250,, 0.40)",sizeW:25,sizeH:18,offsetX:-21,offsetY:-2},{color:"rgba(85, 195, 235,, 0.45)",sizeW:21,sizeH:24,offsetX:66,offsetY:0},{color:"rgba(65, 180, 245,, 0.52)",sizeW:18,sizeH:26,offsetX:-66,offsetY:-1}],gold:[{color:"rgba(240, 190, 60,, 0.48)",sizeW:33,sizeH:30,offsetX:0,offsetY:0},{color:"rgba(255, 210, 90,, 0.42)",sizeW:24,sizeH:26,offsetX:39,offsetY:-3},{color:"rgba(225, 165, 40,, 0.48)",sizeW:27,sizeH:24,offsetX:-36,offsetY:0},{color:"rgba(250, 200, 70,, 0.42)",sizeW:23,sizeH:28,offsetX:-54,offsetY:-2},{color:"rgba(255, 225, 120,, 0.50)",sizeW:24,sizeH:24,offsetX:51,offsetY:-1},{color:"rgba(230, 175, 50,, 0.45)",sizeW:30,sizeH:20,offsetX:21,offsetY:0},{color:"rgba(245, 205, 85,, 0.40)",sizeW:25,sizeH:18,offsetX:-21,offsetY:-2},{color:"rgba(215, 155, 35,, 0.45)",sizeW:21,sizeH:24,offsetX:66,offsetY:0},{color:"rgba(255, 215, 100,, 0.52)",sizeW:18,sizeH:26,offsetX:-66,offsetY:-1}]};function Ya(r,e){return Ha[r].map(t=>{const a=t.offsetX===0?"":t.offsetX>0?` + ${t.offsetX}px`:` - ${Math.abs(t.offsetX)}px`,s=t.offsetY===0?"":` - ${Math.abs(t.offsetY)}px`;return`radial-gradient(ellipse calc(${t.sizeW}px * var(--beam-w-${e})) calc(${t.sizeH}px * var(--beam-h-${e})) at calc(var(--beam-x-${e}) * 100%${a}) calc(100%${s}), ${t.color}, transparent)`}).join(`,
    `)}const Wa={colorful:{dark:{spikes:[{color1:"rgb(100, 70, 255)",color2:"rgba(100, 70, 255, 1)"},{color1:"rgba(255, 170, 40, 0.59)",color2:"rgba(255, 170, 40, 0.29)"},{color1:"rgb(50, 200, 100)",color2:"rgba(50, 200, 100, 1)"},{color1:"rgba(200, 50, 240, 0.91)",color2:"rgba(200, 50, 240, 0.45)"},{color1:"rgb(40, 140, 255)",color2:"rgba(40, 140, 255, 1)"}]},light:{spikes:[{color1:"rgb(80, 50, 200)",color2:"rgba(80, 50, 200, 0.8)"},{color1:"rgba(210, 130, 0, 0.7)",color2:"rgba(210, 130, 0, 0.46)"},{color1:"rgb(30, 160, 70)",color2:"rgba(30, 160, 70, 0.82)"},{color1:"rgb(160, 30, 190)",color2:"rgba(160, 30, 190, 0.7)"},{color1:"rgb(30, 100, 200)",color2:"rgba(30, 100, 200, 0.78)"}]}},mono:{dark:{spikes:[{color1:"rgb(200, 200, 200)",color2:"rgba(200, 200, 200, 1)"},{color1:"rgba(180, 180, 180, 0.59)",color2:"rgba(180, 180, 180, 0.29)"},{color1:"rgb(190, 190, 190)",color2:"rgba(190, 190, 190, 1)"},{color1:"rgba(170, 170, 170, 0.91)",color2:"rgba(170, 170, 170, 0.45)"},{color1:"rgb(185, 185, 185)",color2:"rgba(185, 185, 185, 1)"}]},light:{spikes:[{color1:"rgb(80, 80, 80)",color2:"rgba(80, 80, 80, 0.8)"},{color1:"rgba(100, 100, 100, 0.7)",color2:"rgba(100, 100, 100, 0.46)"},{color1:"rgb(70, 70, 70)",color2:"rgba(70, 70, 70, 0.82)"},{color1:"rgb(90, 90, 90)",color2:"rgba(90, 90, 90, 0.7)"},{color1:"rgb(85, 85, 85)",color2:"rgba(85, 85, 85, 0.78)"}]}},ocean:{dark:{spikes:[{color1:"rgb(100, 80, 255)",color2:"rgb(100, 80, 255)"},{color1:"rgba(80, 130, 220, 0.59)",color2:"rgba(80, 130, 220, 0.29)"},{color1:"rgb(60, 100, 255)",color2:"rgb(60, 100, 255)"},{color1:"rgba(90, 120, 200, 0.91)",color2:"rgba(90, 120, 200, 0.45)"},{color1:"rgb(120, 90, 255)",color2:"rgb(120, 90, 255)"}]},light:{spikes:[{color1:"rgb(50, 40, 180)",color2:"rgba(50, 40, 180, 0.8)"},{color1:"rgba(40, 80, 200, 0.7)",color2:"rgba(40, 80, 200, 0.46)"},{color1:"rgb(30, 50, 190)",color2:"rgba(30, 50, 190, 0.82)"},{color1:"rgb(60, 90, 180)",color2:"rgba(60, 90, 180, 0.7)"},{color1:"rgb(70, 60, 200)",color2:"rgba(70, 60, 200, 0.78)"}]}},sunset:{dark:{spikes:[{color1:"rgb(255, 100, 80)",color2:"rgb(255, 100, 80)"},{color1:"rgba(255, 150, 80, 0.59)",color2:"rgba(255, 150, 80, 0.29)"},{color1:"rgb(255, 80, 60)",color2:"rgb(255, 80, 60)"},{color1:"rgba(255, 120, 50, 0.91)",color2:"rgba(255, 120, 50, 0.45)"},{color1:"rgb(255, 140, 70)",color2:"rgb(255, 140, 70)"}]},light:{spikes:[{color1:"rgb(200, 60, 30)",color2:"rgba(200, 60, 30, 0.8)"},{color1:"rgba(220, 100, 20, 0.7)",color2:"rgba(220, 100, 20, 0.46)"},{color1:"rgb(180, 40, 20)",color2:"rgba(180, 40, 20, 0.82)"},{color1:"rgb(210, 80, 10)",color2:"rgba(210, 80, 10, 0.7)"},{color1:"rgb(190, 70, 30)",color2:"rgba(190, 70, 30, 0.78)"}]}},forest:{dark:{spikes:[{color1:"rgb(46, 160, 90)",color2:"rgb(30, 190, 120)"},{color1:"rgba(70, 180, 70,, 0.59)",color2:"rgba(20, 150, 130,, 0.29)"},{color1:"rgb(90, 200, 80)",color2:"rgb(40, 170, 110)"},{color1:"rgba(120, 210, 70,, 0.91)",color2:"rgba(35, 145, 100,, 0.45)"},{color1:"rgb(60, 195, 140)",color2:"rgb(46, 160, 90)"}]},light:{spikes:[{color1:"rgb(33, 115, 65)",color2:"rgba(22, 137, 86,, 0.8)"},{color1:"rgba(50, 130, 50,, 0.7)",color2:"rgba(14, 108, 94,, 0.46)"},{color1:"rgb(65, 144, 58)",color2:"rgba(29, 122, 79,, 0.82)"},{color1:"rgb(86, 151, 50)",color2:"rgba(25, 104, 72,, 0.7)"},{color1:"rgb(43, 140, 101)",color2:"rgba(33, 115, 65,, 0.78)"}]}},candy:{dark:{spikes:[{color1:"rgb(240, 70, 170)",color2:"rgb(255, 90, 140)"},{color1:"rgba(215, 60, 200,, 0.59)",color2:"rgba(255, 110, 180,, 0.29)"},{color1:"rgb(200, 80, 240)",color2:"rgb(250, 60, 150)"},{color1:"rgba(230, 120, 220,, 0.91)",color2:"rgba(245, 85, 165,, 0.45)"},{color1:"rgb(210, 70, 230)",color2:"rgb(240, 70, 170)"}]},light:{spikes:[{color1:"rgb(173, 50, 122)",color2:"rgba(184, 65, 101,, 0.8)"},{color1:"rgba(155, 43, 144,, 0.7)",color2:"rgba(184, 79, 130,, 0.46)"},{color1:"rgb(144, 58, 173)",color2:"rgba(180, 43, 108,, 0.82)"},{color1:"rgb(166, 86, 158)",color2:"rgba(176, 61, 119,, 0.7)"},{color1:"rgb(151, 50, 166)",color2:"rgba(173, 50, 122,, 0.78)"}]}},ice:{dark:{spikes:[{color1:"rgb(90, 200, 240)",color2:"rgb(60, 175, 230)"},{color1:"rgba(130, 220, 250,, 0.59)",color2:"rgba(70, 190, 215,, 0.29)"},{color1:"rgb(110, 210, 255)",color2:"rgb(50, 165, 220)"},{color1:"rgba(150, 230, 250,, 0.91)",color2:"rgba(85, 195, 235,, 0.45)"},{color1:"rgb(65, 180, 245)",color2:"rgb(90, 200, 240)"}]},light:{spikes:[{color1:"rgb(65, 144, 173)",color2:"rgba(43, 126, 166,, 0.8)"},{color1:"rgba(94, 158, 180,, 0.7)",color2:"rgba(50, 137, 155,, 0.46)"},{color1:"rgb(79, 151, 184)",color2:"rgba(36, 119, 158,, 0.82)"},{color1:"rgb(108, 166, 180)",color2:"rgba(61, 140, 169,, 0.7)"},{color1:"rgb(47, 130, 176)",color2:"rgba(65, 144, 173,, 0.78)"}]}},gold:{dark:{spikes:[{color1:"rgb(240, 190, 60)",color2:"rgb(255, 210, 90)"},{color1:"rgba(225, 165, 40,, 0.59)",color2:"rgba(250, 200, 70,, 0.29)"},{color1:"rgb(255, 225, 120)",color2:"rgb(230, 175, 50)"},{color1:"rgba(245, 205, 85,, 0.91)",color2:"rgba(215, 155, 35,, 0.45)"},{color1:"rgb(255, 215, 100)",color2:"rgb(240, 190, 60)"}]},light:{spikes:[{color1:"rgb(173, 137, 43)",color2:"rgba(184, 151, 65,, 0.8)"},{color1:"rgba(162, 119, 29,, 0.7)",color2:"rgba(180, 144, 50,, 0.46)"},{color1:"rgb(184, 162, 86)",color2:"rgba(166, 126, 36,, 0.82)"},{color1:"rgb(176, 148, 61)",color2:"rgba(155, 112, 25,, 0.7)"},{color1:"rgb(184, 155, 72)",color2:"rgba(173, 137, 43,, 0.78)"}]}}};function ke(r,e){const t=r.match(/^rgba\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*[\d.]+\s*\)$/);if(t)return`rgba(${t[1]}, ${t[2]}, ${t[3]}, ${e})`;const a=r.match(/^rgb\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*\)$/);return a?`rgba(${a[1]}, ${a[2]}, ${a[3]}, ${e})`:r}function ne(r,e){const t=r.match(/^rgba\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*\)$/);if(t)return`rgba(${t[1]}, ${t[2]}, ${t[3]}, ${(parseFloat(t[4])*e).toFixed(2)})`;const a=r.match(/^rgb\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*\)$/);return a?`rgba(${a[1]}, ${a[2]}, ${a[3]}, ${e.toFixed(2)})`:r}function Xa(r,e,t){const a=Ma(r,e),s=Wa[r][e?"dark":"light"],o=r==="mono",i=o?.14:1,c=o?ne(a.primary,.14):a.primary,p=o?ne(a.primary,.09):a.primary,g=o?ne(a.secondary,.12):a.secondary,b=o?ke(a.secondary,.06):ke(a.secondary,.49),n=s.spikes.map(E=>o?{color1:ne(E.color1,i),color2:ne(E.color2,i*.7)}:E),f=o?"12px":"0.8px",u=o?"14px":"2px",h=o?"12px":"1.2px",x=o?"10px":"0.6px",d=o?"42px":"92px",m=o?"38px":"72px",z=o?"40px":"85px",M=o?"32px":"60px",w=o?"12px":"1px",v=o?"rgba(255, 255, 255, 0.5)":"rgba(255, 255, 255, 1)",H=o?"rgba(255, 255, 255, 0.45)":"rgba(255, 255, 255, 0.9)",k=o?"rgba(255, 255, 255, 0.25)":"rgba(255, 255, 255, 0.5)",y=o?"rgba(255, 255, 255, 0.15)":"rgba(255, 255, 255, 0.3)",_=o?"rgba(255, 255, 255, 0.06)":"rgba(255, 255, 255, 0.12)",X=o?"rgba(255, 255, 255, 0.015)":"rgba(255, 255, 255, 0.03)";if(e)return`radial-gradient(ellipse calc(${f} * var(--beam-spike-${t}) * var(--beam-spike-mul, 1)) calc(${d} * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 8% calc(100% - 2px), ${c}, ${p} 30%, transparent 88%),
       radial-gradient(ellipse calc(10px * var(--beam-spike2-${t}) * var(--beam-spike-mul, 1)) calc(35px * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 22% calc(100% - 4px), ${g}, ${b} 50%, transparent 95%),
       radial-gradient(ellipse calc(${u} * (2 - var(--beam-spike-${t})) * var(--beam-spike-mul, 1)) calc(${m} * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 36% calc(100% - 3px), ${n[0].color1}, ${n[0].color2} 40%, transparent 90%),
       radial-gradient(ellipse calc(14px * var(--beam-spike2-${t}) * var(--beam-spike-mul, 1)) calc(28px * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 50% calc(100% - 2px), ${n[1].color1}, ${n[1].color2} 55%, transparent 96%),
       radial-gradient(ellipse calc(${h} * (2 - var(--beam-spike2-${t})) * var(--beam-spike-mul, 1)) calc(${z} * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 64% calc(100% - 4px), ${n[2].color1}, ${n[2].color2} 35%, transparent 89%),
       radial-gradient(ellipse calc(7px * var(--beam-spike-${t}) * var(--beam-spike-mul, 1)) calc(45px * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 78% calc(100% - 2px), ${n[3].color1}, ${n[3].color2} 48%, transparent 94%),
       radial-gradient(ellipse calc(${x} * (2 - var(--beam-spike-${t})) * var(--beam-spike-mul, 1)) calc(${M} * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 92% calc(100% - 3px), ${n[4].color1}, ${n[4].color2} 42%, transparent 91%),
       radial-gradient(ellipse calc(21px * var(--beam-spike-${t})) calc(15px * var(--beam-spike2-${t})) at calc(var(--beam-x-${t}) * 100%) calc(100% + 1px), ${v} 0%, ${H} 20%, ${k} 50%, transparent 100%),
       radial-gradient(ellipse calc(42px * var(--beam-w-${t})) calc(40px * var(--beam-h-${t})) at calc(var(--beam-x-${t}) * 100%) 100%, ${y} 0%, ${_} 25%, ${X} 55%, transparent 80%)`;{const E=o?ne(a.primary,.11):ke(a.primary,.85),O=o?ne(a.secondary,.09):ke(a.secondary,.7);return`radial-gradient(ellipse calc(${f} * var(--beam-spike-${t}) * var(--beam-spike-mul, 1)) calc(${d} * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 8% calc(100% - 2px), ${c}, ${E} 30%, transparent 88%),
       radial-gradient(ellipse calc(10px * var(--beam-spike2-${t}) * var(--beam-spike-mul, 1)) calc(35px * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 22% calc(100% - 4px), ${g}, ${O} 50%, transparent 95%),
       radial-gradient(ellipse calc(${u} * (2 - var(--beam-spike-${t})) * var(--beam-spike-mul, 1)) calc(${m} * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 36% calc(100% - 3px), ${n[0].color1}, ${n[0].color2} 40%, transparent 90%),
       radial-gradient(ellipse calc(14px * var(--beam-spike2-${t}) * var(--beam-spike-mul, 1)) calc(28px * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 50% calc(100% - 2px), ${n[1].color1}, ${n[1].color2} 55%, transparent 96%),
       radial-gradient(ellipse calc(${h} * (2 - var(--beam-spike2-${t})) * var(--beam-spike-mul, 1)) calc(${z} * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 64% calc(100% - 4px), ${n[2].color1}, ${n[2].color2} 35%, transparent 89%),
       radial-gradient(ellipse calc(7px * var(--beam-spike-${t}) * var(--beam-spike-mul, 1)) calc(45px * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 78% calc(100% - 2px), ${n[3].color1}, ${n[3].color2} 48%, transparent 94%),
       radial-gradient(ellipse calc(${w} * (2 - var(--beam-spike-${t})) * var(--beam-spike-mul, 1)) calc(${M} * var(--beam-h-${t}) * var(--beam-spike-mul, 1)) at 92% calc(100% - 3px), ${n[4].color1}, ${n[4].color2} 42%, transparent 91%),
       radial-gradient(ellipse calc(50px * var(--beam-w-${t})) calc(32px * var(--beam-h-${t})) at calc(var(--beam-x-${t}) * 100%) calc(100%), rgba(0, 0, 0, 0.5) 0%, rgba(0, 0, 0, 0.18) 30%, rgba(0, 0, 0, 0.03) 60%, transparent 85%)`}}const qr=[{region:1,quad:"tl"},{region:2,quad:"tl"},{region:3,quad:"bl"},{region:1,quad:"bl"},{region:2,quad:"br"},{region:3,quad:"br"},{region:1,quad:"tr"},{region:2,quad:"tr"},{region:3,quad:"tr"}],_a=[[65,35],[55,30],[35,65],[15,30],[173,28],[80,22],[69,28],[22,38],[47,44]],Ra=[{ci:0,region:1,quad:"tl",w:84,h:48},{ci:1,region:2,quad:"tl",w:72,h:42},{ci:2,region:3,quad:"bl",w:48,h:84},{ci:4,region:2,quad:"br",w:216,h:38},{ci:5,region:3,quad:"br",w:102,h:31},{ci:6,region:1,quad:"tr",w:89,h:38},{ci:8,region:3,quad:"tr",w:62,h:58}],$r=[{ci:0,region:1,quad:"tl",w:80,h:19,x:"27%",y:"0%"},{ci:6,region:2,quad:"tr",w:74,h:11,x:"73%",y:"-1%"},{ci:7,region:3,quad:"tr",w:15,h:44,x:"100%",y:"33%"},{ci:8,region:1,quad:"br",w:19,h:38,x:"101%",y:"72%"},{ci:4,region:2,quad:"br",w:84,h:13,x:"67%",y:"100%"},{ci:1,region:3,quad:"bl",w:60,h:21,x:"24%",y:"101%"},{ci:2,region:1,quad:"bl",w:17,h:40,x:"0%",y:"60%"},{ci:3,region:2,quad:"tl",w:13,h:32,x:"-1%",y:"28%"}],Ea=[{ci:0,region:1,quad:"tl",w:110,h:30,x:"27%",y:"3%"},{ci:6,region:2,quad:"tr",w:100,h:20,x:"73%",y:"1%"},{ci:7,region:3,quad:"tr",w:26,h:62,x:"100%",y:"33%"},{ci:8,region:1,quad:"br",w:30,h:56,x:"101%",y:"72%"},{ci:4,region:2,quad:"br",w:120,h:22,x:"67%",y:"99%"},{ci:1,region:3,quad:"bl",w:88,h:32,x:"24%",y:"99%"},{ci:2,region:1,quad:"bl",w:28,h:58,x:"0%",y:"60%"}];function Oa(r,e,t){const a=r.match(/^rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)$/);return`rgba(${a?`${a[1]}, ${a[2]}, ${a[3]}`:"255, 255, 255"}, var(--bop-${e}-${t}))`}function Ge(r,e,t,a,s,o,i,c){return`radial-gradient(ellipse calc(${e}px * var(--bw${a}-${c}) * var(--pulse-glow-sx, 1) * var(--pulse-glow-boost, 1)) calc(${t}px * var(--bh${a}-${c}) * var(--bgh-${c}) * var(--pulse-glow-sy, 1) * var(--pulse-glow-boost, 1)) at calc(${o} + var(--bx${a}-${c})) calc(${i} + var(--by${a}-${c})), ${Oa(r,s,c)}, transparent)`}function Sa(r,e){return pe[r].border.map((t,a)=>{const{region:s,quad:o}=qr[a],[i,c]=t.pos.split(" "),[p,g]=t.size.split(" ").map(parseFloat);return Ge(t.color,p,g,s,o,i,c,e)}).join(`,
    `)}function Pa(r,e,t){const a=pe[r].border.map((c,p)=>{const{region:g,quad:b}=qr[p],[n,f]=c.pos.split(" "),[u,h]=_a[p];return Ge(c.color,u,h,g,b,n,f,e)}),s=t?"255, 255, 255":"0, 0, 0",o=t?.18:.08,i=[["0%","0%","tl"],["100%","0%","tr"],["0%","100%","bl"],["100%","100%","br"]].map(([c,p,g])=>`radial-gradient(ellipse 60px 60px at ${c} ${p}, rgba(${s}, calc(${o} * var(--bop-${g}-${e}))), transparent 70%)`);return[...a,...i].join(`,
    `)}function yr(r,e,t){const a=pe[e].border;return r.map(s=>{const o=a[s.ci],[i,c]=o.pos.split(" ");return Ge(o.color,s.w,s.h,s.region,s.quad,s.x??i,s.y??c,t)}).join(`,
    `)}function Dr(r,e,t){const a=pe[e].border,s=+t.toFixed(3);return r.map(o=>{const i=a[o.ci],[c,p]=i.pos.split(" "),g=o.x??c,b=o.y??p,n=i.color.match(/^rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)$/),f=n?`${n[1]}, ${n[2]}, ${n[3]}`:"255, 255, 255";return`radial-gradient(ellipse calc(${o.w}px * var(--pulse-glow-sx, 1) * var(--pulse-glow-boost, 1)) calc(${o.h}px * var(--pulse-glow-sy, 1) * var(--pulse-glow-boost, 1)) at ${g} ${b}, rgba(${f}, ${s}), transparent)`}).join(`,
    `)}function he(r){return`
[data-beam="${r}"][data-paused],
[data-beam="${r}"][data-paused]::after,
[data-beam="${r}"][data-paused]::before,
[data-beam="${r}"][data-paused] [data-beam-bloom] {
  animation-play-state: paused !important;
}`}function Nr(r){const e=["bw1","bh1","bw2","bh2","bw3","bh3","bgh","bop-tl","bop-tr","bop-bl","bop-br"],t=["bx1","by1","bx2","by2","bx3","by3"],a=e.map(o=>`@property --${o}-${r} {
  syntax: "<number>";
  initial-value: 1;
  inherits: true;
}`).join(`

`),s=t.map(o=>`@property --${o}-${r} {
  syntax: "<length>";
  initial-value: 0px;
  inherits: true;
}`).join(`

`);return`${a}

${s}

@property --beam-opacity-${r} {
  syntax: "<number>";
  initial-value: 0;
  inherits: true;
}

@property --beam-hue-${r} {
  syntax: "<angle>";
  initial-value: 0deg;
  inherits: true;
}`}function Ze(r,e,t){const a=e==="dark",s=t/2.3;return r==="pulse-inner"?{sp:.28,dr:a?33:40,op:a?.48:.45,gh:a?.34:.22,bs:(a?1.9:2.6)*s,ss:(a?2.6:4.6)*s,ghs:(a?2.4:5.5)*s,huePeriod:16}:{sp:a?.28:.36,dr:a?14:19,op:a?.46:0,gh:a?.16:.58,bs:(a?2.3:3.7)*s,ss:(a?6.4:4.6)*s,ghs:(a?2.4:3.8)*s,huePeriod:14}}function Ta(r,e){const{sp:t,dr:a,op:s,gh:o,bs:i,ss:c,ghs:p}=e;return[{prop:`--bw1-${r}`,a:1-t,b:1+t*1.1,period:c*.9,delay:0,unit:""},{prop:`--bh1-${r}`,a:1+t*.9,b:1-t*.85,period:c*1.26,delay:0,unit:""},{prop:`--bx1-${r}`,a:-a,b:a*.9,period:i*1.6,delay:0,unit:"px"},{prop:`--by1-${r}`,a:a*.55,b:-a*.7,period:i*1.6,delay:0,unit:"px"},{prop:`--bw2-${r}`,a:1+t,b:1-t*.85,period:c*1.1,delay:0,unit:""},{prop:`--bh2-${r}`,a:1-t*.8,b:1+t*1.05,period:c*.81,delay:0,unit:""},{prop:`--bx2-${r}`,a:a*.8,b:-a*.9,period:i*1.88,delay:0,unit:"px"},{prop:`--by2-${r}`,a:-a,b:a*.65,period:i*1.88,delay:0,unit:"px"},{prop:`--bw3-${r}`,a:1-t*.6,b:1+t*1.15,period:c*.98,delay:0,unit:""},{prop:`--bh3-${r}`,a:1+t*.75,b:1-t,period:c*1.4,delay:0,unit:""},{prop:`--bx3-${r}`,a:-a*.6,b:a,period:i*1.45,delay:0,unit:"px"},{prop:`--by3-${r}`,a:-a*.85,b:a*.45,period:i*1.45,delay:0,unit:"px"},{prop:`--bgh-${r}`,a:1-o,b:1+o,period:p,delay:0,unit:""},{prop:`--bop-tl-${r}`,a:1-s,b:1,period:i,delay:0,unit:""},{prop:`--bop-tr-${r}`,a:1-s,b:1,period:i*1.32,delay:i*.28,unit:""},{prop:`--bop-bl-${r}`,a:1-s,b:1,period:i*.84,delay:i*.55,unit:""},{prop:`--bop-br-${r}`,a:1-s,b:1,period:i*1.58,delay:i*.83,unit:""}]}function Ca(r,e,t,a,s,o){if(r!=="pulse-inner"&&r!=="pulse-outside")return null;const i=Ze(r,e,t);return{oscillators:Ta(o,i),hue:s?null:{prop:`--beam-hue-${o}`,range:360,period:i.huePeriod,continuous:!0}}}function we(r,e,t){return`  animation: ${e}-${r} ${t}s ease forwards;`}function ae(r,e=1){return Math.max(.5,Math.round(r*e*100)/100)}function Aa(r){const{size:e}=r;return e==="line"?Da(r):e==="sm"?Ia(r):e==="pulse-inner"?Fa(r):e==="pulse-outside"?qa(r):ja(r)}function Ia(r){const{id:e,borderRadius:t,borderWidth:a,duration:s,strokeOpacity:o,innerOpacity:i,bloomOpacity:c,innerShadow:p,colorVariant:g,staticColors:b,brightness:n,saturation:f,hueRange:u,theme:h,glowSize:x=1}=r,d=Math.max(0,t-a),m=g==="mono"?.5:1,z=o*m,M=i*m,w=c*m,v=b?"":`animation: beam-hue-shift-${e} 12s ease-in-out infinite;`,H=b?"":`
@keyframes beam-hue-shift-${e} {
  0% { filter: hue-rotate(calc(var(--beam-hue-base, 0deg) - ${u}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
  50% { filter: hue-rotate(calc(var(--beam-hue-base, 0deg) + ${u}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
  100% { filter: hue-rotate(calc(var(--beam-hue-base, 0deg) - ${u}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
}`,k=h==="dark",y=k?`conic-gradient(
        from var(--beam-angle-${e}),
        transparent 0%, transparent 54%,
        rgba(255, 255, 255, 0.1) 57%,
        rgba(255, 255, 255, 0.3) 60%,
        rgba(255, 255, 255, 0.6) 63%,
        rgba(255, 255, 255, 0.75) 66%,
        rgba(255, 255, 255, 0.6) 69%,
        rgba(255, 255, 255, 0.3) 72%,
        rgba(255, 255, 255, 0.1) 75%,
        transparent 78%, transparent 100%
      )`:`conic-gradient(
        from var(--beam-angle-${e}),
        transparent 0%, transparent 54%,
        rgba(0, 0, 0, 0.08) 57%,
        rgba(0, 0, 0, 0.2) 60%,
        rgba(0, 0, 0, 0.4) 63%,
        rgba(0, 0, 0, 0.55) 66%,
        rgba(0, 0, 0, 0.4) 69%,
        rgba(0, 0, 0, 0.2) 72%,
        rgba(0, 0, 0, 0.08) 75%,
        transparent 78%, transparent 100%
      )`,_=$a(g),X=ya(g),E=k?`conic-gradient(
        from var(--beam-angle-${e}),
        transparent 0%, transparent 58%,
        rgba(255, 255, 255, 0.03) 62%,
        rgba(255, 255, 255, 0.08) 65%,
        rgba(255, 255, 255, 0.2) 67%,
        rgba(255, 255, 255, 0.45) 69%,
        rgba(255, 255, 255, 0.85) 70%,
        rgba(255, 255, 255, 0.85) 70.5%,
        rgba(255, 255, 255, 0.45) 71.5%,
        rgba(255, 255, 255, 0.2) 73%,
        rgba(255, 255, 255, 0.08) 75%,
        rgba(255, 255, 255, 0.03) 78%,
        transparent 82%
      )`:`conic-gradient(
        from var(--beam-angle-${e}),
        transparent 0%, transparent 58%,
        rgba(0, 0, 0, 0.02) 62%,
        rgba(0, 0, 0, 0.08) 65%,
        rgba(0, 0, 0, 0.2) 67%,
        rgba(0, 0, 0, 0.4) 69%,
        rgba(0, 0, 0, 0.6) 70%,
        rgba(0, 0, 0, 0.6) 70.5%,
        rgba(0, 0, 0, 0.4) 71.5%,
        rgba(0, 0, 0, 0.2) 73%,
        rgba(0, 0, 0, 0.08) 75%,
        rgba(0, 0, 0, 0.02) 78%,
        transparent 82%
      )`,O=`conic-gradient(
    from var(--beam-angle-${e}),
    transparent 0%, transparent 22%,
    rgba(255, 255, 255, 0.12) 28%, rgba(255, 255, 255, 0.4) 36%,
    white 46%, white 82%,
    rgba(255, 255, 255, 0.4) 88%, rgba(255, 255, 255, 0.12) 94%,
    transparent 97%, transparent 100%
  )`;return`
@property --beam-angle-${e} {
  syntax: "<angle>";
  initial-value: 0deg;
  inherits: true;
}

@property --beam-opacity-${e} {
  syntax: "<number>";
  initial-value: 0;
  inherits: true;
}

[data-beam="${e}"] {
  position: relative;
  border-radius: ${t}px;
  overflow: hidden;
}

[data-beam="${e}"][data-active] {
  animation:
    beam-spin-${e} ${s}s linear infinite,
    beam-fade-in-${e} 0.6s ease forwards;
}

[data-beam="${e}"][data-fading] {
  animation:
    beam-spin-${e} ${s}s linear infinite,
    beam-fade-out-${e} 0.5s ease forwards;
}

[data-beam="${e}"][data-active]::after,
[data-beam="${e}"][data-fading]::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: ${d}px;
  padding: ${a}px;
  clip-path: inset(0 round ${t}px);
  background: ${y},${_};
  -webkit-mask:
    conic-gradient(
      from var(--beam-angle-${e}),
      transparent 0%, transparent 30%,
      rgba(255, 255, 255, 0.1) 36%, rgba(255, 255, 255, 0.35) 44%,
      white 52%, white 80%,
      rgba(255, 255, 255, 0.35) 86%, rgba(255, 255, 255, 0.1) 92%,
      transparent 95%, transparent 100%
    ),
    linear-gradient(#fff 0 0) content-box,
    linear-gradient(#fff 0 0);
  -webkit-mask-composite: source-in, xor;
  mask:
    conic-gradient(
      from var(--beam-angle-${e}),
      transparent 0%, transparent 30%,
      rgba(255, 255, 255, 0.1) 36%, rgba(255, 255, 255, 0.35) 44%,
      white 52%, white 80%,
      rgba(255, 255, 255, 0.35) 86%, rgba(255, 255, 255, 0.1) 92%,
      transparent 95%, transparent 100%
    ),
    linear-gradient(#fff 0 0) content-box,
    linear-gradient(#fff 0 0);
  mask-composite: intersect, exclude;
  pointer-events: none;
  z-index: 2;
  opacity: calc(var(--beam-opacity-${e}) * ${z.toFixed(2)} * var(--beam-stroke-opacity, 1) * var(--beam-strength, 1));
  ${v}
}

[data-beam="${e}"][data-active]::before,
[data-beam="${e}"][data-fading]::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: ${t}px;
  clip-path: inset(0 round ${t}px);
  background: ${X};
  box-shadow: inset 0 0 5px 1px ${p};
  -webkit-mask-image: ${O};
  -webkit-mask-composite: source-over;
  mask-image: ${O};
  mask-composite: add;
  pointer-events: none;
  z-index: 1;
  opacity: calc(var(--beam-opacity-${e}) * ${M.toFixed(2)} * var(--beam-inner-opacity, 1) * var(--beam-strength, 1));
  ${v}
}

[data-beam="${e}"] [data-beam-bloom] {
  display: none;
  position: absolute;
  inset: 0;
  border-radius: ${d}px;
  clip-path: inset(0 round ${t}px);
  background: ${E};
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  mask-composite: exclude;
  padding: ${a}px;
  filter: blur(${ae(8,x)}px) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)});
  pointer-events: none;
  z-index: 3;
  opacity: 0;
}

[data-beam="${e}"][data-active] [data-beam-bloom],
[data-beam="${e}"][data-fading] [data-beam-bloom] {
  display: block;
  opacity: calc(var(--beam-opacity-${e}) * ${w.toFixed(2)} * var(--beam-bloom-opacity, 1) * var(--beam-strength, 1));
}

@keyframes beam-spin-${e} {
  to { --beam-angle-${e}: 360deg; }
}

@keyframes beam-fade-in-${e} {
  to { --beam-opacity-${e}: 1; }
}

@keyframes beam-fade-out-${e} {
  from { --beam-opacity-${e}: 1; }
  to { --beam-opacity-${e}: 0; }
}
${H}
${he(e)}
`}function ja(r){const{id:e,borderRadius:t,borderWidth:a,duration:s,strokeOpacity:o,innerOpacity:i,bloomOpacity:c,innerShadow:p,colorVariant:g,staticColors:b,brightness:n,saturation:f,hueRange:u,theme:h,glowSize:x=1}=r,d=Math.max(0,t-a),m=g==="mono"?.5:1,z=o*m,M=i*m,w=c*m,v=b?"":`animation: beam-hue-shift-${e} 12s ease-in-out infinite;`,H=b?"":`
@keyframes beam-hue-shift-${e} {
  0% { filter: hue-rotate(calc(var(--beam-hue-base, 0deg) - ${u}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
  50% { filter: hue-rotate(calc(var(--beam-hue-base, 0deg) + ${u}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
  100% { filter: hue-rotate(calc(var(--beam-hue-base, 0deg) - ${u}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
}`,k=h==="dark",y=k?`conic-gradient(
        from var(--beam-angle-${e}),
        transparent 0%, transparent 54%,
        rgba(255, 255, 255, 0.1) 57%,
        rgba(255, 255, 255, 0.3) 60%,
        rgba(255, 255, 255, 0.6) 63%,
        rgba(255, 255, 255, 0.75) 66%,
        rgba(255, 255, 255, 0.6) 69%,
        rgba(255, 255, 255, 0.3) 72%,
        rgba(255, 255, 255, 0.1) 75%,
        transparent 78%, transparent 100%
      )`:`conic-gradient(
        from var(--beam-angle-${e}),
        transparent 0%, transparent 54%,
        rgba(0, 0, 0, 0.08) 57%,
        rgba(0, 0, 0, 0.2) 60%,
        rgba(0, 0, 0, 0.4) 63%,
        rgba(0, 0, 0, 0.55) 66%,
        rgba(0, 0, 0, 0.4) 69%,
        rgba(0, 0, 0, 0.2) 72%,
        rgba(0, 0, 0, 0.08) 75%,
        transparent 78%, transparent 100%
      )`,_=za(g),X=va(g),E=k?`conic-gradient(
        from var(--beam-angle-${e}),
        transparent 0%, transparent 58%,
        rgba(255, 255, 255, 0.03) 62%,
        rgba(255, 255, 255, 0.08) 65%,
        rgba(255, 255, 255, 0.2) 67%,
        rgba(255, 255, 255, 0.45) 69%,
        rgba(255, 255, 255, 0.85) 70%,
        rgba(255, 255, 255, 0.85) 70.5%,
        rgba(255, 255, 255, 0.45) 71.5%,
        rgba(255, 255, 255, 0.2) 73%,
        rgba(255, 255, 255, 0.08) 75%,
        rgba(255, 255, 255, 0.03) 78%,
        transparent 82%
      )`:`conic-gradient(
        from var(--beam-angle-${e}),
        transparent 0%, transparent 58%,
        rgba(0, 0, 0, 0.02) 62%,
        rgba(0, 0, 0, 0.08) 65%,
        rgba(0, 0, 0, 0.2) 67%,
        rgba(0, 0, 0, 0.4) 69%,
        rgba(0, 0, 0, 0.6) 70%,
        rgba(0, 0, 0, 0.6) 70.5%,
        rgba(0, 0, 0, 0.4) 71.5%,
        rgba(0, 0, 0, 0.2) 73%,
        rgba(0, 0, 0, 0.08) 75%,
        rgba(0, 0, 0, 0.02) 78%,
        transparent 82%
      )`;return`
@property --beam-angle-${e} {
  syntax: "<angle>";
  initial-value: 0deg;
  inherits: true;
}

@property --beam-opacity-${e} {
  syntax: "<number>";
  initial-value: 0;
  inherits: true;
}

[data-beam="${e}"] {
  position: relative;
  border-radius: ${t}px;
  overflow: hidden;
}

[data-beam="${e}"][data-active] {
  animation:
    beam-spin-${e} ${s}s linear infinite,
    beam-fade-in-${e} 0.6s ease forwards;
}

[data-beam="${e}"][data-fading] {
  animation:
    beam-spin-${e} ${s}s linear infinite,
    beam-fade-out-${e} 0.5s ease forwards;
}

[data-beam="${e}"][data-active]::after,
[data-beam="${e}"][data-fading]::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: ${d}px;
  padding: ${a}px;
  clip-path: inset(0 round ${t}px);
  background: ${y},${_};
  -webkit-mask:
    conic-gradient(
      from var(--beam-angle-${e}),
      transparent 0%, transparent 30%,
      rgba(255, 255, 255, 0.1) 36%, rgba(255, 255, 255, 0.35) 44%,
      white 52%, white 80%,
      rgba(255, 255, 255, 0.35) 86%, rgba(255, 255, 255, 0.1) 92%,
      transparent 95%, transparent 100%
    ),
    linear-gradient(#fff 0 0) content-box,
    linear-gradient(#fff 0 0);
  -webkit-mask-composite: source-in, xor;
  mask:
    conic-gradient(
      from var(--beam-angle-${e}),
      transparent 0%, transparent 30%,
      rgba(255, 255, 255, 0.1) 36%, rgba(255, 255, 255, 0.35) 44%,
      white 52%, white 80%,
      rgba(255, 255, 255, 0.35) 86%, rgba(255, 255, 255, 0.1) 92%,
      transparent 95%, transparent 100%
    ),
    linear-gradient(#fff 0 0) content-box,
    linear-gradient(#fff 0 0);
  mask-composite: intersect, exclude;
  pointer-events: none;
  z-index: 2;
  opacity: calc(var(--beam-opacity-${e}) * ${z.toFixed(2)} * var(--beam-stroke-opacity, 1) * var(--beam-strength, 1));
  ${v}
}

[data-beam="${e}"][data-active]::before,
[data-beam="${e}"][data-fading]::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: ${t}px;
  background: ${X};
  box-shadow: inset 0 0 9px 1px ${p};
  -webkit-mask-image:
    conic-gradient(
      from var(--beam-angle-${e}),
      transparent 0%, transparent 30%,
      rgba(255, 255, 255, 0.1) 36%, rgba(255, 255, 255, 0.35) 44%,
      white 52%, white 80%,
      rgba(255, 255, 255, 0.35) 86%, rgba(255, 255, 255, 0.1) 92%,
      transparent 95%, transparent 100%
    ),
    linear-gradient(white, transparent 28px, transparent calc(100% - 28px), white),
    linear-gradient(to right, white, transparent 28px, transparent calc(100% - 28px), white);
  -webkit-mask-composite: source-in, source-over;
  mask-image:
    conic-gradient(
      from var(--beam-angle-${e}),
      transparent 0%, transparent 30%,
      rgba(255, 255, 255, 0.1) 36%, rgba(255, 255, 255, 0.35) 44%,
      white 52%, white 80%,
      rgba(255, 255, 255, 0.35) 86%, rgba(255, 255, 255, 0.1) 92%,
      transparent 95%, transparent 100%
    ),
    linear-gradient(white, transparent 28px, transparent calc(100% - 28px), white),
    linear-gradient(to right, white, transparent 28px, transparent calc(100% - 28px), white);
  mask-composite: intersect, add;
  pointer-events: none;
  z-index: 1;
  opacity: calc(var(--beam-opacity-${e}) * ${M.toFixed(2)} * var(--beam-inner-opacity, 1) * var(--beam-strength, 1));
  clip-path: inset(0 round ${t}px);
  ${v}
}

[data-beam="${e}"] [data-beam-bloom] {
  display: none;
  position: absolute;
  inset: 0;
  border-radius: ${d}px;
  clip-path: inset(0 round ${t}px);
  background: ${E};
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  mask-composite: exclude;
  padding: ${a}px;
  filter: blur(${ae(8,x)}px) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)});
  pointer-events: none;
  z-index: 3;
  opacity: 0;
}

[data-beam="${e}"][data-active] [data-beam-bloom],
[data-beam="${e}"][data-fading] [data-beam-bloom] {
  display: block;
  opacity: calc(var(--beam-opacity-${e}) * ${w.toFixed(2)} * var(--beam-bloom-opacity, 1) * var(--beam-strength, 1));
}

@keyframes beam-spin-${e} {
  to { --beam-angle-${e}: 360deg; }
}

@keyframes beam-fade-in-${e} {
  to { --beam-opacity-${e}: 1; }
}

@keyframes beam-fade-out-${e} {
  from { --beam-opacity-${e}: 1; }
  to { --beam-opacity-${e}: 0; }
}
${H}
${he(e)}
`}function Fa(r){const{id:e,borderRadius:t,borderWidth:a,duration:s,strokeOpacity:o,innerOpacity:i,bloomOpacity:c,colorVariant:p,staticColors:g,brightness:b,saturation:n,hueRange:f,theme:u,glowSize:h=1}=r,x=u==="dark",d=p==="mono"?.5:1,m=(o*d).toFixed(2),z=(i*d).toFixed(2),M=(c*d).toFixed(2),{op:w}=Ze("pulse-inner",u,s),v=ae(8,h),H=b.toFixed(2),k=n.toFixed(2),y=g?`filter: brightness(${H}) saturate(${k});`:`filter: hue-rotate(calc(var(--beam-hue-base, 0deg) + var(--beam-hue-${e}))) brightness(${H}) saturate(${k});`,_=g?`filter: blur(${v}px) brightness(${H}) saturate(${k});`:`filter: blur(${v}px) hue-rotate(calc(var(--beam-hue-base, 0deg) + var(--beam-hue-${e}))) brightness(${H}) saturate(${k});`,X=Sa(p,e),E=Pa(p,e,x),O=Dr(Ra,p,1-w*.5);return`
${Nr(e)}

[data-beam="${e}"] {
  position: relative;
  border-radius: ${t}px;
  overflow: hidden;
  isolation: isolate;
}

[data-beam="${e}"][data-active] {
${we(e,"beam-fade-in",.6)}
}

[data-beam="${e}"][data-fading] {
${we(e,"beam-fade-out",.5)}
}

[data-beam="${e}"][data-active]::after,
[data-beam="${e}"][data-fading]::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: ${t}px;
  padding: ${a}px;
  clip-path: inset(0 round ${t}px);
  background: ${X};
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  mask-composite: exclude;
  pointer-events: none;
  z-index: 2;
  will-change: opacity, filter;
  opacity: calc(var(--beam-opacity-${e}) * ${m} * var(--beam-stroke-opacity, 1) * var(--beam-strength, 1));
  ${y}
}

[data-beam="${e}"][data-active]::before,
[data-beam="${e}"][data-fading]::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: ${t}px;
  clip-path: inset(0 round ${t}px);
  background: ${E};
  -webkit-mask-image:
    linear-gradient(white, transparent 28px, transparent calc(100% - 28px), white),
    linear-gradient(to right, white, transparent 28px, transparent calc(100% - 28px), white);
  -webkit-mask-composite: source-over;
  mask-image:
    linear-gradient(white, transparent 28px, transparent calc(100% - 28px), white),
    linear-gradient(to right, white, transparent 28px, transparent calc(100% - 28px), white);
  mask-composite: add;
  pointer-events: none;
  z-index: 1;
  will-change: opacity, filter;
  opacity: calc(var(--beam-opacity-${e}) * ${z} * var(--beam-inner-opacity, 1) * var(--beam-strength, 1));
  ${y}
}

[data-beam="${e}"] [data-beam-bloom] {
  display: none;
  position: absolute;
  inset: 0;
  border-radius: ${t}px;
  clip-path: inset(0 round ${t}px);
  background: ${O};
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  mask-composite: exclude;
  padding: ${a}px;
  pointer-events: none;
  z-index: 3;
  will-change: opacity;
  opacity: 0;
}

[data-beam="${e}"][data-active] [data-beam-bloom],
[data-beam="${e}"][data-fading] [data-beam-bloom] {
  display: block;
  opacity: calc(var(--beam-opacity-${e}) * ${M} * var(--beam-bloom-opacity, 1) * var(--beam-strength, 1));
  ${_}
}

@keyframes beam-fade-in-${e} { to { --beam-opacity-${e}: 1; } }
@keyframes beam-fade-out-${e} { from { --beam-opacity-${e}: 1; } to { --beam-opacity-${e}: 0; } }
${he(e)}

@media (prefers-reduced-motion: reduce) {
  [data-beam="${e}"][data-active],
  [data-beam="${e}"][data-fading],
  [data-beam="${e}"][data-active]::after,
  [data-beam="${e}"][data-fading]::after,
  [data-beam="${e}"][data-active]::before,
  [data-beam="${e}"][data-fading]::before,
  [data-beam="${e}"][data-active] [data-beam-bloom],
  [data-beam="${e}"][data-fading] [data-beam-bloom] {
    animation: none !important;
  }
}
`}function qa(r){const{id:e,borderRadius:t,duration:a,strokeOpacity:s,innerOpacity:o,bloomOpacity:i,colorVariant:c,staticColors:p,brightness:g,saturation:b,hueRange:n,theme:f,hairlineOpacity:u=0,glowSize:h=1}=r,x=f==="dark",d=c==="mono"?.5:1,m=(s*d).toFixed(2),z=(o*d).toFixed(2),M=(i*d).toFixed(2),w=x?"70, 70, 70":"0, 0, 0",v=u.toFixed(2),H=`linear-gradient(rgba(${w}, ${v}), rgba(${w}, ${v}))`,{op:k}=Ze("pulse-outside",f,a),y=.95,_=.9,X=ae(x?3:6,h),E=ae(x?22.5:15,h),O=g.toFixed(2),B=b.toFixed(2),N=p?`filter: brightness(${O}) saturate(${B});`:`filter: hue-rotate(calc(var(--beam-hue-base, 0deg) + var(--beam-hue-${e}))) brightness(${O}) saturate(${B});`,T=`brightness(var(--beam-glow-brightness, ${O})) saturate(var(--beam-glow-saturate, ${B}))`,j=p?`filter: blur(var(--beam-core-blur, ${X}px)) ${T};`:`filter: blur(var(--beam-core-blur, ${X}px)) hue-rotate(calc(var(--beam-hue-base, 0deg) + var(--beam-hue-${e}))) ${T};`,D=p?`filter: blur(var(--beam-bloom-blur, ${E}px)) ${T};`:`filter: blur(var(--beam-bloom-blur, ${E}px)) hue-rotate(calc(var(--beam-hue-base, 0deg) + var(--beam-hue-${e}))) ${T};`,V=yr($r,c,e),U=yr($r,c,e),q=Dr(Ea,c,1-k*.5),K=u>0?`${V},
    ${H}`:V;return`
${Nr(e)}

[data-beam="${e}"] {
  position: relative;
  border-radius: ${t}px;
  overflow: visible;
  isolation: isolate;
}

[data-beam="${e}"][data-active] {
${we(e,"beam-fade-in",.6)}
}

[data-beam="${e}"][data-fading] {
${we(e,"beam-fade-out",.5)}
}
${u>0?`
/* Idle hairline — painted above the (opaque) child in the inner 1px edge ring so
   it overlaps a standard inset component border exactly. */
[data-beam="${e}"]::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: ${t}px;
  padding: 1px;
  clip-path: inset(0 round ${t}px);
  background: ${H};
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  mask-composite: exclude;
  pointer-events: none;
  z-index: 2;
}
`:""}
[data-beam="${e}"][data-active]::after,
[data-beam="${e}"][data-fading]::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: ${t}px;
  padding: 1px;
  clip-path: inset(0 round ${t}px);
  background: ${K};
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  mask-composite: exclude;
  pointer-events: none;
  z-index: 2;
  will-change: opacity, filter;
  opacity: calc(var(--beam-opacity-${e}) * ${m} * var(--beam-stroke-opacity, 1) * var(--beam-strength, 1));
  ${N}
}

[data-beam="${e}"][data-active]::before,
[data-beam="${e}"][data-fading]::before {
  content: "";
  position: absolute;
  inset: -10px;
  z-index: -1;
  border-radius: ${t+10}px;
  background: ${U};
  transform: scale(${y}, ${_});
  pointer-events: none;
  will-change: opacity, filter;
  opacity: calc(var(--beam-opacity-${e}) * ${z} * var(--beam-inner-opacity, 1) * var(--beam-strength, 1));
  ${j}
}

[data-beam="${e}"] [data-beam-bloom] {
  display: none;
  position: absolute;
  inset: -30px;
  z-index: -1;
  border-radius: ${t+30}px;
  background: ${q};
  transform: scale(${y}, ${_});
  pointer-events: none;
  will-change: transform;
  opacity: 0;
}

[data-beam="${e}"][data-active] [data-beam-bloom],
[data-beam="${e}"][data-fading] [data-beam-bloom] {
  display: block;
  opacity: calc(var(--beam-opacity-${e}) * ${M} * var(--beam-bloom-opacity, 1) * var(--beam-strength, 1));
  ${D}
}

@keyframes beam-fade-in-${e} { to { --beam-opacity-${e}: 1; } }
@keyframes beam-fade-out-${e} { from { --beam-opacity-${e}: 1; } to { --beam-opacity-${e}: 0; } }
${he(e)}

@media (prefers-reduced-motion: reduce) {
  [data-beam="${e}"][data-active],
  [data-beam="${e}"][data-fading],
  [data-beam="${e}"][data-active]::after,
  [data-beam="${e}"][data-fading]::after,
  [data-beam="${e}"][data-active]::before,
  [data-beam="${e}"][data-fading]::before,
  [data-beam="${e}"][data-active] [data-beam-bloom],
  [data-beam="${e}"][data-fading] [data-beam-bloom] {
    animation: none !important;
  }
}
`}function Da(r){const{id:e,borderRadius:t,borderWidth:a,duration:s,strokeOpacity:o,innerOpacity:i,bloomOpacity:c,innerShadow:p,colorVariant:g,staticColors:b,brightness:n,saturation:f,hueRange:u,theme:h,glowSize:x=1}=r,d=Math.max(0,t-a),m=h==="dark",z=o,M=i,w=c,v=b?"":`animation: beam-hue-shift-${e} 12s ease-in-out infinite;`,H=b?"":`animation: beam-hue-shift-bloom-${e} 8s ease-in-out infinite;`,k=b?"":`
@keyframes beam-hue-shift-${e} {
  0% { filter: hue-rotate(calc(var(--beam-hue-base, 0deg) - ${u}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
  50% { filter: hue-rotate(calc(var(--beam-hue-base, 0deg) + ${u}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
  100% { filter: hue-rotate(calc(var(--beam-hue-base, 0deg) - ${u}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
}

@keyframes beam-hue-shift-bloom-${e} {
  0% { filter: blur(${ae(8,x)}px) hue-rotate(calc(var(--beam-hue-base, 0deg) - ${u+10}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
  50% { filter: blur(${ae(8,x)}px) hue-rotate(calc(var(--beam-hue-base, 0deg) + ${u+10}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
  100% { filter: blur(${ae(8,x)}px) hue-rotate(calc(var(--beam-hue-base, 0deg) - ${u+10}deg)) brightness(${n.toFixed(2)}) saturate(${f.toFixed(2)}); }
}`,y=m?`radial-gradient(
        ellipse calc(24px * var(--beam-w-${e})) calc(28px * var(--beam-h-${e})) at calc(var(--beam-x-${e}) * 100%) calc(100% + 2px),
        rgba(255, 255, 255, 0.38) 0%,
        rgba(255, 255, 255, 0.12) 30%,
        transparent 65%
      )`:`radial-gradient(
        ellipse calc(35px * var(--beam-w-${e})) calc(28px * var(--beam-h-${e})) at calc(var(--beam-x-${e}) * 100%) calc(100% + 2px),
        rgba(0, 0, 0, 0.6) 0%,
        rgba(0, 0, 0, 0.25) 35%,
        transparent 70%
      )`,_=wa(g,m,e),X=Ya(g,e),E=Xa(g,m,e),O=g==="mono"?"filter: blur(6px);":"";return`
@property --beam-x-${e} {
  syntax: "<number>";
  initial-value: 0;
  inherits: true;
}

@property --beam-w-${e} {
  syntax: "<number>";
  initial-value: 1;
  inherits: true;
}

@property --beam-h-${e} {
  syntax: "<number>";
  initial-value: 1;
  inherits: true;
}

@property --beam-spike-${e} {
  syntax: "<number>";
  initial-value: 1;
  inherits: true;
}

@property --beam-spike2-${e} {
  syntax: "<number>";
  initial-value: 1;
  inherits: true;
}

@property --beam-edge-${e} {
  syntax: "<number>";
  initial-value: 1;
  inherits: true;
}

@property --beam-opacity-${e} {
  syntax: "<number>";
  initial-value: 0;
  inherits: true;
}

[data-beam="${e}"] {
  position: relative;
  border-radius: ${t}px;
  overflow: hidden;
}

[data-beam="${e}"][data-active] {
  animation:
    beam-travel-${e} ${s}s linear infinite,
    beam-edge-fade-${e} ${s}s linear infinite,
    beam-breathe-${e} ${(s*1.3).toFixed(1)}s ease-in-out infinite,
    beam-spike-${e} ${(s*1.33).toFixed(1)}s ease-in-out infinite,
    beam-spike2-${e} ${(s*1.7).toFixed(1)}s ease-in-out infinite,
    beam-fade-in-${e} 0.6s ease forwards;
}

[data-beam="${e}"][data-fading] {
  animation:
    beam-travel-${e} ${s}s linear infinite,
    beam-edge-fade-${e} ${s}s linear infinite,
    beam-breathe-${e} ${(s*1.3).toFixed(1)}s ease-in-out infinite,
    beam-spike-${e} ${(s*1.33).toFixed(1)}s ease-in-out infinite,
    beam-spike2-${e} ${(s*1.7).toFixed(1)}s ease-in-out infinite,
    beam-fade-out-${e} 0.5s ease forwards;
}

[data-beam="${e}"][data-active]::after,
[data-beam="${e}"][data-fading]::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: ${d}px;
  padding: ${a}px;
  clip-path: inset(0 round ${t}px);
  background: ${y}, ${_};
  -webkit-mask:
    radial-gradient(
      ellipse calc(78px * var(--beam-w-${e})) calc(60px * var(--beam-h-${e})) at calc(var(--beam-x-${e}) * 100%) 100%,
      white 0%, rgba(255, 255, 255, 0.5) 45%, transparent 100%
    ),
    linear-gradient(#fff 0 0) content-box,
    linear-gradient(#fff 0 0);
  -webkit-mask-composite: source-in, xor;
  mask:
    radial-gradient(
      ellipse calc(78px * var(--beam-w-${e})) calc(60px * var(--beam-h-${e})) at calc(var(--beam-x-${e}) * 100%) 100%,
      white 0%, rgba(255, 255, 255, 0.5) 45%, transparent 100%
    ),
    linear-gradient(#fff 0 0) content-box,
    linear-gradient(#fff 0 0);
  mask-composite: intersect, exclude;
  pointer-events: none;
  z-index: 2;
  opacity: calc(var(--beam-opacity-${e}) * var(--beam-edge-${e}) * ${z.toFixed(2)} * var(--beam-stroke-opacity, 1) * var(--beam-strength, 1));
  ${v}
}

[data-beam="${e}"][data-active]::before,
[data-beam="${e}"][data-fading]::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: ${t}px;
  background: ${X};
  box-shadow: inset 0 0 9px 1px ${p};
  -webkit-mask-image:
    radial-gradient(
      ellipse calc(78px * var(--beam-w-${e})) calc(60px * var(--beam-h-${e})) at calc(var(--beam-x-${e}) * 100%) 100%,
      white 0%, rgba(255, 255, 255, 0.5) 45%, transparent 100%
    ),
    linear-gradient(white, transparent 28px, transparent calc(100% - 28px), white),
    linear-gradient(to right, white, transparent 28px, transparent calc(100% - 28px), white);
  -webkit-mask-composite: source-in, source-over;
  mask-image:
    radial-gradient(
      ellipse calc(78px * var(--beam-w-${e})) calc(60px * var(--beam-h-${e})) at calc(var(--beam-x-${e}) * 100%) 100%,
      white 0%, rgba(255, 255, 255, 0.5) 45%, transparent 100%
    ),
    linear-gradient(white, transparent 28px, transparent calc(100% - 28px), white),
    linear-gradient(to right, white, transparent 28px, transparent calc(100% - 28px), white);
  mask-composite: intersect, add;
  pointer-events: none;
  z-index: 1;
  opacity: calc(var(--beam-opacity-${e}) * var(--beam-edge-${e}) * ${M.toFixed(2)} * var(--beam-inner-opacity, 1) * var(--beam-strength, 1));
  clip-path: inset(0 round ${t}px);
  ${v}
}

[data-beam="${e}"] [data-beam-bloom] {
  display: none;
  position: absolute;
  inset: 0;
  border-radius: ${d}px;
  clip-path: inset(0 round ${t}px);
  padding: 0;
  -webkit-mask: radial-gradient(
    ellipse calc(84px * var(--beam-w-${e})) calc(110px * var(--beam-h-${e})) at calc(var(--beam-x-${e}) * 100%) 100%,
    white 0%, rgba(255, 255, 255, 0.5) 35%, transparent 100%
  );
  -webkit-mask-composite: source-over;
  mask: radial-gradient(
    ellipse calc(84px * var(--beam-w-${e})) calc(110px * var(--beam-h-${e})) at calc(var(--beam-x-${e}) * 100%) 100%,
    white 0%, rgba(255, 255, 255, 0.5) 35%, transparent 100%
  );
  mask-composite: add;
  background: ${E};
  ${O}
  pointer-events: none;
  z-index: 3;
  opacity: 0;
}

[data-beam="${e}"][data-active] [data-beam-bloom],
[data-beam="${e}"][data-fading] [data-beam-bloom] {
  display: block;
  opacity: calc(var(--beam-opacity-${e}) * var(--beam-edge-${e}) * ${w.toFixed(2)} * var(--beam-bloom-opacity, 1) * var(--beam-strength, 1));
  ${H}
}

@keyframes beam-travel-${e} {
  0%   { --beam-x-${e}: 0.06;  --beam-w-${e}: 0.5; }
  10%  { --beam-x-${e}: 0.15;  --beam-w-${e}: 0.8; }
  20%  { --beam-x-${e}: 0.25;  --beam-w-${e}: 1.1; }
  30%  { --beam-x-${e}: 0.35;  --beam-w-${e}: 1.3; }
  40%  { --beam-x-${e}: 0.44;  --beam-w-${e}: 1.45; }
  50%  { --beam-x-${e}: 0.5;   --beam-w-${e}: 1.5; }
  60%  { --beam-x-${e}: 0.56;  --beam-w-${e}: 1.45; }
  70%  { --beam-x-${e}: 0.65;  --beam-w-${e}: 1.3; }
  80%  { --beam-x-${e}: 0.75;  --beam-w-${e}: 1.1; }
  90%  { --beam-x-${e}: 0.85;  --beam-w-${e}: 0.8; }
  100% { --beam-x-${e}: 0.94;  --beam-w-${e}: 0.5; }
}

@keyframes beam-edge-fade-${e} {
  0%    { --beam-edge-${e}: 0; }
  12.5% { --beam-edge-${e}: 0; }
  32.5% { --beam-edge-${e}: 1; }
  67.5% { --beam-edge-${e}: 1; }
  87.5% { --beam-edge-${e}: 0; }
  100%  { --beam-edge-${e}: 0; }
}

@keyframes beam-breathe-${e} {
  0%, 100% { --beam-h-${e}: 0.8; }
  25%      { --beam-h-${e}: 1.25; }
  55%      { --beam-h-${e}: 0.85; }
  80%      { --beam-h-${e}: 1.3; }
}

@keyframes beam-spike-${e} {
  0%   { --beam-spike-${e}: 0.8; }
  25%  { --beam-spike-${e}: 1.3; }
  50%  { --beam-spike-${e}: 0.9; }
  75%  { --beam-spike-${e}: 1.4; }
  100% { --beam-spike-${e}: 0.8; }
}

@keyframes beam-spike2-${e} {
  0%   { --beam-spike2-${e}: 1.2; }
  25%  { --beam-spike2-${e}: 0.7; }
  50%  { --beam-spike2-${e}: 1.4; }
  75%  { --beam-spike2-${e}: 0.8; }
  100% { --beam-spike2-${e}: 1.2; }
}

@keyframes beam-fade-in-${e} {
  to { --beam-opacity-${e}: 1; }
}

@keyframes beam-fade-out-${e} {
  from { --beam-opacity-${e}: 1; }
  to { --beam-opacity-${e}: 0; }
}
${k}
${he(e)}
`}const He=new Set;let ge=null,Ne=0;const Na=1e3/30-2,La=Math.PI*2;function zr(r){return(1-Math.cos(La*r))/2}function Lr(r){if(ge=requestAnimationFrame(Lr),r-Ne<Na)return;Ne=r;const e=r/1e3;He.forEach(({el:t,config:a})=>{for(const s of a.oscillators){const o=(e-s.delay)/s.period,i=s.a+(s.b-s.a)*zr(o);t.style.setProperty(s.prop,s.unit==="px"?`${i.toFixed(2)}px`:i.toFixed(4))}if(a.hue){const{prop:s,range:o,period:i,continuous:c}=a.hue,p=c?e/i%1*o:-o+2*o*zr(e/i);t.style.setProperty(s,`${p.toFixed(2)}deg`)}})}function Ba(){ge==null&&(Ne=0,ge=requestAnimationFrame(Lr))}function Ua(){He.size===0&&ge!=null&&(cancelAnimationFrame(ge),ge=null)}function Va(r,e){const t={el:r,config:e};return He.add(t),Ba(),()=>{He.delete(t),Ua()}}function Ga(){const[r,e]=L.useState(()=>typeof window>"u"||window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light");return L.useEffect(()=>{if(typeof window>"u")return;const t=window.matchMedia("(prefers-color-scheme: dark)"),a=s=>{e(s.matches?"dark":"light")};return t.addEventListener("change",a),()=>t.removeEventListener("change",a)},[]),r}function Za(r,e){return r==="auto"?e:r}const r0=L.forwardRef(function({children:r,size:e="md",colorVariant:t="colorful",theme:a="dark",staticColors:s=!1,duration:o,active:i=!0,borderRadius:c,brightness:p,saturation:g,hueRange:b=30,glowSize:n=1,strength:f=1,className:u,style:h,css:x,onActivate:d,onDeactivate:m,onAnimationEnd:z,...M},w){const v=L.useId().replace(/:/g,"-"),H=Ga(),k=L.useRef(null),[y,_]=L.useState(i),[X,E]=L.useState(!1),[O,B]=L.useState(!0),[N,T]=L.useState(null),[j,D]=L.useState({x:1,y:1});L.useEffect(()=>{if(c!=null)return;const F=k.current;if(!F)return;const J=()=>{const be=F.firstElementChild;if(!be)return;const Ye=getComputedStyle(be),ue=parseFloat(Ye.borderTopLeftRadius);!isNaN(ue)&&ue>0&&T(ue)};J();const fe=new MutationObserver(J);return fe.observe(F,{childList:!0,subtree:!1}),()=>fe.disconnect()},[c,r]),L.useEffect(()=>{i&&!y&&!X?_(!0):!i&&y&&!X&&E(!0)},[i,y,X]),L.useEffect(()=>{const F=k.current;if(!F||typeof IntersectionObserver>"u")return;const J=new IntersectionObserver(fe=>{for(const be of fe)B(be.isIntersecting)},{rootMargin:"256px"});return J.observe(F),()=>J.disconnect()},[]),L.useEffect(()=>{if(e!=="pulse-outside"){D({x:1,y:1});return}const F=k.current;if(!F)return;const J=350,fe=140,be=.35,Ye=4,ue=$e=>Math.max(be,Math.min(Ye,$e)),Qe=()=>{const $e=F.firstElementChild;if(!$e)return;const ye=$e.getBoundingClientRect();if(!ye.width||!ye.height)return;const er=+ue(ye.width/J).toFixed(3),rr=+ue(ye.height/fe).toFixed(3);D(We=>We.x===er&&We.y===rr?We:{x:er,y:rr})};if(Qe(),typeof ResizeObserver>"u")return;const Je=F.firstElementChild;if(!Je)return;const Ke=new ResizeObserver(Qe);return Ke.observe(Je),()=>Ke.disconnect()},[e,r]);const V=L.useCallback(F=>{const J=F.animationName;J.includes("fade-out")?(_(!1),E(!1),m?.()):J.includes("fade-in")&&d?.(),z?.(F)},[d,m,z]),U=Za(a,H),q=ha[e][U],K=xa[e],te=e==="pulse-inner"||e==="pulse-outside",oe=c??N??K.borderRadius,l=o??(e==="line"?3.1:te?2.3:1.96),$=g??q.saturation,Y=p??q.brightness??1.3,W=e==="line"?Math.min(b,13):b,S=t==="mono"?!0:s,P=L.useMemo(()=>Aa({id:v,borderRadius:oe,borderWidth:K.borderWidth,duration:l,strokeOpacity:q.strokeOpacity,innerOpacity:q.innerOpacity,bloomOpacity:q.bloomOpacity,innerShadow:q.innerShadow,size:e,colorVariant:t,staticColors:S,brightness:Y,saturation:$,hueRange:W,theme:U,hairlineOpacity:q.hairlineOpacity,glowSize:n}),[v,oe,K.borderWidth,l,q.strokeOpacity,q.innerOpacity,q.bloomOpacity,q.innerShadow,q.hairlineOpacity,e,t,S,Y,$,W,n,U]),C=L.useMemo(()=>te?Ca(e,U,l,W,S,v):null,[te,e,U,l,W,S,v]);L.useEffect(()=>{var F;if(!C||!(y||X)||!O)return;const J=k.current;if(J&&!(typeof window<"u"&&(F=window.matchMedia)!=null&&F.call(window,"(prefers-reduced-motion: reduce)").matches))return Va(J,C)},[C,y,X,O]);const ee=L.useCallback(F=>{k.current=F,typeof w=="function"?w(F):w&&(w.current=F)},[w]),Z={...h??{},"--beam-strength":Math.max(0,Math.min(1,f)),...e==="pulse-outside"?{"--pulse-glow-sx":j.x,"--pulse-glow-sy":j.y}:{}};return xe.jsxs(xe.Fragment,{children:[xe.jsx("style",{children:x?`${P}
${x.split("{id}").join(v)}`:P}),xe.jsxs("div",{...M,ref:ee,"data-beam":v,"data-active":y&&!X?"":void 0,"data-fading":X?"":void 0,"data-paused":y&&!X&&!O?"":void 0,className:u,style:Z,onAnimationEnd:V,children:[r,xe.jsx("div",{"data-beam-bloom":!0})]})]})});var Ae={exports:{}},G={};var vr;function Qa(){if(vr)return G;vr=1;var r=jr();function e(b){var n="https://react.dev/errors/"+b;if(1<arguments.length){n+="?args[]="+encodeURIComponent(arguments[1]);for(var f=2;f<arguments.length;f++)n+="&args[]="+encodeURIComponent(arguments[f])}return"Minified React error #"+b+"; visit "+n+" for the full message or use the non-minified dev environment for full errors and additional helpful warnings."}function t(){}var a={d:{f:t,r:function(){throw Error(e(522))},D:t,C:t,L:t,m:t,X:t,S:t,M:t},p:0,findDOMNode:null},s=Symbol.for("react.portal"),o=Symbol.for("react.recoverable"),i=Symbol.for("react.optimistic_key");function c(b,n,f){var u=3<arguments.length&&arguments[3]!==void 0?arguments[3]:null;return{$$typeof:s,key:u==null?null:u===i?i:""+u,children:b,containerInfo:n,implementation:f}}var p=r.__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE;function g(b,n){if(b==="font")return"";if(typeof n=="string")return n==="use-credentials"?n:""}return G.__DOM_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE=a,G.browser=function(b){return{$$typeof:o,_reason:b}},G.createPortal=function(b,n){var f=2<arguments.length&&arguments[2]!==void 0?arguments[2]:null;if(!n||n.nodeType!==1&&n.nodeType!==9&&n.nodeType!==11)throw Error(e(299));return c(b,n,null,f)},G.flushSync=function(b){var n=p.T,f=a.p;try{if(p.T=null,a.p=2,b)return b()}finally{p.T=n,a.p=f,a.d.f()}},G.preconnect=function(b,n){typeof b=="string"&&(n?(n=n.crossOrigin,n=typeof n=="string"?n==="use-credentials"?n:"":void 0):n=null,a.d.C(b,n))},G.prefetchDNS=function(b){typeof b=="string"&&a.d.D(b)},G.preinit=function(b,n){if(typeof b=="string"&&n&&typeof n.as=="string"){var f=n.as,u=g(f,n.crossOrigin),h=typeof n.integrity=="string"?n.integrity:void 0,x=typeof n.fetchPriority=="string"?n.fetchPriority:void 0;f==="style"?a.d.S(b,typeof n.precedence=="string"?n.precedence:void 0,{crossOrigin:u,integrity:h,fetchPriority:x}):f==="script"&&a.d.X(b,{crossOrigin:u,integrity:h,fetchPriority:x,nonce:typeof n.nonce=="string"?n.nonce:void 0})}},G.preinitModule=function(b,n){if(typeof b=="string")if(typeof n=="object"&&n!==null){if(n.as==null||n.as==="script"){var f=g(n.as,n.crossOrigin);a.d.M(b,{crossOrigin:f,integrity:typeof n.integrity=="string"?n.integrity:void 0,nonce:typeof n.nonce=="string"?n.nonce:void 0,fetchPriority:typeof n.fetchPriority=="string"?n.fetchPriority:void 0})}}else n==null&&a.d.M(b)},G.preload=function(b,n){if(typeof b=="string"&&typeof n=="object"&&n!==null&&typeof n.as=="string"){var f=n.as,u=g(f,n.crossOrigin);a.d.L(b,f,{crossOrigin:u,integrity:typeof n.integrity=="string"?n.integrity:void 0,nonce:typeof n.nonce=="string"?n.nonce:void 0,type:typeof n.type=="string"?n.type:void 0,fetchPriority:typeof n.fetchPriority=="string"?n.fetchPriority:void 0,referrerPolicy:typeof n.referrerPolicy=="string"?n.referrerPolicy:void 0,imageSrcSet:typeof n.imageSrcSet=="string"?n.imageSrcSet:void 0,imageSizes:typeof n.imageSizes=="string"?n.imageSizes:void 0,media:typeof n.media=="string"?n.media:void 0})}},G.preloadModule=function(b,n){if(typeof b=="string")if(n){var f=g(n.as,n.crossOrigin);a.d.m(b,{as:typeof n.as=="string"&&n.as!=="script"?n.as:void 0,crossOrigin:f,integrity:typeof n.integrity=="string"?n.integrity:void 0,nonce:typeof n.nonce=="string"?n.nonce:void 0,fetchPriority:typeof n.fetchPriority=="string"?n.fetchPriority:void 0})}else a.d.m(b)},G.requestFormReset=function(b){a.d.r(b)},G.unstable_batchedUpdates=function(b,n){return b(n)},G.useFormState=function(b,n,f){return p.H.useFormState(b,n,f)},G.useFormStatus=function(){return p.H.useHostTransitionStatus()},G.version="19.3.0",G}var Mr;function t0(){if(Mr)return Ae.exports;Mr=1;function r(){if(!(typeof __REACT_DEVTOOLS_GLOBAL_HOOK__>"u"||typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE!="function"))try{__REACT_DEVTOOLS_GLOBAL_HOOK__.checkDCE(r)}catch(e){console.error(e)}}return r(),Ae.exports=Qa(),Ae.exports}export{Ka as B,fa as M,r0 as W,jr as a,t0 as b,L as c,qo as p,e0 as r};
