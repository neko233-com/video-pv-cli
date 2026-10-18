#version 300 es
precision highp float;
uniform vec2 u_resolution;
uniform float u_time;
out vec4 fragColor;
#include "library.glsl"
void main() {
 vec2 uv=gl_FragCoord.xy/u_resolution;
 vec2 p=(gl_FragCoord.xy-0.5*u_resolution)/u_resolution.y;
 float loop=6.28318530718*u_time/6.0;
 vec2 centre=vec2(0.37,0.03);vec2 q=p-centre;
 float radius=length(q),angle=atan(q.y,q.x);
 vec2 drift=0.18*vec2(cos(loop),sin(loop));
 float smoke=fbm(p*3.4+drift);
 vec3 color=vec3(0.002,0.006,0.018)+vec3(0.012,0.036,0.075)*smoke;
 float distort=0.015*sin(8.0*angle+loop)+0.008*sin(17.0*angle-loop*2.0);
 float rim=exp(-abs(radius-0.325-distort)*110.0);
 float halo=exp(-abs(radius-0.325)*14.0);
 float arc=pow(0.5+0.5*cos(angle-loop),4.0);
 color+=vec3(0.08,0.55,1.05)*(rim*(0.7+2.0*arc)+halo*0.17);
 color+=vec3(0.53,0.22,0.055)*exp(-abs(radius-0.35)*130.0)*(0.65+0.35*sin(angle*13.0-loop));
 float mist=fbm(q*6.0+drift*2.0);
 color+=vec3(0.025,0.08,0.15)*mist*smoothstep(0.34,0.13,radius);
 vec2 grid=p*90.0;vec2 cell=floor(grid);float star=step(0.991,hash21(cell));
 float point=exp(-length(fract(grid)-0.5)*13.0);
 color+=vec3(0.32,0.55,0.8)*star*point*(0.7+0.3*cos(loop+hash21(cell)*6.28));
 color*=1.0-smoothstep(0.2,1.2,length(p))*0.5;
 fragColor=vec4(toneMap(color),1.0);
}
