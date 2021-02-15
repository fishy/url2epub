package rmapi

import (
	"fmt"
	"strconv"
	"time"
)

// TimestampMillisecond is used to marshal timestamp into milliseconds since
// unix epoch in json.
type TimestampMillisecond time.Time

// MarshalJSON implements json.Marshaler.
//
// It converts time into milliseconds since unix epoch,
// but omits any sub-second precision.
func (ts TimestampMillisecond) MarshalJSON() ([]byte, error) {
	t := time.Time(ts)
	if t.IsZero() {
		return []byte("null"), nil
	}

	return []byte(strconv.FormatInt(t.Unix()*1e3, 10)), nil
}

// ItemInfo defines the json format of an item metadata.
//
// Some of the fields are only used in requests and some of the fields are only
// used in responses, as a result it's important for all of them to have
// omitempty json tag.
type ItemInfo struct {
	ID             string    `json:"ID,omitempty"`
	Type           string    `json:"Type,omitempty"`
	Name           string    `json:"VissibleName,omitempty"`
	Parent         string    `json:"Parent,omitempty"`
	Version        int       `json:"Version,omitempty"`
	UploadURL      string    `json:"BlobURLPut,omitempty"`
	ModifiedClient time.Time `json:"ModifiedClient,omitempty"`

	// responses only
	Message string `json:"Message,omitempty"`
	Success *bool  `json:"Success,omitempty"`
}

// IsSuccess checks the Success field
func (i ItemInfo) IsSuccess() error {
	if i.Success == nil || *i.Success {
		return nil
	}
	return fmt.Errorf("failed for item %q: %q", i.ID, i.Message)
}

// ToMetadata converts ItemInfo into ItemMetadata
func (i ItemInfo) ToMetadata() ItemMetadata {
	return ItemMetadata{
		Type:         i.Type,
		Name:         i.Name,
		Parent:       i.Parent,
		Version:      i.Version,
		LastModified: TimestampMillisecond(i.ModifiedClient),
	}
}

// ItemMetadata defines the json format for the .metadata files.
type ItemMetadata struct {
	Type         string               `json:"type"`
	Name         string               `json:"visibleName"`
	Parent       string               `json:"parent"`
	Version      int                  `json:"version"`
	LastModified TimestampMillisecond `json:"lastModified"`
}
