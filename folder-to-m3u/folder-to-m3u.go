// Playlist Generator for Jellyfin
//
// This Go program scans a given root music directory and automatically creates
// `.m3u` playlists for each top-level subfolder. The generated playlists are
// placed inside a `playlists/` folder within the root directory.
//
// Example directory structure:
//
//   Music/
//     playlists/          <- generated playlists go here
//     BenHoward/
//       Album1/
//         track1.mp3
//         track2.mp3
//       Album2/
//         song.mp3
//     Chill/
//       example.mp3
//
// After running the program, you will get:
//
//   Music/playlists/BenHoward.m3u
//   Music/playlists/Chill.m3u
//
// Each `.m3u` file contains relative paths (from the playlists folder) to all
// audio files in the corresponding subfolder, including nested albums.
// Files are sorted alphabetically for predictable ordering.
//
// Usage:
//   go run main.go /path/to/Music
//
// Supported file types: .mp3, .flac, .wav, .ogg, .m4a
//
// This is designed so Jellyfin can correctly load and resolve the playlists.
//
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	// Require exactly one argument (the root folder)
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <music-root>")
		os.Exit(1)
	}
	root := os.Args[1]

	// Verify root exists and is a directory
	info, err := os.Stat(root)
	if err != nil {
		fmt.Println("Error: cannot access root folder:", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Println("Error: root path is not a directory")
		os.Exit(1)
	}

	playlistsDir := filepath.Join(root, "playlists")
	if err := os.MkdirAll(playlistsDir, 0755); err != nil {
		fmt.Println("Error creating playlists directory:", err)
		os.Exit(1)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		fmt.Println("Error reading root directory:", err)
		os.Exit(1)
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "playlists" {
			folderName := entry.Name()
			playlistPath := filepath.Join(playlistsDir, folderName+".m3u")

			var tracks []string
			err := filepath.WalkDir(filepath.Join(root, folderName), func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && isAudioFile(path) {
					// Compute relative path from playlistsDir
					rel, err := filepath.Rel(playlistsDir, path)
					if err != nil {
						return err
					}
					tracks = append(tracks, rel)
				}
				return nil
			})
			if err != nil {
				fmt.Println("Error walking directory:", err)
				continue
			}

			// Sort tracks alphabetically (includes nested folders)
			sort.Strings(tracks)

			if err := writeM3U(playlistPath, tracks); err != nil {
				fmt.Println("Error writing playlist:", err)
			} else {
				fmt.Println("Created playlist:", playlistPath)
			}
		}
	}
}

func isAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3", ".flac", ".wav", ".ogg", ".m4a":
		return true
	}
	return false
}

func writeM3U(filename string, tracks []string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("#EXTM3U\n")
	if err != nil {
		return err
	}

	for _, track := range tracks {
		_, err := f.WriteString(track + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}
