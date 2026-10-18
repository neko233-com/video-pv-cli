package main

import (
	"fmt"
	"io"
	"sort"
)

func styleCases(templates map[string]TemplateData, transitions map[string]TransitionData, styles map[string]StylePreset, deep bool) []AtlasCase {
	ids := make([]string, 0, len(styles))
	for id := range styles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	cases := []AtlasCase{}
	for _, id := range ids {
		s := styles[id]
		t := applyStyle(templates[s.Render.BaseTemplate], s)
		for i, at := range []float64{0, templateDuration(t) / 2, templateDuration(t) - 1/t.FPS} {
			cases = append(cases, AtlasCase{ID: fmt.Sprintf("style/%s/%d", id, i), Template: t, Time: at})
		}
		if deep {
			loop := applyStyle(templates["website-atmosphere-midnight"], s)
			loop.Scenes[0].Title = ""
			loop.Scenes[0].Subtitle = ""
			loop.Scenes[0].Overline = ""
			cases = append(cases, AtlasCase{ID: "style/" + id + "/loop-start", Template: loop, Time: 0}, AtlasCase{ID: "style/" + id + "/loop-end", Template: loop, Time: templateDuration(loop)})
		}
	}
	return cases
}

func runStyleVerify(args []string, stdout, stderr io.Writer) int {
	return runCatalogVerify(append([]string{"--styles-only"}, args...), stdout, stderr)
}
