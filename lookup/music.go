package lookup

import "swansea/store"

// MusicFor returns the lookup function for the given source name.
// Unknown sources default to gnudb.
func MusicFor(source string) func(string) (*store.MusicAlbumInput, error) {
	return GnudbByArtistTitle
}
