package main

// File is a pending change in the working tree.
type File struct {
	Path string
	Size int64
}

// chunkFiles groups files into chunks respecting maxFiles and maxSize.
// A zero criterion means unlimited. A single file larger than maxSize
// still gets its own chunk, since a file cannot be split.
func chunkFiles(files []File, maxFiles int, maxSize int64) [][]File {
	var chunks [][]File
	var current []File
	var currentSize int64

	for _, f := range files {
		tooMany := maxFiles > 0 && len(current) >= maxFiles
		tooBig := maxSize > 0 && len(current) > 0 && currentSize+f.Size > maxSize
		if tooMany || tooBig {
			chunks = append(chunks, current)
			current, currentSize = nil, 0
		}
		current = append(current, f)
		currentSize += f.Size
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

func chunkSize(chunk []File) int64 {
	var total int64
	for _, f := range chunk {
		total += f.Size
	}
	return total
}
