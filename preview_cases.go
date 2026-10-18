package main

import (
	"fmt"
	"sort"
)

type BrowserCase struct {
	Kind         string  `json:"kind"`
	ID           string  `json:"id"`
	Time         float64 `json:"time"`
	Frame        int     `json:"frame"`
	Repeat       bool    `json:"repeat"`
	AllowUniform bool    `json:"allow_uniform"`
}

// One adapter/pipeline for a whole browser audit prevents context accumulation.
func browserAtlas(templates map[string]TemplateData, transitions map[string]TransitionData) (AtlasData, []BrowserCase) {
	atlas := AtlasData{Kind: "atlas", ID: "edge-catalog-audit", Width: 1920, Height: 1080, FPS: 30}
	rows := []BrowserCase{}
	keys := []string{}
	for id := range templates {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		t := templates[id]
		for i, at := range []float64{0, templateDuration(t) / 2, templateDuration(t) - 1/t.FPS, 0} {
			frame := len(rows)
			atlas.Cases = append(atlas.Cases, AtlasCase{ID: fmt.Sprintf("%s/%d", id, i), Template: t, Time: at})
			rows = append(rows, BrowserCase{Kind: "template", ID: id, Time: at, Frame: frame, Repeat: i == 3, AllowUniform: templateUniformTransition(t, at, transitions)})
		}
	}
	keys = nil
	for id := range transitions {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	base := templates["keynote-product-porcelain"]
	base.Scenes = append([]SceneData(nil), base.Scenes[:2]...)
	base.Scenes[0].Title = "BEFORE"
	base.Scenes[1].Title = "AFTER"
	base.Scenes[0].Type = "hero-device"
	base.Scenes[1].Type = "bento"
	for _, id := range keys {
		for i, phase := range []float64{0, .25, .5, .75, 1, 0} {
			frame := len(rows)
			atlas.Cases = append(atlas.Cases, AtlasCase{ID: fmt.Sprintf("transition/%s/%d", id, i), Template: base, Transition: id, Phase: phase})
			rows = append(rows, BrowserCase{Kind: "transition", ID: id, Time: phase, Frame: frame, Repeat: i == 5})
		}
	}
	return atlas, rows
}

func templateUniformTransition(t TemplateData, at float64, transitions map[string]TransitionData) bool {
	start := 0.0
	for i, s := range t.Scenes {
		if i == len(t.Scenes)-1 {
			break
		}
		end := start + s.Duration
		incoming := end - s.TransitionDuration
		if s.Transition != "cut" && s.TransitionDuration > 0 && at >= incoming && at < end {
			mode := transitions[s.Transition].Mode
			return mode == 2 || mode == 3 || mode == 35
		}
		start = incoming
	}
	return false
}
