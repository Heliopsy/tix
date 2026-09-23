// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import "hash/fnv"

// FriendlyName renders an opaque identifier as a stable two-word name, such as
// "brave-otter", for the places a person has to read one.
//
// The name is derived from the identifier's own bytes with FNV-1a, a fixed
// algorithm carrying no seed and no state, so one identifier yields one name
// in every process, on every machine and for all time. It is a display aid
// and never a replacement: the identifier stays the identifier wherever a
// machine reads one.
func FriendlyName(id string) string {
	if id == "" {
		return ""
	}
	sum := fnv.New64a()
	_, _ = sum.Write([]byte(id))
	h := sum.Sum64()
	adjectives, animals := uint64(len(friendlyAdjectives)), uint64(len(friendlyAnimals))
	return friendlyAdjectives[h%adjectives] + "-" + friendlyAnimals[h/adjectives%animals]
}

// FriendlyNames is how many distinct names the scheme can produce.
func FriendlyNames() int { return len(friendlyAdjectives) * len(friendlyAnimals) }

// friendlyAdjectives is the first half of a generated name. Adding, removing
// or reordering a word renames every identifier, so the list is append-only in
// spirit: it is a stable mapping, not a vocabulary to curate.
var friendlyAdjectives = []string{
	"amber", "ancient", "arctic", "astral", "autumn", "azure", "balmy", "bold", "bouncy", "brave", "brazen", "breezy", "briar", "bright", "brisk", "bronze",
	"bubbly", "burly", "calm", "candid", "canny", "carefree", "cheery", "chipper", "chrome", "civic", "classic", "clever", "cobalt", "coral", "cosmic", "cozy",
	"crafty", "crimson", "crisp", "crystal", "curious", "daring", "dapper", "dawn", "deft", "dewy", "diligent", "dreamy", "driven", "dusky", "dusty", "eager",
	"earnest", "earthy", "easy", "electric", "elegant", "ember", "emerald", "endless", "epic", "eternal", "fabled", "fair", "falcon", "fancy", "fearless", "feisty",
	"fern", "fiery", "fleet", "fluent", "fluffy", "fond", "forest", "frank", "free", "fresh", "frosty", "gallant", "gentle", "giddy", "gifted", "gilded",
	"glad", "gleaming", "glossy", "golden", "graceful", "grand", "granite", "grassy", "gritty", "hardy", "harmony", "hazel", "hearty", "helpful", "hidden", "honest",
	"hopeful", "humble", "icy", "indigo", "ivory", "jade", "jaunty", "jolly", "jovial", "joyful", "keen", "kind", "laughing", "lavender", "leafy", "lively",
	"loyal", "lucid", "lucky", "lunar", "lush", "magenta", "marble", "mellow", "merry", "mighty", "mindful", "minty", "misty", "modest", "morning", "mossy",
	"noble", "nimble", "northern", "olive", "opal", "orchid", "patient", "peaceful", "pearl", "peppy", "perky", "placid", "plucky", "polar", "polished", "prairie",
	"prime", "prompt", "proud", "pure", "quaint", "quick", "quiet", "radiant", "rapid", "rare", "ready", "regal", "restless", "rising", "river", "robust",
	"rosy", "rugged", "rustic", "sable", "sage", "sandy", "sapphire", "scarlet", "seaside", "secret", "serene", "shining", "shy", "silent", "silken", "silver",
	"sincere", "sleek", "slender", "smart", "smiling", "smooth", "snappy", "snowy", "solar", "solid", "sonic", "soothing", "sparkling", "spirited", "spry", "stalwart",
	"steady", "stellar", "sterling", "still", "stony", "storied", "stormy", "stout", "sturdy", "sunny", "sunset", "swift", "tender", "thoughtful", "thrifty", "tidal",
	"tidy", "timber", "tireless", "topaz", "tranquil", "trusty", "twilight", "umber", "upbeat", "urban", "valiant", "velvet", "verdant", "vermilion", "vibrant", "vigilant",
	"violet", "vivid", "wandering", "warm", "watchful", "wavy", "welcome", "western", "whimsical", "whispering", "willing", "windy", "winsome", "wise", "witty", "wondrous",
	"woodland", "woolly", "worthy", "zealous", "zesty", "zippy", "alpine", "autumnal", "boreal", "coastal", "crescent", "daybreak", "driftwood", "eastern", "evening", "ferrous",
}

// friendlyAnimals is the second half of a generated name, and carries the same
// stability guarantee as the adjectives.
var friendlyAnimals = []string{
	"albatross", "alpaca", "anteater", "antelope", "armadillo", "auk", "avocet", "axolotl", "badger", "barnacle", "barracuda", "basilisk", "bass", "bat", "beagle", "bear",
	"beaver", "bee", "beetle", "bison", "bittern", "blackbird", "bluebird", "boar", "bobcat", "bonobo", "booby", "bowerbird", "bream", "buffalo", "bullfinch", "bumblebee",
	"bunting", "bushbaby", "bustard", "butterfly", "buzzard", "caiman", "camel", "capybara", "caracal", "cardinal", "caribou", "carp", "cassowary", "cat", "caterpillar", "catfish",
	"chameleon", "chamois", "cheetah", "chickadee", "chinchilla", "chipmunk", "chough", "cicada", "civet", "clam", "cobra", "cockatoo", "colt", "condor", "conger", "coot",
	"crab", "cormorant", "corncrake", "cougar", "cow", "coyote", "crane", "crayfish", "cricket", "crow", "cuckoo", "curlew", "cuttlefish", "deer", "dingo", "dipper",
	"dodo", "dolphin", "donkey", "dormouse", "dove", "dragonfly", "drake", "duck", "dugong", "dunlin", "eagle", "earwig", "echidna", "eel", "egret", "eider",
	"eland", "elephant", "elk", "emu", "ermine", "falcon", "fennec", "ferret", "finch", "firefly", "fisher", "flamingo", "flounder", "fossa", "fox", "frigate",
	"frog", "gannet", "gar", "gazelle", "gecko", "gerbil", "gibbon", "giraffe", "glowworm", "gnat", "gnu", "goat", "godwit", "goldcrest", "goldfinch", "goose",
	"gopher", "goral", "gorilla", "goshawk", "grackle", "grebe", "greenfinch", "grouse", "guanaco", "guillemot", "gull", "guppy", "gyrfalcon", "haddock", "hake", "halibut",
	"hamster", "hare", "harrier", "hawk", "hedgehog", "heron", "herring", "hoopoe", "hornbill", "hornet", "horse", "hound", "hummingbird", "hyena", "hyrax", "ibex",
	"ibis", "iguana", "impala", "indri", "jackal", "jackdaw", "jaguar", "jay", "jellyfish", "jerboa", "kakapo", "kangaroo", "katydid", "kestrel", "killdeer", "kingfisher",
	"kinkajou", "kite", "kiwi", "koala", "kodiak", "koi", "krill", "kudu", "ladybird", "lamprey", "langur", "lapwing", "lark", "lemming", "lemur", "leopard",
	"limpet", "linnet", "lion", "lizard", "llama", "lobster", "locust", "loon", "loris", "lynx", "macaque", "macaw", "mackerel", "magpie", "mallard", "manatee",
	"mandrill", "mantis", "marlin", "marmot", "marten", "meerkat", "merlin", "minnow", "mink", "mole", "mongoose", "monkey", "moorhen", "moose", "moth", "mouse",
	"mudskipper", "mule", "muskrat", "narwhal", "newt", "nightingale", "nuthatch", "ocelot", "octopus", "okapi", "opossum", "orca", "oriole", "oryx", "osprey", "ostrich",
	"otter", "owl", "ox", "oyster", "panda", "pangolin", "panther", "parakeet", "parrot", "partridge", "peacock", "pelican", "penguin", "perch", "petrel", "pheasant",
}
