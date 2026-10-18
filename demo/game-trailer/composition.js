// One native HyperFrames timeline. Registration must precede GPU initialization.
(() => {
  'use strict';
  const config=JSON.parse(document.getElementById('project-data').textContent);
  const look=JSON.parse(document.getElementById('style-data').textContent);
  let activeLook;
  function applyLook(style){if(activeLook===style.id)return;activeLook=style.id;const surface=style.palette.background;
    const vars={'--accent':style.palette.accent,'--foreground':style.palette.foreground,'--shade':surface+'ed','--shade-soft':surface+'66','--type-scale':style.typography.scale,'--leading':style.typography.leading};
    for(const [key,value] of Object.entries(vars))document.getElementById('root').style.setProperty(key,value);
  }applyLook(look);
  window.__timelines=window.__timelines||{};
  const tl=gsap.timeline({paused:true});
  window.__timelines.main=tl;
  if(!config.loop) for(const scene of config.scenes){
    if(!scene.title)continue;
    const base=`#${scene.id} `,start=scene.start;
    tl.fromTo(base+'.rule',{scaleX:0},{scaleX:1,duration:.8,ease:'power3.out'},start+.12);
    tl.fromTo(base+'.kicker',{opacity:0,y:12},{opacity:1,y:0,duration:.75,ease:'power3.out'},start+.18);
    tl.fromTo(base+'h1',{opacity:0,y:48},{opacity:1,y:0,duration:1.05,ease:'power3.out'},start+.28);
    tl.fromTo(base+'.subtitle',{opacity:0,y:18},{opacity:1,y:0,duration:.85,ease:'power3.out'},start+.65);
    tl.to(base+'.scene-shell',{opacity:0,duration:.3,ease:'power2.in'},start+scene.duration-.3);
  }
  tl.to({}, {duration:.001},config.duration-.001);
  // Reject WebGL requests made by installed components as well as authored code.
  const getContext=HTMLCanvasElement.prototype.getContext;
  HTMLCanvasElement.prototype.getContext=function(type,...args){
    if(/^(webgl2?|experimental-webgl)$/i.test(type))throw new Error('WEB2VIDEO_WEBGL_FORBIDDEN');
    return getContext.call(this,type,...args);
  };
  let fatal;
  const ready=import('./webgpu-runtime.js').then(async({createGPUShader})=>{
    const images=new Map();
    for(const scene of config.scenes){if(!images.has(scene.image)){const img=new Image();img.src=scene.image;await img.decode();images.set(scene.image,img);}}
    const buffers=[0,1].map(()=>{const c=document.createElement('canvas');c.width=config.width;c.height=config.height;return c;});
    const gpu=await createGPUShader(document.getElementById('stage'),'./catalog.wgsl',{textures:{u_from:buffers[0],u_to:buffers[1]}});
    if(gpu.metadata.isFallbackAdapter)throw new Error('WEB2VIDEO_SOFTWARE_GPU_FORBIDDEN');
    window.__web2videoGPU=gpu.metadata;
    gpu.setUniform('u_base',[.01,.02,.04,1]);gpu.setUniform('u_accent',[.55,.7,1,1]);gpu.setUniform('u_secondary',[.9,.65,.4,1]);
    function draw(scene,time,buffer){
      const img=images.get(scene.image),ctx=buffer.getContext('2d',{willReadFrequently:true});
      const phase=config.loop?(1-Math.cos(2*Math.PI*time/config.duration))*.5:Math.max(0,Math.min(1,(time-scene.start)/scene.duration));
      const scale=Math.max(config.width/img.width,config.height/img.height)*scene.zoom*(1+.035*phase);
      const w=img.width*scale,h=img.height*scale;
      ctx.clearRect(0,0,buffer.width,buffer.height);ctx.drawImage(img,(config.width-w)*scene.x,(config.height-h)*scene.y,w,h);
    }
    return async time=>{
      if(fatal)throw fatal;
      const t=config.loop?((time%config.duration)+config.duration)%config.duration:Math.max(0,Math.min(time,config.duration-.000001));
      let i=config.scenes.findLastIndex(s=>t>=s.start);i=Math.max(0,i);
      const to=config.scenes[i],from=config.scenes[Math.max(0,i-1)],transition=config.transitions[to.transition];
      applyLook(config.looks[to.style||config.style]);
      const p=i===0?1:Math.max(0,Math.min(1,(t-to.start)/.65));
      draw(from,t,buffers[0]);draw(to,t,buffers[1]);gpu.updateTexture('u_from',buffers[0]);gpu.updateTexture('u_to',buffers[1]);
      gpu.setUniform('u_progress',p);gpu.setUniform('u_mode',transition.mode);gpu.setUniform('u_strength',transition.strength);gpu.setUniform('u_softness',transition.softness);gpu.setUniform('u_tiles',transition.tiles);gpu.setUniform('u_direction',transition.direction);gpu.setUniform('u_loop',config.loop?config.duration:0);
      gpu.render(t);await gpu.flush();window.__web2videoTime=time;
    };
  });
  ready.catch(error=>{fatal=error;window.__web2videoError=error.message;console.error(error);});
  window.addEventListener('hf-seek',event=>{
    const job=ready.then(render=>render(event.detail.time));
    event.detail.waitUntil(job);
  });
})();
