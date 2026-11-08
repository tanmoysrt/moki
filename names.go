package main

import (
	"fmt"
	"math/rand/v2"
)

// Token names such as "brave-otter". They tell agents apart in the log and in
// "moki token list", and each token starts with its name.

var adjectives = []string{
	"amber", "bold", "brave", "bright", "calm", "clever", "cosmic", "crisp",
	"dusty", "eager", "fuzzy", "gentle", "golden", "happy", "hidden", "humble",
	"jolly", "keen", "lively", "lucky", "mellow", "misty", "nimble", "noble",
	"plucky", "polite", "proud", "quick", "quiet", "rosy", "rusty", "shiny",
	"silent", "silver", "sleepy", "snowy", "solar", "spicy", "sunny", "swift",
	"tidy", "tiny", "velvet", "vivid", "wild", "witty", "woolly", "zesty",
}

var nouns = []string{
	"badger", "beacon", "birch", "comet", "coral", "crane", "falcon", "fern",
	"finch", "fox", "gecko", "heron", "ibis", "koala", "lark", "lemur",
	"lynx", "maple", "meadow", "moth", "newt", "otter", "owl", "panda",
	"pebble", "pine", "quail", "raven", "reef", "robin", "sparrow", "squid",
	"tapir", "thistle", "tiger", "toucan", "walrus", "willow", "wren", "yak",
}

// newName picks a name that no token in taken has. When random tries keep
// hitting taken names, it adds a number.
func newName(taken map[string]bool) string {
	for i := 0; ; i++ {
		name := pick(adjectives) + "-" + pick(nouns)
		if i >= 50 {
			name += fmt.Sprintf("-%d", rand.IntN(1000))
		}
		if !taken[name] {
			return name
		}
	}
}

func pick(words []string) string {
	return words[rand.IntN(len(words))]
}
