package naming

import (
	"hash/fnv"
	"strings"
)

// adjectives — positive, short (3-8 letter) adjectives.
var adjectives = []string{
	"Swift", "Bright", "Calm", "Dazzling", "Eager",
	"Fancy", "Gentle", "Honest", "Jolly", "Keen",
	"Lively", "Merry", "Noble", "Polite", "Quiet",
	"Rapid", "Silly", "Tender", "Unity", "Vivid",
	"Warm", "Yonder", "Zesty", "Brave", "Cherry",
	"Drift", "Ember", "Frost", "Glow", "Haze",
	"Indie", "Jade", "Kind", "Lunar", "Misty",
	"Nifty", "Opal", "Petal", "Quill", "Ridge",
	"Solar", "Thorn", "Umber", "Velvet", "Witty",
	"Amber", "Bliss", "Coral", "Dawn", "Elm",
	"Flint", "Grace", "Haven", "Ivy", "Joy",
	"Knot", "Leaf", "Marsh", "Nova", "Olive",
	"Pine", "Reed", "Sage", "Tide", "Unity",
	"Vine", "Wave", "Astral", "Breeze", "Cedar",
	"Dew", "Echo", "Fawn", "Glen", "Hill",
	"Iris", "Jest", "Kite", "Lark", "Mint",
	"Nest", "Owl", "Plum", "Rust", "Silk",
	"Teak", "Val", "Wren", "Aura", "Bay",
	"Crest", "Dusk", "Eve", "Fern", "Gold",
	"Hope", "Isle", "Jewel", "Lake", "Meadow",
	"Nut", "Oak", "Peak", "River", "Star",
	"Tower", "Urban", "Vast", "Wells", "Yew",
	"Zen", "Azure", "Brook", "Cloud", "Dale",
	"Eden", "Fair", "Gleam", "Halo", "Iron",
	"Jet", "Kay", "Lyn", "Max", "Neon",
	"Oryx", "Pax", "Ray", "Sky", "True",
	"Wise", "Yarn", "Zeal", "Bold", "Clear",
	"Cool", "Deep", "Fast", "Good", "Hard",
	"High", "Just", "Late", "Long", "Next",
	"Open", "Pure", "Rich", "Safe", "Sharp",
	"Soft", "Solid", "Sure", "Wild",
}

// nouns — positive, short (3-8 letter) nouns.
var nouns = []string{
	"Candy", "Daisy", "Fairy", "Gem", "Honey",
	"Lily", "Magic", "Peach", "Ruby", "Sugar",
	"Tulip", "Velvet", "Angel", "Blossom", "Charm",
	"Dew", "Feather", "Grace", "Harmony", "Jewel",
	"Lotus", "Meadow", "Opal", "Pearl", "Rose",
	"Sapphire", "Tiger", "Unity", "Violet", "Wish",
	"Amber", "Breeze", "Crystal", "Dream", "Echo",
	"Flame", "Glow", "Haven", "Iris", "Joy",
	"Kite", "Lark", "Moon", "Nova", "Owl",
	"Petal", "Quill", "Reed", "Star", "Thyme",
	"Umber", "Vine", "Wave", "Xenon", "Yew",
	"Zephyr", "Almond", "Birch", "Cedar", "Dove",
	"Elm", "Finch", "Gull", "Heron", "Indigo",
	"Jade", "Knot", "Lake", "Mint", "Nettle",
	"Onyx", "Pine", "Quartz", "Robin", "Sage",
	"Thrush", "Umbra", "Valley", "Willow", "Aspen",
	"Blaze", "Cobalt", "Drift", "Ember", "Frost",
	"Granite", "Haze", "Ivory", "Jasper", "Kelp",
	"Limestone", "Moss", "Nectar", "Obsidian", "Prism",
	"Ridge", "Silk", "Talon", "Vortex", "Wren",
	"Coral", "Delta", "Flint", "Glade", "Hollow",
	"Isle", "Jungle", "Marsh", "Nook", "Orchid",
	"Pond", "Ridge", "Shore", "Tide", "Upland",
	"Creek", "Den", "Field", "Grove", "Knoll",
	"Lagoon", "Mesa", "Pass", "Rill", "Spring",
	"Bolt", "Cipher", "Dart", "Edge", "Flow",
	"Gleam", "Hymn", "Jolt", "Key", "Lux",
	"Mark", "Nexus", "Orb", "Pulse", "Ring",
	"Spark", "Trace", "Unit", "Vault", "Wire",
	"Arrow", "Badge", "Crown", "Disk", "Expo",
	"Fork", "Grip", "Hook", "Icon", "Jam",
	"Kick", "Lamp", "Maze", "Note", "Oasis",
	"Path", "Quest", "Rally", "Scope", "Torch",
}

// AgentName generates a deterministic single-word name from an agent ID.
// The same ID always produces the same name.
func AgentName(id string) string {
	if id == "" {
		return ""
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte("agent:" + id))
	idx := h.Sum32() % uint32(len(nouns))
	return nouns[idx]
}

// SessionName generates a deterministic three-word PascalCase name from a session ID.
// Pattern: Adjective + Noun + Noun (e.g., "SwiftCrystalEcho").
// The same ID always produces the same name.
func SessionName(id string) string {
	if id == "" {
		return ""
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte("session:" + id))
	v := h.Sum32()

	adjIdx := v % uint32(len(adjectives))
	noun1Idx := (v / uint32(len(adjectives))) % uint32(len(nouns))
	noun2Idx := (v / uint32(len(adjectives)) / uint32(len(nouns))) % uint32(len(nouns))

	var b strings.Builder
	b.WriteString(adjectives[adjIdx])
	b.WriteString(nouns[noun1Idx])
	b.WriteString(nouns[noun2Idx])
	return b.String()
}
