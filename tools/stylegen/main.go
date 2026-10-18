// Explicit catalog authoring utility. Never runs during install/render.
package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed palettes.txt
var palettes string

const seeds = `keynote-silent-reveal|静场揭幕|negative space,restrained reveal,hero silhouette,single promise,quiet stage,slow release|one hero isolated inside a wide empty field|soft overhead pool|matte aluminum and diffused glass
keynote-spatial-type|空间文字|oversized type,depth planes,spatial hierarchy,word choreography,precise pause,controlled parallax|one word crosses a product-sized depth plane|broad studio key|frosted layers and crisp type
keynote-monochrome-stage|黑白舞台|monochrome,architectural balance,hard silhouette,editorial rhythm,confident hold,clean horizon|balanced black and white masses around a centered object|single edge strip|black ceramic and white paper
keynote-material-closeup|材质微距|macro detail,surface fidelity,material proof,slow push,tactile light,quiet specification|alternate a texture-sized crop with the complete product|raking softbox|brushed metal and satin highlights
keynote-color-field|纯色主场|color field,saturated backdrop,isolated product,graphic contrast,measured cut,one accent|one saturated field supports a single contrasting subject|even cyclorama|painted enamel and solid pigment
keynote-bento-proof|模块证据|modular proof,bento grid,feature hierarchy,measured rhythm,readable labels,ordered reveal|one dominant feature leads a secondary grid of evidence|flat readable fill|clean white panels and technical glass
keynote-camera-window|视窗推镜|camera aperture,framed reveal,screen detail,depth cue,smooth zoom,focus exchange|use one window as the bridge between overview and detail|lens-like falloff|polished glass and optical coating
keynote-ultralight|轻盈留白|airy composition,thin strokes,small label,slow float,pale space,soft geometry|keep weight near the center and generous light margins|large diffuse ceiling|translucent polymer and pearl finish
keynote-performance-pulse|性能脉冲|numeric emphasis,performance beat,precise hit,short contrast,clean spec,confident cadence|let one meaningful number take the hero position|crisp directional key|graphite and illuminated markings
keynote-one-more-reveal|压轴再揭|delayed payoff,chapter break,final silhouette,measured silence,scale shift,hero return|hold the main reveal until the last large composition|dark-to-light rim|unbroken metal and calm reflections
luxury-obsidian-gold|曜石鎏金|deep black,gold edge,jewel silence,wide tracking,slow dissolve,formal symmetry|a narrow luminous object sits inside a large dark field|thin warm rim|obsidian and brushed gold
luxury-pearl-satin|珍珠缎光|pearl white,satin fold,soft specular,intimate macro,graceful drift,delicate serif|soft layered planes cradle a restrained product close-up|wraparound pearl glow|mother-of-pearl and satin
luxury-watch-macro|精密时计|mechanical detail,radial index,precision macro,patient hold,metal edge,measured rotation|expand one small mechanism to a monumental focal point|grazing watchmaker light|steel and sapphire crystal
luxury-chrome-sculpture|铬面雕塑|chrome mass,sculptural form,curved reflection,gallery space,slow orbit,black pedestal|treat a product form as a gallery sculpture|long reflection cards|mirror chrome and smoked glass
luxury-perfume-mist|香氛薄雾|translucent bottle,soft mist,editorial silence,amber glint,delicate reveal,lingering tail|one translucent volume emerges from a quiet atmospheric field|backlit diffusion|amber glass and fine vapor
luxury-automotive-night|夜行车语|night silhouette,long highlight,low horizon,controlled speed,red tail,wide frame|stretch a low hero silhouette across a dark horizon|moving-looking strip reflections|lacquered black metal and red glass
luxury-fashion-editorial|时装版面|fashion crop,asymmetric margin,serif headline,confident cut,textile depth,editorial pose|offset a close crop against an assertive editorial headline|large fashion key|woven textile and satin trim
luxury-jewelry-caustic|珠宝折光|faceted glint,caustic pattern,miniature scale,precise sparkle,quiet contrast,velvet field|a miniature object becomes a frame-filling faceted subject|small sharp light|cut crystal and velvet
luxury-ceramic-gallery|陶瓷展厅|ceramic curve,museum spacing,soft shadow,neutral field,still-life balance,slow arc|arrange one rounded form with museum-like breathing room|north-window softness|unglazed ceramic and plaster
luxury-leather-craft|皮革匠作|stitched detail,handcrafted texture,warm shadow,close framing,patient gesture,material honesty|follow a seam-sized detail before returning to the whole form|warm raking light|full-grain leather and aged brass
tech-schematic-precision|工程图谱|schematic lines,dimension marks,technical grid,exact alignment,calibrated motion,clear evidence|align the subject to dimension lines and one readable specification|cool technical fill|etched aluminum and blueprint ink
tech-glass-interface|玻璃界面|layered interface,frosted plane,soft depth,thin boundary,focus handoff,clear control|stack a small number of readable interface planes|soft edge glow|frosted glass and transparent acrylic
tech-graphite-terminal|石墨终端|terminal contrast,code rhythm,dark graphite,signal green,precise cursor,developer clarity|use a terminal-sized panel as the proof surface|low ambient and screen emission|graphite and phosphor-like marks
tech-neural-lattice|神经晶格|neural structure,network depth,coherent nodes,signal travel,quiet futurism,emergent pattern|organize visible information as a legible node lattice|cool volumetric suggestion|luminous fibers and dark glass
tech-orbital-data|轨道数据|orbit diagram,radial hierarchy,telemetry ring,measured rotation,clean numerals,spatial signal|place a main metric inside an ordered orbital system|focused cyan rim|dark titanium and emissive traces
tech-medical-clean|洁净精密|clinical clarity,white field,calm data,hygienic geometry,trustworthy scale,soft blue|present one verified measurement in a spacious clinical frame|shadowless studio wash|sterile polymer and clear glass
tech-energy-flow|能量流线|directed flow,charged contour,high contrast,measured surge,technical reveal,signal trail|lead the eye along one energized route into the subject|electric edge lighting|conductive metal and luminous channels
tech-device-exploded|分层器件|exploded hierarchy,component stack,assembly order,precise spacing,mechanical clarity,depth separation|separate conceptual product layers along one readable axis|cool assembly light|anodized parts and translucent separators
tech-industrial-robotics|工业机械|industrial rhythm,warning accent,structural mass,measured joint,hard boundary,workshop precision|anchor a heavy product mass with functional marks|hard workshop key|powder-coated steel and safety pigment
tech-optical-lens|光学透镜|lens anatomy,concentric focus,optical refraction,crisp edge,slow focus,scientific elegance|use concentric framing to move attention between whole and detail|controlled lens flare|optical glass and blackened metal
editorial-swiss-grid|瑞士网格|swiss grid,objective type,baseline rhythm,asymmetric balance,clear numbering,precise rules|lock type and product evidence to a strict column grid|flat graphic light|uncoated paper and crisp black ink
editorial-magazine-serif|杂志衬线|magazine spread,expressive serif,generous margin,large image,editorial pause,thin rule|give one image and one serif headline unequal visual weight|soft editorial studio|warm paper and fine ink
editorial-ink-paper|纸墨印象|paper grain,ink contrast,printed label,warm restraint,cut rhythm,crafted margin|build a tactile print composition with a dominant headline|gentle paper shadow|cream stock and dense ink
editorial-typographic-poster|字体海报|typographic monument,poster scale,word contrast,bold geometry,clear accent,graphic stop|make a small number of words the main visual subject|flat poster fill|solid pigment and letterform edges
editorial-cutout-collage|剪贴拼版|cutout image,layered paper,offset crop,tactile collage,playful juxtaposition,editorial glue|overlap deliberate paper-like crops around one product proof|small layered shadows|cut paper and photographic print
editorial-architectural-index|建筑索引|architectural order,index labels,thin frame,measured spacing,neutral mass,quiet rhythm|frame the subject as a numbered architectural study|soft gallery wash|concrete and technical tracing paper
editorial-newspaper-sequence|报刊序列|headline rail,column rhythm,archive tone,strong hierarchy,monochrome field,sequential proof|lead with a headline and follow a clear column sequence|even reading light|newsprint and black ink
editorial-kinetic-headline|动态头条|kinetic headline,word ladder,staggered rhythm,high contrast,sharp arrival,readable pause|stage a short headline as a succession of large words|punchy graphic key|printed blocks and sharp typographic edges
editorial-line-art-explainer|线描讲解|line drawing,diagram logic,sparse color,stepwise proof,readable annotation,gentle reveal|one line diagram explains a single product relationship|clean white fill|technical pen and white stock
editorial-archive-contact|档案接触表|contact sheet,archive numbering,photographic rhythm,small annotation,measured crop,documentary restraint|build a numbered image sequence around one lead frame|neutral photographic light|archival print and black mount
game-epic-fantasy|史诗奇幻|mythic scale,ruined horizon,gold embers,hero silhouette,slow escalation,monumental title|a small hero silhouette faces a monumental world|dawn rim and deep atmosphere|weathered stone and worn metal
game-sci-fi-tactical|科幻战术|tactical framing,telemetry marks,cold metal,controlled tension,precise cut,mission clarity|frame real action with sparse mission-like marks|cold directional edge|titanium and emissive indicators
game-cyberpunk-night|赛博夜城|wet neon,urban depth,chromatic edge,signal noise,night silhouette,sharp pulse|hold one readable subject against a layered night city|cyan-magenta edge light|wet asphalt and illuminated glass
game-dark-soulslike|暗黑残境|oppressive scale,ash palette,lonely figure,low key,slow threat,weighty impact|a lonely focal subject sits under heavy dark negative space|weak amber rim|ash stone and scarred iron
game-cozy-world|治愈世界|soft adventure,warm dawn,rounded shapes,gentle drift,friendly scale,open horizon|invite entry through a warm and readable world frame|soft morning wash|painted wood and soft foliage
game-racing-redline|竞速红线|speed framing,low camera,red accent,hard cut,directional force,clean telemetry|push the eye along a strong horizontal racing line|hard track-side streak|carbon fiber and hot lacquer
game-mecha-arsenal|机甲武库|mechanical mass,hangar reveal,assembly grid,industrial labels,heavy pause,precision flash|reveal mechanical weight through a large industrial frame|hangar strip light|armored steel and machined joints
game-roguelike-impact|肉鸽冲击|impact rhythm,graphic silhouette,compressed action,bold accent,quick contrast,readable hit|isolate a clear action silhouette before each impact beat|short high-contrast hit|ink-like shapes and hot emissive accents
game-openworld-dawn|旷野破晓|openworld vista,atmospheric layers,dawn horizon,slow reveal,scale contrast,inviting depth|use foreground and distant horizon to establish explorable scale|wide dawn glow|grassland haze and weathered rock
game-esports-scorecard|电竞赛报|scoreboard hierarchy,team contrast,numeric punch,fast precision,bracket order,competitive energy|make the result or competitive objective the dominant information|arena rim lighting|dark panels and saturated team accents
anime-celestial-heroine|星辉角色|celestial motif,delicate rim,hero portrait,quiet resolve,orbital detail,lyrical reveal|balance a character portrait with a restrained identity title|moonlit cyan rim|silver ornament and dark satin
anime-shojo-petal|花瓣少女|petal drift,delicate type,pastel depth,soft emotion,light bloom,gentle reveal|frame a character inside airy botanical negative space|soft pink backlight|translucent petals and silk ribbon
anime-shonen-impact|热血冲线|speed lines,bold name,impact pause,high contrast,angular action,energetic cut|let the character silhouette and name share one decisive action line|hard warm rim|cel-shaded armor and graphic impact marks
anime-mecha-blueprint|机甲设定|blueprint crop,mechanical annotation,character dossier,cool edge,technical rhythm,precise reveal|combine a character or unit portrait with a readable technical index|cool hangar light|steel plates and luminous schematic lines
anime-visual-novel-noir|夜谈叙事|noir portrait,quiet dialogue,deep shadow,serif title,slow memory,small light|reserve a quiet text area beside an intimate character portrait|single window-like key|dark cloth and rain-softened glass
anime-idol-neon-stage|偶像霓台|stage color,lightstick rhythm,bright silhouette,lyrical type,controlled sparkle,chorus lift|treat the identity reveal as one clean stage-light moment|pink-cyan stage rim|reflective costume and luminous acrylic
anime-watercolor-memory|水彩记忆|watercolor edge,paper atmosphere,gentle memory,soft contour,poetic pause,restrained color|use a portrait as a soft memory surrounded by quiet paper space|diffused daylight|watercolor wash and textured stock
anime-samurai-ink|墨刃武者|ink silhouette,sharp diagonal,red seal,measured strike,brush texture,quiet tension|build the reveal around one diagonal blade-like composition|stark side key|ink wash and worn steel
anime-magical-girl-prism|棱彩变身|prismatic accent,hero emblem,bright confidence,layered reveal,crystal motif,lyrical rhythm|organize the identity around a single prismatic emblem|clean rainbow rim|cut crystal and bright lacquer
anime-summer-solstice|夏至物语|summer haze,blue distance,warm skin light,airy title,gentle camera,youthful memory|give the portrait a wide summer horizon and light typography|late-afternoon warmth|sunlit fabric and clear sky
cinema-anamorphic-night|宽银夜景|wide frame,anamorphic suggestion,night contrast,low horizon,slow pursuit,thin light|use a wide horizontal subject with strong dark margins|horizontal practical glow|wet metal and smoked glass
cinema-noir-silhouette|黑色剪影|noir geometry,hard shadow,single silhouette,quiet suspense,low key,formal contrast|tell the beat through one strong silhouette and its shadow|single hard side light|black lacquer and shadowed plaster
cinema-desert-epic|沙海史诗|desert scale,amber haze,monumental title,slow horizon,hard sun,sparse subject|reduce the subject to a powerful mark inside a vast field|low desert sun|sandstone and worn bronze
cinema-oceanic-deep|深海幽蓝|ocean depth,submerged light,slow drift,deep blue,caustic rhythm,quiet pressure|surround the hero with layered dark blue depth|filtered underwater shafts|deep water and translucent glass
cinema-urban-documentary|都市纪实|observed detail,documentary crop,neutral type,city rhythm,proof sequence,quiet humanity|favor readable real-world captures with understated labels|available daylight|concrete and photographic grain
cinema-film-grain-35mm|胶片余韵|analog mood,film grain,soft shoulder,warm shadow,patient cut,photographic frame|treat each shot as a deliberate photographic frame|soft practical key|film-like texture and warm print
cinema-high-key-studio|高调摄影棚|high key,white infinity,clean shadow,product clarity,soft edge,confident space|isolate the subject inside a bright seamless studio|large shadowless softboxes|white enamel and translucent glass
cinema-red-room|绯红密室|red field,psychological tension,hard silhouette,small highlight,slow pressure,dramatic cut|place one dark focal form against an intense red field|narrow red practical|black steel and red lacquer
cinema-slow-burn|缓燃悬念|patient hold,deep shadow,delayed reveal,small movement,quiet tension,measured payoff|withhold the main focal detail behind a long controlled hold|restrained edge pool|charcoal surfaces and dim amber glass
cinema-abstract-title|抽象片头|abstract geometry,title sequence,layered symbol,graphic mystery,precise rhythm,controlled texture|turn a meaningful symbol into a title-sized abstract composition|sculpted graphic contrast|matte shapes and fine optical texture
organic-botanical-light|植物微光|botanical shadow,living curve,warm paper,soft daylight,gentle growth,tactile calm|allow organic shadows to lead toward one quiet subject|leaf-filtered daylight|botanical fibers and warm plaster
organic-underwater-caustics|水下光纹|caustic field,liquid rhythm,clear depth,slow refraction,cool calm,soft boundary|let water-like light form a restrained field around the hero|refracted soft sunlight|clear water and polished stone
organic-fluid-ink|流墨扩散|fluid ink,organic edge,controlled spread,paper contrast,quiet motion,expressive form|a single flowing boundary reveals a compact focal subject|flat paper light|pigment in water and cream stock
organic-silk-fold|丝绸褶光|silk fold,soft gradient,tactile depth,gentle orbit,quiet sheen,flowing contour|use broad fabric-like folds to cradle a hero shape|large grazing softbox|silk and pearl pigment
organic-sandstone|砂岩暖境|sandstone mass,warm neutrality,weathered texture,architectural calm,soft horizon,slow reveal|balance a solid subject with a warm eroded spatial field|late warm side light|sandstone and dry mineral pigment
organic-wood-craft|木作温度|wood grain,crafted detail,warm shadow,material honesty,patient macro,human scale|lead from a material-sized crop to a crafted whole|warm workshop window|oiled wood and brushed brass
organic-water-drop|一滴清透|single droplet,clean refraction,macro intimacy,soft blue,quiet precision,clear field|make one transparent detail the entire focal story|small controlled backlight|clear liquid and smooth glass
organic-ice-crystal|冰晶秩序|ice facet,cold clarity,prismatic edge,slow glint,quiet geometry,blue white|use a faceted subject and sparse cold negative space|sharp cold rim|ice crystal and frosted glass
organic-smoke-sculpture|烟雾雕形|smoke volume,soft silhouette,airy depth,slow curl,low key,quiet emergence|allow one soft volume to reveal an anchored silhouette|backlit smoke glow|fine vapor and matte dark stone
organic-living-mesh|生长网格|organic lattice,coherent flow,soft signal,adaptive geometry,gentle pulse,natural order|organize an organic field around a clear central signal|soft luminous fill|translucent fibers and satin mesh
graphic-memphis-play|孟菲斯游戏|memphis geometry,playful offset,bold dots,color rhythm,graphic bounce,clear silhouette|arrange playful graphic masses around one obvious focal element|flat bright fill|painted shapes and printed dots
graphic-bauhaus-motion|包豪斯运动|primary geometry,functional type,bold axis,measured motion,graphic balance,clean edge|build the frame from a small set of balanced primary forms|flat geometric light|solid pigment and crisp paper
graphic-pop-print|波普印刷|pop contrast,halftone texture,poster type,comic framing,bold crop,graphic hit|make one bold image crop collide with a short poster headline|high graphic contrast|screenprint ink and comic paper
graphic-acid-rave|酸性狂想|acid color,extreme type,night pulse,synthetic contour,club energy,sharp cut|use an off-center high-energy headline against a synthetic field|electric club glow|fluorescent pigment and glossy vinyl
graphic-vaporwave-dusk|蒸汽暮色|vaporwave dusk,retro grid,pastel neon,slow horizon,nostalgic geometry,soft glow|place a simple hero over a receding dusk-colored spatial field|pink-blue dusk emission|chrome and sunset gradients
graphic-retrofuturism|复古未来|retro console,space-age curve,optimistic signal,geometric label,clean orbit,warm future|frame the product as an optimistic instrument from another future|warm console light|enamel and polished acrylic
graphic-riso-spot|孔版专色|riso texture,spot color,print misregister,editorial warmth,graphic grain,flat layering|use two deliberate print-like color layers with clear hierarchy|flat print illumination|porous stock and spot ink
graphic-brutalist-type|粗野字体|brutalist scale,raw type,hard margin,bare structure,forceful cut,graphic weight|let one oversized unpolished headline carry the frame|hard flat contrast|raw paper and dense black pigment
graphic-neo-brutal-blocks|新粗野块面|thick boundary,offset block,poster hierarchy,punchy accent,clear grid,graphic snap|stack a small number of strong rectangular proof surfaces|bold flat fill|solid painted blocks and black rules
graphic-holographic-prism|全息棱镜|holographic edge,prism split,iridescent field,clean silhouette,futuristic sheen,slow tilt|hold a calm central silhouette inside a controlled prismatic field|spectral edge glow|iridescent foil and optical glass
digital-saas-clean|清晰软件|real interface,clean hierarchy,calm workspace,benefit headline,readable proof,focused cursor|one real interface proof occupies a large readable panel|clean neutral screen light|soft white panels and thin separators
digital-workflow-proof|工作流证明|stepwise workflow,visible action,before after,clear consequence,proof rail,measured zoom|show one action and its observable consequence at readable scale|even interface fill|crisp UI surfaces and sparse annotations
digital-code-to-value|代码到价值|code proof,developer rhythm,terminal contrast,visible result,concise headline,technical trust|connect a small code surface to a large visible result|low ambient and clear screen|dark graphite panels and signal ink
digital-dashboard-signal|仪表信号|dashboard focus,large metric,quiet grid,credible data,signal hierarchy,precise callout|isolate one meaningful metric from a larger factual dashboard|neutral data-display light|dark glass and calibrated accent lines
digital-mobile-gesture|移动手势|mobile viewport,gesture path,close framing,clear tap,screen continuity,light depth|use one mobile-sized screen and one explicit interaction route|soft mobile studio|rounded glass and clean polymer
digital-startup-launch|新创发布|concise promise,confident launch,bold headline,fast proof,clean payoff,controlled energy|lead with one promise then fill the frame with its evidence|bright launch-stage fill|clean panels and saturated accent blocks
digital-creator-toolkit|创作工具|creative output,tool detail,layered workspace,craft rhythm,visible result,playful precision|let a real output dominate while tools support its creation story|soft creative studio|paper-like panels and colorful output surfaces
digital-commerce-premium|精品电商|product fidelity,clear offer,tactile detail,calm comparison,trustworthy scale,quiet premium|balance true product detail with a short readable offer|warm retail softbox|matte packaging and tactile stock
digital-scientific-discovery|科学发现|scientific clarity,experimental proof,ordered diagram,precise label,quiet wonder,measured reveal|move from one observed detail to a legible explanatory structure|cool lab-like fill|clear glass and precise technical marks
digital-data-story|数据叙事|data narrative,one takeaway,clear comparison,ordered reveal,credible scale,editorial chart|give one factual comparison the visual weight of a headline|flat readable graphic light|clean chart surfaces and controlled signal color`

func main() {
	out := flag.String("out", "assets/styles", "new/empty style catalog directory")
	flag.Parse()
	if entries, err := os.ReadDir(*out); err == nil && len(entries) > 0 {
		panic("refusing nonempty destination")
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	families := strings.Fields("keynote luxury tech editorial game anime cinema organic graphic digital")
	styleColors := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(palettes), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 6 {
			panic("invalid explicit palette")
		}
		if _, ok := styleColors[fields[0]]; ok {
			panic("duplicate explicit palette")
		}
		styleColors[fields[0]] = fields[1:]
	}
	if len(styleColors) != 100 {
		panic("100 explicit style palettes required")
	}
	bases := [][]string{{"keynote-product-porcelain", "keynote-type-porcelain", "keynote-features-porcelain", "keynote-material-porcelain", "keynote-gallery-porcelain"}, {"keynote-material-obsidian", "keynote-product-obsidian", "keynote-gallery-graphite"}, {"keynote-features-graphite", "keynote-product-midnight", "keynote-material-midnight"}, {"keynote-type-porcelain", "keynote-gallery-porcelain", "keynote-features-monochrome"}, {"game-launch-midnight", "game-combat-ember", "game-world-midnight", "game-event-graphite"}, {"character-reveal-midnight", "character-story-sakura", "character-profile-midnight"}, {"game-world-graphite", "character-story-midnight", "keynote-gallery-obsidian"}, {"keynote-material-porcelain", "keynote-product-porcelain", "keynote-gallery-sakura"}, {"keynote-type-sakura", "brand-logo-aurora", "social-teaser-ember"}, {"keynote-features-porcelain", "keynote-product-graphite", "keynote-gallery-porcelain"}}
	motions := strings.Fields("none push-in pull-out drift-left drift-right drift-up orbit float tilt rise")
	bgs := strings.Fields("solid mesh aurora nebula grid particles rings caustics fluid prism horizon silk")
	overlays := strings.Fields("none frame registration scanlines halftone letterbox crosshair corner-ticks contours dot-field")
	eases := strings.Fields("inOutCubic outExpo inOutQuint swift settle spring linear")
	transitions := [][]string{{"crossfade", "blur-cross", "zoom-in"}, {"crossfade", "light-sweep", "iris"}, {"soft-wipe", "center-wipe", "lens"}, {"cut", "page-turn", "crossfade"}, {"cut", "chromatic", "shutter"}, {"ink", "prism", "crossfade"}, {"dip-black", "crossfade", "exposure"}, {"ripple", "noise-dissolve", "soft-wipe"}, {"split", "diagonal", "pixelate"}, {"push-left", "zoom-in", "crossfade"}}
	layouts := [][]string{{"center", "left"}, {"asymmetric", "center"}, {"left", "right"}, {"left", "asymmetric"}, {"asymmetric", "left"}, {"left", "center"}, {"left", "asymmetric"}, {"center", "left"}, {"center", "asymmetric"}, {"left", "right"}}
	for i, line := range strings.Split(strings.TrimSpace(seeds), "\n") {
		p := strings.Split(line, "|")
		if len(p) != 6 {
			panic(fmt.Sprintf("row %d", i))
		}
		family, variant := i/10, i%10
		if family >= len(families) {
			panic("too many styles")
		}
		c, ok := styleColors[p[0]]
		if !ok {
			panic("missing palette " + p[0])
		}
		font := "sans"
		if family == 1 || family == 5 || family == 6 || variant == 1 {
			font = "display"
		}
		// Every recipe changes multiple structural axes; colors alone never define a style.
		bg := []string{bgs[(family*3+variant)%len(bgs)], bgs[(family+variant+1)%len(bgs)]}
		if family == 0 && variant < 3 {
			bg = []string{"solid", "mesh"}
		}
		tr := append([]string(nil), transitions[family]...)
		if variant%2 == 1 {
			tr[0], tr[1] = tr[1], tr[0]
		}
		direction := map[string]string{"composition": p[3] + "; keep one dominant focal hierarchy and legible supporting evidence.", "typography": "Use " + p[2] + " as hierarchy cues; keep display copy short and body copy readable at final delivery size.", "lighting": p[4] + "; preserve subject contour and separate it from the supporting field.", "materials": p[5] + "; actual material appearance comes from approved imagery or a real 3D asset.", "camera": "Guide attention with " + strings.Split(p[2], ",")[0] + "; use a deliberate hold, a measured approach and a stable payoff.", "editing": "Open with " + strings.Split(p[2], ",")[1] + "; alternate proof and breathing room, then resolve with " + strings.Split(p[2], ",")[5] + ".", "sound": "Create a restrained rhythm around " + strings.Split(p[2], ",")[4] + "; place licensed accents on intentional cuts and keep speech clear."}
		theme := map[string]any{"background": c[0], "foreground": c[1], "accent": c[2], "secondary": c[3], "surface": c[4], "font": font}
		render := map[string]any{"base_template": bases[family][variant%len(bases[family])], "theme": theme, "backgrounds": bg, "layouts": layouts[(family+variant)%len(layouts)], "motion": map[string]any{"name": motions[(family+variant)%len(motions)], "strength": .2 + float64(variant)*.07, "ease": eases[(family+variant)%len(eases)]}, "transitions": tr, "overlap_ratio": .08 + float64(variant%5)*.025, "typography": map[string]any{"scale": .8 + float64(variant)*.025, "weight_offset": -100 + (family*30+variant*20)%250, "tracking": float64((family+variant)%5) * .45, "leading": 1.05 + float64((family+variant)%4)*.06}, "graphics": map[string]any{"panel_radius": float64((family*7 + variant*11) % 65), "overlay": overlays[(family+variant)%len(overlays)], "opacity": .04 + float64(variant)*.009, "line_width": .8 + float64((family+variant)%5)*.3}}
		style := map[string]any{"schema_version": "1.0", "id": p[0], "name": p[1], "family": families[family], "summary": p[1] + "：" + p[3] + ". Original direction recipe; real footage and bespoke assets determine final visual fidelity.", "keywords": strings.Split(p[2], ","), "avoid": []string{"unmotivated decorative motion", "unreadable small copy", "invented product or gameplay proof"}, "best_for": []string{families[family] + " promotional video", "product or character identity reveal"}, "direction": direction, "beat_plan": []string{"Hook: " + strings.Split(p[2], ",")[0], "Establish: " + p[3], "Prove: use an approved local capture or truthful designed-motion detail", "Payoff: " + strings.Split(p[2], ",")[5] + " with one clear final identity"}, "reference_ids": []string{"hyperframes-creative", "hyperframes-product-launch", "video-shotcraft", "product-demo-video"}, "render": render}
		raw, err := json.MarshalIndent(style, "", "  ")
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(*out, p[0]+".json"), append(raw, '\n'), 0644); err != nil {
			panic(err)
		}
	}
	fmt.Println("Wrote 100 original style recipes.")
}
