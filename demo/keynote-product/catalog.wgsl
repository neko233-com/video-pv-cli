// All 48 original transition algorithms execute as native WGSL.
// @include "library.wgsl"
struct Params {
 resolution:vec2f,time:f32,progress:f32,
 mode:f32,strength:f32,softness:f32,tiles:f32,
 direction:vec2f,bgA:f32,bgB:f32,
 base:vec4f,accent:vec4f,secondary:vec4f,
 loopSeconds:f32,padding0:f32,padding1:f32,padding2:f32,
}
@group(0) @binding(0) var<uniform> u:Params;
@group(0) @binding(1) var texSampler:sampler;
@group(0) @binding(2) var fromTexture:texture_2d<f32>;
@group(0) @binding(3) var toTexture:texture_2d<f32>;
var<private> backdropA:vec3f;
var<private> backdropB:vec3f;
const PI=3.14159265359;
@vertex fn vertexMain(@builtin(vertex_index) index:u32)->@builtin(position) vec4f {
 let positions=array<vec2f,3>(vec2f(-1,-1),vec2f(3,-1),vec2f(-1,3));return vec4f(positions[index],0,1);
}
fn background(uv:vec2f,kind:f32)->vec3f {
 var time=u.time*.25;var drift=vec2f(time*.1,0);
 if(u.loopSeconds>0){let angle=u.time*2*PI/u.loopSeconds;time=sin(angle);drift=vec2f(cos(angle),sin(angle));}
 let p=(uv-.5)*vec2f(u.resolution.x/u.resolution.y,1);var f=0.;let k=i32(kind+.5);
 if(k==1){f=.1+.5*fbm(p*2+drift*.3);}
 else if(k==2){f=pow(max(0.,1-abs(p.y+sin(p.x*3+time)*.17)*4),3.)*.5;}
 else if(k==3){f=pow(fbm(p*3+drift*.3),2.)*.65;}
 else if(k==4){f=(1-smoothstep(.01,.025,min(abs(fract(p.x*12)-.5),abs(fract(p.y*12)-.5))))*.12;}
 else if(k==5){let q=p*40;f=step(.975,hash21(floor(q)))*exp(-length(fract(q)-.5)*12)*(.7+.3*sin(time+hash21(floor(q))*PI*2));}
 else if(k==6){f=exp(-abs(length(p-vec2f(.35,0))-.31-.015*sin(atan2(p.y,p.x)*8+time))*55)*.6;}
 else if(k==7){f=pow(1-abs(sin((p.x+p.y)*6+fbm(p*4+drift*.3)*4)),6.)*.3;}
 else if(k==8){f=fbm(rotate(time*.2)*p*3+drift*.2)*.45;}
 else if(k==9){f=smoothstep(.3,.8,sin(p.x*2+p.y+time))*.24;}
 else if(k==10){f=exp(-abs(p.y+.15)*17)*.3;}
 else if(k==11){f=pow(.5+.5*sin(p.x*7+sin(p.y*2+time)*3),7.)*.28;}
 return mix(u.base.rgb,mix(u.accent.rgb,u.secondary.rgb,uv.x),clamp(f,0.,.8));
}
fn A(uv:vec2f)->vec3f {var c=textureSampleLevel(fromTexture,texSampler,clamp(uv,vec2f(0),vec2f(1)),0);if(any(uv<vec2f(0))||any(uv>vec2f(1))){c=vec4f(0);}return mix(backdropA,c.rgb,c.a);}
fn B(uv:vec2f)->vec3f {var c=textureSampleLevel(toTexture,texSampler,clamp(uv,vec2f(0),vec2f(1)),0);if(any(uv<vec2f(0))||any(uv>vec2f(1))){c=vec4f(0);}return mix(backdropB,c.rgb,c.a);}
fn reveal(field:f32,p:f32)->f32{return smoothstep(field-u.softness,field+u.softness,p);}
@fragment fn fragmentMain(@builtin(position) position:vec4f)->@location(0) vec4f {
 let uv=position.xy/u.resolution;backdropA=background(uv,u.bgA);backdropB=background(uv,u.bgB);
 let p=clamp(u.progress,0.,1.);var a=A(uv);var b=B(uv);var c:vec3f;
 if(p<=0){return vec4f(a,1);}if(p>=1){return vec4f(b,1);}
 var dir=vec2f(1,0);if(length(u.direction)>0){dir=normalize(u.direction);}
 let centre=uv-.5;let edge=p*(1-p);let m=i32(u.mode+.5);var mask=p;
 if(m==0){mask=step(.5,p);}
 else if(m==1){mask=p;}
 else if(m==2||m==3){let flash=select(vec3f(0),vec3f(1),m==3);if(p<.5){c=mix(a,flash,p*2);}else{c=mix(flash,b,(p-.5)*2);}return vec4f(c,1);}
 else if(m==35){return vec4f(clamp(mix(a,b,p)*(1+sin(p*PI)*2),vec3f(0),vec3f(1)),1);}
 else if(m==4){let q=uv+dir*(1-p);mask=select(0.,1.,all(q>=vec2f(0))&&all(q<=vec2f(1)));b=B(q);}
 else if(m==5){let qa=uv-dir*p;let qb=uv+dir*(1-p);mask=select(0.,1.,all(qb>=vec2f(0))&&all(qb<=vec2f(1)));a=A(qa);b=B(qb);}
 else if(m==6||m==7){let f=dot(uv-.5,dir)/1.42+.5;mask=reveal(f,p);if(m==7){a=A(uv-dir*p*.15);}else{b=B(uv+dir*(1-p)*.1);}}
 else if(m==8){a=A(centre*(1+p*1.5)+.5);b=B(centre/(.5+.5*p)+.5);mask=smoothstep(.15,.85,p);}
 else if(m==9){a=A(centre/(1-.65*p)+.5);b=B(centre*(1.5-.5*p)+.5);mask=smoothstep(.15,.85,p);}
 else if(m==10){mask=reveal(length(centre)*1.42,p);}
 else if(m==29){mask=reveal(mix(max(abs(centre.x),abs(centre.y))*2,length(centre)*1.42,p),p);}
 else if(m==11){mask=reveal(abs(centre.x)+abs(centre.y),p);}
 else if(m==12){mask=reveal((uv.x+uv.y)*.5,p);}
 else if(m==13){mask=reveal(fract(uv.x*u.tiles),p);}
 else if(m==14){let cell=floor(uv*u.tiles);mask=reveal(pmod(cell.x+cell.y,2)*.45+fract(uv.y*u.tiles)*.5,p);}
 else if(m==15){let cell=1+edge*100*u.strength;let q=floor(uv*u.resolution/cell)*cell/u.resolution;a=A(q);b=B(q);mask=p;}
 else if(m==16){mask=reveal((atan2(centre.y,centre.x)+PI)/(2*PI),p);}
 else if(m==17){let offset=normalize(centre+.00001)*sin(length(centre)*60-p*14)*edge*.05*u.strength;a=A(uv+offset);b=B(uv-offset);mask=p;}
 else if(m==18){a=A(rotate(p*edge*6)*centre+.5);b=B(rotate(-(1-p)*edge*6)*centre+.5);mask=p;}
 else if(m==19){let x=(hash21(vec2f(floor(uv.y*45),floor(p*20)))-.5)*edge*.4;a=A(uv+vec2f(x,0));b=B(uv-vec2f(x,0));mask=reveal(uv.x+x,p);}
 else if(m==20){let f=uv.y*.5+fbm(uv*6)*.5;mask=reveal(f,p);a+=u.accent.rgb*exp(-abs(f-p)*90)*edge*2;}
 else if(m==21){mask=reveal(fbm(uv*10),p);}
 else if(m==22){let offset=vec2f(fbm(uv*4),fbm(uv.yx*5))-.5;a=A(uv+offset*p*.3);b=B(uv-offset*(1-p)*.3);mask=p;}
 else if(m==23||m==45){let offset=dir*edge*.08*u.strength;a=vec3f(A(uv+offset).r,A(uv).g,A(uv-offset).b);b=vec3f(B(uv-offset).r,B(uv).g,B(uv+offset).b);if(m==45){mask=reveal(uv.x+sin(uv.y*8)*edge*.1,p);}else{mask=p;}}
 else if(m==24){var aa=vec3f(0);var bb=vec3f(0);for(var i=-4;i<=4;i++){let off=dir*f32(i)*edge*.02;aa+=A(uv+off)/9;bb+=B(uv+off)/9;}a=aa;b=bb;mask=p;}
 else if(m==25){a=A(rotate(p*PI)*centre*(1+p)+.5);b=B(rotate((p-1)*PI)*centre*(2-p)+.5);mask=p;}
 else if(m==26){mask=reveal(abs(uv.x-.5)*2,p);}
 else if(m==43){mask=reveal(abs(uv.y-.5)*2,p);}
 else if(m==27){let side=select(-1.,1.,uv.y>.5);a=A(uv+vec2f(side*p,0));b=B(uv);mask=smoothstep(.1,.8,p);}
 else if(m==28){mask=reveal(dot(a,vec3f(.2126,.7152,.0722)),p);}
 else if(m==30){mask=reveal(uv.x*.65+floor(uv.y*7)/7*.35,p);}
 else if(m==31){let f=fract(uv.y*u.tiles);mask=reveal(f,p);a=A(vec2f(uv.x,uv.y+sin(p*PI)*.01));}
 else if(m==32){let distance=abs(centre.x)*2;mask=reveal(distance,p);a=A(vec2f(.5+centre.x/max(.01,1-p),uv.y));}
 else if(m==33){a=A(vec2f(.5+centre.x/(1-.7*p),uv.y+centre.x*p*.3));b=B(vec2f(.5+centre.x/(.3+.7*p),uv.y-centre.x*(1-p)*.3));mask=p;}
 else if(m==34){let fold=p+sin(uv.y*PI)*edge*.2;mask=reveal(1-uv.x,fold);a*=.6+.4*abs(uv.x-p);}
 else if(m==36){let field=uv.x+uv.y*.2;mask=reveal(field/1.2,p);a+=u.accent.rgb*exp(-abs(field-p*1.2)*60)*edge*2;}
 else if(m==37){mask=reveal(clamp(length(centre)+fbm(uv*8)*.4,0.,1.),p);}
 else if(m==38){let cell=floor(uv*u.tiles);mask=reveal(hash21(cell)*.75+length(fract(uv*u.tiles)-.5)*.25,p);}
 else if(m==39){var q=uv*u.tiles;q.x+=pmod(floor(q.y),2)*.5;mask=reveal(hash21(floor(q))*.7+max(abs(fract(q).x-.5)*.866+abs(fract(q).y-.5)*.5,abs(fract(q).y-.5))*.3,p);}
 else if(m==40){mask=reveal(uv.x+sin(uv.y*PI*4+p*PI)*edge*.3,p);}
 else if(m==41){mask=reveal(length(uv-vec2f(-.3,.5))/1.4,p);}
 else if(m==42){mask=smoothstep(uv.x-.24,uv.x+.24,p*1.48-.24);}
 else if(m==44){let k=sin(p*PI)*.25;a=A(centre*(1+k*dot(centre,centre)*3)+.5);b=B(centre*(1-k*dot(centre,centre)*2)+.5);mask=p;}
 else if(m==46){mask=reveal(length(fract(uv*u.tiles)-.5)*1.42,p);}
 else if(m==47){let band=sin(p*PI);a=A(vec2f(uv.x,uv.y+p));b=B(vec2f(uv.x,uv.y-(1-p)));mask=step(1-p,uv.y);a*=1-.3*band;}
 c=mix(a,b,clamp(mask,0.,1.));return vec4f(clamp(c,vec3f(0),vec3f(1)),1);
}
