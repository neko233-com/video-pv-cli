// Shared renderer. Every scene, transition choice and visible string comes from JSON.
import {createGPUShader} from './webgpu-runtime.js';
const backgrounds=['solid','mesh','aurora','nebula','grid','particles','rings','caustics','fluid','prism','horizon','silk'];
const clamp=x=>Math.max(0,Math.min(1,x));
function ease(x,name){x=clamp(x);if(name==='linear')return x;if(name==='outExpo')return x===1?1:1-Math.pow(2,-10*x);if(name==='inOutQuint')return x<.5?16*x**5:1-(-2*x+2)**5/2;if(name==='spring')return 1-Math.exp(-7*x)*Math.cos(10*x);if(name==='settle')return 1-(1-x)**4;if(name==='swift')return x*x*(3-2*x);return x<.5?4*x*x*x:1-(-2*x+2)**3/2;}
function rgb(hex){return [1,3,5].map(i=>parseInt(hex.slice(i,i+2),16)/255);}
async function localJSON(path){const url=new URL(path,location.href);if(url.origin!==location.origin)throw new Error('PV data must be local');const response=await fetch(url);if(!response.ok)throw new Error(`Missing PV data: ${path}`);return response.json();}
function schedule(template){let start=0;const rows=template.scenes.map((scene,index)=>{const row={scene,start,end:start+scene.duration};start+=scene.duration-(index<template.scenes.length-1?scene.transition_duration:0);return row;});return {rows,duration:rows.at(-1).end};}

export async function createCatalogPlayer(canvas){
 const config=await localJSON('./template.json'),transitions=await localJSON('./transitions.json');
 const templates=config.kind==='atlas'?config.cases.map(item=>item.template):[config];
 if(!templates.length)throw new Error('No templates to render');
 const images=new Map();
 for(const template of templates)for(const scene of template.scenes)if(scene.image&&!images.has(scene.image)){
  const url=new URL(scene.image,location.href);if(url.origin!==location.origin)throw new Error('Images must be local');const image=new Image();image.src=url.href;await image.decode();images.set(scene.image,image);
 }
 const width=config.width,height=config.height;document.getElementById('stage').style.width=`${width}px`;document.getElementById('stage').style.height=`${height}px`;
 const a=new OffscreenCanvas(Math.round(width*devicePixelRatio),Math.round(height*devicePixelRatio)),b=new OffscreenCanvas(a.width,a.height);
 for(const target of [a,b]){const ctx=target.getContext('2d',{willReadFrequently:true});ctx.clearRect(0,0,target.width,target.height);}
 const shader=await createGPUShader(canvas,'./catalog.wgsl',{textures:{u_from:a,u_to:b}});
 globalThis.__PV_WEBGPU_FLUSH=()=>shader.flush();
 const layouts=new Map(templates.map(t=>[t,schedule(t)]));
 const title=(text,template)=>text.replaceAll('{{title}}',template.project_title);
 function font(template,size,weight=500){return `${weight} ${size}px ${template.theme.font==='display'?'PVDisplay':'PVSans'}`;}
 function round(ctx,x,y,w,h,r,fill){ctx.fillStyle=fill;ctx.beginPath();ctx.roundRect(x,y,w,h,r);ctx.fill();}
 function text(ctx,value,x,y,size,template,color,align='left',width=1500,weight=500){
  ctx.font=font(template,size,weight);ctx.fillStyle=color;ctx.textAlign=align;ctx.textBaseline='top';
  const raw=title(value,template),words=/[\u3400-\u9fff]/.test(raw)?Array.from(raw):raw.split(' ');const join=/[\u3400-\u9fff]/.test(raw)?'':' ';let line='',row=0;
  for(const word of words){if(word.includes('\n')){const parts=word.split('\n');for(let i=0;i<parts.length;i++){if(parts[i])line+=(line?join:'')+parts[i];if(i<parts.length-1){ctx.fillText(line,x,y+row*size*1.16);row++;line='';}}continue;}
   const next=line?line+join+word:word;if(ctx.measureText(next).width>width&&line){ctx.fillText(line,x,y+row*size*1.16);row++;line=word;}else line=next;
  }ctx.fillText(line,x,y+row*size*1.16);return (row+1)*size*1.16;
 }
 function image(ctx,path,x,y,w,h,zoom=1){const img=images.get(path);if(!img)return;ctx.save();ctx.beginPath();ctx.rect(x,y,w,h);ctx.clip();const scale=Math.max(w/img.width,h/img.height)*zoom;ctx.drawImage(img,x+(w-img.width*scale)/2,y+(h-img.height*scale)/2,img.width*scale,img.height*scale);ctx.restore();}
 function device(ctx,x,y,w,h,t,template,tilt=0){ctx.save();ctx.translate(x+w/2,y+h/2);ctx.transform(1,tilt,.05*Math.sin(t*.4),1,0,0);ctx.shadowColor='#0006';ctx.shadowBlur=45;ctx.shadowOffsetY=26;
  const metal=ctx.createLinearGradient(-w/2,0,w/2,0);metal.addColorStop(0,'#343b48');metal.addColorStop(.06,'#e4e9ee');metal.addColorStop(.11,'#5c6370');metal.addColorStop(.9,'#262d39');metal.addColorStop(1,'#d2dce9');round(ctx,-w/2,-h/2,w,h,42,metal);ctx.shadowColor='transparent';
  const screen=ctx.createLinearGradient(-w/2,-h/2,w/2,h/2);screen.addColorStop(0,template.theme.accent);screen.addColorStop(.5,template.theme.background);screen.addColorStop(1,template.theme.secondary);round(ctx,-w/2+9,-h/2+9,w-18,h-18,34,screen);
  ctx.strokeStyle=template.theme.foreground+'88';ctx.lineWidth=1.5;for(let i=0;i<4;i++){ctx.beginPath();ctx.ellipse(0,0,w*.25+i*13,h*.20+i*15,t*.15+i*.2,0,Math.PI*2);ctx.stroke();}
  round(ctx,-34,-h/2+20,68,9,5,'#131820');ctx.restore();
 }
 function grid(ctx,scene,template,x,y,w,h,columns=2,progress=1){const gap=22,rows=Math.ceil(scene.items.length/columns),cw=(w-gap*(columns-1))/columns,ch=(h-gap*(rows-1))/rows;
  scene.items.forEach((item,index)=>{ctx.save();const entry=scene.motion.name==='stagger'?ease(clamp(progress*5-index*.25),'settle'):1;ctx.globalAlpha=entry;const px=x+index%columns*(cw+gap),py=y+Math.floor(index/columns)*(ch+gap)+(1-entry)*24;round(ctx,px,py,cw,ch,30,template.theme.surface);text(ctx,item.value,px+32,py+26,58,template,template.theme.accent,'left',cw-64,650);text(ctx,item.title,px+32,py+108,27,template,template.theme.foreground,'left',cw-64,650);text(ctx,item.body,px+32,py+153,19,template,template.theme.foreground,'left',cw-64);ctx.restore();});
 }
 function paint(target,scene,local,template){
  // Reset the complete 2D drawing state and backing store for arbitrary time jumps.
  target.width=target.width;
  const ctx=target.getContext('2d',{willReadFrequently:true});ctx.clearRect(0,0,target.width,target.height);ctx.save();ctx.scale(target.width/1920,target.height/1080);
  const theme=template.theme,progress=clamp(local/scene.duration),e=ease(progress,scene.motion.ease),k=scene.motion.strength;
  const animationTime=template.loop?Math.sin(progress*Math.PI*2):local;
  let sx=1,tx=0,ty=0,angle=0;const periodic=template.loop?Math.sin(progress*Math.PI*2):e;
  switch(scene.motion.name){case'push-in':sx=1+.045*k*periodic;break;case'pull-out':sx=1+.045*k*(template.loop?periodic:1-e);break;case'drift-left':tx=-35*k*periodic;break;case'drift-right':tx=35*k*periodic;break;case'drift-up':ty=-24*k*periodic;break;case'orbit':tx=Math.sin(progress*Math.PI*2)*12*k;ty=Math.cos(progress*Math.PI*2)*10*k;break;case'float':ty=Math.sin(progress*Math.PI*2)*14*k;break;case'tilt':angle=(template.loop?periodic:e-.5)*.018*k;break;case'rise':ty=(template.loop?periodic:1-e)*32*k;break;}
  ctx.translate(960+tx,540+ty);ctx.rotate(angle);ctx.scale(sx,sx);ctx.translate(-960,-540);
  const kind=scene.type,white=theme.foreground;const center=scene.layout==='center';
  if(scene.image){
   if(['gallery','film-strip','zoom-gallery','profile-grid','split-screen','action-panel'].includes(kind)){}else{image(ctx,scene.image,0,0,1920,1080,kind==='character-detail'?1.3:1.02+.025*periodic);const shade=ctx.createLinearGradient(0,0,1450,0);shade.addColorStop(0,theme.background+'ee');shade.addColorStop(.65,theme.background+'66');shade.addColorStop(1,'#00000000');ctx.fillStyle=shade;ctx.fillRect(0,0,1920,1080);}
  }
  const over=(x=128,y=138)=>text(ctx,scene.overline,x,y,19,template,theme.accent,'left',1500,600);
  const heading=(x=128,y=365,size=108,align='left',w=1200)=>text(ctx,scene.title,x,y,size,template,white,align,w,600);
  const subtitle=(x=128,y=720,w=1200)=>text(ctx,scene.subtitle,x,y,26,template,white,'left',w,400);
  if(['title','endcard','giant-type','kinetic-type','quote','chapter','impact','countdown'].includes(kind)){
   const align=kind==='giant-type'||kind==='quote'||kind==='countdown'||center?'center':'left',x=align==='center'?960:128;over();
   if(kind==='countdown')text(ctx,String(Math.max(1,3-Math.floor(progress*3))),960,250,340,template,theme.accent,'center',1600,650);
   else if(kind==='kinetic-type'){const words=title(scene.title,template).split(' ');words.forEach((word,i)=>{ctx.globalAlpha=clamp((progress-i*.12)*6);text(ctx,word,128+i*100,260+i*145,150,template,i%2?theme.accent:white,'left',1500,650);});ctx.globalAlpha=1;}
   else{if(kind==='impact'){ctx.shadowColor=theme.accent;ctx.shadowBlur=20*Math.sin(progress*Math.PI);}heading(x,kind==='quote'?350:330,kind==='giant-type'?215:kind==='quote'?82:118,align,align==='center'?1600:1500);ctx.shadowColor='transparent';}
   if(kind==='chapter'){ctx.fillStyle=theme.accent;ctx.fillRect(128,265,90,3);}subtitle(align==='center'?400:128,790,1400);
  }else if(['logo','monogram'].includes(kind)){
   ctx.save();ctx.translate(960,405);ctx.rotate(progress*.14);ctx.strokeStyle=theme.accent;ctx.lineWidth=2;ctx.beginPath();ctx.moveTo(0,-110);ctx.lineTo(120,0);ctx.lineTo(0,110);ctx.lineTo(-120,0);ctx.closePath();ctx.stroke();ctx.beginPath();ctx.arc(0,0,60,0,Math.PI*2);ctx.stroke();ctx.restore();heading(960,585,kind==='monogram'?155:85,'center',1600);subtitle(400,780,1300);
  }else if(['hero-device','feature-focus','product-spin','macro','orbital'].includes(kind)){
   const isMacro=kind==='macro',isCenter=kind==='hero-device'||kind==='orbital';over();
   if(isCenter){heading(960,205,92,'center',1600);device(ctx,660,isMacro?300:365,600,450,animationTime,template,0);}
   else{heading(128,365,100,'left',850);subtitle(128,710,780);device(ctx,isMacro?1030:1150,isMacro?60:210,isMacro?920:520,isMacro?1120:660,local,template,kind==='product-spin'?Math.sin(local*.6)*.07:0);}
  }else if(['bento','spec-grid','stat-cards','profile-grid'].includes(kind)){
   over();heading(128,210,78,'left',1650);
   if(kind==='profile-grid'){image(ctx,scene.image,128,360,730,590);grid(ctx,scene,template,900,360,890,590,2,progress);}else grid(ctx,scene,template,128,390,1664,570,kind==='stat-cards'?4:2,progress);
  }else if(kind==='compare'){
   over();heading(960,230,80,'center',1600);device(ctx,350,410,410,480,local,template,-.015);device(ctx,1160,410,410,480,local,template,.015);text(ctx,scene.items[0].title,550,930,24,template,theme.accent,'center');text(ctx,scene.items[1].title,1360,930,24,template,theme.accent,'center');
  }else if(kind==='material-panels'){
   over();heading(128,235,80);for(let i=0;i<3;i++){const g=ctx.createLinearGradient(0,0,1920,1080);g.addColorStop(0,theme.accent);g.addColorStop(.5,theme.surface);g.addColorStop(1,theme.secondary);round(ctx,128+i*565,410,530,510,36,g);text(ctx,scene.items[i].title,164+i*565,810,34,template,white);}
  }else if(['gallery','film-strip','zoom-gallery','split-screen','action-panel'].includes(kind)){
   const count=kind==='zoom-gallery'?1:kind==='split-screen'?2:3;const gap=25,w=(1664-gap*(count-1))/count;over();heading(128,225,78);for(let i=0;i<count;i++){ctx.save();if(kind==='action-panel'){ctx.transform(1,0,-.08,1,35,0);}image(ctx,scene.image,128+i*(w+gap),400,w,500,1.05+i*.12+progress*.03);ctx.restore();}subtitle(128,940);
  }else if(kind==='map-trace'){
   over();heading(128,260,90);const points=[[200,730],[600,570],[1060,760],[1650,480]];ctx.strokeStyle=theme.accent;ctx.lineWidth=2;ctx.beginPath();points.forEach(([x,y],i)=>{if(i===0)ctx.moveTo(x,y);else ctx.lineTo(x,y)});ctx.stroke();points.forEach(([x,y],i)=>{ctx.fillStyle=theme.accent;ctx.beginPath();ctx.arc(x,y,8+4*Math.sin(progress*Math.PI*2+i),0,Math.PI*2);ctx.fill();text(ctx,scene.items[i].title,x,y+30,24,template,white,'center',300)});
  }else if(kind==='ambient'){
   text(ctx,scene.overline,128,160,18,template,theme.accent);heading(128,425,110,'left',1250);subtitle(128,710);
  }else{
   over();if(kind==='landscape'){ctx.fillStyle='#000000';ctx.fillRect(0,0,1920,70);ctx.fillRect(0,1010,1920,70);heading(128,720,95);}else if(kind==='signature'){heading(128,420,88,'left',900);subtitle(128,690,850);}else if(kind==='character-detail'){subtitle(128,830,1000);}else{heading(128,420,105,'left',1050);subtitle(128,690,930);}
  }
  ctx.restore();
 }
 function compose(template,s1,s2,local1,local2,progress,transition,t){
  paint(a,s1,local1,template);paint(b,s2,local2,template);shader.updateTexture('u_from',a);shader.updateTexture('u_to',b);
  for(const [name,value] of Object.entries({u_progress:progress,u_mode:transition.mode,u_direction:transition.direction,u_strength:transition.strength,u_softness:transition.softness,u_tiles:transition.tiles,u_bgA:backgrounds.indexOf(s1.background),u_bgB:backgrounds.indexOf(s2.background),u_loop:template.loop?schedule(template).duration:0,u_base:rgb(template.theme.background),u_accent:rgb(template.theme.accent),u_secondary:rgb(template.theme.secondary)}))shader.setUniform(name,value);
  shader.render(t);
 }
 function renderTemplate(template,t){const {rows,duration}=layouts.get(template)||schedule(template);t=Math.max(0,Math.min(duration,t));let index=0;for(let i=1;i<rows.length;i++)if(t>=rows[i].start)index=i;
  const row=rows[index],previous=index>0?rows[index-1]:null;
  if(previous&&t<previous.end&&previous.scene.transition_duration>0&&previous.scene.transition!=='cut'){const tr=transitions[previous.scene.transition];compose(template,previous.scene,row.scene,t-previous.start,t-row.start,ease((t-row.start)/previous.scene.transition_duration,tr.ease),tr,t);}
  else compose(template,row.scene,row.scene,t-row.start,t-row.start,0,transitions[row.scene.transition],t);
 }
 return {duration:config.kind==='atlas'?config.cases.length/config.fps:layouts.get(config).duration,fps:config.fps,
  cuts:config.kind==='atlas'?config.cases.map((_,i)=>i/config.fps):layouts.get(config).rows.filter((row,i)=>i>0&&(config.scenes[i-1].transition_duration===0||config.scenes[i-1].transition==='cut')).map(row=>row.start),
  meta:{template:config.id||'catalog-atlas',data:'template.json',gpu:shader.metadata,transitionCount:Object.keys(transitions).length,truth:'Data-driven designed motion; fictional sample material, no gameplay or product evidence',cases:config.kind==='atlas'?config.cases.map(c=>c.id):undefined},
  render(t){if(config.kind==='atlas'){const item=config.cases[Math.min(config.cases.length-1,Math.floor(t*config.fps+1e-5))];if(item.transition){const tr=transitions[item.transition],sc=item.template.scenes;compose(item.template,sc[0],sc[1],.8,1.2,item.phase,tr,item.phase);}else renderTemplate(item.template,item.time);}else renderTemplate(config,t);}
 };
}
