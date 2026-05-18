package lookup

import "swansea/store"

// GameFor returns the lookup function for the given source name.
// Unknown sources default to ScreenScraper.
func GameFor(source string) func(string) (*store.VideoGameInput, error) {
	return ScreenScraperByTitle
}
