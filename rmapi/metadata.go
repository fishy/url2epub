package rmapi

import (
	"encoding/json"
	"strconv"
	"time"
)

// TimestampMillisecond is used to marshal timestamp into milliseconds since
// unix epoch in json.
type TimestampMillisecond time.Time

// MarshalJSON implements json.Marshaler.
//
// It converts time into milliseconds since unix epoch.
func (ts TimestampMillisecond) MarshalJSON() ([]byte, error) {
	t := time.Time(ts)
	if t.IsZero() {
		return []byte("null"), nil
	}

	str := strconv.FormatInt(t.Unix()*1e3+int64(t.Nanosecond())/1e3, 10)
	return json.Marshal(str)
}

// UnmarshalJSON implements json.Unmarshaler.
//
// It converts milliseconds since unix epoch into time.
func (ts *TimestampMillisecond) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	if str == "" {
		*ts = TimestampMillisecond{}
		return nil
	}

	ms, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return err
	}
	sec := ms / 1e3
	nano := (ms % 1e3) * 1e6
	*ts = TimestampMillisecond(time.Unix(sec, nano))
	return nil
}

// MetadataSuffix is the suffix (file extension) used by metadata files.
const MetadataSuffix = ".metadata"

// Metadata defines the json format for the .metadata files.
type Metadata struct {
	Type         string               `json:"type"`
	Name         string               `json:"visibleName"`
	Parent       string               `json:"parent"`
	Version      int                  `json:"version"`
	LastModified TimestampMillisecond `json:"lastModified"`
}
