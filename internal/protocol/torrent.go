package protocol

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"

	"github.com/HeartBtz/tempest/internal/protocol/bencode"
)

type TorrentFile struct {
	Name        string   `json:"name"`
	InfoHash    [20]byte `json:"info_hash"`
	InfoHashHex string   `json:"info_hash_hex"`
	Size        int64    `json:"size"`
	PieceLength int64    `json:"piece_length"`
	Trackers    []string `json:"trackers"`
	Comment     string   `json:"comment"`
	CreatedBy   string   `json:"created_by"`
	Files       []File   `json:"files"`
}

type File struct {
	Path   string `json:"path"`
	Length int64  `json:"length"`
}

func ParseTorrent(r io.Reader) (*TorrentFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read torrent: %w", err)
	}

	decoded, err := bencode.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode torrent: %w", err)
	}

	dict, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("torrent is not a dictionary")
	}

	tf := &TorrentFile{}

	// Extract trackers
	if announce, ok := dict["announce"]; ok {
		if a, ok := announce.([]byte); ok {
			tf.Trackers = append(tf.Trackers, string(a))
		}
	}

	if announceList, ok := dict["announce-list"]; ok {
		if tiers, ok := announceList.([]interface{}); ok {
			for _, tier := range tiers {
				if urls, ok := tier.([]interface{}); ok {
					for _, url := range urls {
						if u, ok := url.([]byte); ok {
							tf.Trackers = append(tf.Trackers, string(u))
						}
					}
				}
			}
		}
	}

	if comment, ok := dict["comment"]; ok {
		if c, ok := comment.([]byte); ok {
			tf.Comment = string(c)
		}
	}

	if createdBy, ok := dict["created by"]; ok {
		if c, ok := createdBy.([]byte); ok {
			tf.CreatedBy = string(c)
		}
	}

	// Parse info dictionary
	infoVal, ok := dict["info"]
	if !ok {
		return nil, fmt.Errorf("torrent missing info dictionary")
	}

	infoDict, ok := infoVal.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("info is not a dictionary")
	}

	// Compute info_hash by re-encoding the info dictionary
	infoEncoded, err := bencode.EncodeToBytes(infoVal)
	if err != nil {
		return nil, fmt.Errorf("encode info dict: %w", err)
	}
	tf.InfoHash = sha1.Sum(infoEncoded)
	tf.InfoHashHex = fmt.Sprintf("%x", tf.InfoHash)

	if name, ok := infoDict["name"]; ok {
		if n, ok := name.([]byte); ok {
			tf.Name = string(n)
		}
	}

	if pl, ok := infoDict["piece length"]; ok {
		if p, ok := pl.(int64); ok {
			tf.PieceLength = p
		}
	}

	// Single file mode
	if length, ok := infoDict["length"]; ok {
		if l, ok := length.(int64); ok {
			tf.Size = l
			tf.Files = []File{{Path: tf.Name, Length: l}}
		}
	}

	// Multi-file mode
	if files, ok := infoDict["files"]; ok {
		if fileList, ok := files.([]interface{}); ok {
			for _, f := range fileList {
				if fd, ok := f.(map[string]interface{}); ok {
					file := File{}
					if length, ok := fd["length"].(int64); ok {
						file.Length = length
						tf.Size += length
					}
					if path, ok := fd["path"].([]interface{}); ok {
						for i, p := range path {
							if pb, ok := p.([]byte); ok {
								if i > 0 {
									file.Path += "/"
								}
								file.Path += string(pb)
							}
						}
					}
					tf.Files = append(tf.Files, file)
				}
			}
		}
	}

	return tf, nil
}
