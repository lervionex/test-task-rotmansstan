package withdrawal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

type Amount int64

func (a Amount) Int64() int64 {
	return int64(a)
}

func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(a), 10)), nil
}

func (a *Amount) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return errors.New("amount is required")
	}

	var value int64
	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return fmt.Errorf("decode amount string: %w", err)
		}

		parsed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return errors.New("amount must be an integer")
		}

		value = parsed
	} else {
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return errors.New("amount must be an integer")
		}
	}

	*a = Amount(value)
	return nil
}
