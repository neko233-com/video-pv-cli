#version 300 es
precision highp float;
uniform vec2 u_resolution;
uniform float u_time;
uniform sampler2D u_character;
out vec4 fragColor;
#include "library.glsl"
void main(){
 vec2 uv=gl_FragCoord.xy/u_resolution;
 float progress=clamp(u_time/10.0,0.0,1.0);
 vec2 camera=(uv-vec2(0.68,0.54))/(1.07-0.05*progress)+vec2(0.68-0.012*progress,0.54);
 vec3 art=texture(u_character,camera).rgb;
 vec2 q=(uv-vec2(0.55,0.54))*vec2(u_resolution.x/u_resolution.y,1.0);
 float distanceFromHand=length(q);
 float pulse=smoothstep(2.4,4.1,u_time)*(1.0-smoothstep(5.5,7.0,u_time));
 float aura=exp(-abs(distanceFromHand-0.09-0.006*sin(u_time*2.0))*70.0)*pulse;
 float mist=fbm(uv*5.0+vec2(u_time*0.025,0.0));
 vec3 color=pow(art,vec3(2.2));
 color+=vec3(0.025,0.13,0.24)*aura+vec3(0.006,0.016,0.03)*mist;
 float mask=1.0-smoothstep(0.03,0.62,uv.x);
 color*=1.0-0.7*mask;
 fragColor=vec4(pow(max(color,vec3(0.0)),vec3(1.0/2.2)),1.0);
}
