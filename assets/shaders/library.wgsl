// Original local WGSL helpers, Apache-2.0, video-pv-cli.
fn hash21(p: vec2f) -> f32 { return fract(sin(dot(p,vec2f(127.1,311.7)))*43758.5453123); }
fn noise2(p: vec2f) -> f32 {
 let i=floor(p);var f=fract(p);f=f*f*(vec2f(3)-2*f);
 return mix(mix(hash21(i),hash21(i+vec2f(1,0)),f.x),mix(hash21(i+vec2f(0,1)),hash21(i+vec2f(1)),f.x),f.y);
}
fn fbm(input: vec2f) -> f32 {var p=input;var n=0.;var a=.5;for(var i=0;i<4;i++){n+=a*noise2(p);p=mat2x2f(vec2f(1.6,-1.2),vec2f(1.2,1.6))*p+3.7;a*=.5;}return n;}
fn rotate(a: f32) -> mat2x2f {return mat2x2f(vec2f(cos(a),-sin(a)),vec2f(sin(a),cos(a)));}
fn pmod(x:f32,y:f32)->f32{return x-y*floor(x/y);}
