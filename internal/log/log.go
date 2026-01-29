package log

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
)

const (
	completionMarker uint32 = 0xDEADBEEF
	metaTmpFile             = "log.meta.tmp"

	entryHeaderSize = 8 + 4 // index(uint64) + length(uint32)
	trailerSize     = 4 + 4 // checksum + marker
)

type Log struct {
	dir         string
	dataFile    *os.File
	metaFile    *os.File
	lastIndex   uint64
	commitIndex uint64
}

type Entry struct {
	Index    uint64
	Payload  []byte
	Checksum uint32
}

func Open(dir string) (*Log, error) {
	dataPath := dir + "/log.data"
	metaPath := dir + "/log.meta"

	dataFile, err := os.OpenFile(dataPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	l := &Log{
		dir:      dir,
		dataFile: dataFile,
	}

	// Executes crash recovery routine and caches Log.lastIndex
	if err := l.recoverData(); err != nil {
		return nil, err
	}

	// Caches Log.commitIndex
	if err := l.openMeta(metaPath); err != nil {
		return nil, err
	}

	if l.commitIndex > l.lastIndex {
		return nil, errors.New("commit index beyond last log entry")
	}

	return l, nil
}

// Method for crash simulation testing
func (l *Log) Close() {
	if l.dataFile != nil {
		l.dataFile.Close()
	}
	if l.metaFile != nil {
		l.metaFile.Close()
	}
}

func (l *Log) recoverData() error {
	offset := int64(0)
	l.lastIndex = 0

	for {
		header := make([]byte, entryHeaderSize)
		_, err := l.dataFile.ReadAt(header, offset)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		index := binary.BigEndian.Uint64(header[0:8])
		length := binary.BigEndian.Uint32(header[8:12])
		// unused uint32 4 bytes (padding / future)

		entrySize := int64(entryHeaderSize + length + trailerSize)
		buf := make([]byte, entrySize)

		_, err = l.dataFile.ReadAt(buf, offset)
		if err != nil {
			break
		}

		checksum := binary.BigEndian.Uint32(buf[entryHeaderSize+length : entryHeaderSize+length+4])
		marker := binary.BigEndian.Uint32(buf[entryHeaderSize+length+4:])

		calculated := crc32.ChecksumIEEE(buf[:entryHeaderSize+length])
		if checksum != calculated || marker != completionMarker {
			break
		}

		l.lastIndex = index
		offset += entrySize
	}

	if err := l.dataFile.Truncate(offset); err != nil {
		return err
	}
	if err := l.dataFile.Sync(); err != nil {
		return err
	}
	return nil
}

func (l *Log) openMeta(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	l.metaFile = f

	buf := make([]byte, 8)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return err
	}
	if n == 8 {
		l.commitIndex = binary.BigEndian.Uint64(buf)
	}
	return nil
}

func (l *Log) Append(index uint64, payload []byte) error {
	if index != l.lastIndex+1 {
		return errors.New("index out of order")
	}

	buf := make([]byte, entryHeaderSize+len(payload))
	binary.BigEndian.PutUint64(buf[0:8], index)
	binary.BigEndian.PutUint32(buf[8:12], uint32(len(payload)))
	copy(buf[entryHeaderSize:], payload)

	// Write and fsync header + payload
	if _, err := l.dataFile.Write(buf); err != nil {
		return err
	}
	if err := l.dataFile.Sync(); err != nil {
		return err
	}

	checksum := crc32.ChecksumIEEE(buf)
	markerBuf := make([]byte, 8)
	binary.BigEndian.PutUint32(markerBuf[0:4], checksum)
	binary.BigEndian.PutUint32(markerBuf[4:8], completionMarker)

	if _, err := l.dataFile.Write(markerBuf); err != nil {
		return err
	}
	if err := l.dataFile.Sync(); err != nil {
		return err
	}

	l.lastIndex = index
	return nil
}

func (l *Log) Commit(index uint64) error {
	if index <= l.commitIndex {
		return nil
	}
	if index > l.lastIndex {
		return errors.New("commit beyond last index")
	}

	tmpPath := l.dir + "/" + metaTmpFile
	tmp, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, index)
	if _, err := tmp.Write(buf); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	tmp.Close()

	if err := os.Rename(tmpPath, l.metaFile.Name()); err != nil {
		return err
	}

	dirFd, err := os.Open(l.dir)
	if err == nil {
		dirFd.Sync()
		dirFd.Close()
	}

	l.commitIndex = index
	return nil
}

func (l *Log) LastIndex() uint64 {
	return l.lastIndex
}

func (l *Log) CommitIndex() uint64 {
	return l.commitIndex
}

func (l *Log) Read(target uint64) (Entry, error) {
	if target == 0 || target > l.lastIndex {
		return Entry{}, errors.New("entry does not exist")
	}

	offset := int64(0)

	for {
		header := make([]byte, entryHeaderSize)
		_, err := l.dataFile.ReadAt(header, offset)
		if err != nil {
			return Entry{}, err
		}

		index := binary.BigEndian.Uint64(header[0:8])
		length := binary.BigEndian.Uint32(header[8:12])

		entrySize := int64(entryHeaderSize + length + trailerSize)
		buf := make([]byte, entrySize)

		_, err = l.dataFile.ReadAt(buf, offset)
		if err != nil {
			return Entry{}, err
		}

		checksum := binary.BigEndian.Uint32(buf[entryHeaderSize+length : entryHeaderSize+length+4])
		marker := binary.BigEndian.Uint32(buf[entryHeaderSize+length+4:])

		calculated := crc32.ChecksumIEEE(buf[:entryHeaderSize+length])
		if checksum != calculated || marker != completionMarker {
			return Entry{}, errors.New("corrupted entry")
		}

		if index == target {
			payload := make([]byte, length)
			copy(payload, buf[entryHeaderSize:entryHeaderSize+length])

			return Entry{
				Index:    index,
				Payload:  payload,
				Checksum: checksum,
			}, nil
		}

		offset += entrySize
	}
}

func (l *Log) TruncateFrom(index uint64) error {
	if index <= l.commitIndex {
		return errors.New("cannot truncate committed entries")
	}

	if index > l.lastIndex {
		return nil
	}

	offset := int64(0)
	newLast := uint64(0)

	for {
		header := make([]byte, entryHeaderSize)
		_, err := l.dataFile.ReadAt(header, offset)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		entryIndex := binary.BigEndian.Uint64(header[0:8])
		length := binary.BigEndian.Uint32(header[8:12])

		entrySize := int64(entryHeaderSize + length + trailerSize)

		if entryIndex >= index {
			break
		}

		newLast = entryIndex
		offset += entrySize
	}

	if err := l.dataFile.Truncate(offset); err != nil {
		return err
	}
	if err := l.dataFile.Sync(); err != nil {
		return err
	}

	l.lastIndex = newLast
	return nil
}
