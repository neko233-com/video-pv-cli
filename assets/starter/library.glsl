// Original reusable shader functions, Apache-2.0, video-pv-cli.
float hash21(vec2 p) { return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453123); }
float noise2(vec2 p) {
 vec2 i=floor(p),f=fract(p);f=f*f*(3.0-2.0*f);
 return mix(mix(hash21(i),hash21(i+vec2(1,0)),f.x),mix(hash21(i+vec2(0,1)),hash21(i+vec2(1)),f.x),f.y);
}
float fbm(vec2 p) { float n=0.0,a=0.5; for(int i=0;i<4;i++){n+=a*noise2(p);p=mat2(1.6,-1.2,1.2,1.6)*p+3.7;a*=0.5;}return n; }
vec3 toneMap(vec3 c) {return pow(vec3(1.0)-exp(-c),vec3(1.0/2.2));}
