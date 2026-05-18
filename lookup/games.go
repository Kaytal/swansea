package lookup

import "swansea/store"

// GameFor returns the lookup function for the given source name.
// Only ScreenScraper is supported; source is reserved for future additional sources.
func GameFor(source string) func(string) (*store.VideoGameInput, error) {
	return ScreenScraperByTitle
}
