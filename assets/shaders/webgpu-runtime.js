// Native WebGPU only. No graphics API fallback, RAF or wall-clock animation.
export async function createGPUShader(canvas, shaderPath, {textures={}}={}) {
 if(!globalThis.isSecureContext||!navigator.gpu)throw new Error('PV_WEBGPU_UNAVAILABLE: use a WebGPU-capable Chromium on localhost/HTTPS');
 const adapter=await navigator.gpu.requestAdapter();
 if(!adapter)throw new Error('PV_WEBGPU_NO_ADAPTER');
 const device=await adapter.requestDevice({label:'web2video WebGPU'});
 let fatal;
 const fail=reason=>{fatal=new Error(reason);queueMicrotask(()=>{throw fatal;});};
 device.addEventListener('uncapturederror',event=>{event.preventDefault();fail('PV_WEBGPU_VALIDATION: '+event.error.message);});
 device.lost.then(info=>fail(`PV_WEBGPU_DEVICE_LOST: ${info.reason}: ${info.message}`));
 async function source(path,parents=[]) {
  const url=new URL(path,document.baseURI);
  if(url.origin!==location.origin)throw new Error('WGSL resources must be local');
  if(parents.includes(url.href)||parents.length>24)throw new Error('WGSL include cycle/depth limit');
  const response=await fetch(url);if(!response.ok)throw new Error(`WGSL load failed: ${url.pathname}`);
  const text=await response.text();const pieces=[];let start=0;
  for(const match of text.matchAll(/^\s*\/\/\s*@include\s+"([^"\n]+)"\s*$/gm)){pieces.push(text.slice(start,match.index));pieces.push(await source(new URL(match[1],url).href,[...parents,url.href]));start=match.index+match[0].length;}
  pieces.push(text.slice(start));return pieces.join('\n');
 }
 const code=await source(shaderPath);
 const module=device.createShaderModule({label:shaderPath,code});
 const compilation=await module.getCompilationInfo();
 const errors=compilation.messages.filter(message=>message.type==='error');
 if(errors.length)throw new Error('PV_WGSL_COMPILE: '+errors.map(e=>`${e.lineNum}:${e.linePos} ${e.message}`).join('\n'));
 const context=canvas.getContext('webgpu');if(!context)throw new Error('PV_WEBGPU_CANVAS_UNAVAILABLE');
 const format=navigator.gpu.getPreferredCanvasFormat();
 device.pushErrorScope('validation');
 const pipeline=await device.createRenderPipelineAsync({label:'PV WGSL compositor',layout:'auto',vertex:{module,entryPoint:'vertexMain'},fragment:{module,entryPoint:'fragmentMain',targets:[{format}]},primitive:{topology:'triangle-list'}});
 const pipelineError=await device.popErrorScope();if(pipelineError)throw new Error('PV_WEBGPU_PIPELINE: '+pipelineError.message);
 const uniformData=new Float32Array(28),uniformBuffer=device.createBuffer({label:'PV explicit frame uniforms',size:uniformData.byteLength,usage:GPUBufferUsage.UNIFORM|GPUBufferUsage.COPY_DST});
 const sampler=device.createSampler({magFilter:'linear',minFilter:'linear',addressModeU:'clamp-to-edge',addressModeV:'clamp-to-edge'});
 const bindings=new Map();
 for(const [name,input] of Object.entries(textures)){
  let source=input;
  if(typeof input==='string'){const url=new URL(input,document.baseURI);if(url.origin!==location.origin)throw new Error('GPU textures must be local');source=new Image();source.src=url.href;await source.decode();}
  const texture=device.createTexture({label:name,size:[source.width,source.height],format:'rgba8unorm',usage:GPUTextureUsage.TEXTURE_BINDING|GPUTextureUsage.COPY_DST|GPUTextureUsage.RENDER_ATTACHMENT});
  bindings.set(name,{texture,width:source.width,height:source.height});
  if(typeof input==='string')device.queue.copyExternalImageToTexture({source},{texture,premultipliedAlpha:false},[source.width,source.height]);
 }
 if(!bindings.has('u_from')||!bindings.has('u_to'))throw new Error('PV WebGPU compositor needs u_from/u_to textures');
 const group=device.createBindGroup({layout:pipeline.getBindGroupLayout(0),entries:[{binding:0,resource:{buffer:uniformBuffer}},{binding:1,resource:sampler},{binding:2,resource:bindings.get('u_from').texture.createView()},{binding:3,resource:bindings.get('u_to').texture.createView()}]});
 const offsets={u_time:2,u_progress:3,u_mode:4,u_strength:5,u_softness:6,u_tiles:7,u_direction:8,u_bgA:10,u_bgB:11,u_base:12,u_accent:16,u_secondary:20,u_loop:24};
 let width=0,height=0;
 const info=adapter.info||{};
 const metadata={backend:'webgpu',shaderLanguage:'wgsl',vendor:info.vendor||'',architecture:info.architecture||'',device:info.device||'',description:info.description||'',isFallbackAdapter:!!info.isFallbackAdapter,features:[...device.features].sort()};
 return {
  metadata,device,
  async flush(){await device.queue.onSubmittedWorkDone();if(fatal)throw fatal;},
  setUniform(name,value){if(!(name in offsets))throw new Error(`Unknown GPU uniform ${name}`);const values=Array.isArray(value)?value:[value];if(values.some(v=>!Number.isFinite(v)))throw new Error(`Invalid GPU uniform ${name}`);uniformData.set(values,offsets[name]);},
  updateTexture(name,source){if(fatal)throw fatal;const binding=bindings.get(name);if(!binding||source.width!==binding.width||source.height!==binding.height)throw new Error(`GPU texture dimensions changed: ${name}`);const pixels=source.getContext('2d',{willReadFrequently:true}).getImageData(0,0,source.width,source.height);device.queue.writeTexture({texture:binding.texture},pixels.data,{bytesPerRow:source.width*4,rowsPerImage:source.height},[source.width,source.height]);},
  render(t){
   if(fatal)throw fatal;if(!Number.isFinite(t))throw new Error('Invalid frame time');
   const w=Math.round(canvas.clientWidth*devicePixelRatio),h=Math.round(canvas.clientHeight*devicePixelRatio);
   if(w!==width||h!==height){width=w;height=h;canvas.width=w;canvas.height=h;context.configure({device,format,alphaMode:'opaque',usage:GPUTextureUsage.RENDER_ATTACHMENT|GPUTextureUsage.COPY_SRC});}
   uniformData[0]=w;uniformData[1]=h;uniformData[2]=t;device.queue.writeBuffer(uniformBuffer,0,uniformData);
   const encoder=device.createCommandEncoder({label:`PV frame ${t}`});
   const pass=encoder.beginRenderPass({colorAttachments:[{view:context.getCurrentTexture().createView(),clearValue:{r:0,g:0,b:0,a:1},loadOp:'clear',storeOp:'store'}]});pass.setPipeline(pipeline);pass.setBindGroup(0,group);pass.draw(3);pass.end();device.queue.submit([encoder.finish()]);
  }
 };
}
