// cataloggen seeds independent editable JSON presets. Runtime reads the files,
// not this generator. Regeneration is an explicit maintainer operation.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type obj = map[string]any

func write(dir, id string, data any) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(err)
	}
	dataBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), append(dataBytes, '\n'), 0644); err != nil {
		panic(err)
	}
}
func main() {
	root := "assets/catalog"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	names := []string{"cut|直接切换", "crossfade|柔和叠化", "dip-black|暗场呼吸", "dip-white|白场过渡", "slide|滑入", "push|画面推移", "cover|层叠覆盖", "reveal|抽离揭示", "zoom-in|聚焦推进", "zoom-out|拉远换景", "iris|圆形揭幕", "diamond|菱形遮罩", "diagonal|斜向切面", "blinds|条栅展开", "checker|棋盘分区", "pixelate|像素聚散", "radial|径向扫描", "ripple|涟漪折射", "swirl|漩涡换景", "glitch|数字错位", "burn|边缘燃蚀", "noise-dissolve|颗粒溶解", "displacement|流体位移", "chromatic|色散推镜", "blur-cross|散焦叠化", "spin|旋转换景", "shutter|双侧快门", "split|画面分裂", "luma|亮度溶解", "circle-morph|圆形形变", "bars|阶梯条带", "venetian|百叶窗", "door|双扇开门", "tilt|透视倾转", "page-turn|翻页换景", "exposure|曝光过渡", "light-sweep|光带扫场", "ink|水墨扩散", "cells|细胞扩散", "hexagon|蜂巢展开", "wave|波浪擦除", "arc|弧线展开", "soft-wipe|柔边擦除", "center-wipe|中轴展开", "lens|镜头折射", "prism|棱镜分色", "halftone|网点揭示", "film-roll|胶片卷动"}
	transitions := []string{}
	dirs := []struct {
		name, zh string
		x, y     float64
	}{{"left", "向左", -1, 0}, {"right", "向右", 1, 0}, {"up", "向上", 0, -1}, {"down", "向下", 0, 1}, {"up-left", "左上", -1, -1}, {"up-right", "右上", 1, -1}, {"down-left", "左下", -1, 1}, {"down-right", "右下", 1, 1}}
	for mode, entry := range names {
		parts := strings.Split(entry, "|")
		variants := dirs[:1]
		if mode >= 4 && mode <= 7 {
			variants = dirs
		}
		for _, d := range variants {
			id := parts[0]
			name := parts[1]
			if len(variants) > 1 {
				id += "-" + d.name
				name += " · " + d.zh
			}
			category := "optical"
			if mode < 4 {
				category = "editorial"
			} else if mode < 17 {
				category = "geometry"
			} else if mode >= 36 {
				category = "shader"
			}
			write(filepath.Join(root, "transitions"), id, obj{"schema_version": "1.0", "id": id, "name": name, "category": category, "mode": mode, "direction": []float64{d.x, d.y}, "strength": 0.65, "softness": 0.065, "tiles": 12, "ease": "inOutQuint"})
			transitions = append(transitions, id)
		}
	}
	palettes := []struct{ id, zh, bg, fg, a, b, s string }{{"obsidian", "曜石", "#08090d", "#f4f5f7", "#b9d6ff", "#8573ff", "#181b23"}, {"porcelain", "瓷白", "#f2f3f5", "#17191e", "#1368de", "#8caef8", "#ffffff"}, {"graphite", "石墨", "#171a1d", "#eceff1", "#c5c9cf", "#7c8c9f", "#282e35"}, {"midnight", "深海", "#040b16", "#e1edf3", "#77cef2", "#bca579", "#13293c"}, {"aurora", "极光", "#051616", "#edfff8", "#6aebbd", "#7a80e8", "#143836"}, {"ember", "余烬", "#160b0a", "#fff0df", "#ffa76c", "#e65a63", "#35201c"}, {"sakura", "樱雾", "#1a101a", "#fff2fb", "#eeb0d8", "#988bff", "#382638"}, {"monochrome", "黑白", "#060606", "#fafafa", "#ffffff", "#8e8e8e", "#202020"}}
	families := []struct {
		id, zh, cat, title string
		types              []string
	}{
		{"keynote-product", "发布会 · 产品揭幕", "keynote", "AURA", strings.Fields("hero-device macro feature-focus bento spec-grid endcard")},
		{"keynote-type", "发布会 · 巨型文字", "keynote", "ONE MORE IDEA", strings.Fields("giant-type kinetic-type quote title endcard")},
		{"keynote-features", "发布会 · 功能编排", "keynote", "CREATE MORE", strings.Fields("title bento stat-cards compare endcard")},
		{"keynote-material", "发布会 · 材质与形态", "keynote", "FORM IN MOTION", strings.Fields("macro product-spin material-panels logo endcard")},
		{"keynote-gallery", "发布会 · 影像长廊", "keynote", "A NEW PERSPECTIVE", strings.Fields("gallery zoom-gallery film-strip split-screen endcard")},
		{"game-launch", "游戏 · 发售宣传", "game", "STELLAR ECHO", strings.Fields("image-hero chapter action-panel bento title endcard")},
		{"game-world", "游戏 · 世界观序章", "game", "BEYOND THE HORIZON", strings.Fields("landscape map-trace quote image-hero title")},
		{"game-combat", "游戏 · 战斗节奏", "game", "BREAK THE SILENCE", strings.Fields("impact action-panel split-screen stat-cards title")},
		{"game-event", "游戏 · 活动更新", "game", "NEXT CHAPTER", strings.Fields("countdown chapter bento image-hero endcard")},
		{"character-reveal", "角色 · 身份揭示", "character", "ASTERIA", strings.Fields("character-detail character-name signature title endcard")},
		{"character-profile", "角色 · 设定展示", "character", "THE LAST ORBIT", strings.Fields("image-hero profile-grid quote signature title")},
		{"character-story", "角色 · 情绪叙事", "character", "THE NIGHT REMEMBERS", strings.Fields("chapter character-detail quote character-name endcard")},
		{"website-atmosphere", "官网 · 氛围循环", "website", "STELLAR ECHO", strings.Fields("ambient")},
		{"website-product", "官网 · 产品主视觉", "website", "AURA", strings.Fields("orbital")},
		{"social-teaser", "社媒 · 短篇预告", "social", "MAKE IT COUNT", strings.Fields("giant-type image-hero impact endcard")},
		{"brand-logo", "品牌 · 标识演绎", "brand", "ORBIT", strings.Fields("logo monogram title endcard")},
	}
	backgrounds := strings.Fields("solid mesh aurora nebula grid particles rings caustics fluid prism horizon silk")
	motions := strings.Fields("push-in pull-out drift-left drift-right drift-up orbit float tilt rise stagger")
	count := 0
	for fi, f := range families {
		for pi, p := range palettes {
			id := f.id + "-" + p.id
			scenes := []obj{}
			for si, kind := range f.types {
				duration := 3.6 + float64((fi+si)%3)*0.4
				transitionDuration := 0.7
				if len(f.types) == 1 {
					duration = 6
					transitionDuration = 0
				}
				image := ""
				if f.cat == "game" || f.cat == "character" || kind == "gallery" || kind == "zoom-gallery" || kind == "film-strip" || kind == "image-hero" || kind == "split-screen" {
					image = "astral-knight.png"
				}
				overline := "ORIGINAL MOTION STUDY"
				title := "{{title}}"
				subtitle := "Design. Motion. Possibility."
				switch kind {
				case "character-name":
					overline = "KEEPER OF THE LAST ORBIT"
					title = "ASTERIA"
					subtitle = "Even the dark has a guiding star."
				case "quote":
					title = "Every detail has a purpose."
					subtitle = "A moment worth remembering."
				case "bento":
					title = "Everything, in harmony."
				case "macro":
					title = "Crafted to the last detail."
				case "feature-focus":
					title = "One idea. Beautifully clear."
				case "countdown":
					title = "03"
					subtitle = "A NEW CHAPTER"
				case "endcard":
					subtitle = "A FICTIONAL CONCEPT / SOURCE-EDITABLE TEMPLATE"
				case "chapter":
					overline = "CHAPTER 01"
					title = "THE WORLD BEYOND"
				case "signature":
					title = "ORBITAL RESONANCE"
					subtitle = "An original character key-art study."
				}
				items := []obj{{"title": "Form", "body": "Refined from every angle.", "value": "01"}, {"title": "Light", "body": "A controlled focal point.", "value": "02"}, {"title": "Motion", "body": "Every beat with intention.", "value": "03"}, {"title": "Detail", "body": "Make the important visible.", "value": "04"}}
				transition := transitions[(fi*13+pi*7+si*3)%len(transitions)]
				background := backgrounds[(fi+pi+si)%len(backgrounds)]
				if id == "keynote-product-porcelain" {
					transition = []string{"crossfade", "zoom-in", "soft-wipe", "crossfade", "lens", "crossfade"}[si]
					background = []string{"solid", "mesh", "solid", "solid", "mesh", "solid"}[si]
				}
				scenes = append(scenes, obj{"id": fmt.Sprintf("scene-%02d", si+1), "type": kind, "duration": duration, "title": title, "subtitle": subtitle, "overline": overline, "image": image, "background": background, "layout": []string{"center", "left", "right", "asymmetric"}[(pi+si)%4], "items": items, "motion": obj{"name": motions[(fi+pi+si)%len(motions)], "strength": 0.45 + float64(pi%3)*0.15, "ease": []string{"inOutCubic", "outExpo", "inOutQuint", "settle"}[(pi+si)%4]}, "transition": transition, "transition_duration": transitionDuration})
			}
			font := "sans"
			if f.cat == "character" || f.id == "game-world" {
				font = "display"
			}
			write(filepath.Join(root, "templates"), id, obj{"schema_version": "1.0", "id": id, "name": f.zh + " / " + p.zh, "category": f.cat, "description": "Independent editable cinematic sequence; theme, copy, images, scene rhythm and transitions live in this JSON. Fictional sample content, not product/gameplay evidence.", "project_title": f.title, "width": 1920, "height": 1080, "fps": 30, "loop": len(f.types) == 1, "theme": obj{"background": p.bg, "foreground": p.fg, "accent": p.a, "secondary": p.b, "surface": p.s, "font": font}, "scenes": scenes})
			count++
		}
	}
	fmt.Printf("Wrote %d template files and %d transition files\n", count, len(transitions))
}
