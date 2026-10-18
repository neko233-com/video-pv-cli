// Original local WebGL2 helper. All animation time comes from render(t).
export async function createShader(canvas, fragmentPath, {textures={}}={}) {
 const gl=canvas.getContext('webgl2',{alpha:false,antialias:false,preserveDrawingBuffer:true});
 if(!gl)throw new Error('WebGL2 unavailable: cannot export this shader scene');
 async function source(path,parents=[]) {
  const url=new URL(path,location.href);
  if(url.origin!==location.origin)throw new Error('Shader resources must be local');
  if(parents.includes(url.href)||parents.length>24)throw new Error('Shader include cycle or depth limit');
  const response=await fetch(url);if(!response.ok)throw new Error(`Shader load failed: ${url.pathname}`);
  const text=await response.text();
  const pieces=[];let start=0;
  for(const match of text.matchAll(/^\s*#include\s+"([^"\n]+)"\s*$/gm)) {
   pieces.push(text.slice(start,match.index));pieces.push(await source(new URL(match[1],url).href,[...parents,url.href]));start=match.index+match[0].length;
  }
  pieces.push(text.slice(start));return pieces.join('\n');
 }
 function compile(type,text) {const shader=gl.createShader(type);gl.shaderSource(shader,text);gl.compileShader(shader);if(!gl.getShaderParameter(shader,gl.COMPILE_STATUS))throw new Error(gl.getShaderInfoLog(shader));return shader;}
 const vertex=compile(gl.VERTEX_SHADER,'#version 300 es\nin vec2 position;void main(){gl_Position=vec4(position,0.,1.);}');
 const fragment=compile(gl.FRAGMENT_SHADER,await source(fragmentPath));
 const program=gl.createProgram();gl.attachShader(program,vertex);gl.attachShader(program,fragment);gl.linkProgram(program);
 if(!gl.getProgramParameter(program,gl.LINK_STATUS))throw new Error(gl.getProgramInfoLog(program));
 gl.useProgram(program);const buffer=gl.createBuffer();gl.bindBuffer(gl.ARRAY_BUFFER,buffer);gl.bufferData(gl.ARRAY_BUFFER,new Float32Array([-1,-1,3,-1,-1,3]),gl.STATIC_DRAW);
 const attribute=gl.getAttribLocation(program,'position');gl.enableVertexAttribArray(attribute);gl.vertexAttribPointer(attribute,2,gl.FLOAT,false,0,0);
 for(const [unit,[uniform,path]] of Object.entries(textures).entries()) {
  const url=new URL(path,location.href);if(url.origin!==location.origin)throw new Error('Textures must be local');
  const image=new Image();image.src=url.href;await image.decode();
  const texture=gl.createTexture();gl.activeTexture(gl.TEXTURE0+unit);gl.bindTexture(gl.TEXTURE_2D,texture);gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL,true);
  gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_S,gl.CLAMP_TO_EDGE);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_T,gl.CLAMP_TO_EDGE);
  gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MIN_FILTER,gl.LINEAR);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MAG_FILTER,gl.LINEAR);
  gl.texImage2D(gl.TEXTURE_2D,0,gl.RGBA,gl.RGBA,gl.UNSIGNED_BYTE,image);gl.uniform1i(gl.getUniformLocation(program,uniform),unit);
 }
 const time=gl.getUniformLocation(program,'u_time'),resolution=gl.getUniformLocation(program,'u_resolution');
 return { render(t) {
  if(gl.isContextLost())throw new Error('WebGL context lost');
  const width=Math.round(canvas.clientWidth*devicePixelRatio),height=Math.round(canvas.clientHeight*devicePixelRatio);
  if(canvas.width!==width||canvas.height!==height){canvas.width=width;canvas.height=height;}
  gl.viewport(0,0,width,height);gl.useProgram(program);gl.uniform1f(time,t);gl.uniform2f(resolution,width,height);gl.drawArrays(gl.TRIANGLES,0,3);gl.finish();
  const error=gl.getError();if(error!==gl.NO_ERROR)throw new Error(`WebGL render error: ${error}`);
 }};
}
