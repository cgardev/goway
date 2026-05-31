package goway

import "hash/crc32"

// utf8ByteOrderMark is the three byte sequence that marks the start of some
// UTF-8 encoded files. It is removed from the first line before hashing so that
// the presence or absence of the marker does not change the checksum.
var utf8ByteOrderMark = []byte{0xEF, 0xBB, 0xBF}

// calculateChecksum computes the CRC32 checksum of a migration script using the
// same algorithm as Flyway. The content is split into lines, the byte order
// marker is stripped from the first line, and the raw bytes of each line are fed
// to the checksum without the line terminators. The result is interpreted as a
// signed 32 bit integer to match how Flyway stores the value.
func calculateChecksum(content []byte) int32 {
	digest := crc32.NewIEEE()
	for index, line := range splitChecksumLines(content) {
		if index == 0 {
			line = stripByteOrderMark(line)
		}
		digest.Write(line)
	}
	return int32(digest.Sum32())
}

// stripByteOrderMark removes a leading UTF-8 byte order marker from a line.
func stripByteOrderMark(line []byte) []byte {
	if len(line) >= len(utf8ByteOrderMark) &&
		line[0] == utf8ByteOrderMark[0] &&
		line[1] == utf8ByteOrderMark[1] &&
		line[2] == utf8ByteOrderMark[2] {
		return line[len(utf8ByteOrderMark):]
	}
	return line
}

// splitChecksumLines splits content into lines using the same rules as the Java
// BufferedReader.readLine method. A line is terminated by a line feed, a
// carriage return, or a carriage return followed by a line feed, and the
// terminators are not included in the returned lines. A trailing terminator
// does not produce an empty final line, while a terminator immediately followed
// by another terminator does produce an empty line.
func splitChecksumLines(content []byte) [][]byte {
	var lines [][]byte
	start := 0
	index := 0
	for index < len(content) {
		switch content[index] {
		case '\n':
			lines = append(lines, content[start:index])
			index++
			start = index
		case '\r':
			lines = append(lines, content[start:index])
			index++
			if index < len(content) && content[index] == '\n' {
				index++
			}
			start = index
		default:
			index++
		}
	}
	if start < len(content) {
		lines = append(lines, content[start:])
	}
	return lines
}
